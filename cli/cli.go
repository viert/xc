package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/viert/xc/config"
	"github.com/viert/xc/log"
	"github.com/viert/xc/passmgr"
	"github.com/viert/xc/remote"
	"github.com/viert/xc/store"
	"github.com/viert/xc/term"
)

type cmdHandler func(string, string, ...string)
type execMode int

// Cli is the comand line interface object
type Cli struct {
	rl      *readline.Instance
	stopped bool

	handlers       map[string]cmdHandler
	aliases        map[string]*alias
	completer      *completer
	store          *store.Store
	mode           execMode
	user           string
	raiseType      remote.RaiseType
	distributeType remote.CopyType
	raisePasswd    string
	remoteTmpDir   string
	delay          int
	sshThreads     int
	connectTimeout int

	exitConfirm      bool
	execConfirm      bool
	prependHostnames bool
	progressBar      bool
	debug            bool
	usePasswordMgr   bool

	interpreter     string
	sudoInterpreter string
	suInterpreter   string

	curDir              string
	outputFile          *os.File
	outputFileName      string
	aliasRecursionCount int
}

const (
	emSerial execMode = iota
	emParallel
	emCollapse

	maxAliasRecursion = 10
	maxSSHThreadsSane = 1024
)

var (
	whitespace = regexp.MustCompile(`\s+`)
	modeMap    = map[execMode]string{
		emSerial:   "serial",
		emParallel: "parallel",
		emCollapse: "collapse",
	}
)

// New creates a new instance of CLI
func New(cfg *config.XCConfig, backend store.Backend) (*Cli, error) {
	var err error

	err = log.Initialize(cfg.LogFile)
	if err != nil {
		term.Errorf("Error initializing logger: %s\n", err)
	}

	cli := new(Cli)
	st, err := store.CreateStore(backend)
	if err != nil {
		term.Errorf("Error initializing backend: %s\n", err)
		return nil, err
	}
	cli.store = st
	cli.stopped = false
	cli.aliases = make(map[string]*alias)
	cli.setupCmdHandlers()
	setEnvironment(cfg.LocalEnvironment)

	cfg.Readline.AutoComplete = cli.completer
	cli.rl, err = readline.NewEx(cfg.Readline)
	if err != nil {
		return nil, err
	}

	cli.exitConfirm = cfg.ExitConfirm
	cli.execConfirm = cfg.ExecConfirm
	cli.delay = cfg.Delay
	cli.user = cfg.User
	cli.sshThreads = cfg.SSHThreads
	cli.prependHostnames = cfg.PrependHostnames
	cli.progressBar = cfg.ProgressBar
	cli.debug = cfg.Debug
	cli.connectTimeout = cfg.SSHConnectTimeout
	cli.remoteTmpDir = cfg.RemoteTmpdir

	// output
	cli.outputFileName = ""
	cli.outputFile = nil

	if cfg.PasswordManagerPath != "" {
		term.Warnf("Loading password manager from %s\n", cfg.PasswordManagerPath)
		err = passmgr.Load(cfg.PasswordManagerPath, cfg.PasswordManagerOptions)
		if err != nil {
			term.Errorf("Error initializing password manager: %s\n", err)
		} else {
			cli.usePasswordMgr = true
		}
	}
	remote.Initialize(cli.sshThreads, cli.user)
	remote.SetPrependHostnames(cli.prependHostnames)
	remote.SetRemoteTmpdir(cfg.RemoteTmpdir)
	remote.SetProgressBar(cli.progressBar)
	remote.SetConnectTimeout(cli.connectTimeout)
	remote.SetDebug(cli.debug)
	remote.SetUsePasswordManager(cli.usePasswordMgr)
	remote.SetNumThreads(cli.sshThreads)
	remote.SetRemoteEnvironment(cfg.RemoteEnvironment)

	// interpreter
	cli.setInterpreter("none", cfg.Interpreter)
	cli.setInterpreter("sudo", cfg.SudoInterpreter)
	cli.setInterpreter("su", cfg.SuInterpreter)

	cli.setRaiseType(cfg.RaiseType)
	cli.setDistributeType(cfg.Distribute)

	cli.curDir, err = os.Getwd()
	if err != nil {
		term.Errorf("Error determining current directory: %s\n", err)
		cli.curDir = "."
	}

	cli.doMode("mode", "mode", cfg.Mode)

	cli.setPrompt()
	cli.runRC(cfg.RCfile)

	return cli, nil
}

func (c *Cli) setPrompt() {
	rts := ""
	rtbold := false
	rtcolor := term.CGreen

	pr := fmt.Sprintf("[%s]", strings.Title(modeMap[c.mode]))
	switch c.mode {
	case emSerial:
		if c.delay > 0 {
			pr = fmt.Sprintf("[Serial:%d]", c.delay)
		}
		pr = term.Cyan(pr)
	case emParallel:
		pr = term.Yellow(pr)
	case emCollapse:
		pr = term.Green(pr)
	}

	pr += " " + term.Colored(c.user, term.CLightBlue, true)
	switch c.raiseType {
	case remote.RTSu:
		rts = "(su"
		rtcolor = term.CRed
	case remote.RTSudo:
		rts = "(sudo"
		rtcolor = term.CGreen
	default:
		rts = ""
	}

	if rts != "" {
		if c.raisePasswd == "" && !c.usePasswordMgr {
			rts += "*"
			rtbold = true
		}
		rts += ")"
		pr += term.Colored(rts, rtcolor, rtbold)
	}
	pr += "> "
	c.rl.SetPrompt(pr)
}

func (c *Cli) setInterpreter(iType string, interpreter string) {
	switch iType {
	case "none":
		c.interpreter = interpreter
		remote.SetInterpreter(interpreter)
	case "sudo":
		c.sudoInterpreter = interpreter
		remote.SetSudoInterpreter(interpreter)
	case "su":
		c.suInterpreter = interpreter
		remote.SetSuInterpreter(interpreter)
	default:
		term.Errorf("Invalid raise type: %s\n", iType)
	}
	term.Warnf("Using \"%s\" for commands with %s-type raise\n", interpreter, iType)
}

// Finalize closes resources at xc's exit. Must be called explicitly
func (c *Cli) Finalize() {
	if c.outputFile != nil {
		c.outputFile.Close()
		c.outputFile = nil
	}
}

// OneCmd is the main method which literally runs one command
// according to line given in arguments
func (c *Cli) OneCmd(line string) {
	var args []string
	var argsLine string

	line = strings.Trim(line, " \n\t")
	if strings.HasPrefix(line, "#") {
		return
	}

	cmdRunes, rest := split([]rune(line))
	cmd := string(cmdRunes)

	if cmd == "" {
		return
	}

	if rest == nil {
		args = make([]string, 0)
		argsLine = ""
	} else {
		argsLine = string(rest)
		args = whitespace.Split(argsLine, -1)
	}

	if handler, ok := c.handlers[cmd]; ok {
		handler(cmd, argsLine, args...)
	} else {
		term.Errorf("Unknown command: %s\n", cmd)
	}
}

// CmdLoop reads commands and runs OneCmd
func (c *Cli) CmdLoop() {
	for !c.stopped {
		// Python cmd-style run setPrompt every time in case something has changed
		c.setPrompt()

		line, err := c.rl.Readline()
		if err == readline.ErrInterrupt {
			continue
		} else if err == io.EOF {
			if !c.exitConfirm || c.confirm("Are you sure to exit?") {
				c.stopped = true
			}
			continue
		}
		c.aliasRecursionCount = maxAliasRecursion
		c.OneCmd(line)
	}
}

func setEnvironment(environ map[string]string) {
	for key, value := range environ {
		os.Setenv(key, value)
	}
}

func (c *Cli) confirm(msg string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [Y/n] ", msg)
		response, err := reader.ReadString('\n')
		if err == nil {
			response = strings.TrimSpace(strings.ToLower(response))
			switch response {
			case "":
				fallthrough
			case "y":
				return true
			case "n":
				return false
			}
		}
		fmt.Println()
	}
}

func (c *Cli) acquirePasswd() {
	if c.raiseType == remote.RTNone || c.usePasswordMgr {
		return
	}

	if c.raisePasswd == "" {
		c.doPasswd("passwd", "")
	}
}

func (c *Cli) setOutput(filename string) error {
	var err error
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		if c.outputFile != nil {
			c.outputFile.Close()
		}
		c.outputFile = f
		remote.SetOutputFile(c.outputFile)
	}
	return err
}

func (c *Cli) doexec(mode execMode, argsLine string) {
	var r *remote.ExecResult

	expr, rest := split([]rune(argsLine))
	if rest == nil {
		term.Errorf("Usage: exec <inventoree_expr> commands...\n")
		return
	}

	hosts, err := c.store.HostList(expr)
	if err != nil {
		term.Errorf("Error parsing expression %s: %s\n", string(expr), err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

	c.acquirePasswd()
	remote.SetPassword(c.raisePasswd)
	cmd := string(rest)

	if c.execConfirm {
		fmt.Printf("%s\n", term.Yellow(term.HR(len(cmd)+5)))
		fmt.Printf("%s\n%s\n\n", term.Yellow("Hosts:"), strings.Join(hosts, ", "))
		fmt.Printf("%s\n%s\n\n", term.Yellow("Command:"), cmd)
		if !c.confirm("Are you sure?") {
			return
		}
		fmt.Printf("%s\n\n", term.Yellow(term.HR(len(cmd)+5)))
	}

	remote.WriteOutput(fmt.Sprintf("==== exec %s\n", argsLine))

	switch mode {
	case emParallel:
		r = remote.RunParallel(hosts, cmd)
	case emCollapse:
		r = remote.RunCollapse(hosts, cmd)
		r.PrintOutputMap()
	case emSerial:
		r = remote.RunSerial(hosts, cmd, c.delay)
	}
	r.Print()
}

func (c *Cli) dorunscript(mode execMode, argsLine string) {
	var (
		r              *remote.ExecResult
		expr           []rune
		rest           []rune
		hosts          []string
		localFilename  string
		remoteFilename string
		err            error
		st             os.FileInfo
	)

	expr, rest = split([]rune(argsLine))
	if rest == nil {
		term.Errorf("Usage: runscript <inventoree_expr> filename\n")
		return
	}

	hosts, err = c.store.HostList(expr)
	if err != nil {
		term.Errorf("Error parsing expression %s: %s\n", string(expr), err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

	c.acquirePasswd()
	localFilename = string(rest)
	st, err = os.Stat(localFilename)
	if err != nil {
		term.Errorf("Error stat %s: %s\n", localFilename, err)
		return
	}
	if st.IsDir() {
		term.Errorf("%s is a directory\n", localFilename)
		return
	}

	now := time.Now().Format("20060102-150405")
	remoteFilename = fmt.Sprintf("tmp.xc.%s_%s", now, filepath.Base(localFilename))
	remoteFilename = filepath.Join(c.remoteTmpDir, remoteFilename)

	if c.distributeType != remote.CTScp {
		currentDistributeType := c.distributeType
		remote.SetDistributeType(remote.CTScp)
		defer remote.SetDistributeType(currentDistributeType)
	}

	dr := remote.Distribute(hosts, localFilename, remoteFilename, false)
	copyError := dr.ErrorHosts
	hosts = dr.SuccessHosts

	cmd := fmt.Sprintf("%s; rm %s", remoteFilename, remoteFilename)
	switch mode {
	case emParallel:
		r = remote.RunParallel(hosts, cmd)
	case emCollapse:
		r = remote.RunCollapse(hosts, cmd)
		r.PrintOutputMap()
	case emSerial:
		r = remote.RunSerial(hosts, cmd, c.delay)
	}
	r.ErrorHosts = append(r.ErrorHosts, copyError...)
	r.Print()
}

func doOnOff(propName string, propRef *bool, args []string) bool {
	if len(args) < 1 {
		value := "off"
		if *propRef {
			value = "on"
		}
		term.Warnf("%s is %s\n", propName, value)
		return false
	}
	prev := *propRef
	switch args[0] {
	case "on":
		*propRef = true
	case "off":
		*propRef = false
	default:
		term.Errorf("Invalid %s vaue. Please use either \"on\" or \"off\"\n", propName)
		return false
	}
	return prev != *propRef
}

func (c *Cli) setRaiseType(rt string) {
	currentRaiseType := c.raiseType
	switch rt {
	case "su":
		c.raiseType = remote.RTSu
	case "sudo":
		c.raiseType = remote.RTSudo
	case "none":
		c.raiseType = remote.RTNone
	default:
		term.Errorf("Unknown raise type: %s\n", rt)
		return
	}

	if c.raiseType != currentRaiseType {
		// Drop passwd in case of changing raise type
		c.raisePasswd = ""
	}
	remote.SetRaise(c.raiseType)
}

func (c *Cli) setDistributeType(dtr string) {
	switch dtr {
	case "tar":
		c.distributeType = remote.CTTar
	case "scp":
		c.distributeType = remote.CTScp
	default:
		term.Errorf("Unknown distribute type: %s\n", dtr)
		return
	}
	remote.SetDistributeType(c.distributeType)
}

func (c *Cli) runRC(rcfile string) {
	f, err := os.Open(rcfile)
	if err != nil {
		if !os.IsNotExist(err) {
			term.Errorf("Error loading rcfile: %s\n", err)
		}
		return
	}
	defer f.Close()

	term.Successf("Running rcfile %s...\n", rcfile)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		cmd := sc.Text()
		fmt.Println(term.Green(cmd))
		c.OneCmd(cmd)
	}
}

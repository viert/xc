package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/viert/xc/passmgr"
	"github.com/viert/xc/remote"
	"github.com/viert/xc/term"
)

func (c *Cli) setupCmdHandlers() {
	c.handlers = make(map[string]cmdHandler)
	c.handlers["exit"] = c.doExit
	c.handlers["mode"] = c.doMode
	c.handlers["parallel"] = c.doParallel
	c.handlers["collapse"] = c.doCollapse
	c.handlers["serial"] = c.doSerial
	c.handlers["user"] = c.doUser
	c.handlers["hostlist"] = c.doHostlist
	c.handlers["exec"] = c.doExec
	c.handlers["s_exec"] = c.doSExec
	c.handlers["c_exec"] = c.doCExec
	c.handlers["p_exec"] = c.doPExec
	c.handlers["ssh"] = c.doSSH
	c.handlers["raise"] = c.doRaise
	c.handlers["passwd"] = c.doPasswd
	c.handlers["cd"] = c.doCD
	c.handlers["local"] = c.doLocal
	c.handlers["alias"] = c.doAlias
	c.handlers["delay"] = c.doDelay
	c.handlers["debug"] = c.doDebug
	c.handlers["reload"] = c.doReload
	c.handlers["interpreter"] = c.doInterpreter
	c.handlers["connect_timeout"] = c.doConnectTimeout
	c.handlers["progressbar"] = c.doProgressBar
	c.handlers["prepend_hostnames"] = c.doPrependHostnames
	c.handlers["help"] = c.doHelp
	c.handlers["output"] = c.doOutput
	c.handlers["threads"] = c.doThreads
	c.handlers["distribute"] = c.doDistribute
	c.handlers["runscript"] = c.doRunScript
	c.handlers["s_runscript"] = c.doSRunScript
	c.handlers["c_runscript"] = c.doCRunScript
	c.handlers["p_runscript"] = c.doPRunScript
	c.handlers["use_password_manager"] = c.doUsePasswordManager
	c.handlers["distribute_type"] = c.doDistributeType
	c.handlers["_passmgr_debug"] = c.doPassmgrDebug

	commands := make([]string, len(c.handlers))
	i := 0
	for cmd := range c.handlers {
		commands[i] = cmd
		i++
	}
	c.completer = newCompleter(c.store, commands)
}

func (c *Cli) doExit(name string, argsLine string, args ...string) {
	c.stopped = true
}

func (c *Cli) doMode(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: mode <[serial,parallel,collapse]>\n")
		return
	}
	newMode := args[0]
	for mode, modeStr := range modeMap {
		if newMode == modeStr {
			c.mode = mode
			return
		}
	}
	term.Errorf("Unknown mode: %s\n", newMode)
}

func (c *Cli) doCollapse(name string, argsLine string, args ...string) {
	c.doMode("mode", "collapse", "collapse")
}

func (c *Cli) doParallel(name string, argsLine string, args ...string) {
	c.doMode("mode", "parallel", "parallel")
}

func (c *Cli) doSerial(name string, argsLine string, args ...string) {
	c.doMode("mode", "serial", "serial")
}

func (c *Cli) doUser(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: user <username>\n")
		return
	}
	c.user = args[0]
	remote.SetUser(c.user)
}

func (c *Cli) doHostlist(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: hostlist <xc_expr>\n")
		return
	}

	hosts, err := c.store.HostList([]rune(args[0]))
	if err != nil {
		term.Errorf("%s\n", err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

	maxHostnameLen := 0
	for _, host := range hosts {
		if len(host) > maxHostnameLen {
			maxHostnameLen = len(host)
		}
	}

	title := fmt.Sprintf(" Hostlist %s    ", args[0])
	hrlen := len(title)
	if hrlen < maxHostnameLen+2 {
		hrlen = maxHostnameLen + 2
	}

	hr := term.HR(hrlen)

	fmt.Println(term.Green(hr))
	fmt.Println(term.Green(title))
	fmt.Println(term.Green(hr))
	for _, host := range hosts {
		fmt.Println(host)
	}
	term.Successf("Total: %d hosts\n", len(hosts))
}

func (c *Cli) doRaise(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: raise <su/sudo>\n")
		return
	}
	c.setRaiseType(args[0])
}

func (c *Cli) doDistributeType(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		dtype := "scp"
		if c.distributeType == remote.CTTar {
			dtype = "tar"
		}
		term.Warnf("distribute_type is %s\n", dtype)
		return
	}
	c.setDistributeType(args[0])
	term.Successf("distribute_type set to %s\n", args[0])
}

func (c *Cli) doPasswd(name string, argsLine string, args ...string) {
	passwd, err := c.rl.ReadPassword("Set su/sudo password: ")
	if err != nil {
		term.Errorf("%s\n", err)
		return
	}
	c.raisePasswd = string(passwd)
}

func (c *Cli) doExec(name string, argsLine string, args ...string) {
	c.doexec(c.mode, argsLine)
}

func (c *Cli) doCExec(name string, argsLine string, args ...string) {
	c.doexec(emCollapse, argsLine)
}

func (c *Cli) doSExec(name string, argsLine string, args ...string) {
	c.doexec(emSerial, argsLine)
}

func (c *Cli) doPExec(name string, argsLine string, args ...string) {
	c.doexec(emParallel, argsLine)
}

func (c *Cli) doSSH(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: ssh <inventoree_expr>\n")
		return
	}

	c.acquirePasswd()
	remote.SetPassword(c.raisePasswd)

	expr, rest := split([]rune(argsLine))

	hosts, err := c.store.HostList([]rune(expr))
	if err != nil {
		term.Errorf("Error parsing expression %s: %s\n", string(expr), err)
		return
	}

	if len(hosts) == 0 {
		term.Errorf("Empty hostlist\n")
		return
	}

	cmd := string(rest)
	remote.RunSerial(hosts, cmd, 0)
}

func (c *Cli) doCD(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: cd <directory>\n")
		return
	}
	err := os.Chdir(argsLine)
	if err != nil {
		term.Errorf("Error changing directory: %s\n", err)
	}
}

func (c *Cli) doLocal(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: local <localcmd> [...args]\n")
		return
	}

	// ignore keyboard interrupt signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	cmd := exec.Command("bash", "-c", fmt.Sprintf("%s", argsLine))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Run()
}

func (c *Cli) doAlias(name string, argsLine string, args ...string) {
	aliasName, rest := split([]rune(argsLine))
	if len(aliasName) == 0 {
		term.Errorf("Usage: alias <alias_name> <command> [...args]\n")
		return
	}

	if rest == nil || len(rest) == 0 {
		err := c.removeAlias(aliasName)
		if err != nil {
			term.Errorf("Error removing alias \"%s\": %s\n", string(aliasName), err)
		}
	} else {
		err := c.createAlias(aliasName, rest)
		if err != nil {
			term.Errorf("Error creating alias %s: %s\n", string(aliasName), err)
		}
	}
}

func (c *Cli) doDelay(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Errorf("Usage: delay <seconds>\n")
		return
	}
	sec, err := strconv.ParseInt(args[0], 10, 8)
	if err != nil {
		term.Errorf("Invalid delay format: %s\n", err)
		return
	}
	c.delay = int(sec)
}

func (c *Cli) doDebug(name string, argsLine string, args ...string) {
	if doOnOff("debug", &c.debug, args) {
		remote.SetDebug(c.debug)
	}
}

func (c *Cli) doProgressBar(name string, argsLine string, args ...string) {
	if doOnOff("progressbar", &c.progressBar, args) {
		remote.SetProgressBar(c.progressBar)
	}
}

func (c *Cli) doPrependHostnames(name string, argsLine string, args ...string) {
	if doOnOff("prepend_hostnames", &c.prependHostnames, args) {
		remote.SetPrependHostnames(c.prependHostnames)
	}
}

func (c *Cli) doUsePasswordManager(name string, argsLine string, args ...string) {
	if doOnOff("use_password_manager", &c.usePasswordMgr, args) {
		if c.usePasswordMgr && !passmgr.Ready() {
			term.Errorf("Password manager is not ready\n")
			c.usePasswordMgr = false
		}
		if c.usePasswordMgr {
			passmgr.Reload()
		}
		remote.SetUsePasswordManager(c.usePasswordMgr)
	}
}

func (c *Cli) doReload(name string, argsLine string, args ...string) {
	err := c.store.BackendReload()
	if err != nil {
		term.Errorf("Error reloading data from backend\n")
	}
}

func (c *Cli) doInterpreter(name string, argsLine string, args ...string) {
	if len(args) == 0 {
		term.Warnf("Using \"%s\" for commands with none-type raise\n", c.interpreter)
		term.Warnf("Using \"%s\" for commands with sudo-type raise\n", c.sudoInterpreter)
		term.Warnf("Using \"%s\" for commands with su-type raise\n", c.suInterpreter)
		return
	}
	iType, interpreter := split([]rune(argsLine))
	c.setInterpreter(string(iType), string(interpreter))
}

func (c *Cli) doConnectTimeout(name string, argsLine string, args ...string) {
	if len(args) < 1 {
		term.Warnf("connect_timeout = %d\n", c.connectTimeout)
		return
	}
	ct, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		term.Errorf("Error reading connect timeout value: %s\n", err)
		return
	}
	c.connectTimeout = int(ct)
	remote.SetConnectTimeout(c.connectTimeout)
}

func (c *Cli) doOutput(name string, argsLine string, args ...string) {
	if len(args) == 0 {
		if c.outputFile == nil {
			term.Warnf("Output is switched off\n")
		} else {
			term.Successf("Output is copied to %s\n", c.outputFileName)
		}
		return
	}

	// special filename to switch off the output
	if argsLine == "_" {
		c.outputFileName = ""
		if c.outputFile != nil {
			c.outputFile.Close()
			c.outputFile = nil
			remote.SetOutputFile(nil)
		}
		term.Warnf("Output is switched off\n")
		return
	}

	err := c.setOutput(argsLine)
	if err == nil {
		c.outputFileName = argsLine
		term.Successf("Output is copied to %s\n", c.outputFileName)
	} else {
		term.Errorf("Error setting output file to %s: %s\n", argsLine, err)
	}
}

func (c *Cli) doThreads(name string, argsLine string, args ...string) {
	if len(args) == 0 {
		term.Successf("Max SSH threads: %d\n", c.sshThreads)
		return
	}

	threads, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		term.Errorf("Error setting max SSH threads value: %s\n", err)
		return
	}

	if int(threads) == c.sshThreads {
		term.Warnf("Max SSH threads value remains unchanged\n")
		return
	}

	if threads < 1 {
		term.Errorf("Max SSH threads can't be lower than 1\n")
		return
	}

	if threads > maxSSHThreadsSane {
		term.Errorf("Max SSH threads can't be higher than %d\n", maxSSHThreadsSane)
		return
	}

	c.sshThreads = int(threads)
	term.Successf("Max SSH threads set to %d\n", c.sshThreads)
	remote.SetNumThreads(c.sshThreads)
	term.Successf("Execution pool re-created\n")
}

func (c *Cli) doDistribute(name string, argsLine string, args ...string) {
	var (
		r              *remote.ExecResult
		expr           []rune
		rest           []rune
		lcl            []rune
		rmt            []rune
		hosts          []string
		localFilename  string
		remoteFilename string
		err            error
		st             os.FileInfo
	)

	expr, rest = split([]rune(argsLine))
	if rest == nil {
		term.Errorf("Usage: distribute <inventoree_expr> filename [remote_filename]\n")
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

	lcl, rmt = split(rest)
	localFilename = string(lcl)
	if rmt == nil {
		remoteFilename = localFilename
	} else {
		remoteFilename = string(rmt)
	}

	st, err = os.Stat(localFilename)
	if err != nil {
		term.Errorf("Error stat %s: %s\n", localFilename, err)
		return
	}

	r = remote.Distribute(hosts, localFilename, remoteFilename, st.IsDir())
	r.Print()
}

func (c *Cli) doRunScript(name string, argsLine string, args ...string) {
	c.dorunscript(c.mode, argsLine)
}

func (c *Cli) doSRunScript(name string, argsLine string, args ...string) {
	c.dorunscript(emSerial, argsLine)
}

func (c *Cli) doCRunScript(name string, argsLine string, args ...string) {
	c.dorunscript(emCollapse, argsLine)
}

func (c *Cli) doPRunScript(name string, argsLine string, args ...string) {
	c.dorunscript(emParallel, argsLine)
}

func (c *Cli) doPassmgrDebug(name string, argsLine string, args ...string) {
	passmgr.PrintDebug()
}

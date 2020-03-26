package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/viert/properties"
)

const defaultConfigContents = `[main]
user = 
mode = parallel
history_file = ~/.xc_history
cache_dir = ~/.xc_cache
cache_ttl = 336 # 24 * 7 * 2
rc_file = ~/.xcrc
log_file = 
raise = none
exit_confirm = true
exec_confirm = true
distribute = tar

[executer]
ssh_threads = 50
ssh_connect_timeout = 1
ssh_command = /usr/bin/ssh
progress_bar = true
prepend_hostnames = true
remote_tmpdir = /tmp
delay = 0

interpreter = bash
interpreter_sudo = sudo bash
interpreter_su = su -

[backend]
type = conductor
url = http://c.inventoree.ru
work_groups = 

[passmgr]
path =
`

// BackendType is a backend type enum
type BackendType int

// Backend types
const (
	BTIni BackendType = iota
	BTJSON
	BTConductor
	BTInventoree
)

// BackendConfig is a backend configuration struct
type BackendConfig struct {
	Type       BackendType
	TypeString string
	Options    map[string]string
}

// XCConfig represents a configuration struct for XC
type XCConfig struct {
	Readline               *readline.Config
	BackendCfg             *BackendConfig
	User                   string
	SSHThreads             int
	SSHConnectTimeout      int
	SSHCommand             string
	PingCount              int
	RemoteTmpdir           string
	Mode                   string
	RaiseType              string
	Delay                  int
	RCfile                 string
	CacheDir               string
	CacheTTL               time.Duration
	Debug                  bool
	ProgressBar            bool
	PrependHostnames       bool
	LogFile                string
	ExitConfirm            bool
	ExecConfirm            bool
	SudoInterpreter        string
	SuInterpreter          string
	Interpreter            string
	PasswordManagerPath    string
	PasswordManagerOptions map[string]string
	LocalEnvironment       map[string]string
	RemoteEnvironment      map[string]string
	Distribute             string
}

const (
	defaultHistoryFile       = "~/.xc_history"
	defaultCacheDir          = "~/.xc_cache"
	defaultRCfile            = "~/.xcrc"
	defaultCacheTTL          = 24
	defaultThreads           = 50
	defaultRemoteTmpDir      = "/tmp"
	defaultPingCount         = 5
	defaultDelay             = 0
	defaultMode              = "parallel"
	defaultRaiseType         = "none"
	defaultDebug             = false
	defaultProgressbar       = true
	defaultPrependHostnames  = true
	defaultSSHConnectTimeout = 1
	defaultSSHCommand        = "/usr/bin/ssh"
	defaultLogFile           = ""
	defaultExitConfirm       = true
	defaultExecConfirm       = true
	defaultInterpreter       = "/bin/bash"
	defaultSudoInterpreter   = "sudo /bin/bash"
	defaultSuInterpreter     = "su -"
	defaultDistribute        = "tar"
)

var (
	defaultReadlineConfig = &readline.Config{
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	}
	defaultUser = os.Getenv("USER")
)

// ExpandPath helper helps to expand ~ as a home directory
// as well as it expands any env variable usage in path
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		path = "$HOME/" + path[2:]
	}
	return os.ExpandEnv(path)
}

// Read reads and parses a configuration file
func Read(filename string) (*XCConfig, error) {
	return read(filename, false)
}

func read(filename string, secondPass bool) (*XCConfig, error) {
	var props *properties.Properties
	var err error

	props, err = properties.Load(filename)
	if err != nil {
		if secondPass {
			return nil, err
		}

		if os.IsNotExist(err) {
			err = ioutil.WriteFile(filename, []byte(defaultConfigContents), 0644)
			if err != nil {
				return nil, err
			}
		}
		return read(filename, true)
	}

	cfg := new(XCConfig)
	cfg.Readline = defaultReadlineConfig
	cfg.BackendCfg = &BackendConfig{Type: BTIni, Options: make(map[string]string)}
	cfg.LocalEnvironment = make(map[string]string)
	cfg.RemoteEnvironment = make(map[string]string)

	hf, err := props.GetString("main.history_file")
	if err != nil {
		hf = defaultHistoryFile
	}
	cfg.Readline.HistoryFile = ExpandPath(hf)

	rcf, err := props.GetString("main.rc_file")
	if err != nil {
		rcf = defaultRCfile
	}
	cfg.RCfile = ExpandPath(rcf)

	lf, err := props.GetString("main.log_file")
	if err != nil {
		lf = defaultLogFile
	}
	cfg.LogFile = ExpandPath(lf)

	cttl, err := props.GetInt("main.cache_ttl")
	if err != nil {
		cttl = defaultCacheTTL
	}
	cfg.CacheTTL = time.Hour * time.Duration(cttl)

	cd, err := props.GetString("main.cache_dir")
	if err != nil {
		cd = defaultCacheDir
	}
	cfg.CacheDir = ExpandPath(cd)

	user, err := props.GetString("main.user")
	if err != nil || user == "" {
		user = defaultUser
	}
	cfg.User = user

	threads, err := props.GetInt("executer.ssh_threads")
	if err != nil {
		threads = defaultThreads
	}
	cfg.SSHThreads = threads

	sshCommand, err := props.GetString("executer.ssh_command")
	if err != nil {
		sshCommand = defaultSSHCommand
	}
	cfg.SSHCommand = sshCommand

	ctimeout, err := props.GetInt("executer.ssh_connect_timeout")
	if err != nil {
		ctimeout = defaultSSHConnectTimeout
	}
	cfg.SSHConnectTimeout = ctimeout

	delay, err := props.GetInt("executer.delay")
	if err != nil || delay < 0 {
		delay = defaultDelay
	}
	cfg.Delay = delay

	tmpdir, err := props.GetString("executer.remote_tmpdir")
	if err != nil {
		tmpdir = defaultRemoteTmpDir
	}
	cfg.RemoteTmpdir = tmpdir

	pc, err := props.GetInt("executer.ping_count")
	if err != nil {
		pc = defaultPingCount
	}
	cfg.PingCount = pc

	sdi, err := props.GetString("executer.interpreter_sudo")
	if err != nil {
		sdi = defaultSudoInterpreter
	}
	cfg.SudoInterpreter = sdi

	si, err := props.GetString("executer.interpreter_su")
	if err != nil {
		si = defaultSuInterpreter
	}
	cfg.SuInterpreter = si

	intrpr, err := props.GetString("executer.interpreter")
	if err != nil {
		intrpr = defaultInterpreter
	}
	cfg.Interpreter = intrpr

	rt, err := props.GetString("main.raise")
	if err != nil {
		rt = defaultRaiseType
	}
	cfg.RaiseType = rt

	dtr, err := props.GetString("main.distribute")
	if err != nil {
		dtr = defaultDistribute
	}
	cfg.Distribute = dtr

	mode, err := props.GetString("main.mode")
	if err != nil {
		mode = defaultMode
	}
	cfg.Mode = mode

	dbg, err := props.GetBool("main.debug")
	if err != nil {
		dbg = defaultDebug
	}
	cfg.Debug = dbg

	exitcnfrm, err := props.GetBool("main.exit_confirm")
	if err != nil {
		exitcnfrm = defaultExitConfirm
	}
	cfg.ExitConfirm = exitcnfrm

	execcnfrm, err := props.GetBool("main.exec_confirm")
	if err != nil {
		execcnfrm = defaultExecConfirm
	}
	cfg.ExecConfirm = execcnfrm

	pbar, err := props.GetBool("executer.progress_bar")
	if err != nil {
		pbar = defaultProgressbar
	}
	cfg.ProgressBar = pbar

	phn, err := props.GetBool("executer.prepend_hostnames")
	if err != nil {
		phn = defaultPrependHostnames
	}
	cfg.PrependHostnames = phn

	bkeys, err := props.Subkeys("backend")
	if err != nil {
		return nil, fmt.Errorf("Backend configuration error: %s", err)
	}

	typeFound := false
	for _, key := range bkeys {
		value, _ := props.GetString("backend." + key)
		if key == "type" {
			cfg.BackendCfg.TypeString = value
			switch value {
			case "ini":
				cfg.BackendCfg.Type = BTIni
			case "json":
				cfg.BackendCfg.Type = BTJSON
			case "conductor":
				cfg.BackendCfg.Type = BTConductor
			case "inventoree":
				cfg.BackendCfg.Type = BTInventoree
			default:
				return nil, fmt.Errorf("Invalid backend type \"%s\"", value)
			}
			typeFound = true
		} else {
			cfg.BackendCfg.Options[key] = value
		}
	}

	if !typeFound {
		return nil, fmt.Errorf("Error configuring backend: backend type is not defined")
	}

	cfg.PasswordManagerOptions = make(map[string]string)
	if props.KeyExists("passmgr") {
		pmgrkeys, err := props.Subkeys("passmgr")
		if err == nil {
			for _, key := range pmgrkeys {
				if key == "path" {
					pluginPath, _ := props.GetString("passmgr.path")
					cfg.PasswordManagerPath = ExpandPath(pluginPath)
				} else {
					cfg.PasswordManagerOptions[key], _ = props.GetString(fmt.Sprintf("passmgr.%s", key))
				}
			}
		}
	}

	envkeys, err := props.Subkeys("environ")
	if err == nil {
		for _, key := range envkeys {
			value, _ := props.GetString(fmt.Sprintf("environ.%s", key))
			cfg.LocalEnvironment[key] = ExpandPath(value)
		}
	}

	envkeys, err = props.Subkeys("remote_environ")
	if err == nil {
		for _, key := range envkeys {
			value, _ := props.GetString(fmt.Sprintf("remote_environ.%s", key))
			cfg.RemoteEnvironment[key] = value
		}
	}

	return cfg, nil
}

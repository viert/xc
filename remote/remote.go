package remote

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	pool                      *Pool
	currentUser               string
	currentPassword           string
	currentRaise              RaiseType
	currentDistributeType     CopyType
	currentUsePasswordManager bool
	currentProgressBar        bool
	currentPrependHostnames   bool
	currentRemoteTmpdir       string
	currentDebug              bool
	outputFile                *os.File
	poolLock                  *sync.Mutex
	poolSize                  int
	remoteEnvironment         map[string]string
	sshCommand                string

	noneInterpreter string
	suInterpreter   string
	sudoInterpreter string
)

// Initialize initializes new execution pool
func Initialize(numThreads int, username string) {
	poolLock = new(sync.Mutex)
	SetUser(username)
	SetPassword("")
	SetRaise(RTNone)
}

// SetSSHCommand sets ssh binary path to be used in createSSHCommand
func SetSSHCommand(command string) {
	sshCommand = command
}

// SetInterpreter sets none-raise interpreter
func SetInterpreter(interpreter string) {
	noneInterpreter = interpreter
}

// SetSudoInterpreter sets sudo-raise interpreter
func SetSudoInterpreter(interpreter string) {
	sudoInterpreter = interpreter
}

// SetSuInterpreter sets su-raise interpreter
func SetSuInterpreter(interpreter string) {
	suInterpreter = interpreter
}

// SetUser sets executer username
func SetUser(username string) {
	currentUser = username
}

// SetRaise sets executer raise type
func SetRaise(raise RaiseType) {
	currentRaise = raise
}

// SetDistributeType sets executer distribute type
func SetDistributeType(dtr CopyType) {
	currentDistributeType = dtr
}

// SetPassword sets executer password
func SetPassword(password string) {
	currentPassword = password
}

// SetProgressBar sets current progressbar mode
func SetProgressBar(pbar bool) {
	currentProgressBar = pbar
}

// SetRemoteTmpdir sets current remote temp directory
func SetRemoteTmpdir(tmpDir string) {
	currentRemoteTmpdir = tmpDir
}

// SetDebug sets current debug mode
func SetDebug(debug bool) {
	currentDebug = debug
}

// SetPrependHostnames sets current prepend_hostnames value for parallel mode
func SetPrependHostnames(prependHostnames bool) {
	currentPrependHostnames = prependHostnames
}

// SetUsePasswordManager sets using passmgr on/off
func SetUsePasswordManager(usePasswordMgr bool) {
	currentUsePasswordManager = usePasswordMgr
}

// SetConnectTimeout sets the ssh connect timeout in sshOptions
func SetConnectTimeout(timeout int) {
	sshOptions["ConnectTimeout"] = fmt.Sprintf("%d", timeout)
}

// SetOutputFile sets output file for every command.
// if it's nil, no output will be written to files
func SetOutputFile(f *os.File) {
	outputFile = f
}

// SetNumThreads sets execution pool size
func SetNumThreads(numThreads int) {
	poolSize = numThreads
}

// SetRemoteEnvironment sets remote environ variables
func SetRemoteEnvironment(environ map[string]string) {
	remoteEnvironment = environ
}

func prepareTempFiles(cmd string) (string, string, error) {
	f, err := ioutil.TempFile("", "xc.")
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	remoteFilename := filepath.Join(currentRemoteTmpdir, filepath.Base(f.Name()))
	io.WriteString(f, "#!/bin/bash\n\n")
	for varName, value := range remoteEnvironment {
		io.WriteString(f, fmt.Sprintf("%s=%s\n", varName, value))
	}
	io.WriteString(f, "\n")
	io.WriteString(f, fmt.Sprintf("nohup bash -c \"sleep 1; rm -f $0\" >/dev/null 2>&1 </dev/null &\n")) // self-destroy
	io.WriteString(f, cmd+"\n")                                                                          // run command
	f.Chmod(0755)

	return f.Name(), remoteFilename, nil
}

// WriteOutput writes output to a user-defined logfile
// prepending with the current datetime
func WriteOutput(message string) {
	if outputFile == nil {
		return
	}
	tm := time.Now().Format("2006-01-02 15:04:05")
	message = fmt.Sprintf("[%s] %s", tm, message)
	outputFile.Write([]byte(message))
}

func writeHostOutput(host string, data []byte) {
	message := fmt.Sprintf("%s: %s", host, string(data))
	WriteOutput(message)
}

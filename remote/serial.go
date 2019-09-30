package remote

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/viert/xc/passmgr"

	"github.com/kr/pty"
	"github.com/npat-efault/poller"
	"github.com/viert/xc/log"
	"github.com/viert/xc/term"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	passwordSent   = false
	shouldSkipEcho = false
)

func forwardUserInput(in *poller.FD, out *os.File, stopped *bool) {
	inBuf := make([]byte, bufferSize)
	// processing stdin
	for {
		deadline := time.Now().Add(pollDeadline)
		in.SetReadDeadline(deadline)
		n, err := in.Read(inBuf)
		if n > 0 {
			// copy stdin to process ptmx
			out.Write(inBuf[:n])
			inBuf = make([]byte, bufferSize)
		}
		if err != nil {
			if err != poller.ErrTimeout {
				break
			}
		}
		if *stopped {
			break
		}
	}
}

func interceptProcessOutput(in []byte, ptmx *os.File, password string) (out []byte, err error) {
	out = []byte{}
	err = nil

	if exConnectionClosed.Match(in) {
		log.Debug("Connection closed message catched")
		return
	}

	if exLostConnection.Match(in) {
		log.Debug("Lost connection message catched")
		return
	}

	if !passwordSent && exPasswdPrompt.Match(in) {
		ptmx.Write([]byte(password + "\n"))
		passwordSent = true
		shouldSkipEcho = true
		log.Debug("Password sent")
		return
	}

	if shouldSkipEcho && exEcho.Match(in) {
		log.Debug("Echo skipped")
		shouldSkipEcho = false
		return
	}

	if passwordSent && exWrongPassword.Match(in) {
		log.Debug("Authentication error while raising privileges")
		err = fmt.Errorf("auth_error")
		return
	}

	out = in
	return
}

func runAtHost(host string, cmd *exec.Cmd, r *ExecResult) {
	var (
		ptmx     *os.File
		si       *poller.FD
		buf      []byte
		err      error
		password string

		stopped = false
	)

	password = currentPassword
	passwordSent = false
	shouldSkipEcho = false

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGWINCH)
	defer signal.Reset()

	ptmx, err = pty.Start(cmd)
	if err != nil {
		term.Errorf("Error creating PTY: %s\n", err)
		r.ErrorHosts = append(r.ErrorHosts, host)
		r.Codes[host] = ErrTerminalError
		return
	}
	pty.InheritSize(os.Stdin, ptmx)
	defer ptmx.Close()

	stdinBackup, err := syscall.Dup(int(os.Stdin.Fd()))
	if err != nil {
		term.Errorf("Error duplicating stdin descriptor: %s\n", err)
		r.ErrorHosts = append(r.ErrorHosts, host)
		r.Codes[host] = ErrTerminalError
		return
	}

	stdinState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		term.Errorf("Error setting stdin to raw mode: %s\n", err)
		r.ErrorHosts = append(r.ErrorHosts, host)
		r.Codes[host] = ErrTerminalError
		return
	}
	defer func() {
		terminal.Restore(int(os.Stdin.Fd()), stdinState)
	}()

	si, err = poller.NewFD(int(os.Stdin.Fd()))
	if err != nil {
		term.Errorf("Error initializing poller: %s\n", err)
		r.ErrorHosts = append(r.ErrorHosts, host)
		r.Codes[host] = ErrTerminalError
		return
	}

	defer func() {
		log.Debug("Setting stdin back to blocking mode")
		si.Close()
		syscall.Dup2(stdinBackup, int(os.Stdin.Fd()))
		syscall.SetNonblock(int(os.Stdin.Fd()), false)
	}()

	buf = make([]byte, bufferSize)
	go forwardUserInput(si, ptmx, &stopped)

	if currentUsePasswordManager {
		password = passmgr.GetPass(host)
	}

	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			// TODO random stuff with intercepting and omitting data
			data, err := interceptProcessOutput(buf[:n], ptmx, password)
			if err != nil {
				// auth error, can't proceed
				raise := "su"
				if currentRaise == RTSudo {
					raise = "sudo"
				}
				log.Debugf("Wrong %s password\n", raise)
				term.Errorf("Wrong %s password\n", raise)
				r.ErrorHosts = append(r.ErrorHosts, host)
				r.Codes[host] = ErrAuthenticationError
				break
			}

			if len(data) > 0 {
				// copy stdin to process ptmx
				_, err = os.Stdout.Write(data)
				if err != nil {
					count := stdoutWriteRetry
					for os.IsTimeout(err) && count > 0 {
						time.Sleep(time.Millisecond)
						_, err = os.Stdout.Write(data)
						count--
					}
					if err != nil {
						log.Debugf("error writing to stdout not resolved in %d steps", stdoutWriteRetry)
					}
				}
			}
		}

		if err != nil && err != poller.ErrTimeout {
			log.Debugf("pty read error: %v", err)
			stopped = true
			break
		}

		select {
		case <-sigs:
			pty.InheritSize(os.Stdin, ptmx)
		default:
			continue
		}
	}

}

// RunSerial runs cmd on hosts in serial mode
func RunSerial(hosts []string, argv string, delay int) *ExecResult {
	var (
		err          error
		cmd          *exec.Cmd
		local        string
		remotePrefix string
		remoteCmd    string
		sigs         = make(chan os.Signal, 1)
	)
	r := newExecResult()

	if argv != "" {
		local, remotePrefix, err = prepareTempFiles(argv)
		if err != nil {
			term.Errorf("Error creating tempfile: %s\n", err)
			return r
		}
		defer os.Remove(local)
	}

execLoop:
	for i, host := range hosts {
		msg := term.HR(7) + " " + host + " " + term.HR(36-len(host))
		fmt.Println(term.Blue(msg))

		if argv != "" {
			remoteCmd = fmt.Sprintf("%s.%s.sh", remotePrefix, host)
			cmd = createSCPCmd(host, local, remoteCmd, false)
			signal.Notify(sigs, syscall.SIGINT)
			err = cmd.Run()
			signal.Reset()
			if err != nil {
				term.Errorf("Error copying tempfile: %s\n", err)
				r.ErrorHosts = append(r.ErrorHosts, host)
				r.Codes[host] = ErrCopyFailed
				continue
			}
		}

		cmd = createSSHCmd(host, remoteCmd)
		log.Debugf("Created SSH command: %v", cmd)

		runAtHost(host, cmd, r)

		exitCode := 0
		err = cmd.Wait()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				ws := exitErr.Sys().(syscall.WaitStatus)
				exitCode = ws.ExitStatus()
			} else {
				// MacOS hack
				exitCode = ErrMacOsExit
			}
		}

		r.Codes[host] = exitCode
		if exitCode != 0 {
			r.ErrorHosts = append(r.ErrorHosts, host)
		} else {
			r.SuccessHosts = append(r.SuccessHosts, host)
		}

		// no delay after the last host
		if delay > 0 && i != len(hosts)-1 {
			log.Debugf("Delay %d secs", delay)
			timer := time.After(time.Duration(delay) * time.Second)
			signal.Notify(sigs, syscall.SIGINT)
		timeLoop:
			for {
				select {
				case <-sigs:
					log.Debugf("Delay interrupted by ^C")
					signal.Reset()
					break execLoop
				case <-timer:
					log.Debugf("Delay finished")
					signal.Reset()
					break timeLoop
				default:
					continue
				}
			}
		}
	}

	return r
}

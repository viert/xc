package remote

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/npat-efault/poller"
	"github.com/viert/xc/log"
	"github.com/viert/xc/passmgr"
)

func (w *Worker) runcmd(task *Task) int {
	var (
		err          error
		n            int
		password     string
		passwordSent bool
		fdout        *poller.FD
		fderr        *poller.FD
		sout         io.ReadCloser
		serr         io.ReadCloser
		sin          io.WriteCloser
	)

	cmd := createSSHCmd(task.Hostname, task.Cmd)
	cmd.Env = append(os.Environ(), environment...)

	sout, err = cmd.StdoutPipe()
	if err != nil {
		log.Debugf("WRK[%d]: error creating stdout pipe: %s", w.id, err)
		return ErrTerminalError
	}
	defer sout.Close()

	serr, err = cmd.StderrPipe()
	if err != nil {
		log.Debugf("WRK[%d]: error creating stderr pipe: %s", w.id, err)
		return ErrTerminalError
	}
	defer serr.Close()

	sin, err = cmd.StdinPipe()
	if err != nil {
		log.Debugf("WRK[%d]: error creating stdin pipe: %s", w.id, err)
		return ErrTerminalError
	}
	defer sin.Close()

	soutFile, ok := sout.(*os.File)
	if !ok {
		log.Debugf("WRK[%d]: error getting process stdout file descriptor: %s", w.id)
		return ErrTerminalError
	}
	serrFile, ok := serr.(*os.File)
	if !ok {
		log.Debugf("WRK[%d]: error getting process stderr file descriptor: %s", w.id)
		return ErrTerminalError
	}

	fdout, err = poller.NewFD(int(soutFile.Fd()))
	if err != nil {
		log.Debugf("WRK[%d]: error creating stdout poller: %v", w.id, err)
		return ErrTerminalError
	}
	fderr, err = poller.NewFD(int(serrFile.Fd()))
	if err != nil {
		log.Debugf("WRK[%d]: error creating stderr poller: %v", w.id, err)
		return ErrTerminalError
	}

	buf := make([]byte, bufferSize)
	taskForceStopped := false
	shouldSkipEcho := false
	msgCount := 0

	if currentRaise != RTNone {
		passwordSent = false
		if currentUsePasswordManager {
			password = passmgr.GetPass(task.Hostname)
		} else {
			password = currentPassword
		}
	} else {
		passwordSent = true
	}

	stdoutFinished := false
	stderrFinished := false

	cmd.Start()

execLoop:
	for !(stdoutFinished && stderrFinished) {
		if w.forceStopped() {
			taskForceStopped = true
			break
		}

		commonDeadline := time.Now().Add(pollDeadline)
		fdout.SetDeadline(commonDeadline)
		fderr.SetDeadline(commonDeadline)

		n, err = fdout.Read(buf)
		if err != nil {
			if err != poller.ErrTimeout {
				// EOF, done
				log.Debugf("WRK[%d]: error reading process stdout: %v", w.id, err)
				stdoutFinished = true
			}
		}

		if n != 0 {
			w.data <- &Message{buf[:n], MTDebug, task.Hostname, -1}
			msgCount++

			chunks := bytes.SplitAfter(buf[:n], []byte{'\n'})
			for _, chunk := range chunks {
				// Trying to find Password prompt in first 5 chunks of data from server
				if msgCount < 5 {
					if !passwordSent && exPasswdPrompt.Match(chunk) {
						_, err := sin.Write([]byte(password + "\n"))
						if err != nil {
							log.Debugf("WRK[%d]: Error sending password: %v", w.id, err)
						}
						passwordSent = true
						shouldSkipEcho = true
						continue
					}
					if shouldSkipEcho && exEcho.Match(chunk) {
						shouldSkipEcho = false
						continue
					}
					if passwordSent && exWrongPassword.Match(chunk) {
						w.data <- &Message{[]byte("sudo: Authentication failure\n"), MTData, task.Hostname, -1}
						taskForceStopped = true
						break execLoop
					}

				}

				if len(chunk) == 0 {
					continue
				}

				if exConnectionClosed.Match(chunk) {
					continue
				}

				if exLostConnection.Match(chunk) {
					continue
				}

				// avoiding passing loop variable further as it's going to change its contents
				data := make([]byte, len(chunk))
				copy(data, chunk)
				w.data <- &Message{data, MTData, task.Hostname, -1}
			}
		}

		n, err = fderr.Read(buf)
		if err != nil {
			if err != poller.ErrTimeout {
				// EOF, done
				log.Debugf("WRK[%d]: error reading process stderr: %v", w.id, err)
				stderrFinished = true
			}
		}

		if n != 0 {
			w.data <- &Message{buf[:n], MTDebug, task.Hostname, -1}
			msgCount++

			chunks := bytes.SplitAfter(buf[:n], []byte{'\n'})
			for _, chunk := range chunks {
				if len(chunk) == 0 {
					continue
				}
				if exConnectionClosed.Match(chunk) {
					continue
				}
				if exLostConnection.Match(chunk) {
					continue
				}
				// avoiding passing loop variable further as it's going to change its contents
				data := make([]byte, len(chunk))
				copy(data, chunk)
				w.data <- &Message{data, MTData, task.Hostname, -1}
			}
		}

	}

    fderr.Close()
    fdout.Close()
	exitCode := 0
	if taskForceStopped {
		err = cmd.Process.Kill()
		if err != nil {
			log.Debugf("WRK[%d]: Error killing the process: %v", w.id, err)
		}
		exitCode = ErrForceStop
		log.Debugf("WRK[%d]: Task on %s was force stopped", w.id, task.Hostname)
	}

	err = cmd.Wait()

	if !taskForceStopped {
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				ws := exitErr.Sys().(syscall.WaitStatus)
				exitCode = ws.ExitStatus()
			} else {
				// MacOS hack
				exitCode = ErrMacOsExit
			}
		}
		log.Debugf("WRK[%d]: Task on %s exit code is %d", w.id, task.Hostname, exitCode)
	}
	return exitCode
}

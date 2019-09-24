package remote

import (
	"bytes"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/kr/pty"
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
	)

	cmd := createSSHCmd(task.Hostname, task.Cmd)
	cmd.Env = append(os.Environ(), environment...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return ErrTerminalError
	}
	defer ptmx.Close()

	fd, err := poller.NewFD(int(ptmx.Fd()))
	if err != nil {
		return ErrTerminalError
	}
	defer fd.Close()

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

execLoop:
	for {
		if w.forceStopped() {
			taskForceStopped = true
			break
		}

		fd.SetReadDeadline(time.Now().Add(pollDeadline))
		n, err = fd.Read(buf)
		if err != nil {
			if err != poller.ErrTimeout {
				// EOF, done
				break
			} else {
				continue
			}
		}

		if n == 0 {
			continue
		}

		w.data <- &Message{buf, MTDebug, task.Hostname, -1}
		msgCount++

		chunks := bytes.SplitAfter(buf[:n], []byte{'\n'})
		for _, chunk := range chunks {
			// Trying to find Password prompt in first 5 chunks of data from server
			if msgCount < 5 {
				if !passwordSent && exPasswdPrompt.Match(chunk) {
					ptmx.Write([]byte(password + "\n"))
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

	exitCode := 0
	if taskForceStopped {
		cmd.Process.Kill()
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

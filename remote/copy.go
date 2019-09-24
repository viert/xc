package remote

import (
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/kr/pty"
	"github.com/npat-efault/poller"
	"github.com/viert/xc/log"
)

func (w *Worker) tarcopy(task *Task) int {
	var err error
	var n int

	cmd := createTarCopyCmd(task.Hostname, task.LocalFilename, task.RemoteFilename)
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
		w.data <- &Message{buf[:n], MTData, task.Hostname, 0}
		buf = make([]byte, bufferSize)
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

func (w *Worker) copy(task *Task) int {
	var err error
	var n int

	cmd := createSCPCmd(task.Hostname, task.LocalFilename, task.RemoteFilename, task.RecursiveCopy)
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
		w.data <- &Message{buf[:n], MTDebug, task.Hostname, 0}
		buf = make([]byte, bufferSize)
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

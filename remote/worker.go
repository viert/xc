package remote

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/viert/xc/log"
	"github.com/viert/xc/passmgr"
)

// RaiseType enum
type RaiseType int

// Raise types
const (
	RTNone RaiseType = iota
	RTSu
	RTSudo
)

// CopyType enum
type CopyType int

// Copy types
const (
	CTScp CopyType = iota
	CTTar
)

// Task type represents a worker task descriptor
type Task struct {
	Hostname       string
	LocalFilename  string
	RemoteFilename string
	RecursiveCopy  bool
	Cmd            string
	Copy           CopyType
	WG             *sync.WaitGroup
}

// MessageType describes a type of worker message
type MessageType int

// Message represents a worker message
type Message struct {
	Data       []byte
	Type       MessageType
	Hostname   string
	StatusCode int
}

// Enum of OutputTypes
const (
	MTData MessageType = iota
	MTDebug
	MTCopyFinished
	MTExecFinished
)

// Custom error codes
const (
	ErrMacOsExit = 32500 + iota
	ErrForceStop
	ErrCopyFailed
	ErrTerminalError
	ErrAuthenticationError
	ErrCommandStartFailed
)

const (
	pollDeadline = 50 * time.Millisecond
	bufferSize   = 4096
)

// Worker type represents a worker object
type Worker struct {
	id    int
	queue chan *Task
	data  chan *Message
	stop  chan bool
	busy  bool
}

var (
	wrkseq      = 1
	environment = []string{"LC_ALL=en_US.UTF-8", "LANG=en_US.UTF-8"}

	// remote expressions to catch
	exConnectionClosed = regexp.MustCompile(`([Ss]hared\s+)?[Cc]onnection\s+to\s+.+\s+closed\.?[\n\r]+`)
	exPasswdPrompt     = regexp.MustCompile(`[Pp]assword`)
	exWrongPassword    = regexp.MustCompile(`[Ss]orry.+try.+again\.?`)
	exPermissionDenied = regexp.MustCompile(`[Pp]ermission\s+denied`)
	exLostConnection   = regexp.MustCompile(`[Ll]ost\sconnection`)
	exEcho             = regexp.MustCompile(`^[\n\r]+$`)
)

// NewWorker creates a new worker
func NewWorker(queue chan *Task, data chan *Message) *Worker {
	w := &Worker{
		id:    wrkseq,
		queue: queue,
		data:  data,
		stop:  make(chan bool, 1),
		busy:  false,
	}
	wrkseq++
	go w.run()
	return w
}

// ID is a worker id getter
func (w *Worker) ID() int {
	return w.id
}

func (w *Worker) run() {
	var result int

	log.Debugf("WRK[%d] Started", w.id)
	for task := range w.queue {
		// Every task consists of copying part and executing part
		// It may contain both or just one of them
		// If there are both parts, worker copies data and then runs
		// the given command immediately. This behaviour is handy for runscript
		// command when the script is being copied to a remote server
		// and called right after it.

		w.busy = true
		log.Debugf("WRK[%d] Got a task for host %s by worker", w.id, task.Hostname)

		// does the task have anything to copy?
		if task.RemoteFilename != "" && task.LocalFilename != "" {
			if task.Copy == CTScp {
				result = w.copy(task)
			} else {
				result = w.tarcopy(task)
			}
			log.Debugf("WRK[%d] Copy on %s, status=%d", w.id, task.Hostname, result)
			w.data <- &Message{nil, MTCopyFinished, task.Hostname, result}
			if result != 0 {
				log.Debugf("WRK[%d] Copy on %s, result != 0, catching", w.id, task.Hostname)
				// if copying failed we can't proceed further with the task if there's anything to run
				if task.Cmd != "" {
					log.Debugf("WRK[%d] Copy on %s, result != 0, task.Cmd == \"%s\", sending ExecFinished", w.id, task.Hostname, task.Cmd)
					w.data <- &Message{nil, MTExecFinished, task.Hostname, ErrCopyFailed}
				}
				w.busy = false
				if task.WG != nil {
					task.WG.Done()
				}
				// next task
				continue
			}
		}

		// does the task have anything to run?
		if task.Cmd != "" {
			log.Debugf("WRK[%d] runcmd(%s) at %s", w.id, task.Cmd, task.Hostname)
			result = w.runcmd(task)
			w.data <- &Message{nil, MTExecFinished, task.Hostname, result}
		}

		if task.WG != nil {
			task.WG.Done()
		}

		w.busy = false
	}
	log.Debugf("WRK[%d] Task queue has closed, worker is exiting", w.id)
}

// ForceStop stops the current task execution and returns true
// if any task were actually executed at the moment of calling ForceStop
func (w *Worker) ForceStop() bool {
	if w.busy {
		w.stop <- true
		return true
	}
	return false
}

func (w *Worker) forceStopped() bool {
	select {
	case <-w.stop:
		return true
	default:
		return false
	}
}

func (w *Worker) log(format string, args ...interface{}) {
	format = fmt.Sprintf("WRK[%d]: %s", w.id, format)
	log.Debugf(format, args...)
}

func (w *Worker) processStderr(rd io.ReadCloser, wr io.WriteCloser, finished *bool, task *Task) {
	var (
		n   int
		err error
		buf []byte
	)

	buf = make([]byte, bufferSize)
	w.log("starting stderr processor for host %s", task.Hostname)
	for {
		n, err = rd.Read(buf)
		if err != nil {
			*finished = true
			break
		}

		if n > 0 {
			w.data <- &Message{buf[:n], MTDebug, task.Hostname, -1}
			chunks := bytes.SplitAfter(buf[:n], []byte{'\n'})
			for _, chunk := range chunks {
				if currentDebug {
					w.log("STDERR CHUNK IN @ %s: %v %s", task.Hostname, chunk, string(chunk))
				}

				if exConnectionClosed.Match(chunk) {
					chunk = exConnectionClosed.ReplaceAll(chunk, []byte{})
					w.log("expr connection closed on stderr")
				}

				if exLostConnection.Match(chunk) {
					chunk = exLostConnection.ReplaceAll(chunk, []byte{})
					w.log("expr lost connection on stderr")
				}

				if len(chunk) == 0 {
					continue
				}

				// avoiding passing loop variable further as it's going to change its contents
				data := make([]byte, len(chunk))
				copy(data, chunk)
				if currentDebug {
					w.log("STDERR CHUNK OUT @ %s: %v %s", task.Hostname, data, string(data))
				}
				w.data <- &Message{data, MTData, task.Hostname, -1}

			}
		}
	}
	w.log("exiting stderr processor for host %s", task.Hostname)
}

func (w *Worker) processStdout(rd io.ReadCloser, wr io.WriteCloser, finished *bool, task *Task) {
	var (
		n            int
		msgCount     int
		err          error
		buf          []byte
		password     string
		passwordSent bool
	)

	w.log("starting stdout processor for host %s", task.Hostname)
	buf = make([]byte, bufferSize)
	msgCount = 0

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
		n, err = rd.Read(buf)
		if err != nil {
			*finished = true
			break
		}

		if n > 0 {
			w.data <- &Message{buf[:n], MTDebug, task.Hostname, -1}
			msgCount++

			chunks := bytes.SplitAfter(buf[:n], []byte{'\n'})
			for _, chunk := range chunks {
				if currentDebug {
					w.log("STDOUT CHUNK IN @ %s: %v %s", task.Hostname, chunk, string(chunk))
				}
				// Trying to find Password prompt in first 5 chunks of data from server
				if msgCount < 10 {
					if !passwordSent && exPasswdPrompt.Match(chunk) {
						w.log("sending password for %s, msgCount=%d", task.Hostname, msgCount)
						_, err := wr.Write([]byte(password + "\n"))
						if err != nil {
							w.log("error sending password: %v", err)
						}
						passwordSent = true
						shouldSkipEcho = true
						continue
					}
				}

				if shouldSkipEcho && exEcho.Match(chunk) {
					shouldSkipEcho = false
					continue
				}
				if passwordSent && exWrongPassword.Match(chunk) {
					w.data <- &Message{[]byte("sudo: Authentication failure\n"), MTData, task.Hostname, -1}
					*finished = true
					break execLoop
				}

				if len(chunk) == 0 {
					continue
				}

				// avoiding passing loop variable further as it's going to change its contents
				data := make([]byte, len(chunk))
				copy(data, chunk)
				if currentDebug {
					w.log("STDOUT CHUNK OUT @ %s: %v %s", task.Hostname, data, string(data))
				}
				w.data <- &Message{data, MTData, task.Hostname, -1}
			}
		}
	}
	w.log("exiting stdout processor for host %s", task.Hostname)
}

func (w *Worker) _run(task *Task, cmd *exec.Cmd) int {
	cmd.Env = append(os.Environ(), environment...)

	sout, err := cmd.StdoutPipe()
	if err != nil {
		w.log("error creating stdout pipe: %v", err)
		return ErrTerminalError
	}

	serr, err := cmd.StderrPipe()
	if err != nil {
		w.log("error creating stderr pipe: %v", err)
		w.log("closing stdout pipe, err=%v", sout.Close())
		return ErrTerminalError
	}

	sin, err := cmd.StdinPipe()
	if err != nil {
		w.log("error creating stdin pipe: %v", err)
		w.log("closing stderr pipe, err=%v", serr.Close())
		w.log("closing stdout pipe, err=%v", sout.Close())
		return ErrTerminalError
	}

	err = cmd.Start()
	if err != nil {
		w.log("error starting cmd: %v", err)
		w.log("closing stderr pipe, err=%v", serr.Close())
		w.log("closing stdout pipe, err=%v", sout.Close())
		w.log("closing stdin pipe, err=%v", sin.Close())
		return ErrCommandStartFailed
	}

	stdoutFinished := false
	stderrFinished := false
	taskForceStopped := false
	go w.processStdout(sout, sin, &stdoutFinished, task)
	go w.processStderr(serr, sin, &stderrFinished, task)

	for !(stdoutFinished && stderrFinished) {
		if w.forceStopped() {
			taskForceStopped = true
			err = cmd.Process.Kill()
			if err != nil {
				w.log("error killing process: %v", err)
			}
			break
		}
		time.Sleep(pollDeadline)
	}

	exitCode := 0
	w.log("out of waitloop running cmd.Wait to cleanup")
	err = cmd.Wait()

	if taskForceStopped {
		return ErrForceStop
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			ws := exitErr.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		} else {
			// MacOS hack
			exitCode = ErrMacOsExit
		}
	}
	w.log("Task on %s exit coded is %d", task.Hostname, exitCode)
	return exitCode
}

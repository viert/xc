package remote

import (
	"regexp"
	"sync"
	"time"

	"github.com/viert/xc/log"
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
			result = w.copy(task)
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
			log.Debugf("WRK[%d] runcmd(%s) at %s", task.Cmd, task.Hostname)
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

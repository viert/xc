package remote

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/viert/xc/log"
	"github.com/viert/xc/term"
	pb "gopkg.in/cheggaaa/pb.v1"
)

const (
	stdoutWriteRetry = 25
)

// ExecResult is a struct with execution results
type ExecResult struct {
	Codes   map[string]int
	Outputs map[string][]string

	SuccessHosts      []string
	ErrorHosts        []string
	ForceStoppedHosts int
}

func newExecResult() *ExecResult {
	return &ExecResult{
		Codes:             make(map[string]int),
		Outputs:           make(map[string][]string),
		SuccessHosts:      make([]string, 0),
		ErrorHosts:        make([]string, 0),
		ForceStoppedHosts: 0,
	}
}

// Print prints ExecResults in a nice way
func (r *ExecResult) Print() {
	msg := fmt.Sprintf(" Hosts processed: %d, success: %d, error: %d    ",
		len(r.SuccessHosts)+len(r.ErrorHosts), len(r.SuccessHosts), len(r.ErrorHosts))
	h := term.HR(len(msg))
	fmt.Println(term.Green(h))
	fmt.Println(term.Green(msg))
	fmt.Println(term.Green(h))
}

// PrintOutputMap prints collapsed-style output
func (r *ExecResult) PrintOutputMap() {
	for output, hosts := range r.Outputs {
		msg := fmt.Sprintf(" %d host(s): %s   ", len(hosts), strings.Join(hosts, ","))
		tableWidth := len(msg) + 2
		termWidth := term.GetTerminalWidth()
		if tableWidth > termWidth {
			tableWidth = termWidth
		}
		fmt.Println(term.Blue(term.HR(tableWidth)))
		fmt.Println(term.Blue(msg))
		fmt.Println(term.Blue(term.HR(tableWidth)))
		fmt.Println(output)
	}
}

func enqueue(local string, remote string, hosts []string) {
	// This is in a goroutine because of decreasing the task channel size.
	// If there is a number of hosts greater than pool.dataSizeQueue (i.e. 1024)
	// this loop will actually block on reaching the limit until some tasks are
	// processed and some space in the queue is released.
	//
	// To avoid blocking on task generation this loop was moved into a goroutine
	var wg sync.WaitGroup
	for _, host := range hosts {
		// remoteFile should include hostname for the case we have
		// a number of aliases pointing to one server. With the same
		// remote filename the first task finished removes the file
		// while other tasks on the same server try to remove it afterwards and fail
		remoteFilename := fmt.Sprintf("%s.%s.sh", remote, host)
		task := &Task{
			Hostname:       host,
			LocalFilename:  local,
			RemoteFilename: remoteFilename,
			Cmd:            remoteFilename,
			WG:             &wg,
		}
		pool.AddTask(task)
	}
	wg.Wait()
}

// RunParallel runs cmd on hosts in parallel mode
func RunParallel(hosts []string, cmd string) *ExecResult {
	r := newExecResult()
	if len(hosts) == 0 {
		return r
	}

	local, remote, err := prepareTempFiles(cmd)
	if err != nil {
		term.Errorf("Error creating temporary file: %s\n", err)
		return r
	}
	defer os.Remove(local)

	running := len(hosts)
	copied := 0

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	go enqueue(local, remote, hosts)

	for running > 0 {
		select {
		case d := <-pool.Data:
			switch d.Type {
			case MTData:
				log.Debugf("MSG@%s[DATA](%d): %s", d.Hostname, d.StatusCode, string(d.Data))
				if !bytes.HasSuffix(d.Data, []byte{'\n'}) {
					d.Data = append(d.Data, '\n')
				}
				if currentPrependHostnames {
					fmt.Printf("%s: ", term.Blue(d.Hostname))
				}
				fmt.Print(string(d.Data))
				writeHostOutput(d.Hostname, d.Data)
			case MTDebug:
				if currentDebug {
					log.Debugf("DATASTREAM @ %s\n%v\n[%v]", d.Hostname, d.Data, string(d.Data))
				}
			case MTCopyFinished:
				log.Debugf("MSG@%s[COPYFIN](%d): %s", d.Hostname, d.StatusCode, string(d.Data))
				if d.StatusCode == 0 {
					copied++
				}
			case MTExecFinished:
				log.Debugf("MSG@%s[EXECFIN](%d): %s", d.Hostname, d.StatusCode, string(d.Data))
				r.Codes[d.Hostname] = d.StatusCode
				if d.StatusCode == 0 {
					r.SuccessHosts = append(r.SuccessHosts, d.Hostname)
				} else {
					r.ErrorHosts = append(r.ErrorHosts, d.Hostname)
				}
				running--
			}
		case <-sigs:
			fmt.Println()
			r.ForceStoppedHosts = pool.ForceStopAllTasks()
		}
	}

	return r
}

// RunCollapse runs cmd on hosts in collapse mode
func RunCollapse(hosts []string, cmd string) *ExecResult {
	var bar *pb.ProgressBar
	r := newExecResult()
	if len(hosts) == 0 {
		return r
	}

	local, remote, err := prepareTempFiles(cmd)
	if err != nil {
		term.Errorf("Error creating temporary file: %s\n", err)
		return r
	}
	defer os.Remove(local)

	running := len(hosts)
	copied := 0
	outputs := make(map[string]string)

	if currentProgressBar {
		bar = pb.StartNew(running)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	go enqueue(local, remote, hosts)

	for running > 0 {
		select {
		case d := <-pool.Data:
			switch d.Type {
			case MTData:
				if currentDebug {
					log.Debugf("DATASTREAM @ %s:\n%v\n[%v]\n", d.Hostname, d.Data, string(d.Data))
				}
				outputs[d.Hostname] += string(d.Data)
				logData := make([]byte, len(d.Data))
				copy(logData, d.Data)
				if !bytes.HasSuffix(d.Data, []byte{'\n'}) {
					logData = append(d.Data, '\n')
				}
				writeHostOutput(d.Hostname, logData)
			case MTDebug:
				if currentDebug {
					log.Debugf("DEBUGSTREAM @ %s:\n%v\n[%v]\n", d.Hostname, d.Data, string(d.Data))
				}
			case MTCopyFinished:
				if d.StatusCode == 0 {
					copied++
				}
			case MTExecFinished:
				if currentProgressBar {
					bar.Increment()
				}
				r.Codes[d.Hostname] = d.StatusCode
				if d.StatusCode == 0 {
					r.SuccessHosts = append(r.SuccessHosts, d.Hostname)
				} else {
					r.ErrorHosts = append(r.ErrorHosts, d.Hostname)
				}
				running--
			}
		case <-sigs:
			fmt.Println()
			r.ForceStoppedHosts = pool.ForceStopAllTasks()
		}
	}

	if currentProgressBar {
		bar.Finish()
	}

	for host, hostOutput := range outputs {
		_, found := r.Outputs[hostOutput]
		if !found {
			if currentDebug {
				log.Debugf("Collapse mode found a new output:\n\"%s\"\n%v\n", hostOutput, []byte(hostOutput))
			}
			r.Outputs[hostOutput] = make([]string, 0)
		}
		r.Outputs[hostOutput] = append(r.Outputs[hostOutput], host)
	}

	return r
}

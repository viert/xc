package remote

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/viert/xc/log"
	"github.com/viert/xc/term"
	pb "gopkg.in/cheggaaa/pb.v1"
)

// Distribute distributes a given local file or directory to a number of hosts
func Distribute(hosts []string, localFilename string, remoteFilename string, recursive bool) *ExecResult {
	var (
		wg      sync.WaitGroup
		bar     *pb.ProgressBar
		sigs    chan os.Signal
		r       *ExecResult
		t       *Task
		running int
	)

	r = newExecResult()
	running = len(hosts)
	if currentProgressBar {
		bar = pb.StartNew(running)
	}

	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT)
	defer signal.Reset()

	pool = NewPool()
	defer pool.Close()
	go func() {
		for _, host := range hosts {
			t = &Task{
				Hostname:       host,
				LocalFilename:  localFilename,
				RemoteFilename: remoteFilename,
				RecursiveCopy:  recursive,
				Copy:           currentDistributeType,
				Cmd:            "",
				WG:             &wg,
			}
			pool.AddTask(t)
		}
		wg.Wait()
	}()

	for running > 0 {
		select {
		case d := <-pool.Data:
			switch d.Type {
			case MTData:
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
				running--
				if currentProgressBar {
					bar.Increment()
				}
				r.Codes[d.Hostname] = d.StatusCode
				if d.StatusCode == 0 {
					r.SuccessHosts = append(r.SuccessHosts, d.Hostname)
				} else {
					r.ErrorHosts = append(r.ErrorHosts, d.Hostname)
				}
			}
		case <-sigs:
			r.ForceStoppedHosts = pool.ForceStopAllTasks()
		}
	}

	if currentProgressBar {
		bar.Finish()
	}
	return r
}

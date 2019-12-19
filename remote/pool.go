package remote

import (
	"github.com/viert/xc/log"
)

const (
	dataQueueSize = 1024
)

// Pool is a class representing a worker pool
type Pool struct {
	workers []*Worker
	queue   chan *Task
	Data    chan *Message
}

// NewPool creates a new worker pool of a given size
func NewPool() *Pool {

	p := &Pool{
		workers: make([]*Worker, poolSize),
		queue:   make(chan *Task, dataQueueSize),
		Data:    make(chan *Message, dataQueueSize),
	}

	for i := 0; i < poolSize; i++ {
		p.workers[i] = NewWorker(p.queue, p.Data)
	}
	log.Debugf("Remote execution pool created with %d workers", poolSize)
	log.Debugf("Data Queue Size is %d", dataQueueSize)
	return p
}

// ForceStopAllTasks removes all pending tasks and force stops those in progress
func (p *Pool) ForceStopAllTasks() int {
	// Remove all pending tasks from the queue
	log.Debug("Force stopping all tasks")
	i := 0
rmvLoop:
	for {
		select {
		case <-p.queue:
			i++
			continue
		default:
			break rmvLoop
		}
	}
	log.Debugf("%d queued (and not yet started) tasks removed from the queue", i)

	stopped := 0
	for _, wrk := range p.workers {
		if wrk.ForceStop() {
			log.Debugf("Worker %d was running a task so force stopped", wrk.ID())
			stopped++
		}
	}
	return stopped
}

// Close shuts down the pool itself and all its workers
func (p *Pool) Close() {
	log.Debug("Closing remote execution pool")
	p.ForceStopAllTasks()
	close(p.queue) // this should make all the workers step out of range loop on queue chan and shut down
	log.Debug("Closing the task queue")
	close(p.Data)
}

// AddTask adds a task to the pool queue
func (p *Pool) AddTask(task *Task) {
	if task.WG != nil {
		task.WG.Add(1)
	}
	p.queue <- task
}

// AddTaskHostlist creates multiple tasks to be run on a multiple hosts
func (p *Pool) AddTaskHostlist(task *Task, hosts []string) {
	for _, host := range hosts {
		t := &Task{
			Hostname:       host,
			LocalFilename:  task.LocalFilename,
			RemoteFilename: task.RemoteFilename,
			Cmd:            task.Cmd,
			WG:             task.WG,
		}
		p.AddTask(t)
	}
}

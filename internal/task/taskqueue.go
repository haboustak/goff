package task

import (
	"runtime"
	"sync"
)

type TaskFunc func() error

type taskQueueState struct {
	workers   int
	tasks     []TaskFunc
	waiter    chan struct{}
	LastError error
}

type TaskQueue struct {
	maxWorkers int
	sync.Mutex
	taskQueueState
}

// Return a TaskQueue with up to workerLimit concurrent tasks
//
// If workerLimit is <= 0, number of CPU * 2 will be used
func NewTaskQueue(workerLimit int) *TaskQueue {
	if workerLimit < 1 {
		workerLimit = runtime.NumCPU() * 2
	}

	return &TaskQueue{
		maxWorkers: workerLimit,
	}
}

// Add work to the TaskQueue
func (tq *TaskQueue) Append(f TaskFunc) {
	tq.Lock()
	// Check if this work needs to be queued
	if tq.workers == tq.maxWorkers {
		tq.tasks = append(tq.tasks, f)
		tq.Unlock()
		return
	}
	tq.workers++
	tq.Unlock()

	go tq.work(f)
}

// Wait for all queued tasks to complete
func (tq *TaskQueue) Wait() <-chan struct{} {
	tq.Lock()
	defer tq.Unlock()

	tq.waiter = make(chan struct{})

	// signal immediately if there are no workers
	if tq.workers == 0 {
		close(tq.waiter)
	}

	return tq.waiter
}

// Report an error and stop processing queued tasks
func (tq *TaskQueue) Abort(err error) {
	tq.Lock()
	defer tq.Unlock()

	// First error is enough
	if tq.LastError != nil {
		return
	}

	tq.LastError = err
	tq.tasks = []TaskFunc{}
}

// Process a TaskQueue item
func (tq *TaskQueue) work(f TaskFunc) {
	for {
		if err := f(); err != nil {
			tq.Abort(err)
		}

		tq.Lock()
		if len(tq.tasks) == 0 {
			tq.workers--
			if tq.workers == 0 && tq.waiter != nil {
				close(tq.waiter)
			}
			tq.Unlock()
			return
		}

		// next
		f = tq.tasks[0]
		tq.tasks = tq.tasks[1:]
		tq.Unlock()
	}
}

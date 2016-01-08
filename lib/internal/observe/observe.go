package observe

import (
	"fmt"
	"sync"
	"time"
)

// Process is a running program that can be terminated and kill
// TODO: handle threadsafeness aka locking from outside the process
type Process interface {

	// Terminate ends the process, allowing it to properly shut down
	// If it doesn't terminate after the given timeout, it will be killed
	Terminate(timeout time.Duration) error

	// timeout time.Duration
	// Kill forces the ending of the process
	Kill() error

	// Loop gets one from next everytime it is ready to consume the next file
	// if the next function returns an empty string, the execution must be skipped
	Loop(next chan func() string)
}

type ProcessInfo interface {
	Info() string
}

// FakeProcess is a process that is doing nothing
// The value of FakeProcess is the running state
// Terminate and Kill methods do nothing
type FakeProcess struct {
	sync.Mutex
	stopped bool
}

// Terminate does nothing
func (f *FakeProcess) Terminate(timeout time.Duration) error {
	f.Lock()
	defer f.Unlock()
	f.stopped = true
	return nil
}

// Kill does nothing
func (f *FakeProcess) Kill() error {
	f.Lock()
	defer f.Unlock()
	f.stopped = true
	return nil
}

func (f *FakeProcess) Loop(next chan func() string) {
	for {
		f.Lock()
		stopped := f.stopped
		f.Unlock()
		if stopped {
			break
		}
		n := <-next
		n()
	}
}

type FuncProcess struct {
	sync.Mutex
	stopped bool
	fn      func(file string)
}

// Terminate does nothing
func (f *FuncProcess) Terminate(timeout time.Duration) error {
	f.Lock()
	defer f.Unlock()
	f.stopped = true
	return nil
}

// Kill does nothing
func (f *FuncProcess) Kill() error {
	f.Lock()
	defer f.Unlock()
	f.stopped = true
	return nil
}

func (f *FuncProcess) Loop(next chan func() string) {
	for {
		f.Lock()
		stopped := f.stopped
		f.Unlock()
		if stopped {
			break
		}
		n := <-next
		path := n()
		if path != "" {
			f.fn(path)
		}
	}
}

type Observe struct {
	errors     chan string
	queueMutex sync.Mutex
	queueTrack map[string]bool
	process    Process
	timeout    time.Duration
}

func New(proc Process, timeout time.Duration) (*Observe, error) {
	if proc == nil {
		return nil, fmt.Errorf("missing proc is not allowed")
	}
	return &Observe{
		process: proc,
		timeout: timeout,
	}, nil
}

// if the given file is already inside the queue, it is not added to the q
func (o *Observe) addToQueue(file string) {
	o.queueMutex.Lock()
	defer o.queueMutex.Unlock()
	if !o.queueTrack[file] {
		o.queueTrack[file] = true
		//	o.queue = append(o.queue, file)
		//	o.queueLen = len(o.queue)
	}
}

func (o *Observe) removeFromQueue(file string) {
	o.queueMutex.Lock()
	defer o.queueMutex.Unlock()
	delete(o.queueTrack, file)
}

// next returns a closure over the queue, so that
// when the process is ready to run the next time
// it can call the func to get the filename
// if the filename is empty, this means, that is has already
// been processed in the meantime
func (o *Observe) next(file string) func() string {
	// println("create closure for " + file)
	return func() string {
		// println("looking up " + file)
		o.queueMutex.Lock()
		_, has := o.queueTrack[file]
		if has {
			// remove the file, since we will proceed now
			delete(o.queueTrack, file)
		}
		o.queueMutex.Unlock()
		// if the file has already been processed, return empty string to indicate:
		// do no run the proc
		if !has {
			return ""
		}
		// file was not processed in the meantime, so return it, that it can be processed
		return file
	}
}

// Start may only be called once
func (o *Observe) Start(filechanged <-chan string, finished chan bool) (stop chan bool, kill chan bool) {
	next := make(chan func() string, 10000)

	go func() {
		o.process.Loop(next)
	}()

	o.queueTrack = map[string]bool{}
	stop = make(chan bool)
	kill = make(chan bool)
	go func() {
		for {
			select {
			case <-stop:
				o.process.Terminate(o.timeout)
				finished <- true
				break
			case <-kill:
				o.process.Kill()
				finished <- true
				break
			case f := <-filechanged:
				o.addToQueue(f)
				// here is all the meat:
				// o.next() returns a closure to lookup file
				// this closure is consumed by the process when it is ready
				// for processing the next file.
				// therefor the closure sits in the next channel waiting to be called
				// when it is called, file is deleted from the queue, so that the same file
				// will only be processed, if it changed after the process began to proceed it
				// since we break the loop on stop and kill, there is no need to take care of next
				// wrt to stopped processes
				next <- o.next(f)
			}
		}
	}()
	return
}

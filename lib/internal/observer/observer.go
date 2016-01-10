package observer

import (
	"fmt"
	"strings"
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

	// IsStopped() bool

	// Run runs the command string
	Run(cmd string)

	// Loop gets one from next everytime it is ready to consume the next file
	// if the next function returns an empty string, the execution must be skipped
	// Loop(next chan func() string)
}

/*
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
*/

// Observer should be used to run and terminate the processes
// it guarantees that just one process is running at the same time
// and that its Run(), Kill() and Terminat() aren't run st frr
type Observer struct {
	queueMutex    sync.Mutex
	procMutex     sync.RWMutex
	procStopped   bool
	queueTrack    map[string]bool
	process       Process
	command       string
	watchDir      string
	ReportRemoved bool
	bufSize       int
}

func New(command string, watchDir string, proc Process, bufSize int) (*Observer, error) {
	if proc == nil {
		return nil, fmt.Errorf("missing proc is not allowed")
	}

	return &Observer{
		ReportRemoved: !strings.Contains(command, "$_file"),
		process:       proc,
		//timeout:  timeout,
		command:  command,
		watchDir: watchDir,
		bufSize:  bufSize,
	}, nil
}

// if the given file is already inside the queue, it is not added to the q
func (o *Observer) addToQueue(file string) {
	o.queueMutex.Lock()
	defer o.queueMutex.Unlock()
	/*
		if !o.ReportRemoved {
			o.queueTrack[""] = true
			return
		}
	*/

	if !o.queueTrack[file] {
		o.queueTrack[file] = true
		//	o.queue = append(o.queue, file)
		//	o.queueLen = len(o.queue)
	}
}

/*
func (o *Observer) removeFromQueue(file string) {
	o.queueMutex.Lock()
	defer o.queueMutex.Unlock()
	if !o.ReportRemoved {
		delete(o.queueTrack, "")
		return
	}
	delete(o.queueTrack, file)
}
*/

// next returns a closure over the queue, so that
// when the process is ready to run the next time
// it can call the func to get the filename
// if the filename is empty, this means, that is has already
// been processed in the meantime
func (o *Observer) next(file string) func() (has bool, file string) {
	// println("create closure for " + file)
	return func() (bool, string) {
		// println("looking up " + file)
		o.queueMutex.Lock()
		/*
			if !o.ReportRemoved {
				file = ""
			}
		*/
		_, has := o.queueTrack[file]
		//fmt.Printf("looking up %#v, has: %v\n", file, has)
		if has {
			// remove the file, since we will proceed now
			delete(o.queueTrack, file)
		}
		o.queueMutex.Unlock()
		// if the file has already been processed, return empty string to indicate:
		// do no run the proc
		/*
			if !has {
				return "-"
			}
		*/

		/*
			if !o.ReportRemoved {
				file = "run"
			}
		*/
		// file was not processed in the meantime, so return it, that it can be processed
		return has, file
	}
}

func (o *Observer) Kill() error {
	o.procMutex.Lock()
	defer o.procMutex.Unlock()
	if o.procStopped {
		return nil
	}
	o.procStopped = true
	return o.process.Kill()
}

// tries to terminate process and kills it after timeout and if it does not exit properly
func (o *Observer) Terminate(timeout time.Duration) error {
	o.procMutex.Lock()
	if o.procStopped {
		return nil
	}
	o.procStopped = true
	o.procMutex.Unlock()
	return o.Terminate(timeout)
}

// Run may only be called once
func (o *Observer) Run(filechanged <-chan string, dirchanged <-chan bool, sleep time.Duration) {
	next := make(chan func() (bool, string), o.bufSize)
	o.queueTrack = map[string]bool{}

	go func() {
		// blocking until we get something new
		for fn := range next {
			if sleep > 0 {
				time.Sleep(sleep)
			}
			has, file := fn()

			// println("command got " + file)
			// file was already processed, so skip and wait for the next
			if !has {
				continue
			}
			o.procMutex.RLock()
			stopped := o.procStopped
			o.procMutex.RUnlock()
			if stopped {
				break
			}

			c := strings.Replace(o.command, "$_file", file, -1)
			c = strings.Replace(c, "$_wd", o.watchDir, -1)

			o.procMutex.Lock()
			o.process.Run(c)
			o.procMutex.Unlock()
		}
	}()

	if o.ReportRemoved {
		go func() {
			for hasChanged := range dirchanged {
				o.procMutex.RLock()
				stopped := o.procStopped
				o.procMutex.RUnlock()
				if stopped {
					break
				}
				// fmt.Println("got dirchanged")
				if hasChanged {
					// fmt.Println("adding to q")
					o.addToQueue("")
					// here is all the meat:
					// o.next() returns a closure to lookup file
					// this closure is consumed by the process when it is ready
					// for processing the next file.
					// therefor the closure sits in the next channel waiting to be called
					// when it is called, file is deleted from the queue, so that the same file
					// will only be processed, if it changed after the process began to proceed it
					// since we break the loop on stop and kill, there is no need to take care of next
					// wrt to stopped processes
					next <- o.next("")
				}
			}
		}()
	}

	go func() {
		for f := range filechanged {
			o.procMutex.RLock()
			stopped := o.procStopped
			o.procMutex.RUnlock()
			if stopped {
				break
			}

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
	}()
	return
}

package observer

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Process is a running program that can be terminated and kill
type Process interface {
	// Terminate ends the process, allowing it to properly shut down
	// If it doesn't terminate after the given timeout, it will be killed
	Terminate(timeout time.Duration) error

	// timeout time.Duration
	// Kill forces the ending of the process
	Kill() error

	// Run runs the command string. There will be no two calls of Run at the same time
	Run(cmd string)
}

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
		command:       command,
		watchDir:      watchDir,
		bufSize:       bufSize,
	}, nil
}

// if the given file is already inside the queue, it is not added to the q
func (o *Observer) addToQueue(file string) {
	o.queueMutex.Lock()
	defer o.queueMutex.Unlock()

	if !o.queueTrack[file] {
		o.queueTrack[file] = true
	}
}

// next returns a closure over the queue, so that
// when the process is ready to run the next time
// it can call the func to get the filename
// if the filename is empty, this means, that is has already
// been processed in the meantime
func (o *Observer) next(file string) func() (proceed bool, file string) {
	return func() (bool, string) {
		o.queueMutex.Lock()

		_, proceed := o.queueTrack[file]

		if proceed {
			// remove the file, since we will proceed now
			delete(o.queueTrack, file)
		}
		o.queueMutex.Unlock()

		return proceed, file
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
		o.procMutex.Unlock()
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
			o.procMutex.RLock()
			stopped := o.procStopped
			o.procMutex.RUnlock()
			if stopped {
				break
			}

			if sleep > 0 {
				time.Sleep(sleep)
			}

			proceed, file := fn()

			// file was already processed, so skip and wait for the next
			if !proceed {
				continue
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

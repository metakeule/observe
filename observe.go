package observe

import (

	// "bytes"
	"fmt"
	"sync"
)

/*
// Runner runs a program
// TODO: externalize Watcher thingie, should also run inside own go routine
type Runner interface {
	// Run runs the given program and attaches stdout and stderr. It returns the running process
	Run(program string, stdout, stderr io.Writer) (proc Process, err error)
}
*/

// Process is a running program that can be terminated and kill
// TODO: handle threadsafeness aka locking from outside the process
type Process interface {
	// Terminate ends the process, allowing it to properly shut down
	Terminate() error
	// timeout time.Duration
	// Kill forces the ending of the process
	Kill() error
	// get one from next everytime it is ready to consume the next file
	// if the next function returns an empty string, the execution must be skipped
	Loop(next chan func() string)
}

/*
case <-time.After(3 * time.Second):
 			fmt.Println("timed out")
 		}
*/

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
func (f *FakeProcess) Terminate() error {
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

/*
// RunnerFunc is a runner as a function
type RunnerFunc func(file string, stdout, stderr io.Writer) (proc Process, err error)

// Run calls the runner function
func (rf RunnerFunc) Run(file string, stdout, stderr io.Writer) (proc Process, err error) {
	return rf(file, stdout, stderr)
}

// FuncCall is a runner that is running a function instead of a process.
type FuncCall func(file string, stdout, stderr io.Writer) (err error)

// Run calls the function.  The returned process is a FakeProcess
func (fc FuncCall) Run(file string, stdout, stderr io.Writer) (proc Process, err error) {
	proc = FakeProcess{}
	err = fc(file, stdout, stderr)
	return
}
*/

type channelWriter struct {
	ch chan string
}

func (c *channelWriter) Write(b []byte) (int, error) {
	// println("writing: " + string(b))
	c.ch <- string(b)
	return len(b), nil
}

type Observe struct {
	Skip bool

	//runner Runner

	printers chan string
	errors   chan string
	stopped  bool

	queueMutex   sync.Mutex
	queueTrack   map[string]bool
	queue        []string
	queueLen     int
	runFinished  chan bool
	process      Process
	stdout       *channelWriter
	stderr       *channelWriter
	cmdRunning   bool
	runningMutex sync.RWMutex
}

func New(proc Process) (*Observe, error) {
	if proc == nil {
		return nil, fmt.Errorf("missing proc is not allowed")
	}
	return &Observe{
		process: proc,
	}, nil
}

/*
func (o *Observe) setRunning() {
	o.runningMutex.Lock()
	defer o.runningMutex.Unlock()
	o.cmdRunning = true
}

func (o *Observe) unsetRunning() {
	o.runningMutex.Lock()
	defer o.runningMutex.Unlock()
	o.cmdRunning = false
}

func (o *Observe) IsRunning() bool {
	o.runningMutex.RLock()
	defer o.runningMutex.RUnlock()
	return o.cmdRunning
}
*/

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

/*
func (o *Observe) runFromQueue() {
	if o.queueLen > 0 {
		o.queueMutex.Lock()
		file := o.queue[0]
		if o.queueLen == 1 {
			o.queue = []string{}
			o.queueLen = 0
		} else {
			o.queue = o.queue[1:]
			o.queueLen = len(o.queue)
		}
		delete(o.queueTrack, file)
		o.queueMutex.Unlock()
		o.run(file)
	}
}
*/

/*
func (o *Observe) run(file string) (err error) {
	// o.stdout.Reset()
	// o.stderr.Reset()
	o.setRunning()
	o.process, err = o.runner.Run(file, o.stdout, o.stderr)
	return err
}
*/

//func (o *Observe) Start(printers, errors chan string, finished chan bool, filechanged <-chan string) (runFinished chan bool, stop chan time.Duration, kill chan bool) {
// Start may only be called once
func (o *Observe) Start(filechanged <-chan string, finished chan bool) (stop chan bool, kill chan bool) {
	// println("start called")
	// TODO: protect start assignments with mutext, or allow just one call of start per observer
	next := make(chan func() string, 10000)

	go func() {
		o.process.Loop(next)
	}()

	o.queueTrack = map[string]bool{}
	// o.errors = errors
	// o.printers = printers
	// o.runFinished = make(chan bool)
	// o.stderr = &channelWriter{errors}
	// o.stdout = &channelWriter{printers}
	stop = make(chan bool)
	kill = make(chan bool)
	// runFinished = o.runFinished // make(chan bool, 1)
	go func() {
		for {
			select {
			/*
				case timeout := <-stop:
					// println("<-stop")
					o.stopped = true

					if !o.IsRunning() {
						finished <- true
						// println("is not running")
						return
					}

					// println("terminating")
					term := o.process.Terminate(timeout)
					err := <-term
					if err == nil {
						o.unsetRunning()
						finished <- true
						return
					}

					o.errors <- err.Error()
					go func() {
						//o.printers <- fmt.Sprintf("try to kill PID %v", o.process.Pid)
						kill <- true
					}()
			*/
			case <-stop:
				// println("stopped")
				err := o.process.Terminate()
				if err != nil {
					o.process.Kill()
				}
				finished <- true
				break
			case <-kill:
				// println("killed")
				o.process.Kill()
				finished <- true
				break
				// println("<-kill")
				/*
					if o.IsRunning() {
						if err := o.process.Kill(); err == nil {
							o.unsetRunning()
							finished <- true
							return
						}
					}
				*/
				// return
				/*
					case <-o.runFinished:
						// println("runFinished")
						runFinished <- true
						if !o.stopped && !o.Skip {
							o.runFromQueue()
						} else {
							if o.stopped && o.IsRunning() {
								o.unsetRunning()
								finished <- true
								return
							}
						}
				*/
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

				/*
					// println("<-filechanged")
					if !o.stopped {
						//go func() {

						//}
						// println("call runner")
						err := o.run(f)
						if err == nil {
							var success bool
							success, err = o.process.Wait()
							if err == nil {
								o.unsetRunning()
								if !success {
									if pinf, ok := o.process.(ProcessInfo); ok {
										errors <- fmt.Sprintf("process %s with file %s did not finish sucessfully", pinf.Info(), f)
									} else {
										errors <- fmt.Sprintf("process with file %s did not finish sucessfully", f)
									}

								}
								// println("success", success)
								o.runFinished <- true
							}
						}
						// _ = err
					}
				*/
				// default:
			}

		}
	}()

	//}()
	return
}

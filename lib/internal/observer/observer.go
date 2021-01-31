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

	Kill2() error

	// Run runs the command string. There will be no two calls of Run at the same time
	Run(cmd string, block bool)
}

// Observer should be used to run and terminate the processes
// it guarantees that just one process is running at the same time
// and that its Run(), Kill() and Terminat() aren't run st frr
type Observer struct {
	queueMutex         sync.Mutex
	procMutex          sync.RWMutex
	procStopped        bool
	procKilledOnChange bool
	queueTrack         map[string]bool
	process            Process
	command            string
	watchDir           string
	ReportRemoved      bool
	bufSize            int
	DirOnly            bool
	verbose            bool
}

func New(command string, watchDir string, proc Process, bufSize int, verbose bool) *Observer {
	obs := &Observer{
		process:  proc,
		command:  command,
		watchDir: watchDir,
		bufSize:  bufSize,
		verbose:  verbose,
	}

	if !strings.Contains(command, "$_file") {
		obs.ReportRemoved = true
		obs.DirOnly = true
	}

	return obs
}

// if the given file is already inside the queue, it is not added to the q
func (o *Observer) addToQueue(file string) {
	o.queueMutex.Lock()
	defer o.queueMutex.Unlock()
	o.queueTrack[file] = true
}

// next returns a closure over the queue, so that
// when the process is ready to run the next time
// it can call the func to get the filename
// if the filename is empty, this means, that is has already
// been processed in the meantime
// here is all the meat:
// o.next() returns a closure to lookup file
// this closure is consumed by the process when it is ready
// for processing the next file.
// therefor the closure sits in the next channel waiting to be called
// when it is called, file is deleted from the queue, so that the same file
// will only be processed, if it changed after the process began to proceed it
// since we break the loop on stop and kill, there is no need to take care of next
// wrt to stopped processes
func (o *Observer) next(file string) func() (proceed bool, file string) {
	return func() (bool, string) {
		if o.verbose {
			fmt.Printf("next called for: %#v\n", file)
		}
		o.queueMutex.Lock()

		_, proceed := o.queueTrack[file]

		if proceed {
			if o.verbose {
				fmt.Printf("don't proceed, already processed: %#v\n", file)
			}
			// remove the file, since we will proceed now
			delete(o.queueTrack, file)
		}
		o.queueMutex.Unlock()

		return proceed, file
	}
}

func (o *Observer) killOnChange() error {
	if o.procStopped || o.procKilledOnChange {
		return nil
	}
	o.procKilledOnChange = true
	if o.verbose {
		fmt.Println("killing process (triggered by change)")
	}
	return o.process.Kill2()
}

func (o *Observer) Kill() error {
	o.procMutex.Lock()
	defer o.procMutex.Unlock()
	if o.procStopped {
		return nil
	}
	o.procStopped = true
	if o.verbose {
		fmt.Println("killing process")
	}
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
	if o.verbose {
		fmt.Println("terminating process")
	}
	return o.Terminate(timeout)
}

// Run may only be called once
// killOnChange means: if file changes, previous command is killed
func (o *Observer) Run(filechanged <-chan string, sleep time.Duration, killOnChange bool) {
	next := make(chan func() (bool, string), o.bufSize)
	o.queueTrack = map[string]bool{}

	/*
		   Architecture

		   I. Have a goroutine to  listen on the filechanges and dirchanges and adding reported files to the queue.
		      The queue is used to "cleanup" the needed runs. If n new file changes arrives while running the command,
		      we just want to keep one file change per file, because any other run on the same file would be obsoleted
		      anyway by the next. A directory change (i.e. removal and renaming of a file; file name is unknown) is
		      just a special file change with the filename being empty. When ReportRemoved is true (set when there is no
		      $_file placeholder inside the command), the file name for the queue and the callback is always "".
		      That means, it is just tracked, that the command has to be run again (once for all kind of file changes
		      in the meantime). Since the next method returns a callback with a closure over the requested file, it is
		      able to lookup this file inside the queue when being called.
		  II. A second independant goroutine is listening of the "next" callback channel and stepping through it one by one.
		      The callback is called and is a closure over the requested file name. When being called, it just checks, if
		      the file is still in the queue (and therefor a run is required) and removes the file from the queue, since it's
		      gonna be processed, so the next callback checking for the file will report that no run is needed, unless a new
		      file or dir change arrived in the meantime.

			Since both goroutines are decoupled the reported changes are buffered inside the queue and the callback channel.
			So we need to make sure the callback channel is large enough for some slow runners. Therefor it is tuneable.

	*/

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
			if !proceed || file == "" {
				if o.verbose {
					fmt.Printf("file %#v already processed, skipping\n", file)
				}
				continue
			}

			c := strings.Replace(o.command, "$_file", file, -1)
			c = strings.Replace(c, "$_wd", o.watchDir, -1)

			o.procMutex.Lock()
			if killOnChange && !o.procKilledOnChange {
				o.killOnChange()
			}
			o.procKilledOnChange = false
			if o.verbose {
				fmt.Printf("running: %#v\n", c)
			}
			o.process.Run(c, !killOnChange)
			o.procMutex.Unlock()
		}
	}()

	go func() {
		// blocking until we get something new
		for f := range filechanged {
			if o.verbose {
				fmt.Printf("observer received filechanged: %#v\n", f)
			}
			o.procMutex.RLock()
			stopped := o.procStopped
			o.procMutex.RUnlock()
			if stopped {
				if o.verbose {
					fmt.Printf("proc stopped, ignoring: %#v\n", f)
				}
				break
			}

			// either just add non empty filenames (add ignore file removes and renames) or
			// just add filenames as they are (if o.ReportRemoved is true)
			// we are only interested in changes with filenames (no removed or renamed files) (e.g. runcommand package with command containing $_file)
			if f != "" && !o.ReportRemoved {
				if o.verbose {
					fmt.Printf("adding to queue: %#v\n", f)
				}
				o.addToQueue(f)
				next <- o.next(f)
			}

			// we are only interested in directory changes, so always add empty filename (e.g. runcommand package with command not containing $_file)
			if o.DirOnly {
				if o.verbose {
					fmt.Println("adding directory (DirOnly set)")
				}
				o.addToQueue("")
				next <- o.next("")
			}

			// we are interested in filechanges and directory changes (e.g. runfunc package)
			if !o.DirOnly && o.ReportRemoved {
				if o.verbose {
					fmt.Printf("adding: %#v\n", f)
				}
				o.addToQueue(f)
				next <- o.next(f)
			}
		}
	}()
	return
}

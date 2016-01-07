package shellcmd

import (
	"sync"
	"time"
	// "bytes"
	"fmt"
	"github.com/metakeule/observe"
	"io"
	"os"
	"strings"
	"syscall"
)

type shellProcess struct {
	process *os.Process
	mx      sync.RWMutex
}

// Wait waits for the process to finish, setting the running
// state to false is the process did exit
// The success of the process is reported and if wait did fail,
// an error is returned
// TODO: maybe introduce an timeout for the wait call
func (p *shellProcess) Wait() (success bool, err error) {
	p.mx.RLock()
	defer p.mx.RUnlock()
	state, err0 := p.process.Wait()
	if err0 != nil {
		return false, err0
		//o.errorf(err.Error())
	}

	/*
		if state.Exited() {
			p.process = nil
			p.unsetRunning()
		}
	*/
	if state.Success() {
		return true, nil
	}

	return false, nil
	/*
		if o.stdout.Len() > 0 {
			o.printf("### stdout of %s", strings.Replace(o.Cmd, "$file", file, -1))
			//o.printers <- o.stdout.String()
		}

		if o.stderr.Len() > 0 || err != nil {
			o.errorf("### stderr of %s", strings.Replace(o.Cmd, "$file", file, -1))
		}
	*/

	/*
		if err != nil {
			return
			//o.errorf(err.Error())
		}

		// out = stdout.String()

		p.process = nil
		p.unsetRunning()
		return nil
	*/
}

// TODO: add timeout for waiting until sending sigterm
func (o *shellProcess) Terminate(timeout time.Duration) chan error {

	st := make(chan *os.ProcessState)
	terminator := make(chan error)

	go func() {
		// var state *os.ProcessState
		//println("running")
		state, _ := o.process.Wait()
		//println("running finished")
		st <- state
	}()
	for {
		select {
		case s := <-st:
			//println("did finish")
			// do something
			if !s.Exited() {
				terminator <- fmt.Errorf("does not exit %v", o.process.Pid)
				//o.errorf(err.Error())
				break
			}
			o.mx.Lock()
			o.process = nil
			o.mx.Unlock()
			terminator <- nil
		case <-time.After(timeout):
			//println("timeout ended")
			terminator <- o.process.Signal(syscall.SIGTERM)
			// fmt.Printf("could not terminate %v: %v\n", o.process.Pid, err)
			//return err

			// fmt.Println("timed out")
		}
	}

	_ = syscall.SIGTERM

	/*

	*/

	//	o.unsetRunning()
	//o.finished <- true
	return terminator
}

func (o *shellProcess) Kill() error {
	o.mx.RLock()
	var noProc = o.process == nil
	o.mx.RUnlock()
	if noProc {
		//o.finished <- true
		return nil
	}

	o.mx.Lock()
	defer o.mx.Unlock()
	if err := o.process.Kill(); err != nil {
		// o.errorf("could not kill %v: %v", o.process.Pid, err)
		//o.finished <- true
		return err
	}

	o.process = nil
	//o.unsetRunning()
	//o.finished <- true
	return nil
}

type ShellCMD struct {
	Command  string
	watchdir string
	// Args    []string
	// print the command before running
	Verbose bool
}

func NewShellCMD(watchdir string, cmd string) *ShellCMD {
	return &ShellCMD{
		Command:  cmd,
		watchdir: watchdir,
		// Args:    args,
	}
}

/*
func (s *ShellRunner) finish() {
	s.runFinished <- struct{
Out string
Error error
		}{s.stdout.String(), s.stderr.String()}
}

func (s *ShellRunner) errorf(format string, args ...interface{}) {
	fmt.Fprintf(&s.stdout, format, args...)
}

func (s *ShellRunner) printf(format string, args ...interface{}) {
	fmt.Fprintf(&s.stderr, format, args...)
}
*/

func (sc *ShellCMD) Run(file string, stdout, stderr io.Writer) (proc observe.Process, err error) {
	if sc.Command == "" {
		return nil, fmt.Errorf("command must not be empty")
	}

	/*
		if o.stopped {
			return
		}
		if o.IsRunning() || o.process != nil {
			if o.Skip {
				return
			}
			o.addToQueue(file)
			return
		}
	*/

	/*
	   var stderr bytes.Buffer
	   var stdout bytes.Buffer
	*/
	//	o.stderr.Reset()
	//	o.stdout.Reset()
	/*
		args := make([]string, len(sc.Args))

		for i, a := range sc.Args {
			args[i] = strings.Replace(a, "$file", file, -1)
		}
	*/
	c := strings.Replace(sc.Command, "$file", file, -1)
	c = strings.Replace(c, "$wd", sc.watchdir, -1)

	//cmd := exec.Command("/usr/bin/script", "-qfc", c)
	// cmd := exec.Command("/bin/bash", "-c", c)
	cmd := execCommand(c)

	//cmd := exec.Command(sc.Command, args...)

	if sc.Verbose {
		//fmt.Fprintf(stdout, "$ %s %s\n", sc.Command, strings.Join(args, " "))
		fmt.Fprintf(stdout, "$ %s\n", c)
	}

	cmd.Stderr = stderr
	cmd.Stdout = stdout
	err = cmd.Start()

	if err != nil {
		// fmt.Fprintln(stderr, err.Error())
		//		o.errorf(err.Error())
		// o.unsetRunning()
		return
	}
	pr := &shellProcess{}

	//pr.cmdRunning = true
	// proc.setRunning()

	pr.process = cmd.Process
	proc = pr
	return
	//o.printf("### started %s with pid %v", strings.Replace(o.Cmd, "$file", file, -1), o.process.Pid)

	//	err = cmd.Wait()
	/*
		if o.stdout.Len() > 0 {
			o.printf("### stdout of %s", strings.Replace(o.Cmd, "$file", file, -1))
			//o.printers <- o.stdout.String()
		}

		if o.stderr.Len() > 0 || err != nil {
			o.errorf("### stderr of %s", strings.Replace(o.Cmd, "$file", file, -1))
		}
	*/

	/*
		if err != nil {
			return
			//o.errorf(err.Error())
		}

		out = stdout.String()

		o.process = nil
		o.unsetRunning()
	*/
	//o.finish()
}

var _ observe.Runner = &ShellCMD{}

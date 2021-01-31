package shellproc

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"syscall"
	"time"
)

// there must just be one ShellProc at a time
type ShellProc struct {
	process *os.Process
	stdout  io.Writer
	stderr  io.Writer
	verbose bool
}

func New(stdout, stderr io.Writer, verbose bool) *ShellProc {
	return &ShellProc{
		stdout:  stdout,
		stderr:  stderr,
		verbose: verbose,
	}
}

func (s *ShellProc) Kill2() error {
	if s.process == nil {
		return nil
	}
	if s.verbose {
		fmt.Printf("kill2 process: %#v\n", s.process.Pid)
	}
	err := s.process.Kill()
	s.process.Wait()
	return err
}

// since Kill, Terminate and Run are guaranteed (by observe) to never be called at the same time
// we don't have to care about concurrency here
func (s *ShellProc) Kill() error {
	if s.process == nil {
		return nil
	}

	if s.verbose {
		fmt.Printf("kill process: %#v\n", s.process.Pid)
	}
	err := s.process.Kill()
	s.process.Wait()
	s.process = nil
	return err
}

// tries to terminate process and kills it after timeout and if it does not exit properly
func (s *ShellProc) Terminate(timeout time.Duration) error {
	if s.process == nil {
		return nil
	}
	st := make(chan *os.ProcessState)
	terminator := make(chan error)
	if s.verbose {
		fmt.Printf("terminate process: %#v\n", s.process.Pid)
	}
	err := s.process.Signal(syscall.SIGTERM)

	if err != nil {
		return s.Kill()
	}

	// run the syscall in a separate goroutine not to block the main thread
	go func() {
		for {
			select {
			case sst := <-st:
				s.process = nil
				if !sst.Exited() {
					terminator <- fmt.Errorf("does not exit %v", sst.Pid())
				} else {
					terminator <- nil
				}
				break
			case <-time.After(timeout):
				s.process = nil
				//terminator <- errors.New("timeout")
				terminator <- s.Kill()
				break
			// do we need default: here? test it with timeout
			default:
				runtime.Gosched()
			}
		}
	}()

	state, _ := s.process.Wait()
	st <- state

	return <-terminator
}

// run is blocking
// block blocks until command has finished
func (s *ShellProc) Run(command string, block bool) {
	cmd := execCommand(command)
	cmd.Stderr = s.stderr
	cmd.Stdout = s.stdout
	if s.verbose {
		fmt.Printf("run process: %#v\n", command)
	}
	err := cmd.Start()

	if err != nil {
		fmt.Fprintln(cmd.Stderr, err)
		s.process = nil
		return
	}

	s.process = cmd.Process
	if block {

		// TODO: check under which conditions Wait returns an error and which kind of errors they are
		// and if we want to kill or terminate the process.p then
		err = cmd.Wait()
		if err == nil {
			return
		}
		fmt.Fprintln(cmd.Stderr, err)
		_, err = s.process.Wait()
		if err != nil {
			fmt.Fprintln(cmd.Stderr, err)
		}
	}
	return
}

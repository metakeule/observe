package shellproc

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

// there must just be one ShellProc at a time
type ShellProc struct {
	process *os.Process
	stdout  io.Writer
	stderr  io.Writer
}

func New(stdout, stderr io.Writer) *ShellProc {
	return &ShellProc{
		stdout: stdout,
		stderr: stderr,
	}
}

// since Kill, Terminate and Run are guaranteed (by observe) to never be called at the same time
// we don't have to care about concurrency here
func (s *ShellProc) Kill() error {
	if s.process == nil {
		return nil
	}
	s.process = nil
	return s.process.Kill()
}

// tries to terminate process and kills it after timeout and if it does not exit properly
func (s *ShellProc) Terminate(timeout time.Duration) error {
	if s.process == nil {
		return nil
	}
	st := make(chan *os.ProcessState)
	terminator := make(chan error)
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
			}
		}
	}()

	state, _ := s.process.Wait()
	st <- state

	return <-terminator
}

// run is blocking
func (s *ShellProc) Run(command string) {
	cmd := execCommand(command)
	cmd.Stderr = s.stderr
	cmd.Stdout = s.stdout
	err := cmd.Start()

	if err != nil {
		fmt.Fprintln(cmd.Stderr, err)
		s.process = nil
		return
	}

	s.process = cmd.Process
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
	return
}

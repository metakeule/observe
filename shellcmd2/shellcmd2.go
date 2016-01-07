package shellcmd2

import (
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

// there must just be one shellProcess at a time
type shellProcess struct {
	process  *os.Process
	stopped  bool
	mx       sync.RWMutex
	watchDir string
	Command  string
	stdout   io.Writer
	stderr   io.Writer
	errors   chan string
	sleep    time.Duration
}

func NewShellProcess(watchDir, cmd string, stdout, stderr io.Writer, errors chan string, sleep time.Duration) *shellProcess {
	return &shellProcess{
		watchDir: watchDir,
		Command:  cmd,
		stdout:   stdout,
		stderr:   stderr,
		errors:   errors,
		sleep:    sleep,
	}
}

func (s *shellProcess) Kill() error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.stopped = true
	if s.process == nil {
		return nil
	}
	err := s.process.Kill()
	if err == nil {
		s.process = nil
	}
	return err
}

func (s *shellProcess) Terminate() error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.stopped = true
	if s.process == nil {
		return nil
	}
	err := s.process.Signal(syscall.SIGTERM)
	state, _ := s.process.Wait()
	if state.Exited() {
		s.process = nil
	}
	return err
}

// run is blocking
func (s *shellProcess) run(file string) (err error) {
	s.mx.Lock()
	defer s.mx.Unlock()
	c := strings.Replace(s.Command, "$file", file, -1)
	c = strings.Replace(c, "$wd", s.watchDir, -1)

	//cmd := exec.Command("/usr/bin/script", "-qfc", c)
	// cmd := exec.Command("/bin/bash", "-c", c)
	cmd := execCommand(c)
	cmd.Stderr = s.stderr
	cmd.Stdout = s.stdout
	err = cmd.Start()

	if err != nil {
		s.process = nil
		// fmt.Fprintln(stderr, err.Error())
		//		o.errorf(err.Error())
		// o.unsetRunning()
		return
	}

	s.process = cmd.Process
	// TODO: check under which conditions Wait returns an error and which kind of errors they are
	// and if we want to kill or terminate the process.p then
	err = cmd.Wait()
	if err == nil {
		s.process = nil
		return
	}
	state, _ := s.process.Wait()
	if state.Exited() {
		s.process = nil
	}
	return
}

// Loop must just be called once, it will be called inside its own goroutine
// but from the outside
func (s *shellProcess) Loop(get chan func() string) {
	for f := range get {
		// time.Sleep(s.sleep)
		// blocking until we get something new
		// f := <-get
		file := f()

		// println("command got " + file)
		// file was already processed, so skip and wait for the next
		if file == "" {
			continue
		}
		s.mx.Lock()
		stopped := s.stopped
		s.mx.Unlock()
		if stopped {
			break
		}

		err := s.run(file)
		if err != nil {
			s.errors <- err.Error()
		}
	}
}

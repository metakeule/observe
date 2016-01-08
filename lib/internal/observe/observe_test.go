package observe

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

/*
what to test:

- queuing
- terminate -> kill escalation
- execution in order
- skipping
- running multiple observers at the same time (concurrency)
- filechanged -> command execution
*/

type mockProcess struct {
	terminated bool
	killed     bool
	waitCalled int
	outString  string
	errString  string
	stdout     io.Writer
	stderr     io.Writer
	// sleepSeconds int
	waitError bool
}

func (p *mockProcess) Terminate(timeout time.Duration) chan error {
	term := make(chan error)
	go func() {
		println("terminated")
		p.terminated = true
		term <- nil
	}()

	return term
}

func (p *mockProcess) Kill() error {
	println("killed")
	p.killed = true
	return nil
}

func (p *mockProcess) Wait() (success bool, err error) {
	println("wait called")
	p.waitCalled++
	if p.waitError {
		println("wait error")
		return false, errors.New("waitError")
	}
	println("writing to stdout: " + p.outString)
	p.stdout.Write([]byte(p.outString))
	p.stderr.Write([]byte(p.errString))
	// println("wait start")
	// if p.sleepSeconds != 0 {
	// 	time.Sleep(time.Duration(p.sleepSeconds))
	// }
	// println("wait finished")
	return true, nil
}

type mockRunner struct {
	file      string
	ran       int
	err       error
	outString string
	errString string
	proc      mockProcess
	// sleepSeconds int
	waitError bool
}

func (r *mockRunner) Run(file string, stdout, stderr io.Writer) (proc Process, err error) {
	// println("run called")
	r.ran++
	r.file = file
	r.proc = mockProcess{
		outString: r.outString,
		errString: r.errString,
		stdout:    stdout,
		stderr:    stderr,
		// sleepSeconds: r.sleepSeconds,
		waitError: r.waitError,
	}
	return &r.proc, r.err
}

type action int

const sendfile action = 0
const terminate action = 1
const kill action = 2

// if action == 0 => send file
// if action == 1 => terminate
// if action == 2 => kill
func runOnce(r Runner, file string, a action) (stdout string, stderr string, err error) {
	var o *Observe
	o, err = New(r)
	if err != nil {
		return
	}

	var (
		printers    = make(chan string, 1)
		errors      = make(chan string, 1)
		finished    = make(chan bool, 1)
		filechanged = make(chan string, 30)
	)

	wg := sync.WaitGroup{}
	wg.Add(1)

	if a == sendfile {
		wg.Add(2) // for finished
	}

	var outbf bytes.Buffer
	var errbf bytes.Buffer

	go func() {
		for {
			select {
			case m := <-printers:
				// println("writing from printers: " + m)
				outbf.WriteString(m)
				wg.Done()
			case m := <-errors:
				// println("writing from errors: " + m)
				errbf.WriteString(m)
				wg.Done()
			// case <-finished:
			// println("finished")
			// wg.Done()
			default:
			}
		}
	}()
	runfinished, stopCh, killCh := o.Start(printers, errors, finished, filechanged)
	go func() {
		for {
			select {
			case <-runfinished:
				println("received runfinished")
				wg.Done()
			case <-finished:
				println("received finished")
				wg.Done()
				return
			default:
			}
		}
	}()
	filechanged <- file
	// go func() {
	switch a {
	case terminate:
		// wg.Done()
		// wg.Done()
		time.Sleep(time.Duration(10))
		println("start termination")
		stopCh <- time.Duration(1)
	case kill:
		time.Sleep(time.Duration(10))
		println("start killing")
		killCh <- true
	}
	// println("before wait")
	// }()
	wg.Wait()
	// println("after wait")
	/*
		if a != sendfile {
			<-finished
		}
	*/
	stdout = outbf.String()
	stderr = errbf.String()
	return
}

func TestKilled(t *testing.T) {
	println("starting TestKilled")
	r := &mockRunner{outString: "hi!", errString: "he!", waitError: true}
	file := "abc.txt"
	stdout, stderr, err := runOnce(r, file, kill)
	_ = stdout
	_ = stderr
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	if !r.proc.killed {
		t.Error("process was not killed")
	}

	if r.ran != 1 {
		t.Errorf("runner should have run %d times, but did run %d times", 1, r.ran)
	}

	if r.proc.waitCalled < 1 {
		t.Error("process had no chance to run")
	}
}

func TestTerminated(t *testing.T) {
	println("starting TestTerminated")
	r := &mockRunner{outString: "hi!", errString: "he!", waitError: true}
	file := "abc.txt"
	stdout, stderr, err := runOnce(r, file, terminate)
	_ = stdout
	_ = stderr
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	if !r.proc.terminated {
		t.Error("process did not terminate")
	}

	if r.ran != 1 {
		t.Errorf("runner should have run %d times, but did run %d times", 1, r.ran)
	}

	if r.proc.waitCalled < 1 {
		t.Error("process had no chance to run")
	}
}

func TestFinished(t *testing.T) {
	// t.Skip()
	println("starting TestFinished")
	r := &mockRunner{outString: "hi!", errString: "he!"}
	file := "abc.txt"
	stdout, stderr, err := runOnce(r, file, sendfile)

	if err != nil {
		t.Errorf("error: %s", err)
	}
	if r.file != file {
		t.Errorf("r.file = %#v; expected %#v", r.file, file)
	}

	if r.ran != 1 {
		t.Errorf("runner should have run %d times, but did run %d times", 1, r.ran)
	}

	if r.proc.waitCalled < 1 {
		t.Error("process had no chance to run")
	}

	if stdout != r.outString {
		t.Errorf("stdout = %#v; expected %#v", stdout, r.outString)
	}

	if stderr != r.errString {
		t.Errorf("stderr = %#v; expected %#v", stderr, r.errString)
	}
}

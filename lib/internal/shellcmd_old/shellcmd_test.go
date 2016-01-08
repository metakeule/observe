package shellcmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func pidExists(pid int) bool {
	//ps -p 6694 -o comm=
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=")
	var out, errs bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errs
	err := cmd.Run()
	//out, err := cmd.CombinedOutput()
	//out, err := cmd.Output()
	// println("pidExists stdout " + out.String())
	// println("pidExists stderr " + errs.String())
	if err != nil {
		return false
	}

	return out.Len() != 0

}

func TestKill(t *testing.T) {
	t.Skip()

	s := NewShellCMD(".", "./sleep.sh $file")
	var stdout, stderr bytes.Buffer

	proc, err := s.Run("20", &stdout, &stderr)

	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	shellProc := proc.(*shellProcess)
	pid := shellProc.process.Pid

	if !pidExists(pid) {
		t.Fatalf("not running: pid %d", pid)
	}

	go proc.Wait()

	time.Sleep(500 * time.Millisecond)

	err = proc.Kill()

	if err != nil {
		t.Fatalf("can't kill: %s", err)
	}

	if pidExists(pid) {
		t.Errorf("still alive, pid: %d", pid)
	}

	// if stderr.String() != "" {
	// 	t.Errorf("something in stderr: %#v", stderr.String())
	// }

	// expected := "before sleep 20\n"
	// if stdout.String() != expected {
	// 	t.Errorf("stdout = %#v; expected: %#v", stdout.String(), expected)
	// }

}

func TestTerminateWithResult(t *testing.T) {
	t.Skip()
	s := NewShellCMD(".", "./sleep.sh $file")
	var stdout, stderr bytes.Buffer

	proc, err := s.Run("2", &stdout, &stderr)

	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	// go proc.Wait()
	shellProc := proc.(*shellProcess)
	pid := shellProc.process.Pid

	time.Sleep(500 * time.Millisecond)

	if !pidExists(pid) {
		t.Fatalf("not running: pid %d", pid)
	}

	err = <-proc.Terminate(2500 * time.Millisecond)
	if err != nil {
		t.Fatalf("can't terminate: %s", err)
	}

	if stderr.String() != "" {
		t.Errorf("something in stderr: %#v", stderr.String())
	}

	if pidExists(pid) {
		t.Errorf("still alive, pid: %d", pid)
	}

	expected := "before sleep 2\nafter sleep\n"
	if stdout.String() != expected {
		t.Errorf("stdout = %#v; expected: %#v", stdout.String(), expected)
	}

}

func TestTerminate(t *testing.T) {
	t.Skip()
	s := NewShellCMD(".", "./sleep.sh $file")
	var stdout, stderr bytes.Buffer

	proc, err := s.Run("20", &stdout, &stderr)

	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	// go proc.Wait()
	shellProc := proc.(*shellProcess)
	pid := shellProc.process.Pid

	time.Sleep(500 * time.Millisecond)

	if !pidExists(pid) {
		t.Fatalf("not running: pid %d", pid)
	}

	err = <-proc.Terminate(500 * time.Millisecond)

	if err != nil {
		t.Fatalf("can't terminate: %s", err)
	}

	if stderr.String() != "" {
		t.Errorf("something in stderr: %#v", stderr.String())
	}

	if pidExists(pid) {
		t.Errorf("still alive, pid: %d", pid)
	}

	/*
		expected := "before sleep 20\n"
		if stdout.String() != expected {
			t.Errorf("stdout = %#v; expected: %#v", stdout.String(), expected)
		}
	*/
}

func TestRunner(t *testing.T) {
	s := NewShellCMD(".", "./sleep.sh $file")
	var stdout, stderr bytes.Buffer

	proc, err := s.Run("0.2", &stdout, &stderr)

	if err != nil {
		t.Fatalf("Error: %s", err)
		return
	}

	shellProc := proc.(*shellProcess)
	pid := shellProc.process.Pid

	if !pidExists(pid) {
		t.Fatalf("not running: pid %d", pid)
		return
	}

	success, _ := proc.Wait()
	if !success {
		t.Fail()
		return
	}

	if pidExists(pid) {
		t.Fatalf("still alive: pid %d", pid)
		return
	}

	if stderr.String() != "" {
		t.Errorf("something in stderr: %#v", stderr.String())
	}

	expected := "before sleep 0.2\nafter sleep\n"
	if stdout.String() != expected {
		t.Errorf("stdout = %#v; expected: %#v", stdout.String(), expected)
	}

}

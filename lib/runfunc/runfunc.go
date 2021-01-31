package runfunc

import (
	"github.com/metakeule/observe/lib/internal/observer"
	"github.com/metakeule/observe/lib/internal/watcher"
	"regexp"
	"time"
)

type funcProcess struct {
	fn     func(dir, file string) error
	errors chan error
	dir    string
}

// Terminate does nothing
func (f *funcProcess) Terminate(timeout time.Duration) error {
	return nil
}

// Kill does nothing
func (f *funcProcess) Kill() error {
	return nil
}

// Kill2 does nothing
func (f *funcProcess) Kill2() error {
	return nil
}

func (f *funcProcess) Run(file string, block bool) {
	err := f.fn(f.dir, file)
	if err != nil {
		f.errors <- err
	}
}

type Stoppable interface {
	Kill() error
	Terminate(time.Duration) error
}

type RunFunc struct {
	fn         func(dir, file string) error
	dir        string
	matchFiles *regexp.Regexp
	ignore     *regexp.Regexp
	sleep      time.Duration
	bufSize    int
	verbose    bool
}

const DefaultBufSize = 10000

type Config func(*RunFunc)

func BufSize(bfsize int) Config {
	return func(rc *RunFunc) {
		rc.bufSize = bfsize
	}
}

func MatchFiles(m *regexp.Regexp) Config {
	return func(rc *RunFunc) {
		rc.matchFiles = m
	}
}

func Ignore(i *regexp.Regexp) Config {
	return func(rc *RunFunc) {
		rc.ignore = i
	}
}

func Sleep(s time.Duration) Config {
	return func(rc *RunFunc) {
		rc.sleep = s
	}
}

// New creates a new RunFunc that calls fn on changes within
// watchDir or any of its subdirectories.
// No two instances of fn will be running at the same time.
// When calling fn, dir will be the path of watchDir and file will be the file path of the file that was
// created/modified. If file is the empty string, a file has been removed or renamed.
// Optional Configs are
// BufSize(): setting the buffer size of the reporting channels (default: DefaultBufSize)
// Ignore(): setting the regular expression to which matching files and directories are ignored (default: nil; ignore nothing)
// MatchFiles(): setting the regular expression to which file must match in order to be tracked (default: nil; all files match)
// Sleep(): setting the duration of sleeping time of two invocations of fn (default: 0; no sleeping time)
func New(watchDir string, fn func(dir, file string) error, configs ...Config) *RunFunc {
	rc := &RunFunc{
		fn:  fn,
		dir: watchDir,
	}

	for _, c := range configs {
		c(rc)
	}

	if rc.bufSize == 0 {
		rc.bufSize = DefaultBufSize
	}

	return rc
}

// Run initiates the watching and running goroutines.
// Errors while watching are reported to the error channel.
// So are errors returned by the called function.
// If the watcher or the observer could not be initialized properly,
// an error is returned. Otherwise the returned Stoppable can be used to
// terminate and kill the running process and end the observation
func (rc *RunFunc) Run(errors chan error) (Stoppable, error) {
	filechanged := make(chan string, rc.bufSize)

	watch, err := watcher.New(rc.dir, rc.matchFiles, rc.ignore, rc.verbose)
	if err != nil {
		return nil, err
	}

	proc := &funcProcess{
		fn:     rc.fn,
		errors: errors,
		dir:    rc.dir,
	}

	obs := observer.New("$_file", rc.dir, proc, rc.bufSize, rc.verbose)

	obs.ReportRemoved = true
	obs.DirOnly = false

	watch.Run(filechanged, errors)
	obs.Run(filechanged, rc.sleep, false)
	return obs, nil
}

package runcommand

import (
	"github.com/metakeule/observe/lib/internal/observer"
	"github.com/metakeule/observe/lib/internal/shellproc"
	"github.com/metakeule/observe/lib/internal/watcher"
	"io"
	"os"
	"regexp"
	"time"
)

type Stoppable interface {
	Kill() error
	Terminate(time.Duration) error
}

type RunCommand struct {
	stdout       io.Writer
	stderr       io.Writer
	matchFiles   *regexp.Regexp
	ignore       *regexp.Regexp
	sleep        time.Duration
	cmd          string
	dir          string
	bufSize      int
	killOnChange bool
	verbose      bool
}

const DefaultBufSize = 10000

type Config func(*RunCommand)

func Verbose() Config {
	return func(rc *RunCommand) {
		rc.verbose = true
	}
}

func Stdout(stdout io.Writer) Config {
	return func(rc *RunCommand) {
		rc.stdout = stdout
	}
}

// if killOnChange is true the running process will be killed, if there is a change
func KillOnChange() Config {
	return func(rc *RunCommand) {
		rc.killOnChange = true
	}
}

func Stderr(stderr io.Writer) Config {
	return func(rc *RunCommand) {
		rc.stderr = stderr
	}
}

func BufSize(bfsize int) Config {
	return func(rc *RunCommand) {
		rc.bufSize = bfsize
	}
}

func MatchFiles(m *regexp.Regexp) Config {
	return func(rc *RunCommand) {
		rc.matchFiles = m
	}
}

func Ignore(i *regexp.Regexp) Config {
	return func(rc *RunCommand) {
		rc.ignore = i
	}
}

func Sleep(s time.Duration) Config {
	return func(rc *RunCommand) {
		rc.sleep = s
	}
}

// New creates a new RunCommand that runs runCmd on changes within
// watchDir or any of its subdirectories.
// No two instances of runCmd will be running at the same time.
// Before calling runCmd, the substring $_file will be replaced by the file path of the file that was
// created/modified and $_wd will be replaced by the path of watchDir.
// If runCmd does not contain the placeholder $_file, it will not just be called on file creation and
// file changes, but also when files are renamed of removed.
// Optional Configs are
// Stdout(): setting the stdout of the running process (default: os.Stdout)
// Stderr(): setting the stderr of the running process (default: os.Stderr)
// BufSize(): setting the buffer size of the reporting channels (default: DefaultBufSize)
// Ignore(): setting the regular expression to which matching files and directories are ignored (default: nil; ignore nothing)
// MatchFiles(): setting the regular expression to which file must match in order to be tracked (default: nil; all files match)
// Sleep(): setting the duration of sleeping time of two invocations of the running command (default: 0; no sleeping time)
func New(watchDir, runCmd string, configs ...Config) *RunCommand {
	rc := &RunCommand{
		cmd: runCmd,
		dir: watchDir,
	}

	for _, c := range configs {
		c(rc)
	}

	if rc.stdout == nil {
		rc.stdout = os.Stdout
	}

	if rc.stderr == nil {
		rc.stderr = os.Stderr
	}

	if rc.bufSize == 0 {
		rc.bufSize = DefaultBufSize
	}

	return rc
}

// Run initiates the watching and running goroutines.
// Errors while watching are reported to the error channel.
// If the watcher or the observer could not be initialized properly,
// an error is returned. Otherwise the returned Stoppable can be used to
// terminate and kill the running process and end the observation
func (rc *RunCommand) Run(errors chan error) (Stoppable, error) {
	filechanged := make(chan string, rc.bufSize)

	watch, err := watcher.New(rc.dir, rc.matchFiles, rc.ignore, rc.verbose)
	if err != nil {
		return nil, err
	}
	proc := shellproc.New(rc.stdout, rc.stderr, rc.verbose)
	obs := observer.New(rc.cmd, rc.dir, proc, rc.bufSize, rc.verbose)

	watch.Run(filechanged, errors)
	obs.Run(filechanged, rc.sleep, rc.killOnChange)
	return obs, nil
}

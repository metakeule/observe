package main

import (
	"fmt"
	"github.com/metakeule/observe/lib/runcommand"
	"gopkg.in/metakeule/config.v1"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	args = config.MustNew("observe", "0.0.1",
		"observe runs a command when the content of a directory changes")

	dirArg = args.NewString("dir",
		"directory to be observed",
		config.Shortflag('d'), config.Default("."))

	cmdArg = args.NewString("cmd",
		"command to be executed, $file will be replaced by the changed file",
		config.Shortflag('c'), config.Required)

	matchArg = args.NewString("match",
		"match files based on the given regular expression (posix)",
		config.Shortflag('m'), config.Default("*"))

	recursiveArg = args.NewBool("recursive",
		"watch subdirectories",
		config.Shortflag('r'), config.Default(true))

	ignoreArg = args.NewString("ignore",
		"ignore directories based on the given regular expression (posix)",
		config.Default(""), config.Shortflag('i'))

	//	verboseArg = args.NewBool("verbose",
	//		"show the command that is being run",
	//		config.Shortflag('v'), config.Default(true))

	timeoutArg = args.NewString("timeout",
		"timeout for command termination when pressing CTRL+c once, you need a suffix to indicate the unit (see https://golang.org/pkg/time/#ParseDuration), e.g. \n10ms\n2s\n2h45m",
		config.Shortflag('t'), config.Default("800ms"))

	sleepArg = args.NewString("sleep",
		"time between calls of the command, you need a suffix to indicate the unit (see https://golang.org/pkg/time/#ParseDuration), e.g. \n10ms\n2s\n2h45m",
		config.Default("1000ms"))

	bufSizeArg = args.NewInt32("bufsize", "the size of the message buffer for changed files and changed directories",
		config.Default(int32(runcommand.DefaultBufSize)),
	)
)

func main() {

	var (
		// define the variables here that are shared along the steps
		// most variables should only by defined by the type here
		// and are assigned inside the steps
		err     = args.Run()
		stopper runcommand.Stoppable
		dir     string
		match   *regexp.Regexp
		ignore  *regexp.Regexp
		timeout time.Duration
		sleep   time.Duration
		errors  chan error
	)

steps:
	for jump := 1; err == nil; jump++ {
		switch jump - 1 {
		default:
			break steps
		// count a number up for each following step
		case 0:
			dir = dirArg.Get()
			if dir == "." {
				dir, err = os.Getwd()
			}
		case 1:
			dir, err = filepath.Abs(dir)
		case 2:
			timeout, err = time.ParseDuration(timeoutArg.Get())
		case 3:
			sleep, err = time.ParseDuration(sleepArg.Get())
		case 4:
			switch m := matchArg.Get(); m {
			case "", "*":
			default:
				if strings.ContainsRune(m, filepath.Separator) {
					err = fmt.Errorf("argument -match must not contain path separator %v", filepath.Separator)
				} else {
					match, err = regexp.CompilePOSIX(m)
				}

			}
		case 5:
			switch i := ignoreArg.Get(); i {
			case "":
			default:
				if strings.ContainsRune(i, filepath.Separator) {
					err = fmt.Errorf("argument -ignore must not contain path separator %v", filepath.Separator)
				} else {
					ignore, err = regexp.CompilePOSIX(i)
				}
			}
		case 6:
			rc := runcommand.New(dir, cmdArg.Get(),
				runcommand.BufSize(int(bufSizeArg.Get())),
				runcommand.Ignore(ignore),
				runcommand.MatchFiles(match),
				runcommand.Sleep(sleep),
				runcommand.Stdout(os.Stdout),
				runcommand.Stderr(os.Stderr),
			)

			errors = make(chan error, 1)
			stopper, err = rc.Run(errors)
		case 7:
			var (
				stopped      bool
				stoppedMutex sync.RWMutex
				finished     = make(chan bool, 1)
				c            = make(chan os.Signal, 1)
			)

			// signal the execution of the observer to stop and exit after any running process is finished
			// if interrupt CTRL+C is pressed for the second time, any running process is
			// killed
			signal.Notify(c, os.Interrupt)
			signal.Notify(c, syscall.SIGTERM)
			go func() {
				for {
					select {
					case <-c:
						fmt.Fprintf(os.Stderr, "\ninterupted, waiting for process to finish...")
						stoppedMutex.RLock()
						st := stopped
						stoppedMutex.RUnlock()
						if st {
							stopper.Kill()
							fmt.Fprintf(os.Stderr, "\nforced killing...")
							finished <- true
						} else {
							stoppedMutex.Lock()
							stopped = true
							stoppedMutex.Unlock()
							errTerm := stopper.Terminate(timeout)
							if errTerm != nil {
								stopper.Kill()
							}
							finished <- true
						}
					case e := <-errors:
						fmt.Fprintf(os.Stderr, "Error(%T): %s", e, e)
					}
				}
			}()

			<-finished
			fmt.Fprintf(os.Stderr, "done\n")
			os.Exit(0)
		}
	}

	// use err here
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error(%T): %s\n", err, err)
	}

}

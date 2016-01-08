package main

import (
	"fmt"
	"github.com/metakeule/observe"
	"github.com/metakeule/observe/shellcmd2"
	"github.com/metakeule/observe/watcher"
	"gopkg.in/fsnotify.v1"
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

	/*
		skipArg = args.NewBool("skip",
			"skip next execution, if the command is still running, default is to delay the execution",
			config.Shortflag('s'))
	*/
	verboseArg = args.NewBool("verbose",
		"show the command that is being run",
		config.Shortflag('v'), config.Default(true))

	/*
		noCreatedArg = args.NewBool("icreated",
			"ignore created files",
			config.Default(true))

		noUpdatedArg = args.NewBool("iupdated",
			"ignore updated files",
			config.Default(false))
		noRemovedArg = args.NewBool("iremoved",
			"ignore removed files",
			config.Default(false))
	*/

	timeoutArg = args.NewString("timeout",
		"timeout for command termination when pressing CTRL+c once, you need a suffix to indicate the unit (see https://golang.org/pkg/time/#ParseDuration), e.g. \n10ms\n2s\n2h45m",
		config.Shortflag('t'), config.Default("800ms"))
	sleepArg = args.NewString("sleep",
		"time between calls of the command, you need a suffix to indicate the unit (see https://golang.org/pkg/time/#ParseDuration), e.g. \n10ms\n2s\n2h45m",
		config.Default("1000ms"))
)

func main() {

	var (
		// define the variables here that are shared along the steps
		// most variables should only by defined by the type here
		// and are assigned inside the steps
		err         = args.Run()
		obs         *observe.Observe
		watch       *watcher.Watcher
		w           *fsnotify.Watcher
		dir         string
		match       *regexp.Regexp
		ignore      *regexp.Regexp
		filechanged chan string
		errors      chan string
		timeout     time.Duration
		sleep       time.Duration
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
			//obs, err = observe.New(&shellcmd.ShellCMD{Command: cmdArg.Get(), Verbose: verboseArg.Get()})
			proc := shellcmd2.NewShellProcess(dir, cmdArg.Get(), os.Stdout, os.Stderr, errors, sleep)
			obs, err = observe.New(proc, timeout)
		case 5:
			// obs.Skip = skipArg.Get()
			switch m := matchArg.Get(); m {
			case "", "*":
			default:
				if strings.ContainsRune(m, filepath.Separator) {
					err = fmt.Errorf("argument -match must not contain path separator %v", filepath.Separator)
				} else {
					match, err = regexp.CompilePOSIX(m)
				}

			}
		case 6:
			switch i := ignoreArg.Get(); i {
			case "":
			default:
				if strings.ContainsRune(i, filepath.Separator) {
					err = fmt.Errorf("argument -ignore must not contain path separator %v", filepath.Separator)
				} else {
					ignore, err = regexp.CompilePOSIX(i)
				}
			}
		case 7:
			w, err = fsnotify.NewWatcher()
		case 8:
			watch, err = watcher.New(w, dir, match, ignore)
		case 9:
			//	watch.Recursive = recursiveArg.Get()
			//	watch.NoCreated = noCreatedArg.Get()
			//	watch.NoRemoved = noRemovedArg.Get()
			//	watch.NoUpdated = noUpdatedArg.Get()
			errors = make(chan string, 1)
			filechanged, err = watch.Start(errors)
		case 10:
			printers := make(chan string, 1)

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			signal.Notify(c, syscall.SIGTERM)
			//signal.Notify(c, syscall.SIGHUP)
			finished := make(chan bool, 1)
			//runFinished, stop, kill := obs.Start(printers, errors, finished, filechanged)
			// TODO: signal the execution of the observer to stop and exit after any running process is finished
			// if interrupt CTRL+C is pressed for the second time, any running process is
			// killed
			var stopped bool
			var stoppedMutex sync.RWMutex
			// var stop, kill chan bool
			stop, kill := obs.Start(filechanged, finished)

			go func() {

				for {
					select {
					// case <-finished:
					// fmt.Fprintf(os.Stdout, "done\n")
					// os.Exit(0)
					case <-c:
						fmt.Fprintf(os.Stdout, "interupted, waiting for process to finish\n")
						stoppedMutex.RLock()
						st := stopped
						stoppedMutex.RUnlock()
						if st {
							// println("running kill")
							kill <- true
							fmt.Fprintf(os.Stdout, "forced killing\n")
						} else {
							stoppedMutex.Lock()
							stopped = true
							stoppedMutex.Unlock()
							// println("running stop")
							stop <- true // timeout
						}
					case m := <-printers:
						fmt.Fprintf(os.Stdout, "%s", m)
					case e := <-errors:
						fmt.Fprintf(os.Stderr, "%s", e)
						// case <-runFinished:
						// printers <- fmt.Sprintf("\n--8<--8<--8<--8<--\n")
						// default:
					}

				}
			}()

			<-finished
			// fmt.Fprintf(os.Stdout, "done\n")
			os.Exit(0)
		}
	}

	// use err here
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}

}

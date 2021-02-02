package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"gopkg.in/fsnotify.v1"
)

type Watcher struct {
	w     *fsnotify.Watcher
	match *regexp.Regexp
	dir   string
	sync.Mutex
	ignore  *regexp.Regexp
	verbose bool
}

// New creates a watcher, watching all files and directories inside dir (recursively)
// If matchFiles is not nil, only the files matching matchFiles are respected.
// If ignore is not nil, files and directories matching ignore are ignored.
// An error is returned if the watcher could not be properly initialized.
func New(dir string, matchFiles, ignore *regexp.Regexp, verbose bool) (w *Watcher, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return
	}

	w = &Watcher{
		match:   matchFiles,
		dir:     dir,
		ignore:  ignore,
		verbose: verbose,
	}
	w.w, err = fsnotify.NewWatcher()

	if err != nil {
		return nil, err
	}

	err = filepath.Walk(w.dir, w.walk)

	if err != nil {
		return nil, err
	}

	return
}

func (w *Watcher) shouldIgnore(name string) bool {
	if w.ignore == nil {
		return false
	}

	return w.ignore.MatchString(name)
}

func (w *Watcher) fileMatch(name string) bool {
	if w.match == nil {
		return true
	}

	return w.match.MatchString(name)
}

func (w *Watcher) walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if w.shouldIgnore(info.Name()) {
		if w.verbose {
			fmt.Printf("ignoring: %#v\n", info.Name())
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}

	if !info.IsDir() && !w.fileMatch(info.Name()) {
		if w.verbose {
			fmt.Printf("ignoring: %#v\n", info.Name())
		}
		return nil
	}

	if w.verbose {
		fmt.Printf("adding: %#v\n", path)
	}

	return w.w.Add(path)
}

// Run runs the watching loop, reporting any errors to the errors channel, file modification and
// creation to the filechanged channel and file deletion and file renaming to the dirchanged channel
// for removal and renamed files, an empty filename is added to filechanged, since the command could
// not do anything meaningful with the missing file.
func (w *Watcher) Run(filechanged chan<- string, errors chan<- error) {

	go func() {
		for {
			select {
			case ev := <-w.w.Events:
				if ev.Op&fsnotify.Create == fsnotify.Create {
					if w.verbose {
						fmt.Printf("fsnotify.Create: %#v\n", ev.Name)
					}
					w.Lock()
					n := ev.Name
					d, err := os.Stat(n)
					if err != nil {
						w.Unlock()
						go func(e error) {
							errors <- e
						}(err)
						continue
					}
					nm := d.Name()
					if w.shouldIgnore(nm) {
						if w.verbose {
							fmt.Printf("ignoring: %#v\n", ev.Name)
						}
						w.Unlock()
						continue
					}
					isDir := d.IsDir()
					if !isDir && !w.fileMatch(nm) {
						if w.verbose {
							fmt.Printf("not matching: %#v\n", ev.Name)
						}
						w.Unlock()
						continue
					}
					if w.verbose {
						fmt.Printf("adding matching: %#v\n", ev.Name)
					}
					if err := w.w.Add(n); err != nil {
						if w.verbose {
							fmt.Printf("error while adding: %v\n", err.Error())
						}
						go func(e error) {
							errors <- e
						}(err)
					}
					w.Unlock()
					if !isDir {
						if w.verbose {
							fmt.Printf("file changed: %v\n", n)
						}
						go func(nn string) {
							filechanged <- nn
						}(n)
					}

				}

				// we should not need to handle match and ignores here, since the corresponding
				// files and dirs should not have been tracked/added in the first place
				// but experience shows that it is not the case
				if ev.Op&fsnotify.Write == fsnotify.Write {
					if w.verbose {
						fmt.Printf("fsnotify.Write: %v\n", ev.Name)
					}
					w.Lock()
					n := ev.Name
					d, err := os.Stat(n)
					if err != nil {
						w.Unlock()
						go func(e error) {
							errors <- e
						}(err)
						continue
					}
					nm := d.Name()
					if w.shouldIgnore(nm) {
						if w.verbose {
							fmt.Printf("ignoring: %v\n", ev.Name)
						}
						w.Unlock()
						continue
					}
					isDir := d.IsDir()
					if !isDir && !w.fileMatch(nm) {
						if w.verbose {
							fmt.Printf("not matching: %v\n", ev.Name)
						}
						w.Unlock()
						continue
					}

					if w.verbose {
						fmt.Printf("adding: %v\n", ev.Name)
					}
					if err := w.w.Add(n); err != nil {
						if w.verbose {
							fmt.Printf("error while adding: %v\n", err.Error())
						}
						go func(e error) {
							errors <- e
						}(err)
					}
					w.Unlock()
					if !isDir {
						if w.verbose {
							fmt.Printf("changed file: %v\n", n)
						}
						go func(nn string) {
							filechanged <- nn
						}(n)
					}
				}

				if ev.Op&fsnotify.Rename == fsnotify.Rename {
					if w.verbose {
						fmt.Printf("fsnotify.Rename: %v\n", ev.Name)
					}
					go func() {
						filechanged <- ""
					}()
				}

				if ev.Op&fsnotify.Remove == fsnotify.Remove {
					if w.verbose {
						fmt.Printf("fsnotify.Remove: %v\n", ev.Name)
					}
					go func() {
						filechanged <- ""
					}()
				}
			case err := <-w.w.Errors:
				if w.verbose {
					fmt.Printf("err: %v\n", err.Error())
				}
				go func(e error) {
					errors <- e
				}(err)
			}
		}
	}()

	return

}

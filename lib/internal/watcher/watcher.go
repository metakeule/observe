package watcher

import (
	"gopkg.in/fsnotify.v1"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

type Watcher struct {
	w     *fsnotify.Watcher
	match *regexp.Regexp
	dir   string
	sync.Mutex
	ignore *regexp.Regexp
}

// New creates a watcher, watching all files and directories inside dir (recursively)
// If matchFiles is not nil, only the files matching matchFiles are respected.
// If ignore is not nil, files and directories matching ignore are ignored.
// An error is returned if the watcher could not be properly initialized.
func New(dir string, matchFiles, ignore *regexp.Regexp) (w *Watcher, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return
	}

	w = &Watcher{
		match:  matchFiles,
		dir:    dir,
		ignore: ignore,
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
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}

	if !info.IsDir() && !w.fileMatch(info.Name()) {
		return nil
	}

	return w.w.Add(path)
}

// Run runs the watching loop, reporting any errors to the errors channel, file modification and
// creation to the filechanged channel and file deletion and file renaming to the dirchanged channel
func (w *Watcher) Run(filechanged chan<- string, dirchanged chan<- bool, errors chan<- error) {

	go func() {
		for {
			select {
			case ev := <-w.w.Events:
				if ev.Op&fsnotify.Create == fsnotify.Create {
					w.Lock()
					n := ev.Name
					d, err := os.Stat(n)
					if err != nil {
						w.Unlock()
						go func(e error) {
							errors <- e
						}(err)
					} else {
						nm := d.Name()
						if w.shouldIgnore(nm) {
							w.Unlock()
						} else {
							isDir := d.IsDir()
							if !isDir && w.fileMatch(nm) {
								w.Unlock()
							} else {
								if err := w.w.Add(n); err != nil {
									go func(e error) {
										errors <- e
									}(err)
								}
								w.Unlock()
								if !isDir {
									go func(nn string) {
										filechanged <- nn
									}(n)
								}
							}
						}
					}
				}

				// we should not need to handle match and ignores here, since the corresponding
				// files and dirs should not have been tracked/added in the first place
				if ev.Op&fsnotify.Write == fsnotify.Write {
					go func(n string) {
						filechanged <- n
					}(ev.Name)
				}

				if ev.Op&fsnotify.Rename == fsnotify.Rename {
					go func() {
						dirchanged <- true
					}()
				}

				if ev.Op&fsnotify.Remove == fsnotify.Remove {
					go func() {
						dirchanged <- true
					}()
				}
			case err := <-w.w.Errors:
				go func(e error) {
					errors <- e
				}(err)
			}
		}
	}()

	return

}

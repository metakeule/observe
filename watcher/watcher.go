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

func New(fsw *fsnotify.Watcher, dir string, match, ignore *regexp.Regexp) (w *Watcher, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return
	}

	w = &Watcher{
		w:      fsw,
		match:  match,
		dir:    dir,
		ignore: ignore,
	}

	return
}

func (w *Watcher) Walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if (w.match != nil && !w.match.MatchString(info.Name())) || (w.ignore != nil && w.ignore.MatchString(info.Name())) {
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	return w.w.Add(path)
}

func (w *Watcher) Start(errors chan string) (filechanged chan string, err error) {

	filechanged = make(chan string, 100)
	//filechanged = make(chan string)
	err = filepath.Walk(w.dir, w.Walk)

	if err == nil {

		go func() {
			for {
				select {
				case ev := <-w.w.Events:
					//log.Println("event: (create:%v)", ev, ev.IsCreate())

					if ev.Op&fsnotify.Create == fsnotify.Create {
						println("created " + ev.Name)
						w.Lock()
						d, err := os.Stat(ev.Name)
						if err == nil {
							// w.Unlock()
							// println("locked ")
							println("add " + ev.Name)
							if err := w.w.Add(ev.Name); err != nil {
								// w.Unlock()
								errors <- err.Error()
							} else {
								println("added " + ev.Name)
							}

							// println("unlocked ")
							if !d.IsDir() {
								filechanged <- ev.Name
							}
							w.Unlock()
						} else {
							errors <- err.Error()
						}
					}

					if ev.Op&fsnotify.Remove == fsnotify.Remove {
						// println("removed " + ev.Name)
						// w.Lock()
						w.w.Remove(ev.Name)
						// w.Unlock()
					}

					if ev.Op&fsnotify.Write == fsnotify.Write {
						// println("written " + ev.Name)
						filechanged <- ev.Name
					}
					/*
						if ev.Op&fsnotify.Rename == fsnotify.Rename {
						}
					*/

				case err := <-w.w.Errors:
					errors <- err.Error()
					//ø.Notifier.Error("watcher error: " + err.Error())
					// log.Println("watcher error:", err)
				default:
				}
			}
		}()
	}
	return

}

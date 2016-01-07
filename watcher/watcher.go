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

	// if info.IsDir() {
	return w.w.Add(path)
	// }
	// return nil
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
						w.Lock()
						n := ev.Name
						// println("created " + n)
						d, err := os.Stat(n)
						if err != nil {
							// println("error " + err.Error())
							w.Unlock()
							go func(e string) {
								errors <- e
							}(err.Error())
						} else {
							nm := d.Name()
							if (w.match == nil || w.match.MatchString(nm)) && (w.ignore == nil || !w.ignore.MatchString(nm)) {
								isDir := d.IsDir()
								if err := w.w.Add(n); err != nil {
									go func(e string) {
										errors <- e
									}(err.Error())
								}
								w.Unlock()
								if !isDir {
									go func(nn string) {
										// println("changed " + nn)
										filechanged <- nn
									}(n)
								}
							} else {
								w.Unlock()
							}
						}
					}

					/*
						if ev.Op&fsnotify.Remove == fsnotify.Remove {
							// println("removed " + ev.Name)
							// w.Lock()
							// w.w.Remove(ev.Name)
							// w.Unlock()
						}
					*/
					if ev.Op&fsnotify.Write == fsnotify.Write {
						// println("written " + ev.Name)

						go func(n string) {
							// println("written " + n)
							filechanged <- n
						}(ev.Name)

					}
					/*
						if ev.Op&fsnotify.Rename == fsnotify.Rename {
						}
					*/

				case err := <-w.w.Errors:

					go func(e string) {
						println("error " + e)
						errors <- e
					}(err.Error())

					//ø.Notifier.Error("watcher error: " + err.Error())
					// log.Println("watcher error:", err)
					// default:
				}
			}
		}()
	}
	return

}

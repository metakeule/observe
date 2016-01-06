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

	filechanged = make(chan string)
	err = filepath.Walk(w.dir, w.Walk)

	if err == nil {

		go func() {
			for {
				select {
				case ev := <-w.w.Events:
					//log.Println("event: (create:%v)", ev, ev.IsCreate())

					if ev.Op&fsnotify.Create == fsnotify.Create {
						// println("created " + ev.Name)
						// d, err := os.Stat(ev.Name)
						// if err == nil {
						// if d.IsDir() {
						w.Lock()
						err := w.w.Add(ev.Name)
						w.Unlock()
						if err != nil {
							errors <- err.Error()
						} else {
							filechanged <- ev.Name
						}
						// } else {
						// }
						// }
					}

					if ev.Op&fsnotify.Remove == fsnotify.Remove {
						// println("removed " + ev.Name)
						w.Lock()
						w.w.Remove(ev.Name)
						w.Unlock()
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

// watch, err = watcher.New(w, dir, match, ignore)

/*
package watcher

import (
	"gopkg.in/fsnotify.v1"
	// "log"
	"os"
	"sync"
	"time"
)

type ProjectWatcher struct {
	*sync.Mutex
	Compilers []Compiler
	Watcher   *fsnotify.Watcher
	Notifier  Notifier
	Pool      map[Compiler]string
	Frequency time.Duration
	Ready     chan int
}

func New(notifier Notifier, compilers ...Compiler) (ø *ProjectWatcher) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic("can't create watcher: " + err.Error())
		// log.Fatalf("can't create watcher: %s\n", err.Error())
	}
	return &ProjectWatcher{
		Mutex:     &sync.Mutex{},
		Compilers: compilers,
		Watcher:   watcher,
		Ready:     make(chan int, 1),
		Notifier:  notifier,
		Frequency: time.Millisecond * 20,
		// each compiler should be handled at one time with the last file succeeding
		Pool: map[Compiler]string{},
	}
}

func (ø *ProjectWatcher) SendMessages() {
	for {
		ø.Lock()
		for comp, file := range ø.Pool {
			// log.Printf("handle %s with %s\n", file, comp.Name())
			out, err := comp.Compile(file)
			if err != nil {
				ø.Notifier.Error(out)
			} else {
				ø.Notifier.Success("compiled " + file)
			}
			delete(ø.Pool, comp)
		}
		ø.Unlock()
		time.Sleep(ø.Frequency)
	}
}

func (ø *ProjectWatcher) HandleFile(path string) {
	// log.Printf("trying to handle: %s\n", path)
	for _, c := range ø.Compilers {
		c.Lock()
		if c.Affected(path) {
			ø.Pool[c] = path

		}
		c.Unlock()
	}
}

func (ø *ProjectWatcher) Run() (err error) {
	for _, c := range ø.Compilers {
		for _, d := range c.Dirs() {
			err = ø.Watcher.Add(d)
			if err != nil {
				return
			}
		}
	}

	go ø.SendMessages()

	go func() {
		for {
			select {
			case ev := <-ø.Watcher.Events:
				//log.Println("event: (create:%v)", ev, ev.IsCreate())

				what := ""
				handleIt := true

				if ev.Op&fsnotify.Create == fsnotify.Create {

					what = "created"
					d, err := os.Stat(ev.Name)
					if err == nil {
						if d.IsDir() {
							ø.Lock()
							//ø.Notifier.Success("added " + ev.Name + "- start watching")
							// log.Println("added ", ev.Name, "- start watching")
							ø.Watcher.Add(ev.Name)
							ø.Unlock()
						}
					}
					handleIt = true
				}
				if ev.Op&fsnotify.Remove == fsnotify.Remove {
					handleIt = false
					what = "deleted"
					ø.Lock()
					ø.Watcher.Remove(ev.Name)
					ø.Unlock()
				}
				if ev.Op&fsnotify.Write == fsnotify.Write {
					// case ev.IsModify():
					what = "modified"
				}
				if ev.Op&fsnotify.Rename == fsnotify.Rename {
					// case ev.IsRename():
					handleIt = false
					what = "renamed"
				}
				_ = what
				// log.Println("file: ", ev.Name, " ", what)

				if handleIt {
					ø.Lock()
					ø.HandleFile(ev.Name)
					ø.Unlock()
				}

			case err := <-ø.Watcher.Errors:
				ø.Notifier.Error("watcher error: " + err.Error())
				// log.Println("watcher error:", err)
			}
		}
		ø.Lock()
		ø.Ready <- 1
		ø.Unlock()
	}()
	return
}

/*
   Flags
       FSN_CREATE = 1
       FSN_MODIFY = 2
       FSN_DELETE = 4
       FSN_RENAME = 8

       FSN_ALL = FSN_MODIFY | FSN_DELETE | FSN_RENAME | FSN_CREATE
*/
//watcher.WatchFlags(ev.Name string, flags uint32) error {

// watcher.RemoveWatch(path string)

//watcher.Close()

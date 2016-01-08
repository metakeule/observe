package watcher

import (
	"fmt"
	"gopkg.in/fsnotify.v1"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var testdir string

func init() {
	// println("init called")
	var wd, err = os.Getwd()
	if err != nil {
		panic(err.Error())
	}

	wd, err = filepath.Abs(wd)

	if err != nil {
		panic(err.Error())
	}

	testdir = filepath.Join(wd, "testdir")

	os.RemoveAll(testdir)

	if err := os.Mkdir(testdir, 0755); err != nil {
		panic(err.Error())
	}
}

func TestCreate(t *testing.T) {
	w, errW := fsnotify.NewWatcher()
	if errW != nil {
		t.Fatal(errW)
	}
	wa, err := New(w, testdir, nil, nil)

	if err != nil {
		t.Fatal(err)
	}

	errors := make(chan string, 10)
	changed, err2 := wa.Start(errors)

	if err2 != nil {
		t.Fatal(err2)
	}

	// var ii = int64(0)
	// var i *int64 = &ii

	f1name := filepath.Join(testdir, "created.txt")
	subname := filepath.Join(testdir, "sub")
	f2name := filepath.Join(subname, "subcreated.txt")
	subsubname := filepath.Join(subname, "subsub")
	f3name := filepath.Join(subsubname, "subsubcreated.txt")
	expected := map[string]bool{
		f1name: true,
		f2name: true,
		f3name: true,
	}

	finished := make(chan bool)

	m := sync.Mutex{}

	_ = fmt.Printf

	go func() {
		for {
			select {
			case e := <-errors:
				t.Fatal("Error " + e)
				finished <- true

			case c := <-changed:
				// fmt.Printf("got: %#v\n", c)
				var l int
				m.Lock()
				_, isExpected := expected[c]
				m.Unlock()
				// fmt.Printf("expected: %v\n", isExpected)
				if isExpected {
					m.Lock()
					delete(expected, c)
					l = len(expected)
					m.Unlock()
					// fmt.Printf("length: %d\n", l)
					if l == 0 {
						// fmt.Println("finished")
						finished <- true
					}
				} else {
					// t.Fatalf("unexpected file %#v", c)
					finished <- true
				}
			}
		}
	}()

	// time.Sleep(time.Duration(400000))

	// fmt.Printf("creating: %#v\n", f1name)
	f, errf := os.Create(f1name)
	if errf != nil {
		t.Fatal(errf)
	}
	f.Close()

	// time.Sleep(time.Duration(400000))

	// fmt.Printf("creating: %#v\n", subname)
	if err := os.Mkdir(subname, 0755); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 40)
	// fmt.Printf("creating: %#v\n", f2name)
	f, errf = os.Create(f2name)
	if errf != nil {
		t.Fatal(errf)
	}
	f.Close()
	// time.Sleep(time.Millisecond * 40)

	// fmt.Printf("creating: %#v\n", subsubname)
	if err := os.Mkdir(subsubname, 0755); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 40)
	// fmt.Printf("creating: %#v\n", f3name)
	f, errf = os.Create(f3name)
	if errf != nil {
		t.Fatal(errf)
	}
	f.Close()

	// time.Sleep(time.Duration(100))
	<-finished
}

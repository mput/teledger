package repo

import (
	"fmt"
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)


type Mock struct {
	Files map[string]string
	fs billy.Filesystem
	inited bool
}

func (r *Mock) Init() error {
	if r.inited {
		return fmt.Errorf("already initialized")
	}
	r.fs = memfs.New()
	for fname, content := range r.Files {
		f, err := r.fs.Create(fname)
		if err != nil {
			return err
		}
		_, err = f.Write([]byte(content))
		if err != nil {
			return err
		}

		err = f.Close()

		if err != nil {
			return err
		}
	}
	r.inited = true
	return nil
}

func (r *Mock) Free() {
	if !r.inited {
		panic("not initialized")
	}
	r.inited = false
}

func (r *Mock) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	if !r.inited {
		return nil, fmt.Errorf("not initialized")
	}
	return r.fs.OpenFile(filename, flag, perm)
}


func (r *Mock) Open(filename string) (billy.File, error) {
	return r.OpenFile(filename, os.O_RDONLY, 0)
}

func (r *Mock) OpenForAppend(filename string) (billy.File, error) {
	return r.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
}


func (r *Mock) CommitPush(_, _, _ string) error {
	for fname, _ := range r.Files {
		f, err := r.fs.Open(fname)
		if err != nil {
			return err
		}
		fc, err := io.ReadAll(f)

		if err != nil {
			return err
		}

		r.Files[fname] = string(fc)

		err = f.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

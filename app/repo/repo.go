package repo

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

type Service interface {
	// Pull fresh data from the remote repository
	// and takes lock for the length of the operation
	Init() error
	// Release the lock and free resources
	Free()

	OpenFile(file string, flag int, perm os.FileMode) (billy.File, error)
	Open(file string) (billy.File, error)
	OpenForAppend(file string) (billy.File, error)
	CommitPush(msg, name, email string) error
}

type InMemoryRepo struct {
	url        string
	token      string
	repo       *git.Repository
	dirtyFiles map[string]bool
	inited     bool
	initedMu   sync.Mutex
}

func NewInMemoryRepo(url, token string) *InMemoryRepo {
	return &InMemoryRepo{
		url:    url,
		token:  token,
		inited: false,
	}
}

func (imr *InMemoryRepo) Init() error {
	imr.initedMu.Lock()
	fs := memfs.New()
	r, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL: imr.url,
		Auth: &http.BasicAuth{
			Username: "username",
			Password: imr.token,
		},
		Depth: 1,
	})
	if err != nil {
		return fmt.Errorf("init error, unable to clone %s: %v", imr.url, err)
	}

	ref, err := r.Head()
	if err != nil {
		return fmt.Errorf("init error: %v", err)
	}
	slog.Debug("repo cloned", "head", ref.Hash(), "url", imr.url)

	imr.repo = r
	imr.dirtyFiles = make(map[string]bool)
	imr.inited = true
	return nil
}

func (imr *InMemoryRepo) Free() {
	imr.inited = false
	imr.repo = nil
	imr.initedMu.Unlock()
}

func (imr *InMemoryRepo) Open(filename string) (billy.File, error) {
	return imr.OpenFile(filename, os.O_RDONLY, 0)
}

func (imr *InMemoryRepo) OpenForAppend(filename string) (billy.File, error) {
	return imr.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0)
}

type Closer struct {
	billy.File
	r    *InMemoryRepo
	path string
}

func (w *Closer) Close() error {
	w.r.dirtyFiles[w.path] = true
	return w.File.Close()
}

func (imr *InMemoryRepo) OpenFile(file string, flag int, perm os.FileMode) (billy.File, error) {
	if !imr.inited {
		return nil, fmt.Errorf("not initialized")
	}
	wtr, err := imr.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree receiving error: %v", err)
	}
	f, err := wtr.Filesystem.OpenFile(file, flag, perm)
	wc := Closer{
		r:    imr,
		path: file,
		File: f,
	}
	return &wc, err
}

func (imr *InMemoryRepo) CommitPush(msg, name, email string) error {
	if !imr.inited {
		return fmt.Errorf("not initialized")
	}
	wtr, err := imr.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree receiving error: %v", err)
	}

	for file, dirty := range imr.dirtyFiles {
		if dirty {
			_, err = wtr.Add(file)
			if err != nil {
				return fmt.Errorf("error while adding to worktree: %v", err)
			}
		}
	}
	_, err = wtr.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  name,
			Email: email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("error while committing: %v", err)
	}
	err = imr.repo.Push(&git.PushOptions{
		Auth: &http.BasicAuth{
			Username: "username",
			Password: imr.token,
		},
	})
	if err != nil {
		return fmt.Errorf("error while pushing: %v", err)
	}

	return nil
}

func (imr *InMemoryRepo) resetPush(hash plumbing.Hash) error {
	wtr, err := imr.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree receiving error: %v", err)
	}

	err = wtr.Reset(&git.ResetOptions{
		Commit: hash,
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("error while resetting: %v", err)
	}

	err = imr.repo.Push(&git.PushOptions{
		ForceWithLease: &git.ForceWithLease{},
		Auth: &http.BasicAuth{
			Username: "username",
			Password: imr.token,
		},
	})
	if err != nil {
		return fmt.Errorf("error while pushing reset: %v", err)
	}

	return nil
}

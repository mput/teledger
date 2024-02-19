package repo

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)


type Service interface {
	Open(file string) (io.ReadCloser, error)
	OpenForAppend(file string) (io.WriteCloser, error)
	OpenFile(file string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
	CommitPush(msg, name, email string) error
}

type InMemoryRepo struct {
	url    string
	token  string
	repo   *git.Repository
	dirtyFiles map[string]bool
}

func NewInMemoryRepo(url, token string) (*InMemoryRepo, error) {
	fs := memfs.New()

	r, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL: url,
		Auth: &http.BasicAuth{
			Username: "username",
			Password: token,
		},
		Depth: 1})

	if err != nil {
		return nil, fmt.Errorf("unable to clone %s: %v", url, err)
	}

	ref, err := r.Head()
	if err != nil {
		return nil, err
	}

	slog.Debug("repo cloned", "head", ref.Hash(), "url", url)

	return &InMemoryRepo{
		url:    url,
		token:  token,
		repo:   r,
		dirtyFiles: make(map[string]bool),
	}, nil
}

func (r *InMemoryRepo) Open(file string) (io.ReadCloser, error) {
	wtr, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree receiving error: %v", err)
	}
	return wtr.Filesystem.Open(file)
}

type WriteCloser struct {
	f *billy.File
	r *InMemoryRepo
	path string
}

func (w *WriteCloser) Write(p []byte) (n int, err error) {
	return (*w.f).Write(p)
}

func (w *WriteCloser) Read(p []byte) (n int, err error) {
	return (*w.f).Read(p)
}

func (w *WriteCloser) Close() error {
	w.r.dirtyFiles[w.path] = true
	return (*w.f).Close()
}

func (r *InMemoryRepo) OpenFile(file string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	wtr, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("worktree receiving error: %v", err)
	}
	f, err := wtr.Filesystem.OpenFile(file, flag, perm)
	wc := WriteCloser{
		r: r,
		path: file,
		f: &f,
	}
	return &wc, err
}


func (r *InMemoryRepo) OpenForAppend(file string) (io.WriteCloser, error) {
	return r.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o666)
}


func (r *InMemoryRepo) CommitPush(msg, name, email string) error {
	wtr, err := r.repo.Worktree()

	if err != nil {
		return fmt.Errorf("worktree receiving error: %v", err)
	}

	for file, dirty := range r.dirtyFiles {
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
	err = r.repo.Push(&git.PushOptions{
		Auth: &http.BasicAuth{
			Username: "username",
			Password: r.token,
		},
	})

	if err != nil {
		return fmt.Errorf("error while pushing: %v", err)
	}

	return nil
}

func (r *InMemoryRepo) resetPush(hash plumbing.Hash) error {
	wtr, err := r.repo.Worktree()

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

	err = r.repo.Push(&git.PushOptions{
		ForceWithLease: &git.ForceWithLease{},
		Auth: &http.BasicAuth{
			Username: "username",
			Password: r.token,
		},
	})

	if err != nil {
		return fmt.Errorf("error while pushing reset: %v", err)
	}

	return nil
}

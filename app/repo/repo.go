package repo

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)


type RepoService interface {
	OpenReader(file string) (io.ReadCloser, error)
	OpenWriter(file string) (io.WriteCloser, error)
	// Commit(msg string) error

}

type InMemoryRepo struct {
	url    string
	token  string
	fs     *billy.Filesystem
	repo   *git.Repository
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
	// git.Repository
	ref, err := r.Head()
	if err != nil {
		return nil, err
	}

	slog.Info("repo clonned", "head", ref.Hash())

	return &InMemoryRepo{
		url:    url,
		token:  token,
		fs:     &fs,
		repo:   r,
	}, nil
}

func (r *InMemoryRepo) OpenReader(file string) (io.ReadCloser, error) {
	return (*r.fs).Open(file)
}

func (r *InMemoryRepo) OpenWriter(file string) (io.WriteCloser, error) {
	return (*r.fs).Create(file)
}

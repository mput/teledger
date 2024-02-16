package repo

import (
	"fmt"
	"io"
	"strings"
)


type RepoMock struct {
	Files map[string]string
}

func (r *RepoMock) OpenReader(file string) (io.ReadCloser, error) {
	if content, ok := r.Files[file]; ok {
		return io.NopCloser(strings.NewReader(content)), nil
	}
	return nil, fmt.Errorf("file not found")
}

func (r *RepoMock) OpenWriter(_ string) (io.WriteCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

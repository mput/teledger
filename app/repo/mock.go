package repo

import (
	"fmt"
	"io"
	"os"
	"strings"
)


type Mock struct {
	Files map[string]string
}

func (r *Mock) Open(file string) (io.ReadCloser, error) {
	if content, ok := r.Files[file]; ok {
		return io.NopCloser(strings.NewReader(content)), nil
	}
	return nil, fmt.Errorf("file not found")
}

func (r *Mock) OpenFile(_ string, _ int, _ os.FileMode) (io.ReadWriteCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *Mock) OpenForAppend(_ string) (io.WriteCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *Mock) CommitPush(_, _, _ string) error {
	return nil
}

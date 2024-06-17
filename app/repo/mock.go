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

func (r *Mock) OpenForAppend(file string) (io.WriteCloser, error) {
	if content, ok := r.Files[file]; ok {
		return &WriteCloserT{r: r, f: file, dt: []byte(content)}, nil
	}
	return nil, fmt.Errorf("file not found")
}

type WriteCloserT struct {
	r *Mock
	f string
	dt []byte
}

func (w *WriteCloserT) Write(p []byte) (n int, err error) {
	w.dt = append(w.dt, p...)
	return len(p), nil
}

func (w *WriteCloserT) Close() error {
	w.r.Files[w.f] = string(w.dt)
	return nil
}

func (r *Mock) CommitPush(_, _, _ string) error {
	return nil
}

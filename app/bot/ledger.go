package bot

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/mput/teledger/app/repo"
)


type Ledger struct {
	repo repo.RepoService
	mainFile string
	strict bool
}

func NewLedger(rs repo.RepoService, mainFile string, strict bool) *Ledger {
	return &Ledger{
		repo: rs,
		mainFile: mainFile,
		strict: strict,
	}
}

const ledgerBinary = "ledger"

func resolveIncludesReader(rs repo.RepoService, file string) (io.ReadCloser, error) {
	ledgerFile, err := rs.OpenReader(file)
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()

	var f func(ledReader io.ReadCloser, top bool)
	f = func(ledReader io.ReadCloser, top bool) {
		defer ledReader.Close()
		if top {
			defer w.Close()
		}

		lcnt := 0
		scanner := bufio.NewScanner(ledReader)
		for scanner.Scan() {
			line := scanner.Text()
			lcnt++
			linetr := strings.TrimSpace(line)
			if strings.HasPrefix(linetr, "include") {
				path := strings.TrimPrefix(linetr, "include")
				path = strings.TrimSpace(path)
				ir, rerr := rs.OpenReader(path)
				if rerr == nil {
					f(ir, false)
					continue
				}
				// line will be used as is so ledger will report the error if it's actually an include
				slog.Warn("unable to open include file", "file", path, "error", rerr)
			}
			_, err := fmt.Fprintln(w, line)
			if err != nil {
				slog.Error("unable to write to pipe", "error", err)
				w.CloseWithError(fmt.Errorf("unable to write to pipe: %v", err))
				return
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Error("unable to read ledger file", "error", err)
			w.CloseWithError(fmt.Errorf("unable to read ledger file: %v", err))
			return
		}
		slog.Debug("finish ledger file read", "file", file, "top", top, "lines", lcnt)
	}

	go f(ledgerFile, true)

	return r, nil

}


func (l *Ledger) Execute(args ...string) (result string,err error) {
	r, err := resolveIncludesReader(l.repo, l.mainFile)

	if err != nil {
		return "", fmt.Errorf("ledger file opening error: %v", err)
	}

	fargs := []string{"-f", "-"}
	if l.strict {
		fargs = append(fargs, "--pedantic")
	}
	fargs = append(fargs, args...)

    cmddir, err := os.MkdirTemp("", "ledger")
	if err != nil {
		return "", fmt.Errorf("ledger temp dir creation error: %v", err)
	}
    defer os.RemoveAll(cmddir)

	cmd := exec.Command(ledgerBinary, fargs...)

	// For security reasons, we don't want to pass any environment variables to the ledger command
	cmd.Env = []string{}
	// Temp dir is only exists to not expose any existing directory to ledger command
	cmd.Dir = cmddir

	cmd.Stdin = r
	var out strings.Builder
	var errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err = cmd.Run()
	if err != nil {
		_, isExitError := err.(*exec.ExitError)
		if isExitError {
			return "", fmt.Errorf("ledger error: exited with status %s (%v)", err, errOut.String())
		}
		return "", fmt.Errorf("ledger command executing error: %v", err)
	}

	return out.String(), nil
}

package bot

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func cloneRepo(url, token, workdir string) error {
	log.Printf("[INFO] cloning %s..\n", url)

	_, err := git.PlainClone(workdir, false, &git.CloneOptions{
		URL: url,
		Auth: &http.BasicAuth{
			Username: "username",
			Password: token,
		},
		Depth: 2})

	if err != nil {
		return fmt.Errorf("unable to clone %s into %s: %v", url, workdir, err)
	}

	return nil
}

func FetchOrCloneRepo(url, token, workdir string) error {
	r, err := git.PlainOpen(workdir)

	if err == git.ErrRepositoryNotExists {
		return cloneRepo(url, token, workdir)
	}
	if err != nil {
		return fmt.Errorf("unable to ensure repo %s: %v", workdir, err)
	}

	log.Printf("[INFO] fetching repo %s..\n", url)
	err = r.Fetch(&git.FetchOptions{
		Auth: &http.BasicAuth{
			Username: "username",
			Password: token,
		},
		Depth: 2})

	if err == git.NoErrAlreadyUpToDate {
		return nil
	}

	if err != nil {
		return fmt.Errorf("unable to fetch %s: %v", workdir, err)
	}

	return nil
}

const ledgerBinary = "ledger"

func ExecLedgerCmd(url, token, workdir, file string, arg ...string) (string, error) {
	log.Printf("[INFO] executing ledger command: %s on file: %s\n", arg, file)

	err := FetchOrCloneRepo(url, token, workdir)

	if err != nil {
		return "", fmt.Errorf("unable to ensure repo: %v", err)
	}

	arg = append([]string{"-f", file, "--pedantic"}, arg...)

	cmd := exec.Command(ledgerBinary, arg...)
	cmd.Dir = workdir
	var out strings.Builder
	var errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("unable to run ledger command: %v", errOut.String())
	}
	return out.String(), nil
}

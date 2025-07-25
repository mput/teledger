package repo

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

func checkReadString(t *testing.T, repo Service, fp string) string {
	r, err := repo.Open(fp)
	if err != nil {
		t.Errorf("Unexpected error on file open: %v", err)
	}
	d, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Unexpected error on file read: %v", err)
	}
	return string(d)
}

func TestNewInMemoryRepo(t *testing.T) {
	_ = godotenv.Load("../../.env.dev")

	gitURL := os.Getenv("GITHUB_URL")
	if gitURL == "" {
		t.Fatal("GIT_URL is not set")
	}

	gitToken := os.Getenv("GITHUB_TOKEN")
	if gitToken == "" {
		t.Fatal("GIT_ACCESS_TOKEN is not set")
	}

	repo := NewInMemoryRepo(gitURL, gitToken)

	err := repo.Init()
	if err != nil {
		t.Fatalf("unable to init repo: %v", err)
	}

	t.Run("OpenReader", func(t *testing.T) {
		if checkReadString(t, repo, "main.ledger") == "" {
			t.Fatal("Unexpected empty file.")
		}

		r, _ := repo.Open("./main.ledger")
		d, _ := io.ReadAll(r)

		if len(d) == 0 {
			t.Fatal("Unexpected empty file for relative link.")
		}

		_, err = repo.Open("./nonexisting.ledger")

		if err == nil {
			t.Fatal("Error expected on nonexisting file open.")
		}
	})

	t.Run("Write And Commit File", func(t *testing.T) {
		w, err := repo.OpenForAppend("main.ledger")
		if err != nil {
			t.Fatal(err)
		}

		ref, err := repo.repo.Head()
		if err != nil {
			t.Fatal(err)
		}

		hash := ref.Hash()

		if err != nil {
			t.Fatal(err)
		}

		line := fmt.Sprintf(";; %s: line added by the Write File test", time.Now())
		fmt.Fprintf(w, "\n%s", line)

		require.False(t, repo.dirtyFiles["main.ledger"])
		w.Close()
		require.True(t, repo.dirtyFiles["main.ledger"])

		if !strings.HasSuffix(checkReadString(t, repo, "main.ledger"), line) {
			t.Fatal("Reader doesn't contains written string")
		}

		err = repo.CommitPush("test commit", "teledger", "teledger@github.io")
		if err != nil {
			t.Fatal(err)
		}

		t.Cleanup(func() {
			err = repo.resetPush(hash)
			repo.Free()
			if err != nil {
				t.Fatal(err)
			}
		})

		newRepo := NewInMemoryRepo(gitURL, gitToken)
		err = newRepo.Init()

		if !strings.HasSuffix(checkReadString(t, newRepo, "main.ledger"), line) {
			t.Fatal("Reader doesn't contains committed string")
		}
		newRepo.Free()
	})
}

package repo

import (
	"io"
	"os"
	"testing"

	"github.com/joho/godotenv"
)


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

	repo, err := NewInMemoryRepo(gitURL, gitToken)

	if err != nil {
		t.Fatalf("unable to init repo: %v", err)
	}

	t.Run("OpenReader", func(t *testing.T) {
		r, err := repo.OpenReader("main.ledger")

		if err != nil {
			t.Errorf("Unexpected error on file open: %v", err)
			return
		}

		d, err := io.ReadAll(r)

		if err != nil {
			t.Fatalf("Unexpected error on file read: %v", err)
		}

		if  len(d) == 0 {
			t.Fatal("Unexpected empty file.")
		}

		r, _ = repo.OpenReader("./main.ledger")
		d, _ = io.ReadAll(r)

		if  len(d) == 0 {
			t.Fatal("Unexpected empty file for relative link.")
		}


		_, err = repo.OpenReader("./nonexisting.ledger")

		if err == nil {
			t.Fatal("Error expected on nonexisting file open.")
		}
	})

}

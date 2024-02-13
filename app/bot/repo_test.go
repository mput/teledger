package bot

import (
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)



func TestCloneRepo(t *testing.T) {
	godotenv.Load("../../.env")
	tmpDir := t.TempDir()

	gitURL := os.Getenv("GITHUB_URL")
	if gitURL == "" {
		t.Errorf("GIT_URL is not set")
		return
	}

	gitAccessToken := os.Getenv("GITHUB_TOKEN")
	if gitAccessToken == "" {
		t.Errorf("GIT_ACCESS_TOKEN is not set")
		return
	}

	t.Run("happy path", func(t *testing.T) {
		err := cloneRepo(gitURL, gitAccessToken, tmpDir)
		if err != nil {
			t.Errorf("Error: %v", err)
			return
		}
		_, err = os.Stat(tmpDir + "/main.ledger")
		if err != nil {
			t.Errorf("Can't access main ledger file: %v", err)
			return
		}
	})
}

func TestExecLedgerCmd(t *testing.T) {
	godotenv.Load("../../.env.test")
	tmpDir :=  t.TempDir()

	gitURL := os.Getenv("GITHUB_URL")
	if gitURL == "" {
		t.Errorf("GIT_URL is not set")
		return
	}

	gitToken := os.Getenv("GITHUB_TOKEN")
	if gitToken == "" {
		t.Errorf("GIT_ACCESS_TOKEN is not set")
		return
	}

	t.Run("happy path", func(t *testing.T) {

		res, err := ExecLedgerCmd(gitURL, gitToken, tmpDir, "main.ledger", "bal")
		if err != nil {
			t.Errorf("Command execution error: %v", err)
			return
		}

		expected := `
         1049,30 EUR  Assets
          949,30 EUR    Cards:Bank
          100,00 EUR    Cash:Main
        -1100,00 EUR  Equity:Opening-Balance
           50,70 EUR  Expenses
            4,00 EUR    Addictions:Cigarettes
           40,15 EUR    Food
           11,80 EUR      Coffee
           28,35 EUR      Eat-Out
            6,55 EUR    Personal:Subscriptions
--------------------
                   0`

		if res == "" {
			t.Errorf("Command executed with empty result")
			return
		}

		if strings.TrimSpace(res) != strings.TrimSpace(expected) {
			t.Errorf("Command executed with unexpected result: %s", res)
			return
		}

	})

	t.Run("wrong file", func(t *testing.T) {
		_, err := ExecLedgerCmd(gitURL, gitToken, tmpDir, "not-exists.ledger", "bal")

		if err == nil {
			t.Errorf("Error excpected")
			return
		}

		if !strings.Contains(err.Error(), "Cannot read journal file") {
			t.Errorf("Unexpected error: %v", err)
			return
		}
	})


	t.Run("wrong command", func(t *testing.T) {
		_, err := ExecLedgerCmd(gitURL, gitToken, tmpDir, "main.ledger", "unknown-command")

		if err == nil {
			t.Errorf("Error excpected")
			return
		}

		if !strings.Contains(err.Error(), "Unrecognized command 'unknown-command'") {
			t.Errorf("Unexpected error: %v", err)
			return
		}
	})

}

package ledger

import (
	"strings"
	"testing"

	"github.com/mput/teledger/app/repo"
)

func TestLedger_Execute(t *testing.T) {

	t.Run("one file", func(t *testing.T) {
		t.Parallel()

		const testFile = `
2024-02-13 * Test
  Assets:Cash  100.00 EUR
  Equity
`

		ledger := NewLedger(
			&repo.Mock{Files: map[string]string{"main.ledger": testFile}},
			"main.ledger",
			false,
		)

		res, err := ledger.Execute("bal")

		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		expected := strings.TrimSpace(`
          100.00 EUR  Assets:Cash
         -100.00 EUR  Equity
--------------------
                   0`)

		if strings.TrimSpace(res) != expected {
			t.Fatalf("Expected: '%s', got: '%s'", expected, res)
		}

	})

	t.Run("ledger with includes", func(t *testing.T) {
		t.Parallel()

		files := map[string]string{
			"main.ledger": `
include accounts.ledger
include accounts.ledger

2024-02-13 * Test
  Assets:Cash  100.00 EUR
  Equity
`,
			"accounts.ledger": `
account Assets:Cash
account Equity
include commodities.ledger
`,
			"commodities.ledger": `
commodity EUR
`,
		}
		repomock := &repo.Mock{
			Files: files,
		}

		ledger := NewLedger(repomock, "main.ledger", true)

		res, err := ledger.Execute("bal")

		if err != nil {
			t.Fatalf("Unexpected command execute error: %v", err)
		}

		expected := strings.TrimSpace(`
          100.00 EUR  Assets:Cash
         -100.00 EUR  Equity
--------------------
                   0`)

		if strings.TrimSpace(res) != expected {
			t.Fatalf("Expected: '%s', got: '%s'", expected, res)
		}

	})

}


func TestLedger_AddTransaction(t *testing.T) {
	t.Run("success path", func(t *testing.T) {
		t.Parallel()

		testFile := `
2024-02-13 * Test
  Assets:Cash  100.00 EUR
  Equity
`

		ledger := NewLedger(
			&repo.Mock{Files: map[string]string{"main.ledger": testFile}},
			"main.ledger",
			false,
		)


		err := ledger.AddTransaction(`
2024-02-14 * Test
  Assets:Cash  42.00 EUR
  Equity
`)

		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		res, err := ledger.Execute("bal")

		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		expected := strings.TrimSpace(`
          142.00 EUR  Assets:Cash
         -142.00 EUR  Equity
--------------------
                   0`)

		if strings.TrimSpace(res) != expected {
			t.Fatalf("Expected: '%s', got: '%s'", expected, res)
		}


		err = ledger.AddTransaction(`
dummy
`)
		if err == nil {
			t.Fatalf("Expected error")
		}


		err = ledger.AddTransaction(`
dummy dummy
`)
		if err == nil {
			t.Fatalf("Expected error")
		}

		err = ledger.AddTransaction(``)
		if err == nil {
			t.Fatalf("Expected error")
		}


		err = ledger.AddTransaction(`

`)
		if err == nil {
			t.Fatalf("Expected error")
		}

	})
}

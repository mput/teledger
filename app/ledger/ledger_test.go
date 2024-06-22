package ledger

import (
	"strings"
	"testing"
	"time"

	"github.com/mput/teledger/app/repo"
	"github.com/stretchr/testify/assert"
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
			nil,
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

		ledger := NewLedger(repomock, nil, "main.ledger", true)

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
			nil,
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


func TestLedger_ProposeTransaction(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {

		mockedTransactionGenerator := &TransactionGeneratorMock{
			GenerateTransactionFunc: func(promptCtx PromptCtx) (Transaction, error) {
				dt, _ := time.Parse(time.RFC3339, "2014-11-12T11:45:26.371Z")
				tr := Transaction{
					Date: dt,
					Description: "My tr",
					Postings: []Posting{
						Posting{
							Account: "cash",
							Amount: -30.43,
							Currency: "EUR",
						},
						Posting{
							Account: "taxi",
							Amount: 30.43,
							Currency: "EUR",
						},
					},
				}
				return tr, nil
			},
		}

		const testFile = `
2024-02-13 * Test
  Assets:Cash  100.00 EUR
  Equity
`

		ledger := NewLedger(
			&repo.Mock{Files: map[string]string{"main.ledger": testFile}},
			mockedTransactionGenerator,
			"main.ledger",
			false,
		)

		tr, err := ledger.ProposeTransaction("20 Taco Bell")


		assert.NoError(t, err)

		assert.Equal(t,
			`2014-11-12 My tr
    cash    -30.43 EUR
    taxi     30.43 EUR
`,
			tr,
		)

		assert.Equal(t, len(mockedTransactionGenerator.calls.GenerateTransaction), 1)


		assert.Equal(t,
			mockedTransactionGenerator.calls.GenerateTransaction[0].PromptCtx,
			PromptCtx{
				Accounts:  []string{"Assets:Cards:Wise-EUR", "Assets:Cards:Wise-USD", "Assets:Cards:Wise-RUB"},
				UserInput: "20 Taco Bell",
			},
		)


	})
}

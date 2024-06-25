package ledger

import (
	"strings"
	"testing"

	"github.com/mput/teledger/app/repo"
	"github.com/stretchr/testify/assert"
	"time"
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
	mockCall := 0

	var mockedTransactionGenerator *TransactionGeneratorMock

	mockedTransactionGenerator = &TransactionGeneratorMock{
		GenerateTransactionFunc: func(p PromptCtx) (mocktr Transaction,err error) {
			mockCall++
			dt, _ := time.Parse(time.RFC3339, "2014-11-12T11:45:26.371Z")
			// On the first attempt, return transaction that is not valid
			// for the test Ledger file
			if len(mockedTransactionGenerator.calls.GenerateTransaction) == 1 {
				mocktr = Transaction{
					RealDateTime: dt,
					Description: "My tr",
					Comment: "invalid transaction",
					Postings: []Posting{
						Posting{
							Account: "cash",
							Amount: -3000.43,
							Currency: "EUR",
						},
						Posting{
							Account: "taxi",
							Amount: 3000.43,
							Currency: "EUR",
						},
					},
				}
			} else {
				mocktr = Transaction{
					RealDateTime: dt,
					Comment: "valid transaction\n22 multiple lines",
					Description: "Tacos",
					Postings: []Posting{
						Posting{
							Account: "Assets:Cash",
							Amount: -3000.43,
							Currency: "EUR",
						},
						Posting{
							Account: "Food",
							Amount: 3000.43,
							Currency: "EUR",
						},
					},
				}
			}
			return

		},
	}

	const testFile = `
commodity EUR
commodity USD

account Food
account Assets:Cash
account Equity

2024-02-13 * Test
  Assets:Cash  100.00 EUR
  Equity
`

	ledger := NewLedger(
		&repo.Mock{Files: map[string]string{"main.ledger": testFile}},
		mockedTransactionGenerator,
		"main.ledger",
		true,
	)

	t.Run("happy path", func(t *testing.T) {


		tr, err := ledger.ProposeTransaction("20 Taco Bell", 5)

		assert.NoError(t, err)

		assert.Equal(t, len(mockedTransactionGenerator.calls.GenerateTransaction), 2)

		assert.Equal(t, "valid transaction\n22 multiple lines", tr.Comment)


		assert.Equal(
			t,
			[]string{"Food", "Assets:Cash", "Equity" },
			mockedTransactionGenerator.calls.GenerateTransaction[0].PromptCtx.Accounts,
		)


		assert.Equal(
			t,
			[]string{"EUR", "USD"},
			mockedTransactionGenerator.calls.GenerateTransaction[0].PromptCtx.Commodities,
		)


		assert.Equal(
			t,
			"20 Taco Bell",
			mockedTransactionGenerator.calls.GenerateTransaction[0].PromptCtx.UserInput,
		)


		assert.Equal(t,
			`;; 2014-11-12 11:45:26 Wednesday: valid transaction
;; 22 multiple lines
2014-11-12 * Tacos
    Assets:Cash  -3.000,43 EUR
    Food  3.000,43 EUR
`,
			tr.Format(true),
		)
	})

	t.Run("validation error path", func(t *testing.T) {
		mockedTransactionGenerator.ResetCalls()

		_, err := ledger.ProposeTransaction("20 Taco Bell", 1)

		assert.ErrorContains(t, err, "Unknown account 'cash'")

		assert.Equal(t, len(mockedTransactionGenerator.calls.GenerateTransaction), 1)

	})


}

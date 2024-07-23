package ledger

import (
	"strings"
	"testing"

	"os"
	"time"

	"github.com/joho/godotenv"
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

		ledger := NewLedger(&repo.Mock{Files: map[string]string{"main.ledger": testFile}}, nil)

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

		ledger := NewLedger(repomock, nil)

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

		ledger := NewLedger(&repo.Mock{Files: map[string]string{"main.ledger": testFile}}, nil)

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
		GenerateTransactionFunc: func(_ PromptCtx) (mocktr Transaction, err error) {
			mockCall++
			dt, _ := time.Parse(time.RFC3339, "2014-11-12T11:45:26.371Z")
			// On the first attempt, return transaction that is not valid
			// for the test Ledger file
			if len(mockedTransactionGenerator.calls.GenerateTransaction) == 1 {
				mocktr = Transaction{
					RealDateTime: dt,
					Description:  "My tr",
					Comment:      "invalid transaction",
					Postings: []Posting{
						{
							Account:  "cash",
							Amount:   -3000.43,
							Currency: "EUR",
						},
						{
							Account:  "taxi",
							Amount:   3000.43,
							Currency: "EUR",
						},
					},
				}
			} else {
				mocktr = Transaction{
					RealDateTime: dt,
					Comment:      "valid transaction\n22 multiple lines",
					Description:  "Tacos",
					Postings: []Posting{
						{
							Account:  "Assets:Cash",
							Amount:   -3000.43,
							Currency: "EUR",
						},
						{
							Account:  "Food",
							Amount:   3000.43,
							Currency: "EUR",
						},
					},
				}
			}
			return mocktr, nil
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
	const configYaml = `
strict: true
`

	// strict mode
	ledger := NewLedger(
		&repo.Mock{Files: map[string]string{
			"main.ledger":   testFile,
			"teledger.yaml": configYaml,
		}},
		mockedTransactionGenerator,
	)

	t.Run("happy path", func(t *testing.T) {

		resp := ledger.AddOrProposeTransaction("20 Taco Bell", 5)

		assert.True(t, ledger.Config.StrictMode)

		// assert.True(t, wasGenerated)
		assert.Equal(t, "", resp.UserProvidedTransaction)

		assert.NoError(t, resp.Error)

		assert.Equal(t, len(mockedTransactionGenerator.calls.GenerateTransaction), 2)
		assert.Equal(t, 2, resp.AttemptNumber)

		assert.Equal(t, "valid transaction\n22 multiple lines", resp.GeneratedTransaction.Comment)

		assert.Equal(
			t,
			[]string{"Food", "Assets:Cash", "Equity"},
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

		assert.False(t, resp.Committed)

		assert.Equal(t,
			`;; valid transaction
;; 22 multiple lines
2014-11-12 * Tacos
    Assets:Cash  -3.000,43 EUR
    Food  3.000,43 EUR
`,
			resp.GeneratedTransaction.Format(true),
		)
	})

	t.Run("add an already valid transaction", func(t *testing.T) {
		mockedTransactionGenerator.ResetCalls()

		resp := ledger.AddOrProposeTransaction(`
2014-11-12 * Tacos
    Assets:Cash  -2,43 EUR
    Food  2,43 EUR
`, 1)

		assert.Nil(t, resp.GeneratedTransaction)
		assert.True(t, resp.Committed)
		assert.NoError(t, resp.Error)
		assert.Equal(t, 0, len(mockedTransactionGenerator.calls.GenerateTransaction))
		assert.Equal(t, 0, resp.AttemptNumber)

	})

	t.Run("validation error path", func(t *testing.T) {
		mockedTransactionGenerator.ResetCalls()

		resp := ledger.AddOrProposeTransaction("20 Taco Bell", 1)
		assert.ErrorContains(t, resp.Error, "Unknown account 'cash'")

		assert.Equal(t, len(mockedTransactionGenerator.calls.GenerateTransaction), 1)

	})

}

func TestWithRepo(t *testing.T) {
	_ = godotenv.Load("../../.env.dev")

	gitURL := os.Getenv("GITHUB_URL")
	if gitURL == "" {
		t.Fatal("GIT_URL is not set")
	}

	gitToken := os.Getenv("GITHUB_TOKEN")
	if gitToken == "" {
		t.Fatal("GIT_ACCESS_TOKEN is not set")
	}

	inmemrepo := repo.NewInMemoryRepo(gitURL, gitToken)

	ledger := NewLedger(inmemrepo, nil)

	res, err := ledger.Execute("bal")

	assert.NoError(t, err)

	assert.True(t, ledger.Config.StrictMode)

	assert.NotEmpty(t, res)

}

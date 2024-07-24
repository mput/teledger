package teledger

import (
	"testing"
	"time"

	"github.com/mput/teledger/app/ledger"
	"github.com/mput/teledger/app/repo"

	"github.com/stretchr/testify/assert"
)


func TestTeledger_AddComment(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		initContent := `
2024-02-13 * Test
  Assets:Cash  100.00 EUR
  Equity
`

		r := &repo.Mock{
			Files: map[string]string{"main.ledger": initContent},
		}

		l := ledger.NewLedger(r, nil)

		tldgr := &Teledger{
			Ledger: l,
		}

		_, err := tldgr.AddComment("This is a comment\n multiline")
		assert.NoError(t, err)

		content := r.Files["main.ledger"]

		assert.Regexp(t, `
.*
  Equity

;; \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} (Sunday|Monday|Tuesday|Wednesday|Thursday|Friday|Saturday)
;; This is a comment
;;  multiline
;; \d{4}-\d{2}-\d{2} \*
`, content)




	})
}

func TestTeledger_AddTransaction(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		initContent := `
account Food
account Assets:Cash
account Equity
commodity EUR

2024-02-13 * Test
  Assets:Cash  100.00 EUR
  Equity
`
		const configYaml = `
strict: true
`

		r := &repo.Mock{
			Files: map[string]string{"main.ledger": initContent, "teledger.yaml": configYaml},
		}

		mockedTransactionGenerator := &ledger.TransactionGeneratorMock{
			GenerateTransactionFunc: func(prmt ledger.PromptCtx) (mocktr ledger.Transaction, err error) {
				dt, _ := time.Parse(time.RFC3339, "2014-11-30T11:45:26.371443Z")


				switch prmt.UserInput {
				case "valid":
					mocktr = ledger.Transaction{
						RealDateTime: dt,
						Description:  "My tr",
						Comment:      prmt.UserInput,
						Postings: []ledger.Posting{
							{
								Account:  "Assets:Cash",
								Amount:   -10,
								Currency: "EUR",
							},
							{
								Account:  "Food",
								Amount:   10,
								Currency: "EUR",
							},
						},
					}
				default:
					panic("Should not be here!")

				}
				return
			},
		}


		l := ledger.NewLedger(r, mockedTransactionGenerator)

		tldgr :=  NewTeledger(l)
		resp := tldgr.ProposeTransaction("valid")
		assert.NotEmpty(t, resp.PendingKey)
		assert.Empty(t, resp.Error)
		assert.NotEmpty(t, resp.PendingKey)

		t.Run("attempt to concurrently confirm the same transaction", func(t *testing.T) {
			(*tldgr.WaitingToBeConfirmedResponses)[resp.PendingKey].Mu.Lock()
			_, err := tldgr.ConfirmTransaction(resp.PendingKey)
			assert.ErrorContains(t, err, "already in progress")
			(*tldgr.WaitingToBeConfirmedResponses)[resp.PendingKey].Mu.Unlock()
		})

		t.Run("Success Confirmation",  func(t *testing.T) {
			_, err := tldgr.ConfirmTransaction(resp.PendingKey)
			assert.Empty(t, err)

			assert.Equal(
				t,
				r.Files["main.ledger"],
				`
account Food
account Assets:Cash
account Equity
commodity EUR

2024-02-13 * Test
  Assets:Cash  100.00 EUR
  Equity

;; tid:2014-11-30 11:45:26.371 Sun
;; valid
2014-11-30 * My tr
    Assets:Cash  -10,00 EUR
    Food  10,00 EUR
`,
			)

		})

		t.Run("attempt to confirm for the second time",  func(t *testing.T) {
			_, err := tldgr.ConfirmTransaction(resp.PendingKey)
			assert.ErrorContains(t, err, "missing pending transaction")

		})


		t.Run("attempt to confirm with unknonw key", func(t *testing.T) {
			_, err := tldgr.ConfirmTransaction("unk")
			assert.ErrorContains(t, err, "missing pending transaction")
		})

		t.Run("delete previously confirmed transaction", func(t *testing.T) {
			err := tldgr.DeleteTransaction(resp.PendingKey)
			assert.Empty(t, err)

			assert.Equal(
				t,
				initContent,
				r.Files["main.ledger"],
			)

		})

		t.Run("delete unknown transaction", func(t *testing.T) {
			err := tldgr.DeleteTransaction("unknowntrr")
			assert.ErrorContains(t, err, "no transaction with id")

			assert.Equal(
				t,
				initContent,
				r.Files["main.ledger"],
			)

		})

		t.Run("delete transaction corner cases", func(t *testing.T) {
			initCoreners := `
commodity EUR

;; tid:2014-11-30 11:45:26.111 Sun
;; valid
2014-11-30 * My tr
    Assets:Cash  -10,00 EUR
    Food  10,00 EUR

;; tid:2014-11-30 11:45:26.371 Sun
;; valid
2014-11-30 * My tr
    Assets:Cash  -10,00 EUR
    Food  10,00 EUR

;; tid:2014-11-30 11:45:26.371 Sun
;; valid
2014-11-30 * My tr
    Assets:Cash  -10,00 EUR
    Food  10,00 EUR
`


			r := &repo.Mock{
				Files: map[string]string{"main.ledger": initCoreners},
			}

			l := ledger.NewLedger(r, nil)

			tldgr := &Teledger{
				Ledger: l,
			}

			t.Run("transaction in the middle", func(t *testing.T) {
				err :=tldgr.DeleteTransaction("2014-11-30 11:45:26.111 Sun")
				assert.Empty(t, err)

				assert.Equal(
					t,
					`
commodity EUR

;; tid:2014-11-30 11:45:26.371 Sun
;; valid
2014-11-30 * My tr
    Assets:Cash  -10,00 EUR
    Food  10,00 EUR

;; tid:2014-11-30 11:45:26.371 Sun
;; valid
2014-11-30 * My tr
    Assets:Cash  -10,00 EUR
    Food  10,00 EUR
`,
					r.Files["main.ledger"],
				)

			})

			t.Run("repeating transaction", func(t *testing.T) {
				err :=tldgr.DeleteTransaction("2014-11-30 11:45:26.371 Sun")
				assert.Empty(t, err)

				assert.Equal(
					t,
					`
commodity EUR

;; tid:2014-11-30 11:45:26.371 Sun
;; valid
2014-11-30 * My tr
    Assets:Cash  -10,00 EUR
    Food  10,00 EUR
`,
					r.Files["main.ledger"],
				)

			})



		})

		t.Run("propose valid transaction, not free form explanation", func(t *testing.T) {
			resp := tldgr.ProposeTransaction(`
2014-11-30 * My tr
    Assets:Cash  -10,00 EUR
    Food  10,00 EUR
`)
			assert.Empty(t, resp.PendingKey)
			assert.Equal(t, 0, resp.AttemptNumber)
			assert.Empty(t, resp.Error)


		})



	})
}

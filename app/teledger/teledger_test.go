package teledger

import (
	"testing"


	"github.com/mput/teledger/app/repo"
	"github.com/mput/teledger/app/ledger"

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

		l := ledger.NewLedger(
			r,
			nil,
			"main.ledger",
			false,
		)

		tldgr := &Teledger{
			ledger: l,
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

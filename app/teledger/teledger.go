package teledger

import (
	"fmt"
	"time"

	"github.com/mput/teledger/app/ledger"
)

// Teledger is the service that handles all the
// operations related to the Ledger files
type Teledger struct {
	Ledger *ledger.Ledger
}


func NewTeledger(ldgr *ledger.Ledger) *Teledger {
	return &Teledger{
		Ledger: ldgr,
	}
}

// Add an arbitrary text as a comment to the main ledger file
// The comment will be added at the end of the file, with a timestamp
// and the template of the transaction at the end
func (tel *Teledger) AddComment(comment string) (string, error) {

	// TODO: move timezone to config
	timezoneName := "GMT"
	loc, err := time.LoadLocation(timezoneName)
	if err != nil {
		return "", err
	}

	now := time.Now().In(loc)

	commitLine := fmt.Sprintf(
		"%s\n%s\n%s *",
		now.Format("2006-01-02 15:04:05 Monday"),
		comment,
		now.Format("2006-01-02"),
	)

	res, err := tel.Ledger.AddComment(commitLine)
	if err != nil {
		return "", err
	}
	return res, nil
}

func (tel *Teledger) Balance() (string, error) {
	return tel.Ledger.Execute("bal")
}

func (tel *Teledger) Report(reportTitle string) (string, error) {
	var reportArgs []string
	for _, report := range tel.Ledger.Config.Reports {
		if report.Title == reportTitle {
			reportArgs = report.Command
			break
		}
	}
	if len(reportArgs) == 0 {
		return "", fmt.Errorf("Report not found")
	}
	return tel.Ledger.Execute(reportArgs...)
}


func (tel *Teledger) Init() error {
	_, err := tel.Ledger.Execute("bal")
	return  err
}


func inBacktick(s string) string {
	return fmt.Sprintf("```\n%s\n```", s)
}

// Receive a short free-text description of a transaction
// and propose a formatted transaction validated with the
// ledger file.
// Store the transaction in a state, so the user can confirm
// or reject it.
func (tel *Teledger) ProposeTransaction(desc string) (string, error) {
	wasGenerated, tr, err := tel.Ledger.AddOrProposeTransaction(desc, 2)
	if wasGenerated {
		if err == nil {
			return inBacktick(tr.Format(false)), nil
		}

		if len(tr.Postings) == 0 {
			return fmt.Sprintf(`Proposed but invalid transaction:
%s`,
				inBacktick(tr.Format(false)),
			), nil
		}

		return "", err

	}

	if err == nil {
		return "Transaction Added", nil
	}
	return "", err


}

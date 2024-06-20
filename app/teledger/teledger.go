package teledger

import (
	"github.com/mput/teledger/app/ledger"
	"time"
	"fmt"
)


// Teledger is the service that handles all the
// operations related to the Ledger files
type Teledger struct {
	ledger *ledger.Ledger
}


func NewTeledger(ldgr *ledger.Ledger) *Teledger {
	return &Teledger{
		ledger: ldgr,
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

	res, err := tel.ledger.AddComment(commitLine)
	if err != nil {
		return "", err
	}
	return res, nil
}

func (tel *Teledger) Balance() (string, error) {
	return tel.ledger.Execute("bal")
}

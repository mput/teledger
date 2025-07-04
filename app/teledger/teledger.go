package teledger

import (
	"fmt"
	"sync"
	"time"

	"github.com/mput/teledger/app/ledger"
)

// Teledger is the service that handles all the
// operations related to the Ledger files
type Teledger struct {
	Ledger                        *ledger.Ledger
	WaitingToBeConfirmedResponses *map[string]*PendingTransaction
}

func NewTeledger(ldgr *ledger.Ledger) *Teledger {
	m := make(map[string]*PendingTransaction)
	return &Teledger{
		Ledger:                        ldgr,
		WaitingToBeConfirmedResponses: &m,
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
	return err
}

type PendingTransaction struct {
	ledger.ProposeTransactionRespones
	PendingKey string
	Mu         sync.Mutex
}

// Receive a short free-text description of a transaction
// and propose a formatted transaction validated with the
// ledger file.
// Store the transaction in a state, so the user can confirm
// or reject it.
func (tel *Teledger) ProposeTransaction(desc string) *PendingTransaction {
	resp := tel.Ledger.AddOrProposeTransaction(desc, 2)
	pt := PendingTransaction{
		ProposeTransactionRespones: resp,
	}
	if resp.Error == nil && resp.GeneratedTransaction != nil {
		key := resp.GeneratedTransaction.RealDateTime.Format("2006-01-02 15:04:05.999 Mon")
		pt.PendingKey = key
		(*tel.WaitingToBeConfirmedResponses)[key] = &pt
	}
	return &pt
}

func (tel *Teledger) ConfirmTransaction(pendingKey string) (*PendingTransaction, error) {
	pendTr, ok := (*tel.WaitingToBeConfirmedResponses)[pendingKey]
	if !ok {
		return nil, fmt.Errorf("missing pending transaction: `%s`", pendingKey)
	}
	locked := pendTr.Mu.TryLock()
	if !locked {
		return nil, fmt.Errorf("transaction confirmation already in progress: `%s`", pendingKey)
	}
	defer pendTr.Mu.Unlock()

	err := tel.Ledger.AddTransactionWithID(pendTr.GeneratedTransaction.Format(true), pendingKey)
	if err != nil {
		return nil, err
	}

	pendTr.Committed = true
	delete(*tel.WaitingToBeConfirmedResponses, pendingKey)
	return pendTr, nil
}

func (tel *Teledger) DeleteTransaction(pendingKey string) error {
	return tel.Ledger.DeleteTransactionWithID(pendingKey)
}

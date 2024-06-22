package ledger

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mput/teledger/app/repo"
)

// Ledger is a wrapper around the ledger command line tool
type Ledger struct {
	repo repo.Service
	mainFile string
	strict bool
	generator TransactionGenerator
}

func NewLedger(rs repo.Service,gen TransactionGenerator , mainFile string, strict bool) *Ledger {
	return &Ledger{
		repo: rs,
		generator: gen,
		mainFile: mainFile,
		strict: strict,
	}
}

const ledgerBinary = "ledger"

func resolveIncludesReader(rs repo.Service, file string) (io.ReadCloser, error) {
	ledgerFile, err := rs.Open(file)
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()

	var f func(ledReader io.ReadCloser, top bool)
	f = func(ledReader io.ReadCloser, top bool) {
		defer ledReader.Close()
		if top {
			defer w.Close()
		}

		lcnt := 0
		scanner := bufio.NewScanner(ledReader)
		for scanner.Scan() {
			line := scanner.Text()
			lcnt++
			linetr := strings.TrimSpace(line)
			if strings.HasPrefix(linetr, "include") {
				path := strings.TrimPrefix(linetr, "include")
				path = strings.TrimSpace(path)
				ir, rerr := rs.Open(path)
				if rerr == nil {
					f(ir, false)
					continue
				}
				// line will be used as is so ledger will report the error if it's actually an include
				slog.Warn("unable to open include file", "file", path, "error", rerr)
			}
			_, err := fmt.Fprintln(w, line)
			if err != nil {
				slog.Error("unable to write to pipe", "error", err)
				w.CloseWithError(fmt.Errorf("unable to write to pipe: %v", err))
				return
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Error("unable to read ledger file", "error", err)
			w.CloseWithError(fmt.Errorf("unable to read ledger file: %v", err))
			return
		}
		slog.Debug("finish ledger file read", "file", file, "top", top, "lines", lcnt)
	}

	go f(ledgerFile, true)

	return r, nil

}

func (l *Ledger) execute(args ...string) (string, error) {
	r, err := resolveIncludesReader(l.repo, l.mainFile)

	if err != nil {
		return "", fmt.Errorf("ledger file opening error: %v", err)
	}

	fargs := []string{"-f", "-"}
	if l.strict {
		fargs = append(fargs, "--pedantic")
	}
	fargs = append(fargs, args...)

    cmddir, err := os.MkdirTemp("", "ledger")
	if err != nil {
		return "", fmt.Errorf("ledger temp dir creation error: %v", err)
	}
    defer os.RemoveAll(cmddir)

	cmd := exec.Command(ledgerBinary, fargs...)

	// For security reasons, we don't want to pass any environment variables to the ledger command
	cmd.Env = []string{}
	// Temp dir is only exists to not expose any existing directory to ledger command
	cmd.Dir = cmddir

	cmd.Stdin = r
	var out strings.Builder
	var errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err = cmd.Run()
	if err != nil {
		_, isExitError := err.(*exec.ExitError)
		if isExitError {
			return "", fmt.Errorf("ledger error: exited with status %s (%v)", err, errOut.String())
		}
		return "", fmt.Errorf("ledger command executing error: %v", err)
	}

	res := out.String()
	if res == "" {
		return "", fmt.Errorf("ledger command returned empty result")
	}
	return out.String(), nil
}


func (l *Ledger) Execute(args ...string) (string, error) {
	err := l.repo.Init()
	defer l.repo.Free()
	if err != nil {
		return "", fmt.Errorf("unable to init repo: %v", err)
	}

	return l.execute(args...)
}

func (l *Ledger) AddTransaction(transaction string) error {
	err := l.repo.Init()
	defer l.repo.Free()
	if err != nil {
		return fmt.Errorf("unable to init repo: %v", err)
	}


	balBefore, err := l.execute("balance")
	if err != nil {
		return err
	}
	r, err := l.repo.OpenForAppend(l.mainFile)
	if err != nil {
		return fmt.Errorf("unable to open main ledger file: %v", err)
	}
	_, err = fmt.Fprintf(r, "\n%s\n", transaction)
	r.Close()
	if err != nil {
		return fmt.Errorf("unable to write main ledger file: %v", err)
	}
	balAfter, err :=l.execute("balance")
	if err != nil {
		return err
	}
	if balBefore == balAfter {
		return fmt.Errorf("transaction doesn't change balance")
	}
	return nil
}

func (l *Ledger) validate() error {
	_, err := l.execute("balance")
	return err
}


func (l *Ledger) AddComment(comment string) (string, error) {
	err := l.repo.Init()
	defer l.repo.Free()
	if err != nil {
		return "", fmt.Errorf("unable to init repo: %v", err)
	}

	r, err := l.repo.OpenForAppend(l.mainFile)
	if err != nil {
		return "", fmt.Errorf("unable to open main ledger file: %v", err)
	}

	lines := make([]string, 0)

	for _, l := range strings.Split(comment, "\n") {
		if l != "" {
			lines = append(lines, fmt.Sprintf(";; %s", l))
		}
	}

	if len(lines) == 1 {
		return "", fmt.Errorf("empty comment provided")
	}

	res := strings.Join(lines, "\n")

	_, err = fmt.Fprintf(r, "\n%s\n", res)

	if err != nil {
		return "", fmt.Errorf("unable to write main ledger file: %v", err)
	}

	err = l.validate()
	r.Close()
	if err != nil {
		return "", fmt.Errorf("ledger file become invalid after an attempt to add comment: %v", err)
	}

	err = l.repo.CommitPush(fmt.Sprintf("comment: %s", lines[0]), "teledger", "teledger@example.com")
	if err != nil {
		return "", fmt.Errorf("unable to commit: %v", err)

	}
	return res, nil
}

// Transaction represents a single transaction in a ledger.
type Transaction struct {
	Date        time.Time `json:"date"`         // The date of the transaction
	Description string    `json:"description"`  // A description of the transaction
	Postings    []Posting `json:"postings"`     // A slice of postings that belong to this transaction
}

func (t Transaction) toString() string {
	var res strings.Builder
	res.WriteString(fmt.Sprintf("%s %s\n", t.Date.Format("2006-01-02"), t.Description))
	for _, p := range t.Postings {
		// format float to 2 decimal places
		res.WriteString(fmt.Sprintf("    %s  %8.2f %s\n", p.Account, p.Amount, p.Currency))

	}
	return res.String()
}

// Posting represents a single posting in a transaction, linking an account with an amount and currency.
type Posting struct {
	Account  string  `json:"account"`  // The name of the account
	Amount   float64 `json:"amount"`   // The amount posted to the account
	Currency string  `json:"currency"` // The currency of the amount
}

// TransactionGenerator is an interface for generating transactions from user input
// using LLM.
//go:generate moq -out  transaction_generator_mock.go -with-resets . TransactionGenerator
type TransactionGenerator interface {
	GenerateTransaction(promptCtx PromptCtx) (Transaction, error)
}

type OpenAITransactionGenerator struct {
	token string
}

func (b OpenAITransactionGenerator) GenerateTransaction(promptCtx PromptCtx) (Transaction, error) {
	return Transaction{}, nil
}


// Receive a short free-text description of a transaction
// and returns a formatted transaction validated with the
// ledger file.
func (l *Ledger) ProposeTransaction(userInput string) (string, error) {
	err := l.repo.Init()
	defer l.repo.Free()
	if err != nil {
		return "", err
	}

	accounts := []string{"Assets:Cards:Wise-EUR", "Assets:Cards:Wise-USD", "Assets:Cards:Wise-RUB"}

	promptCtx := PromptCtx{
		Accounts: accounts,
		UserInput: userInput,
	}

	trx, err := l.generator.GenerateTransaction(promptCtx)
	if err != nil {
		return "", fmt.Errorf("unable to generate transaction: %v", err)
	}

	return trx.toString(), nil

}


type PromptCtx struct {
	Accounts []string
	UserInput string
}


const template = `
Your goal is to propose a transaction in the ledger cli format.
Your responses MUST be in JSON and adhere to the Transaction struct ONLY with no additional narrative or markup, backquotes or anything.

Bellow is the list of accounts you MUST use in your transaction:
{{range .Accounts}}
"{{.}}"
{{end}}


Use "EUR" as the default currency if nothing else is specified in user request.
Another possible currency is "USD", "RUB".
All descriptions should be in English.

// Transaction represents a single transaction in a ledger.
type Transaction struct {
	Date        time.Time json:"date"         // The date of the transaction
	Description string    json:"description"  // A description of the transaction
	Postings    []Posting json:"postings"     // A slice of postings that belong to this transaction
}

// Posting represents a single posting in a transaction, linking an account with an amount and currency.
type Posting struct {
	Account  string  json:"account"  // The name of the account
	Amount   float64 json:"amount"   // The amount posted to the account
	Currency string  json:"currency" // The currency of the amount
}

Use Assets:Cards:Wise-EUR as default account  if nothing else is specified in user request

Bellow is the message from the USER for which you need to propose a transaction:

{{.UserInput}}
`

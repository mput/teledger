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
	"text/template"

	"github.com/mput/teledger/app/repo"
	"github.com/mput/teledger/app/utils"
	"github.com/dustin/go-humanize"
	openai "github.com/sashabaranov/go-openai"
	_ "embed"
	"context"
	"bytes"
	"encoding/json"
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


//go:embed templates/default_prompt.txt
var defaultPromtpTemplate string


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

func (l *Ledger) executeWith(additional string, args ...string) (string, error) {
	r, err := resolveIncludesReader(l.repo, l.mainFile)

	if err != nil {
		return "", fmt.Errorf("ledger file opening error: %v", err)
	}

	if additional != "" {
		r = utils.MultiReadCloser(r, io.NopCloser( strings.NewReader(additional) ))
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

func (l *Ledger) execute(args ...string) (string, error) {
	return l.executeWith("", args...)
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

func (l *Ledger) validateWith(addition string) error {
	_, err := l.executeWith(addition, "balance")
	return err
}

func wrapIntoComment(s string) string {
	lines := make([]string, 0)

	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			lines = append(lines, fmt.Sprintf(";; %s", l))
		}
	}

	res := strings.Join(lines, "\n")
	return res
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

	res := wrapIntoComment(comment)

	if res == "" {
		return "", fmt.Errorf("empty comment provided")
	}


	_, err = fmt.Fprintf(r, "\n%s\n", res)

	if err != nil {
		return "", fmt.Errorf("unable to write main ledger file: %v", err)
	}

	err = l.validate()
	r.Close()
	if err != nil {
		return "", fmt.Errorf("ledger file become invalid after an attempt to add comment: %v", err)
	}

	err = l.repo.CommitPush("New comment", "teledger", "teledger@example.com")
	if err != nil {
		return "", fmt.Errorf("unable to commit: %v", err)

	}
	return res, nil
}

// Transaction represents a single transaction in a ledger.
type Transaction struct {
	Date        string    `json:"date"`         // The date of the transaction
	Description string    `json:"description"`  // A description of the transaction
	Postings    []Posting `json:"postings"`     // A slice of postings that belong to this transaction
	Comment	    string
	RealDateTime time.Time
}

func (t *Transaction) Format(withComment bool) string {
	var res strings.Builder
	if withComment {
		res.WriteString(
			wrapIntoComment(fmt.Sprintf("%s: %s",t.RealDateTime.Format("2006-01-02 15:04:05 Monday"), t.Comment)),
		)
		res.WriteString("\n")
	}
	res.WriteString(fmt.Sprintf("%s * %s\n", t.RealDateTime.Format("2006-01-02"), t.Description))
	for _, p := range t.Postings {
		// format float to 2 decimal places
		vf := humanize.FormatFloat("#.###,##", p.Amount)
		res.WriteString(fmt.Sprintf("    %s  %s %s\n",p.Account, vf, p.Currency))

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
	openai *openai.Client
}

func NewOpenAITransactionGenerator(token string) *OpenAITransactionGenerator {
	return &OpenAITransactionGenerator{
		openai: openai.NewClient(token),
	}
}

func (b OpenAITransactionGenerator) GenerateTransaction(promptCtx PromptCtx) (Transaction, error) {
	var buf bytes.Buffer
	prTmp := template.Must(template.New("letter").Parse(defaultPromtpTemplate))
	err := prTmp.Execute(&buf, promptCtx)
	if err != nil {
		return Transaction{}, fmt.Errorf("unable to execute template: %v", err)
	}

	prompt := buf.String()


	resp, err := b.openai.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: prompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: promptCtx.UserInput,
				},
			},
		},
	)

	if err != nil {
		fmt.Println("ChatCompletion error: ", err)
		return Transaction{}, fmt.Errorf("chatCompletion error: %v", err)
	}

	fmt.Println(resp.Choices[0].Message.Content)

	res := Transaction{}
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &res)
	if err != nil {
		return Transaction{}, fmt.Errorf("unable to unmarshal response: %v", err)
	}

	res.Comment = promptCtx.UserInput
	res.RealDateTime = promptCtx.Datetime

	return res, nil
}

func parseCommodityOrAccount(ledger io.Reader, directive string) ([]string, error) {
	if directive != "commodity" && directive != "account" {
		panic("unsupported directive")
	}
	scanner := bufio.NewScanner(ledger)
	var res []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, directive) {
			v := strings.TrimSpace(strings.TrimPrefix(line, directive))
			res = append(res, v)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("unable to read ledger file: %v", err)
	}
	return res, nil
}

func (l *Ledger) extractAccounts() ([]string, error) {
	r, err := resolveIncludesReader(l.repo, l.mainFile)
	if err != nil {
		return nil, err
	}

	accs, err := parseCommodityOrAccount(r, "account")
	if err != nil {
		return nil, fmt.Errorf("unable to extract accounts from directives: %v", err)
	}


	accsFromTrxsS, err := l.execute("accounts")
	if err != nil {
		return nil, fmt.Errorf("unable to extract accounts from transactions: %v", err)
	}
	accsFromTrxs := strings.Split(strings.TrimSpace(accsFromTrxsS), "\n")


	accs = append(accs, accsFromTrxs...)
	accsdedup := make([]string, 0)
	accsmap := make(map[string]struct{})
	for _, a := range accs {
		if _, ok := accsmap[a]; !ok {
			accsmap[a] = struct{}{}
			accsdedup = append(accsdedup, a)
		}
	}
	return accsdedup, nil
}

func (l *Ledger) extractCommodities() ([]string, error) {
	r, err := resolveIncludesReader(l.repo, l.mainFile)
	if err != nil {
		return nil, err
	}

	coms, err := parseCommodityOrAccount(r, "commodity")
	if err != nil {
		return nil, fmt.Errorf("unable to extract accounts from directives: %v", err)
	}


	comsFromTrxsS, err := l.execute("commodities")
	if err != nil {
		return nil, fmt.Errorf("unable to extract accounts from transactions: %v", err)
	}
	comsFromTrxs := strings.Split(strings.TrimSpace(comsFromTrxsS), "\n")


	coms = append(coms, comsFromTrxs...)
	dedup := make([]string, 0)
	dedupm := make(map[string]struct{})
	for _, a := range coms {
		if _, ok := dedupm[a]; !ok {
			dedupm[a] = struct{}{}
			dedup = append(dedup, a)
		}
	}
	return dedup, nil
}


// Receive a short free-text description of a transaction
// and returns a formatted transaction validated with the
// ledger file.
func (l *Ledger) proposeTransaction(userInput string) (Transaction, error) {
	accounts, err := l.extractAccounts()
	if err != nil {
		return Transaction{}, err
	}

	commodities, err := l.extractCommodities()
	if err != nil {
		return Transaction{}, err
	}


	promptCtx := PromptCtx{
		Accounts: accounts,
		Commodities: commodities,
		UserInput: userInput,
		Datetime: time.Now(),
	}

	trx, err := l.generator.GenerateTransaction(promptCtx)
	if err != nil {
		return trx, fmt.Errorf("unable to generate transaction: %v", err)
	}

	fmt.Println(trx.Format(true))

	err = l.validateWith(trx.Format(true))
	if err != nil {
		return trx, fmt.Errorf("unable to validate transaction: %v", err)
	}

	return trx, nil

}

func (l *Ledger) ProposeTransaction(userInput string, attempts int) (tr Transaction,err error) {
	if attempts <= 0 {
		panic("times should be greater than 0")
	}
	err = l.repo.Init()
	defer l.repo.Free()

	if err != nil {
		return tr, err
	}

	for i := 0; i < attempts; i++ {
		if i > 0 {
			slog.Warn("retrying transaction generation", "attempt", i)
		}
		tr, err = l.proposeTransaction(userInput)
		if err == nil {
			return tr, nil
		}
	}

	return tr, err
}


type PromptCtx struct {
	Accounts []string
	Commodities []string
	UserInput string
	Datetime time.Time
}



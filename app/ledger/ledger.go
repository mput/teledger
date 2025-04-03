package ledger

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"bytes"
	"context"
	_ "embed"
	"encoding/json"

	"github.com/dustin/go-humanize"
	"github.com/mput/teledger/app/repo"
	"github.com/mput/teledger/app/utils"
	openai "github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
)

// Ledger is a wrapper around the ledger command line tool
type Ledger struct {
	repo repo.Service
	// mainFile string
	// strict    bool
	generator TransactionGenerator
	Config    *Config
}

type Report struct {
	Title   string
	Command []string
}

type Config struct {
	MainFile       string   `yaml:"mainFile"`       // default: main.ledger, not required
	StrictMode     bool     `yaml:"strict"`         // whether to allow non existing accounts and commodities
	PromptTemplate string   `yaml:"promptTemplate"` // not required
	Version        string   `yaml:"version"`        // do not include in documentation
	Reports        []Report `yaml:"reports"`        //
}

func NewLedger(rs repo.Service, gen TransactionGenerator) *Ledger {
	return &Ledger{
		repo:      rs,
		generator: gen,
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
	r, err := resolveIncludesReader(l.repo, l.Config.MainFile)

	if err != nil {
		return "", fmt.Errorf("ledger file opening error: %v", err)
	}

	if additional != "" {
		r = utils.MultiReadCloser(r, io.NopCloser(strings.NewReader(additional)))
	}

	fargs := []string{"-f", "-"}
	if l.Config.StrictMode {
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
	err = l.setConfig()
	if err != nil {
		return "", fmt.Errorf("unable to set config: %v", err)
	}

	return l.execute(args...)
}

func (l *Ledger) addTransaction(transaction string) error {
	balBefore, err := l.execute("balance")
	if err != nil {
		return fmt.Errorf("invalid transaction: %v", err)
	}

	balAfter, err := l.executeWith(transaction, "balance")

	if err != nil {
		return fmt.Errorf("invalid transaction: %v", err)
	}

	if balBefore == balAfter {
		return fmt.Errorf("invalid transaction: transaction doesn't change balance")
	}

	r, err := l.repo.OpenForAppend(l.Config.MainFile)
	if err != nil {
		return fmt.Errorf("unable to open main ledger file: %v", err)
	}
	_, err = fmt.Fprintf(r, "\n%s", transaction)
	defer r.Close()
	if err != nil {
		return fmt.Errorf("unable to write main ledger file: %v", err)
	}
	return nil
}

func (l *Ledger) AddTransaction(transaction string) error {
	err := l.repo.Init()
	defer l.repo.Free()
	if err != nil {
		return fmt.Errorf("unable to init repo: %v", err)
	}
	err = l.setConfig()
	if err != nil {
		return fmt.Errorf("unable to set config: %v", err)
	}

	err = l.addTransaction(transaction)

	if err != nil {
		return err
	}

	err = l.repo.CommitPush("New comment", "teledger", "teledger@example.com")
	if err != nil {
		return fmt.Errorf("unable to commit: %v", err)
	}
	return nil
}

const transactionIDPrefix = ";; tid:"

func (l *Ledger) AddTransactionWithID(transaction , id string) error {
	return l.AddTransaction(fmt.Sprintf("%s%s\n%s",transactionIDPrefix, id, transaction))
}

func filterOutTransactionWithID(r io.Reader, id string) (content []byte, err error) {
	scanner := bufio.NewScanner(r)
	marker := transactionIDPrefix + id

	afterMarker := 0

	for scanner.Scan() {
		txt := scanner.Text()

		if (afterMarker == 0 && txt == marker) {
			afterMarker++
			if len(content) > 0 && content[len(content) - 1] == '\n' {
				content = content[:len(content) - 1]
			}
			continue
		}
		if (afterMarker == 1 && txt == "") {
			afterMarker++
			content = append(content, '\n')
			continue
		}
		if afterMarker == 1 {
			continue
		}
		content = append(content, scanner.Bytes()...)
		content = append(content, '\n')
	}
	if err := scanner.Err(); err != nil {
		return content, fmt.Errorf("reading standard input: %v", err)
	}
	if afterMarker == 0 {
		return content, fmt.Errorf("no transaction with id '%s' was found", id)
	}
	return content, nil

}

func (l *Ledger) DeleteTransactionWithID(id string) error {
	err := l.repo.Init()
	defer l.repo.Free()
	if err != nil {
		return fmt.Errorf("unable to init repo: %v", err)
	}
	err = l.setConfig()
	if err != nil {
		return fmt.Errorf("unable to set config: %v", err)
	}

	f, err := l.repo.OpenFile(l.Config.MainFile, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("unable to open main ledger file: %v", err)
	}

	newContent, err := filterOutTransactionWithID(f, id)
	if err != nil {
		return err
	}

	err = f.Truncate(0)
	if err != nil {
		return err
	}

	_, err = f.Seek(0, 0)

	if err != nil {
		return err
	}

	_, err = f.Write(newContent)

	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("unable to close main ledger file: %v", err)
	}

	err = l.repo.CommitPush("New comment", "teledger", "teledger@example.com")
	if err != nil {
		return fmt.Errorf("unable to commit: %v", err)
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
	err = l.setConfig()
	if err != nil {
		return "", fmt.Errorf("unable to set config: %v", err)
	}

	r, err := l.repo.OpenForAppend(l.Config.MainFile)
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
	Date         string    `json:"date"`        // The date of the transaction
	Description  string    `json:"description"` // A description of the transaction
	Postings     []Posting `json:"postings"`    // A slice of postings that belong to this transaction
	Comment      string
	RealDateTime time.Time
}

func (t *Transaction) Format(withComment bool) string {
	var res strings.Builder
	if withComment {
		res.WriteString(
			wrapIntoComment(t.Comment),
		)
		res.WriteString("\n")
	}
	res.WriteString(fmt.Sprintf("%s * %s\n", t.RealDateTime.Format("2006-01-02"), t.Description))
	for _, p := range t.Postings {
		// format float to 2 decimal places
		vf := humanize.FormatFloat("#,###.##", p.Amount)
		res.WriteString(fmt.Sprintf("    %s  %s %s\n", p.Account, vf, p.Currency))

	}
	return res.String()
}


func (t *Transaction) String() string {
	return t.Format(false)
}

// Posting represents a single posting in a transaction, linking an account with an amount and currency.
type Posting struct {
	Account  string  `json:"account"`  // The name of the account
	Amount   float64 `json:"amount"`   // The amount posted to the account
	Currency string  `json:"currency"` // The currency of the amount
}

// TransactionGenerator is an interface for generating transactions from user input
// using LLM.
//
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

//nolint:gocritic
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
			Model: openai.GPT4oMini,
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
	r, err := resolveIncludesReader(l.repo, l.Config.MainFile)
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
	r, err := resolveIncludesReader(l.repo, l.Config.MainFile)
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
		Accounts:    accounts,
		Commodities: commodities,
		UserInput:   userInput,
		Datetime:    time.Now(),
	}

	trx, err := l.generator.GenerateTransaction(promptCtx)
	if err != nil {
		return trx, fmt.Errorf("unable to generate transaction: %v", err)
	}

	// try to add for validation
	err = l.addTransaction(trx.Format(false))

	if err != nil {
		return trx, fmt.Errorf("unable to validate transaction: %v", err)
	}

	return trx, nil

}

func parseConfig(r io.Reader, c *Config) error {
	err := yaml.NewDecoder(r).Decode(c)
	if err != nil {
		slog.Warn("error decoding config file", "error", err)
		return err
	}
	return nil
}

func (l *Ledger) setConfig() error {
	if l.Config == nil {
		l.Config = &Config{}
	}
	const configFile = "teledger.yaml"
	r, err := l.repo.Open(configFile)
	if err == nil {
		err = parseConfig(r, l.Config)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		fmt.Println("unable to open config file", "error", err)
		return err
	}
	// set defaults:
	if l.Config.MainFile == "" {
		l.Config.MainFile = "main.ledger"
	}

	if l.Config.PromptTemplate == "" {
		l.Config.PromptTemplate = defaultPromtpTemplate
	}

	if l.Config.Version == "" {
		l.Config.Version = "0"
	}

	return nil
}

type ProposeTransactionRespones struct {
	// If the user provided a valid transaction as
	// a description, it will be stored here
	UserProvidedTransaction string
	// If the user provided just a human-readable description
	// of the transaction, the proposed transaction will be stored here
	GeneratedTransaction *Transaction
	// It's possible that a transaction was generated, but it's invalid
	Error error
	// Attempt from which the transaction was generated
	AttemptNumber int
	Committed      bool
}

func (l *Ledger) AddOrProposeTransaction(userInput string, attempts int) ProposeTransactionRespones {
	resp := ProposeTransactionRespones{}

	err := l.repo.Init()
	defer l.repo.Free()
	if err != nil {
		resp.Error = err
		return resp
	}

	err = l.setConfig()
	if err != nil {
		resp.Error = err
		return resp
	}

	// first try to add userInput as transaction
	err = l.addTransaction(userInput)
	if err == nil {
		// if user input was a valid transaction, commit it
		err = l.repo.CommitPush("New comment", "teledger", "teledger@example.com")
		resp.UserProvidedTransaction = userInput
		if err != nil {
			resp.Error = err
			return resp
		}
		resp.Committed = true
		return resp
	}

	// if the error starts with "invalid transaction", it means that
	// the user input was not a valid transaction.
	// All other errors are returned as is, without trying to generate a transaction
	if !strings.HasPrefix(err.Error(), "invalid transaction:") {
		resp.Error = err
		return resp
	}

	if attempts <= 0 {
		panic("times should be greater than 0")
	}

	var addErr error
	var tr Transaction

	for i := 1; i <= attempts; i++ {
		if i > 1 {
			slog.Warn("retrying transaction generation", "attempt", i)
		}
		tr, addErr = l.proposeTransaction(userInput)
		resp.Error = addErr
		resp.GeneratedTransaction = &tr
		resp.AttemptNumber = i
		if addErr == nil {
			return resp
		}
	}

	return resp
}

type PromptCtx struct {
	Accounts    []string
	Commodities []string
	UserInput   string
	Datetime    time.Time
}

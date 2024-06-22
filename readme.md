## Teledger
Service which should combine powerful double-entry accounting system [ledger](https://ledger-cli.org/), with reliability of a git as a Ledger file storage, and convenience of data entry using Telegram and OpenAI LLM.

## Development
Create `.env.dev` by copying `.env.example` and fill in the values.
```bash
cp .env.example .env.dev
```

Source the `.env.dev` file to set the environment variables
```bash
source .env.dev
```

Start the service
```bash
go run app/main.go
```


## TODO
- [ ] Propose transactions with openAI
- [ ] Commit transaction with openAI (By pressing confirm btn)
- [ ] Add transaction directly when a transaction typed instead of openAI
- [ ] Add templates for reports with yaml in repo

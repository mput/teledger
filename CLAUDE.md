# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Running the application
```bash
# Development mode (sources .env.dev)
make start

# Alternative direct run
go run app/main.go
```

### Testing
```bash
# Run all tests (sources .env.test)
make test

# Alternative direct test
go test -v ./...
```

### Environment Setup
- Copy `.env.example` to `.env.dev` and fill in required values
- Source the environment file: `source .env.dev`
- Required environment variables:
  - `TELEGRAM_TOKEN`: Telegram bot token
  - `GITHUB_URL`: GitHub repository URL for ledger files
  - `GITHUB_TOKEN`: Fine-grained personal access token with RW Contents scope
  - `OPENAI_TOKEN`: OpenAI API token
  - `BASE_URL`: Base URL of the bot service

### Development Workflow
- Use `make lint` to do linting
- Before commit always do a `make format`
- Do the commit message short. before commit, ask me if I like the commit message.
- Never commit if not asked for

### Git Hooks
- Pre-commit hook automatically runs linting and formatting checks
- To install hooks on a new clone: `make install-hooks`
- Hook script is version controlled in `scripts/pre-commit-hook.sh`

## Architecture Overview

### Core Components

**Main Application (`app/main.go`)**
- Entry point that starts both the Telegram bot and HTTP server
- HTTP server runs on port 8080 for the Mini App at `/bot/miniapp`
- Uses `go-flags` for command-line argument parsing

**Bot Layer (`app/bot/`)**
- `bot.go`: Main bot implementation using gotgbot/v2
- `miniapp.go`: Telegram Mini App handler
- `templates/`: HTML templates for transaction proposals

**Teledger Service (`app/teledger/`)**
- Core service layer managing ledger operations
- Handles pending transactions and confirmations
- Provides transaction generation and validation

**Ledger Integration (`app/ledger/`)**
- Interfaces with ledger-cli for double-entry accounting
- Manages ledger file operations and validation
- Uses OpenAI for natural language transaction parsing

**Repository Management (`app/repo/`)**
- Git repository operations for ledger file storage
- Handles cloning, committing, and pushing changes
- Integrates with GitHub using personal access tokens

### Key Dependencies
- `gotgbot/v2`: Telegram Bot API
- `go-git/go-git/v5`: Git operations
- `sashabaranov/go-openai`: OpenAI API integration
- `jessevdk/go-flags`: Command-line argument parsing

### Data Flow
1. User sends message to Telegram bot
2. Bot clones Git repository containing ledger files
3. Extracts available accounts and commodities from ledger
4. Uses OpenAI to generate transaction from natural language
5. Validates transaction against existing ledger data
6. Commits and pushes validated transaction to repository

### Configuration
- `teledger.yaml` in repository root for ledger-specific settings
- Supports strict mode for account/commodity validation
- Configurable reports using ledger-cli commands
- Template customization for transaction prompts

### Testing
- Uses standard Go testing with `testify` for assertions
- Mock implementations available for repository operations
- Test environment configured via `.env.test`

### Known Behaviors
- `make start` starts web server, so it never finishes execution
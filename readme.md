# Teledger
Service which should combine powerful double-entry accounting system [ledger](https://ledger-cli.org/), with reliability of a Git as a Ledger file storage, and convenience of data entry using Telegram and OpenAI LLM.

## How It Works

Teledger is a Telegram bot service that manages Plain-Text Accounting files stored in a Git repository. 
It provides a convenient way to add transactions on the go using natural language. 
Pro `ledger-cli` users can still manually edit Ledger files using a text editor and ledger-cli utility for complex transactions or other changes where the bot interface may not be suitable.

### Process Overview

- **Initiate Transaction**: You send a message to Teledger describing the transaction.
- **Data Extraction**: Teledger clones your Git repository and extracts necessary information such as available accounts and commodities from the Ledger files.
- **Generate and Validate Transaction**: Teledger constructs a prompt and consults ChatGPT to generate a new transaction entry. This transaction is validated against the data in your Ledger repository to ensure accuracy.
- **Commit Changes**: Once the transaction passes validation, it is committed and pushed back into the repository, updating your financial records automatically.

## Ledger Template Repository

A [template repository](https://github.com/mput/teledger-test) is available to help set up a new Ledger project.

## Configuration

### Service Configuration

Configure the service by setting the following environment variables or directly passing them as command-line arguments:

- **Telegram**:
  - `--telegram.token=`, `$TELEGRAM_TOKEN` - Telegram bot token.

- **GitHub**:
  - `--github.url=`, `$GITHUB_URL` - GitHub repository URL.
  - `--github.token=`, `$GITHUB_TOKEN` - Fine-grained personal access tokens for the repository with RW Contents scope.

- **OpenAI**:
  - `--openai.token=`, `$OPENAI_TOKEN` - OpenAI API token.

### Ledger File Configuration

The `teledger.yaml` configuration file may be placed in the root of your Ledger project repository. 
It includes settings specific to your ledger environment. Here is the structure of the expected YAML file:

- **mainFile**: Specifies the main ledger file name, default is `main.ledger`.
- **strict**: Boolean to allow or disallow non-existing accounts and commodities.
- **promptTemplate**: Template for generating prompts, optional.
- **reports**: Array of report configurations where each report includes:
  - **title**: Description of the report.
  - **command**: Ledger-cli command array to generate the report.

Example configuration in [`teledger.yaml`](https://github.com/mput/teledger-test/blob/main/teledger.yaml):
```yaml
strict: true
reports:
  - title: Expenses This Month
    command: [bal, ^Expenses, --cleared, --period, "this month", -X, EUR]
  - title: Expenses Last Month
    command: [bal, ^Expenses, --cleared, --period, "last month", -X, EUR]
  - title: 💶 Assets
    command: [bal, ^Assets]
```

## Demo
TODO

## Deploment
A Docker image for Teledger is available at [Docker Hub](https://hub.docker.com/repository/docker/mput/teledger/general).

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


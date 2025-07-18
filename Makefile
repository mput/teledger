start:
	. ./.env.dev && go run app/main.go

start-op:
	op run --no-masking --env-file="./.env.dev.1password" -- go run app/main.go

test:
	. ./.env.test && go test -v ./...

test-op:
	op run --no-masking --env-file="./.env.test.1password" -- go test -v ./...

format:
	gofumpt -w .

lint:
	golangci-lint run

install-hooks:
	chmod +x scripts/pre-commit-hook.sh
	cp scripts/pre-commit-hook.sh .git/hooks/pre-commit
	echo "#!/bin/bash" > .git/hooks/pre-commit
	echo "./scripts/pre-commit-hook.sh" >> .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit

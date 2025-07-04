start:
	. ./.env.dev && go run app/main.go

test:
	. ./.env.test && go test -v ./...

fmt:
	gofumpt -w .

lint:
	golangci-lint run

format: fmt lint

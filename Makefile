
start:
	. ./.env.dev
	go run app/main.go

test:
	. ./.env.test
	go test -v ./...

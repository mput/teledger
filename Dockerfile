FROM golang:1.22.0-alpine as build


ADD ./ /repo
WORKDIR /repo

ARG VERSION=docker-dev
RUN echo go version: `go version`
RUN echo build version: ${VERSION}

RUN go build -o teledger -ldflags "-X main.version=${VERSION} -s -w" ./app/main.go

FROM golang:1.22.0-alpine

WORKDIR /srv
RUN apk add --no-cache ledger=~3.3.2 && echo ledger: `which ledger`
COPY --from=build /repo/teledger /srv/teledger

CMD ["/srv/teledger"]

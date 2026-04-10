.PHONY: build build-linux test lint fmt coverage clean

BINARY_NAME=apiretry

build:
	go build -o bin/$(BINARY_NAME) ./cmd/proxy/

build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/proxy/

test:
	go test ./... -timeout 60s -v

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

coverage:
	go test ./... -coverprofile=coverage.out -timeout 60s
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf bin/ coverage.out coverage.html
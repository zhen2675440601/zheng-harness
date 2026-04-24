.PHONY: fmt lint test test-race test-cover

fmt:
	gofmt -w .

lint:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -cover ./...

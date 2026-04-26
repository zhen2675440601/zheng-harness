.PHONY: fmt lint test test-race test-cover notecheck

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

notecheck:
	powershell -ExecutionPolicy Bypass -File ./scripts/check-notepads.ps1

.PHONY: help test test-cov coverage vet lint fmt tidy build check clean

help:
	@echo "go.dagstack.dev/logger — Go binding for dagstack/logger-spec"
	@echo ""
	@echo "Targets:"
	@echo "  test          go test -race ./..."
	@echo "  test-cov      go test with coverage report"
	@echo "  coverage      alias for test-cov"
	@echo "  vet           go vet ./..."
	@echo "  lint          go vet + staticcheck (when installed)"
	@echo "  fmt           gofmt -s -w ."
	@echo "  tidy          go mod tidy"
	@echo "  build         go build ./..."
	@echo "  check         vet + test + build"
	@echo "  clean         rm -rf coverage.out coverage.html"

test:
	go test -race ./...

test-cov:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1
	go tool cover -html=coverage.out -o coverage.html
	@echo "coverage report: coverage.html"

coverage: test-cov

vet:
	go vet ./...

lint: vet
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed (skip)"; \
	fi

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

build:
	go build ./...

check: vet test build

clean:
	rm -f coverage.out coverage.html

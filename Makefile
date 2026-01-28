.PHONY: help fmt test vet lint build check

help:
	@echo "Targets:"
	@echo "  fmt   - gofmt -w ."
	@echo "  test  - go test ./..."
	@echo "  vet   - go vet ./..."
	@echo "  lint  - go vet ./... (and staticcheck if installed)"
	@echo "  build - go build -o moleman"
	@echo "  check - fmt, test, vet"

fmt:
	gofmt -w .

test:
	go test ./...

vet:
	go vet ./...

lint:
	go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed; skipping"; \
	fi

build:
	go build -o moleman

check: fmt test vet

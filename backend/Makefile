BINARY  ?= bin/server
VERSION ?= dev

.PHONY: run build test lint docs tidy

run: ## run the API locally (sqlite, console mailer)
	go run .

build: ## build a static (CGO-free) binary
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINARY) .

test: ## run all tests
	go test ./...

lint: ## static checks
	go vet ./...

docs: ## regenerate routers/commentsRouter*.go and swagger/swagger.{json,yml} (needs: go install github.com/beego/bee/v2@latest)
	bee generate routers
	bee generate docs

tidy:
	go mod tidy

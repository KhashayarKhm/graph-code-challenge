TEST_ENV_FILE ?= $(CURDIR)/.env.test
COVERAGE_FILE ?= coverage.out
COVER_PKGS := ./internal/...

.PHONY: help build run vet test test-unit test-integration coverage coverage-html

help:
	@echo "build             compile every package"
	@echo "run               run the API"
	@echo "vet               go vet"
	@echo "test-unit         fast tests, no database required"
	@echo "test-integration  every test including build-tagged ones, needs postgres-up"
	@echo "test              test-unit then test-integration"
	@echo "coverage          merged unit + integration profile, prints the total"
	@echo "coverage-html     open the line-by-line coverage report"

build:
	go build ./...

run:
	go run ./cmd/api

vet:
	go vet ./...
	go vet -tags=integration ./...

test-unit:
	go test ./...

# TEST_ENV_FILE points at its own throwaway database, never the dev one, and is
# absolute because `go test` runs each package with its own directory as the
# working directory, so a bare .env lookup misses the repo root.
test-integration:
	ENV_FILE=$(TEST_ENV_FILE) go test -tags=integration -count=1 ./...

test: test-unit test-integration

coverage:
	ENV_FILE=$(TEST_ENV_FILE) go test -tags=integration -count=1 \
		-coverpkg=$(COVER_PKGS) -coverprofile=$(COVERAGE_FILE) $(COVER_PKGS)
	@go tool cover -func=$(COVERAGE_FILE) | tail -1

coverage-html: coverage
	go tool cover -html=$(COVERAGE_FILE)

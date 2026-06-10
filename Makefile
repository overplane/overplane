SHELL := /usr/bin/env bash
BINARY := overplane
VERSION := $(shell tr -d '\n' < VERSION)
COMMIT := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo dev)
DATE := $(shell if [[ -n "$$SOURCE_DATE_EPOCH" ]]; then date -u -d "@$$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ; else date -u +%Y-%m-%dT%H:%M:%SZ; fi)
COVERAGE_MIN ?= 80.0
LDFLAGS := -s -w -buildid= -X github.com/overplane/overplane/internal/platform/version.Version=$(VERSION) -X github.com/overplane/overplane/internal/platform/version.Commit=$(COMMIT) -X github.com/overplane/overplane/internal/platform/version.Date=$(DATE)
TMPBIN := $(CURDIR)/.tmp/bin
export GOBIN := $(TMPBIN)
export PATH := $(TMPBIN):$(PATH)
export GOTOOLCHAIN := go1.26.4

.PHONY: ci all gate-go-version gate-generate gate-fmt gate-vet gate-lint gate-test gate-integration gate-coverage build clean dist dev ci-rehearse

all: ci

ci: gate-go-version gate-generate gate-fmt gate-vet gate-lint gate-test gate-integration gate-coverage build

gate-go-version:
	./scripts/checkgoversion

gate-generate:
	go generate ./...
	git diff --exit-code -- internal/platform/embed/assets/assets_gen.go

gate-fmt:
	@mkdir -p "$(TMPBIN)"
	@command -v goimports >/dev/null || go install golang.org/x/tools/cmd/goimports@latest
	gofmt -w $$(find . -name '*.go' -not -path './dist/*' -not -path './.tmp/*')
	goimports -w $$(find . -name '*.go' -not -path './dist/*' -not -path './.tmp/*')
	git diff --exit-code -- '*.go'

gate-vet:
	go vet ./...

gate-lint:
	@mkdir -p "$(TMPBIN)"
	@rm -f "$(TMPBIN)/golangci-lint"
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	golangci-lint run ./...

gate-test:
	@mkdir -p .coverage
	go test -race -covermode=atomic -coverprofile=.coverage/unit.out ./...

gate-integration:
	@mkdir -p .coverage
	go test -tags=integration -covermode=atomic -coverprofile=.coverage/integration.out ./...

gate-coverage:
	@mkdir -p .coverage
	@if [[ -f .coverage/unit.out && -f .coverage/integration.out ]]; then \
	  awk 'FNR==1 && NR!=1 { next } { print }' .coverage/unit.out .coverage/integration.out > .coverage/coverage.out; \
	elif [[ -f .coverage/unit.out ]]; then cp .coverage/unit.out .coverage/coverage.out; \
	else echo 'coverage profile missing' >&2; exit 1; fi
	COVERAGE_MIN=$(COVERAGE_MIN) ./scripts/coveragegate .coverage/coverage.out

build:
	@mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -buildvcs=false -o dist/$(BINARY) ./cmd/$(BINARY)

clean:
	rm -rf dist .coverage .tmp

dev:
	@if command -v entr >/dev/null; then find . -name '*.go' | entr -r ./cli.sh; else while sleep 2; do ./cli.sh version; done; fi

dist: clean ci
	@mkdir -p dist
	@for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do \
	  os="$${target%/*}"; arch="$${target#*/}"; ext=""; [[ "$$os" == windows ]] && ext=".exe"; \
	  GOOS="$$os" GOARCH="$$arch" CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -buildvcs=false -o "dist/$(BINARY)-$$os-$$arch$$ext" ./cmd/$(BINARY); \
	done
	(cd dist && sha256sum $(BINARY)-* > SHA256SUMS)

ci-rehearse:
	tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; git clone --quiet --local .. "$$tmp/repo"; cd "$$tmp/repo/go-core"; CI=true make ci

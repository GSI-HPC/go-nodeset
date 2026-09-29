# SPDX-FileCopyrightText: 2026 GSI Helmholtz Centre for Heavy Ion Research GmbH <http://www.gsi.de>
# SPDX-License-Identifier: Apache-2.0

GO       ?= go
MODULE   := github.com/GSI-HPC/go-nodeset
COVER    ?= coverage.out
FUZZTIME ?= 90s

.PHONY: all
all: lint test

## test: run the tests under the race detector
.PHONY: test
test:
	$(GO) test -race ./...

## floor: vet and test with the oldest Go release go.mod allows
.PHONY: floor
floor:
	GOTOOLCHAIN=go$$($(GO) list -m -f '{{.GoVersion}}') $(GO) vet ./...
	GOTOOLCHAIN=go$$($(GO) list -m -f '{{.GoVersion}}') $(GO) test ./...

## cover: measure coverage and hold every file to 100% (.testcoverage.yml)
.PHONY: cover
cover:
	$(GO) test -race -coverprofile=$(COVER) -covermode=atomic ./...
	go-test-coverage --config .testcoverage.yml

## clustershell: compare with ClusterShell itself, and record the corpus again (needs python3)
CLUSTERSHELL       ?= .clustershell
CLUSTERSHELL_CASES ?= 20000
.PHONY: clustershell
clustershell: $(CLUSTERSHELL)/installed
	NODESET_CLUSTERSHELL_PYTHON=$(abspath $(CLUSTERSHELL)/bin/python) NODESET_CLUSTERSHELL_CASES=$(CLUSTERSHELL_CASES) \
		$(GO) test -count=1 -run '^TestClusterShellOracle$$' -v .
	cp testdata/clustershell.txt $(CLUSTERSHELL)/clustershell.txt
	cd testdata && $(abspath $(CLUSTERSHELL)/bin/python) clustershell.py
	@diff -u $(CLUSTERSHELL)/clustershell.txt testdata/clustershell.txt || { echo "ClusterShell answers the corpus differently; testdata/clustershell.txt now holds its answers"; exit 1; }

# A stamp of its own, since bin/python is a link to the system's Python.
$(CLUSTERSHELL)/installed: testdata/requirements.txt
	python3 -m venv $(CLUSTERSHELL)
	$(CLUSTERSHELL)/bin/pip install --quiet --requirement testdata/requirements.txt
	touch $@

## fuzz: fuzz the parser for FUZZTIME (90s); a failing input lands in testdata/fuzz/
.PHONY: fuzz
fuzz:
	$(GO) test -run '^$$' -fuzz FuzzParseFold -fuzztime $(FUZZTIME) .

## lint: golangci-lint (.golangci.yml), gofmt and goimports included
.PHONY: lint
lint:
	golangci-lint run

## tidy: prune the requirements and check that there are none
.PHONY: tidy
tidy:
	$(GO) mod tidy
	@test "$$($(GO) list -m all)" = "$(MODULE)" || { echo "go.mod requires modules; it must require none (doc/decisions.md, decision 3)"; exit 1; }

## vuln: report known vulnerabilities in the code and the toolchain
.PHONY: vuln
vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

## reuse: check that every file states its copyright and licence
.PHONY: reuse
reuse:
	reuse lint

## lint-docs: lint the Markdown files (.markdownlint.yaml)
.PHONY: lint-docs
lint-docs:
	npx --yes markdownlint-cli2 "**/*.md" ".agents/**/*.md" "#node_modules" "#.claude"

## test-release: test the release tag verification against scratch tags
.PHONY: test-release
test-release:
	.github/scripts/verify-release-tag_test.sh

## clean: remove test output
.PHONY: clean
clean:
	rm -f $(COVER) coverage.html

## help: list the available targets
.PHONY: help
help:
	@sed -n 's/^## //p' $(MAKEFILE_LIST)

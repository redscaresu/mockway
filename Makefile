.PHONY: install-hooks build test test-race test-short test-coverage test-examples test-misconfigured vet clean run spec-diff spec-diff-all

# install-hooks wires the tracked hook installer at .githooks/ via
# core.hooksPath so the gitleaks + go test pre-commit gate runs locally
# on every commit. Mirrors fakeaws/fakegcp pattern.
install-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "Hooks installed: pre-commit will run gitleaks then go test."

build:
	go build -o mockway ./cmd/mockway

test:
	go test -count=1 ./...

test-race:
	go test -count=1 -race ./...

test-short:
	go test -count=1 -short ./...

# Aggregate handlers/... coverage. Mirrors fakegcp/fakeaws.
test-coverage:
	go test -count=1 -coverprofile=cov.out -covermode=atomic ./handlers/...
	@go tool cover -func=cov.out | tail -1
	@go tool cover -html=cov.out -o coverage.html
	@echo "coverage report: coverage.html"

# Mockway-specific shell-script examples harness.
test-examples:
	./scripts/test-examples.sh

test-misconfigured:
	./scripts/test-misconfigured.sh

vet:
	go vet ./...

clean:
	rm -f mockway cov.out coverage.html

run: build
	./mockway --port 8080

# Show high-priority gaps: routes the Terraform provider calls during apply/destroy
# that mockway does not yet implement.
spec-diff:
	python3 scripts/spec_diff.py

# Show all gaps against the Scaleway OpenAPI specs (including low-priority routes).
spec-diff-all:
	python3 scripts/spec_diff.py --all

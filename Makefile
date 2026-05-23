.PHONY: install-hooks test test-examples test-misconfigured spec-diff spec-diff-all

# install-hooks wires the tracked hook installer at .githooks/ via
# core.hooksPath so the gitleaks + go test pre-commit gate runs locally
# on every commit. Mirrors fakeaws/fakegcp pattern.
install-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "Hooks installed: pre-commit will run gitleaks then go test."

test:
	go test ./...

test-examples:
	./scripts/test-examples.sh

test-misconfigured:
	./scripts/test-misconfigured.sh

# Show high-priority gaps: routes the Terraform provider calls during apply/destroy
# that mockway does not yet implement.
spec-diff:
	python3 scripts/spec_diff.py

# Show all gaps against the Scaleway OpenAPI specs (including low-priority routes).
spec-diff-all:
	python3 scripts/spec_diff.py --all

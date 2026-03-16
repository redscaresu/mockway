.PHONY: test test-examples test-misconfigured spec-diff spec-diff-all

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

.PHONY: install-hooks build test test-race test-short test-coverage test-examples test-misconfigured vet clean run spec-diff spec-diff-all demo-help demo-up demo-down demo-env demo-shell demo-apply demo-destroy demo-clean

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

# ─── demo targets ───────────────────────────────────────────────────────
# Drive a real scaleway provider through init → apply → plan-no-op → destroy
# against a local mockway. Useful for blog demos, manual exploration, and
# proving the wire-shape contract end-to-end.
#
#   make demo-apply                       # one-shot: up + apply + plan-no-op (default: lb_with_ip)
#   make demo-apply EXAMPLE=load_balancer # pick a different example
#   make demo-shell                       # bash subshell with env set + cd'd to example
#   make demo-down                        # kill mockway + remove temp files
#
# Override the example with EXAMPLE=<dir> (any subdir of examples/working/).
DEMO_PORT      ?= 8080
EXAMPLE        ?= lb_with_ip
DEMO_EXAMPLE_DIR := examples/working/$(EXAMPLE)
DEMO_ENV_FILE  := /tmp/mockway.env
DEMO_BASE      := http://localhost:$(DEMO_PORT)
DEMO_BIN       := $(shell command -v tofu 2>/dev/null || command -v terraform 2>/dev/null)

demo-help:
	@echo "Demo targets (drive real terraform/tofu against this mockway):"
	@echo "  demo-up                        boot mockway + write env to /tmp"
	@echo "  demo-apply [EXAMPLE=<dir>]     one-shot: init + apply + plan-no-op"
	@echo "  demo-shell [EXAMPLE=<dir>]     bash subshell with env set + cd'd to example"
	@echo "  demo-destroy [EXAMPLE=<dir>]   tofu destroy on the current example"
	@echo "  demo-down                      kill mockway + remove temp files"
	@echo "  demo-clean                     demo-destroy + nuke .terraform/ + state files"
	@echo ""
	@echo "Available examples:"
	@ls examples/working/ | sed 's/^/  /'

demo-up:
	@if pgrep -f "mockway --port $(DEMO_PORT)" >/dev/null 2>&1; then \
	  echo "✓ mockway already running on :$(DEMO_PORT)"; \
	else \
	  [ -x ./mockway ] || { echo "ERROR: ./mockway binary not found. Run 'make build' first." >&2; exit 1; }; \
	  ./mockway --port $(DEMO_PORT) --db ':memory:' >/tmp/mockway.log 2>&1 & \
	  for i in 1 2 3 4 5 6 7 8 9 10; do sleep 0.5; curl -sf $(DEMO_BASE)/mock/state >/dev/null 2>&1 && break; done; \
	  echo "✓ mockway booted on :$(DEMO_PORT)  (logs: /tmp/mockway.log)"; \
	fi
	@{ \
	  echo 'export SCW_API_URL=$(DEMO_BASE)'; \
	  echo 'export SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX'; \
	  echo 'export SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000'; \
	  echo 'export SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000'; \
	  echo 'export SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000'; \
	  echo 'export SCW_DEFAULT_REGION=fr-par'; \
	  echo 'export SCW_DEFAULT_ZONE=fr-par-1'; \
	} > $(DEMO_ENV_FILE)
	@echo "✓ env written to $(DEMO_ENV_FILE)"

demo-down:
	@pkill -f "mockway --port $(DEMO_PORT)" 2>/dev/null && echo "✓ killed" || echo "✓ nothing to kill"
	@rm -f $(DEMO_ENV_FILE)

demo-env: demo-up
	@cat $(DEMO_ENV_FILE)

demo-shell: demo-up
	@[ -d "$(DEMO_EXAMPLE_DIR)" ] || { echo "ERROR: $(DEMO_EXAMPLE_DIR) not found" >&2; exit 1; }
	@echo "→ entering subshell with mockway env. Type 'exit' to leave."
	@cd $(DEMO_EXAMPLE_DIR) && /bin/bash --rcfile <(echo "source ~/.bashrc 2>/dev/null; source $(DEMO_ENV_FILE); PS1='[mockway $(EXAMPLE)] $$PS1'")

demo-apply: demo-up
	@[ -n "$(DEMO_BIN)" ] || { echo "ERROR: neither tofu nor terraform on PATH" >&2; exit 1; }
	@[ -d "$(DEMO_EXAMPLE_DIR)" ] || { echo "ERROR: $(DEMO_EXAMPLE_DIR) not found" >&2; exit 1; }
	@set -e; . $(DEMO_ENV_FILE); cd $(DEMO_EXAMPLE_DIR); \
	  echo "=== $(DEMO_BIN) init ==="; $(DEMO_BIN) init -input=false; \
	  echo ""; echo "=== $(DEMO_BIN) apply ==="; $(DEMO_BIN) apply -auto-approve -input=false; \
	  echo ""; echo "=== $(DEMO_BIN) plan -detailed-exitcode (brutal correctness check) ==="; \
	  if $(DEMO_BIN) plan -detailed-exitcode -input=false >/dev/null 2>&1; then \
	    echo "✓ exit 0 — wire shape correct (real provider's state matches mockway's responses)."; \
	  else \
	    echo "✗ exit $$? — drift detected."; exit 1; \
	  fi

demo-destroy:
	@[ -n "$(DEMO_BIN)" ] || { echo "ERROR: neither tofu nor terraform on PATH" >&2; exit 1; }
	@[ -f $(DEMO_ENV_FILE) ] || { echo "ERROR: no env file — run 'make demo-up' first" >&2; exit 1; }
	@set -e; . $(DEMO_ENV_FILE); cd $(DEMO_EXAMPLE_DIR); $(DEMO_BIN) destroy -auto-approve -input=false

demo-clean:
	@-$(MAKE) demo-destroy 2>/dev/null
	@find examples/working -name '.terraform' -type d -prune -exec rm -rf {} + 2>/dev/null || true
	@find examples/working -name '.terraform.lock.hcl' -delete 2>/dev/null || true
	@find examples/working -name 'terraform.tfstate*' -delete 2>/dev/null || true
	@echo "✓ terraform state cleaned"

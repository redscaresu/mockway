# Contributing to mockway

`mockway` is the Scaleway-side mock server for the [InfraFactory](https://github.com/redscaresu/infrafactory) project. It simulates Scaleway APIs against a local SQLite database so InfraFactory's Layer 2 validation can run offline without real API calls.

## TL;DR

1. Open an issue first for non-trivial changes.
2. Each PR is one focused change (a handler, a fidelity fix, a regression test).
3. `make test` must be green.
4. Pre-commit hook (`make install-hooks`) runs `gitleaks` + `go test`.

## Setup

Required: Go 1.24+, `make`. Optional: `gitleaks` for the pre-commit hook.

```bash
git clone https://github.com/redscaresu/mockway.git
cd mockway
make install-hooks
make test
make run    # serves the mock at :8080
```

## Workflow

1. Pick a focused change — usually one handler file, one repository method, or one regression test.
2. Add a handler test in `handlers/handlers_test.go` (success path + 404 + FK-violation path where applicable).
3. Run `make test` locally.
4. Update `README.md` / `AGENTS.md` if the API surface or workflow changed.
5. Open a PR referencing the InfraFactory ticket (if any) or describing the fidelity gap being closed.

## Fidelity issues

`mockway` aims for terraform-provider-scaleway-level fidelity — enough that an HCL plan + apply + destroy cycle against mockway behaves the same as against real Scaleway, modulo documented exceptions in `README.md` § "Mock fidelity limitations".

If you find a case where terraform-provider-scaleway behaves differently against mockway than against real Scaleway, that's a **fidelity issue**. File it with the `fidelity` label and include:

- The exact terraform-provider-scaleway version.
- The HCL block that triggers the divergence.
- The raw HTTP request + response from both real Scaleway and mockway.

## Code of Conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Contributor Covenant v2.1.

## License

By contributing, you agree your work will be released under Apache-2.0.

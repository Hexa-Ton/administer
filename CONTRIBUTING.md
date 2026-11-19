# Contributing

Issues and pull requests are welcome.

## Before opening a PR

    gofmt -l .          # must print nothing
    go vet ./...
    go test ./...
    go test -race ./...

All four must pass. Add or update tests for any behavior change — see
`internal/*/\*_test.go` for the existing style (table-driven where it fits,
one behavior per test name).

## Project layout

- `internal/cli` — argument parsing and the four command groups
- `internal/scan` — ties wordlist + engine + scheduler + ui + report together
- `internal/engine` / `internal/scheduler` — the concurrent scan core
- `internal/netx` — HTTP client policy and rate limiting
- `internal/classify` — candidate/wildcard classification
- `wordlists/` — the embedded built-in wordlist
- `python/` — optional wordlist tooling, never on the scan's critical path

## Design rules this project holds to

- Standard library only, both Go and Python — no new dependencies without
  a real reason in the PR description.
- The mandatory six paths (`/admin`, `/login`, `/admin/login`,
  `/login/admin`, `/owner`, `/master`) stay first, in that order, always.
- A scan stops on the first real candidate — never add anything that waits
  for more results once one is found.
- No credential guessing, auth bypass, or exploitation of any kind.

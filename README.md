# administer

**Concurrent web path & admin-panel discovery — Go engine, Python wordlist tooling.**

[![CI](https://github.com/Hexa-Ton/administer/actions/workflows/ci.yml/badge.svg)](https://github.com/Hexa-Ton/administer/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go 1.21+](https://img.shields.io/badge/go-1.21%2B-00ADD8.svg)](go.mod)

> **Authorized use only.** Run this against targets you own or have
> explicit, documented permission to test. See [SECURITY.md](SECURITY.md).

```
$ administer scan panelbajar.com
administer v1.0.0
target:   https://panelbajar.com
wordlist: built-in (1800 paths)
engines:  20
scanning...
[engine 001] /admin                         -> 302 -> https://panelbajar.com/admin/login (372ms)
[engine 005] /control                       -> cancelled (scan already stopped)
...
[FOUND] https://panelbajar.com/admin/login
checked 28/1800 paths in 392ms
```

## Why this exists

Most directory-brute-force tools either dump every hit and make you sift
through them, or they're slow about it. administer does one specific job
well: **find the first real admin/login surface on a target, as fast as
possible, and stop** — with the result shown in neon green, not buried in
scrollback.

## Features

- **Concurrent engines, not a single worker loop.** Up to 213 instances
  pull from one shared job queue; each path is checked exactly once.
- **Stops on the first real hit.** The moment a response is classified as
  a genuine candidate, every in-flight request is cancelled — no waiting
  for the rest of the wordlist.
- **Wildcard-aware.** A short calibration probe fingerprints the target's
  "everything redirects to a login page" behavior up front, so a blanket
  auth wall doesn't register as 1,800 false positives.
- **Built-in, embedded wordlist.** 1,800 unique, hand-curated real-world
  paths (admin panels, login endpoints, CMS-specific routes, config/backup
  files, API conventions, devops/cloud endpoints) — compiled into the
  binary, so there's no file to lose track of.
- **Fails fast on a dead target.** One reachability probe before the real
  scan; an offline or nonexistent host is reported in under a second
  instead of timing out on 1,800 individual paths.
- **Safe by default.** TLS verified, redirects reported but not followed,
  no credential attempts, no automatic retries that could hammer a target.
- **Zero runtime dependencies.** Standard library only, both Go and
  Python — builds and runs with nothing to `go get` or `pip install`.

## Install

### Linux (including Kali) / macOS

```sh
git clone https://github.com/Hexa-Ton/administer.git
cd administer
go build -o administer .
sudo mv administer /usr/local/bin/   # or anywhere on your PATH

administer doctor
```

### Termux (Android)

```sh
pkg update && pkg upgrade -y
pkg install golang git -y

git clone https://github.com/Hexa-Ton/administer.git
cd administer
go build -o administer .

mkdir -p ~/bin
cp administer ~/bin/
echo 'export PATH=$HOME/bin:$PATH' >> ~/.bashrc
source ~/.bashrc

administer doctor
```

### Go install

```sh
go install github.com/Hexa-Ton/administer@latest
```

## Usage

```sh
administer scan example.com
administer scan https://example.com -engines 100 -timeout 5s
administer scan example.com -o report.json
administer wordlist info
administer wordlist export > my-paths.txt
administer config set engines 50
administer doctor
```

Full reference: `administer -h`.

| Command | Does |
|---|---|
| `scan <target> [opts]` | Run a scan. Target as `-u`/`-d` or bare positional. |
| `wordlist info` | Show the built-in wordlist's size and integrity. |
| `wordlist validate <path>` | Check a custom wordlist before using it with `-w`. |
| `wordlist export [path]` | Write the built-in list to a file (or stdout). |
| `config show` / `config set <key> <value>` | View or change saved defaults. |
| `doctor` | Environment diagnostics: Go/OS info, python3, wordlist integrity, config. |
| `version` | Print the version. |

**Exit codes** (for scripting): `0` found · `1` error · `2` finished, nothing found · `130` interrupted.

## How it works

1. **Reachability probe** — one request to a random, near-certainly-fake
   path. A connection-level failure here means the target is unreachable;
   administer says so and stops, instead of trying 1,800 paths that would
   all fail the same way. A response (any status) becomes the **wildcard
   baseline** — the fingerprint of "this is what nothing looks like here."
2. **Dispatch** — the six mandatory priority paths (`/admin`, `/login`,
   `/admin/login`, `/login/admin`, `/owner`, `/master`) go into the job
   queue first, then the rest of the wordlist, deduplicated.
3. **Concurrent engines** — up to `-engines` goroutines pull from that
   queue. Each one requests a path, classifies the response against the
   wildcard baseline, and reports back.
4. **First hit wins** — the instant any engine reports a genuine
   candidate, the scan's context is cancelled. Every other in-flight
   request aborts (shown as `cancelled`, not `error` — nothing went
   wrong, the scan just finished). The result prints in neon green.

## The wordlist

1,800 unique entries, embedded in the binary (`wordlists/administer-wordlist.txt`
at build time), organized across admin panels & dashboards, login/auth
endpoints, API conventions, config & backup files, CMS-specific routes
(WordPress, Joomla, Drupal, Magento, and others), devops/cloud endpoints,
and general common paths. No randomly-generated filler — every entry is a
real-world convention. Verify it yourself:

```sh
administer wordlist info
python3 python/wordlist_tools.py stats wordlists/administer-wordlist.txt
```

## Testing

```sh
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```

Tests use `httptest` fixtures only — no requests ever go to a real host
during the test suite. Covers: URL normalization, wordlist parsing and the
mandatory-six order, classification against a wildcard baseline, the
scheduler (no path requested twice, first-hit cancellation, interrupt
handling, immediate shutdown even mid rate-limit-wait), the HTTP client's
redirect policy, and full scan runs end to end (report only on request,
never overwritten, correct exit status for found / not-found / interrupted).

## Status

Working: all four command groups, the full embedded wordlist, concurrent
scanning with first-match-stop, wildcard filtering, fast-fail on an
unreachable target, JSON reporting.

Not yet implemented: 429/`Retry-After` backoff and retry, the deeper
soft-404 and login-panel-specific heuristics beyond status + wildcard
classification, a live `--list-engines` view.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT — see [LICENSE](LICENSE).

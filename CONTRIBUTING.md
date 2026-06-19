# Contributing to RedTrace

Thanks for your interest in improving RedTrace! This document covers how to get
set up and the conventions we follow.

## Development setup

Prerequisites: Go 1.24+, Node 20+, and `make`.

```bash
git clone https://github.com/utkarshrai2811/redtrace.git
cd redtrace
make build          # build the Go binary
make web-install    # install frontend deps
make dev            # run proxy + frontend in dev mode
```

## Branch strategy

- `main` — production releases only (protected; requires PR + green CI).
- `develop` — integration branch and the **default target for PRs**.
- `feat/*`, `fix/*`, `chore/*` — topic branches.

Open pull requests against `develop`.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add WebSocket traffic stream
fix: handle CONNECT to non-standard ports
docs: document CA setup on Windows
chore: bump golangci-lint
refactor: extract cert cache
test: cover scope matching
```

Keep commits focused and write them as a human engineer would.

## Code standards

**Go**

- `gofmt -s` + `goimports` clean (`make fmt`).
- `golangci-lint run` clean (`make lint`) — `errcheck`, `govet`, `staticcheck`,
  `unused`, `gosec`, `exhaustive`.
- No `panic()` in library code — return errors.
- Exported identifiers carry godoc comments.
- Prefer table-driven tests; no global mutable state — inject dependencies.

**TypeScript / React**

- Strict TypeScript (no `any`), ESLint + Prettier clean.
- Functional components and hooks only.

## Tests

```bash
make test   # go test -race ./...
```

Please add tests for new behavior. CI runs lint + tests + build on every PR.

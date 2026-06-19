<div align="center">

<img src="docs/assets/logo.svg" alt="RedTrace" width="120" height="120" />

# RedTrace

**Open-source web security testing platform — a free alternative to Burp Suite Pro.**

[![CI](https://github.com/utkarshrai2811/redtrace/actions/workflows/ci.yml/badge.svg)](https://github.com/utkarshrai2811/redtrace/actions/workflows/ci.yml)
[![Go Reference](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/utkarshrai2811/redtrace?style=social)](https://github.com/utkarshrai2811/redtrace/stargazers)

</div>

---

RedTrace is an intercepting proxy and web-application security toolkit for bug
bounty hunters, penetration testers, and application-security engineers. It is
fully open source (MIT), runs as a single Go binary, and ships with a modern
web UI.

> **Status:** early development. Phase 1 (core proxy) is in progress — see the
> [roadmap](#roadmap).

## Features vs. Burp Suite

| Capability | RedTrace | Burp Community | Burp Pro |
|---|:---:|:---:|:---:|
| Intercepting HTTP/HTTPS proxy | ✅ | ✅ | ✅ |
| Live traffic history + search | ✅ | ✅ | ✅ |
| Repeater | ✅ | ✅ | ✅ |
| Decoder / Comparer | ✅ | ✅ | ✅ |
| Target site map | ✅ | ✅ | ✅ |
| Intruder (all attack types, unthrottled) | 🔜 | ⏳ throttled | ✅ |
| Active + passive scanner | 🔜 | ❌ | ✅ |
| Out-of-band (Collaborator) | 🔜 | ❌ | ✅ |
| AI-assisted triage & payloads | 🔜 | ❌ | ❌ |
| CI/CD scan mode + SARIF export | 🔜 | ❌ | ⏳ |
| Price | **Free** | Free | $$$/yr |

✅ available · 🔜 on the roadmap · ⏳ limited · ❌ not available

## Quick start

> Phase 1 binaries and the Homebrew tap are published once the first release is
> tagged. Until then, build from source.

### From source

Requires Go 1.25+ and Node 20+.

```bash
git clone https://github.com/utkarshrai2811/redtrace.git
cd redtrace
make all          # build the web UI and the binary (embeds the UI)
./bin/redtrace serve
```

The proxy listens on `127.0.0.1:8080` and the UI on `http://127.0.0.1:9090`.
(`make build` alone builds just the Go binary with a placeholder UI; use
`make all` or `make web-build` to embed the real interface. For live frontend
development, `make dev` runs the backend plus the Vite dev server.)

### Docker

```bash
docker run --rm -p 8080:8080 -p 9090:9090 ghcr.io/utkarshrai2811/redtrace:latest
```

### Homebrew (coming with the first release)

```bash
brew install utkarshrai2811/tap/redtrace
```

## CA certificate setup

To intercept HTTPS traffic, configure your browser/OS to use RedTrace as a proxy
(`127.0.0.1:8080`) and trust the RedTrace CA. On first run, the CA is generated
at `~/.config/redtrace/ca/`.

```bash
./scripts/install-ca.sh
```

<details>
<summary>Manual install</summary>

- **macOS:** `security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ~/.config/redtrace/ca/redtrace-ca.pem`
- **Linux (Debian/Ubuntu):** copy `redtrace-ca.pem` to `/usr/local/share/ca-certificates/redtrace-ca.crt` and run `sudo update-ca-certificates`
- **Windows:** `certutil -addstore -f "ROOT" redtrace-ca.pem`
- **Firefox:** Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import `redtrace-ca.pem`

</details>

## Screenshots

> _Added on Phase 1 completion._

## Roadmap

RedTrace is built in phases. See [`docs/phases/`](docs/phases) for detail.

1. Foundation & core proxy — ✅ done
2. Core analysis tools (Repeater, Decoder, Comparer, Site Map) — ✅ done
3. Intruder — _next_
4. Scanner (passive + active, OWASP Top 10)
5. Crawler & Sequencer
6. Out-of-band / Collaborator
7. AI integration
8. CI/CD integration & team collaboration
9. Desktop app (Tauri)
10. Plugin system

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

RedTrace binds to `127.0.0.1` by default, stores no telemetry, and never phones
home. To report a vulnerability in RedTrace itself, please follow the process in
the security issue template rather than opening a public issue.

## License

[MIT](LICENSE) © Utkarsh Rai

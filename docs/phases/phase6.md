# Phase 6 — Out-of-band / Collaborator

A self-hosted out-of-band (OOB) interaction server, the RedTrace analog of Burp
Collaborator. It catches the *blind* callbacks that confirm vulnerabilities whose
response reveals nothing — blind SSRF, XXE, RCE, blind XSS, SQLi DNS-exfil, etc.

## How it works

1. Configure an OOB domain and your public IP (`--oob-domain`, `--oob-public-ip`)
   and point that domain's DNS (an NS/A record) at the host running RedTrace.
2. RedTrace runs two catch-all listeners under that domain:
   - an **HTTP listener** (default `0.0.0.0:8888`) that records every request, and
   - a **DNS responder** (default `0.0.0.0:5353`) that answers A queries with the
     configured public IP and records every query.
3. Generate a payload — a unique `<token>.<domain>` hostname (with `http(s)://`
   URLs) — and plant it in a target (a header, parameter, XML entity, …).
4. If the target makes any out-of-band request to it, RedTrace captures the
   interaction, correlates it to the payload by the token (the label adjacent to
   the domain, so DNS-exfil prefixes like `data.<token>.<domain>` still match),
   stores it, and streams it live to the UI.

## Design notes

- **Self-hosted, no third party.** RedTrace never relies on an external
  collaborator service — consistent with its no-telemetry posture. The operator
  supplies the domain and makes the listeners reachable (public host / port
  forward / DNS delegation).
- **Opt-in exposure.** The OOB listeners run only when `--oob-domain` is set, and
  bind to the address the operator chooses (deliberately reachable by targets —
  unlike the rest of RedTrace, which binds `127.0.0.1`). Captured interactions
  are viewable only through the authenticated API/UI; the listeners themselves
  are anonymous catch-alls (callbacks have no credentials).
- **Robust DNS parsing.** The DNS responder is a small, dependency-free,
  fully bounds-checked parser with a per-packet `recover()` — it reads untrusted
  network input, so a malformed packet can never panic the listener.
- HTTP callback bodies are captured up to 64 KiB.

## API surface (Phase 6)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/oob/config` | OOB status + configured domain/IP/listeners |
| `GET` | `/api/oob/payloads` | Generated payloads + per-payload hit counts |
| `POST` | `/api/oob/payloads` | Generate a new payload |
| `GET` | `/api/oob/interactions` | Captured interactions (optional `?token=`) |
| `GET` | `/api/oob/interactions/{id}` | One interaction with its raw bytes |
| `DELETE` | `/api/oob/interactions` | Clear captured interactions |

New interactions stream over the traffic WebSocket as `oob` frames. Payloads and
interactions persist (`oob_payloads`, `oob_interactions`; migration 0007).

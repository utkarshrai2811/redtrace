# Phase 3 — Intruder

**Goal:** automated request fuzzing — substitute payloads into marked positions
of a request and replay the generated requests against a target, then surface
the interesting responses.

## Delivered

- **Attack types** (`internal/intruder`):
  - **Sniper** — one payload set; each payload is tried in one position at a
    time (others stay at their base value). Requests = positions × payloads.
  - **Battering ram** — one payload set placed in every position at once.
  - **Pitchfork** — one payload set per position, iterated in lockstep.
  - **Cluster bomb** — one payload set per position, every combination.
- **Positions**: marked in the request template with `§…§` (Burp's marker).
  Content-Length is recomputed when payload substitution changes the body.
- **Payload processors**: a per-set chain applied before substitution —
  `prefix`, `suffix`, `base64`, `base64url`, `url`, `upper`, `lower`, `sha256`,
  `md5`.
- **Runner**: a bounded concurrent worker pool (default 10, max 64) replays the
  generated requests through the Repeater engine, persists every result, and
  streams progress over the traffic WebSocket as `intruder` frames. Attacks are
  cancellable mid-run.
- **Results**: status, length, time, and the payload(s) per request, persisted
  (`intruder_attacks`, `intruder_results`; migration 0004). The UI highlights
  anomalies — responses whose length or status deviates from the baseline, and
  errors.
- **Send to Intruder**: from the proxy history, seeds a draft attack with the
  captured request as its template.

## API surface (Phase 3)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/intruder/attacks` | List attacks |
| `POST` | `/api/intruder/attacks` | Create a draft attack |
| `GET` | `/api/intruder/attacks/{id}` | Attack + results |
| `PUT` | `/api/intruder/attacks/{id}` | Update a draft |
| `DELETE` | `/api/intruder/attacks/{id}` | Delete (stops it first if running) |
| `POST` | `/api/intruder/attacks/{id}/start` | Validate and run |
| `POST` | `/api/intruder/attacks/{id}/stop` | Cancel a running attack |
| `GET` | `/api/intruder/results/{id}` | One result with raw request/response |
| `POST` | `/api/requests/{id}/send-to-intruder` | Seed a draft from history |

## Notes

- The runner reuses the Repeater transport, so it inherits faithful response
  handling (no transparent decompression) and the redirect behaviour.
- Generation is lazy and cancellation-aware, so a large Cluster bomb stops
  promptly when an attack is stopped or deleted.

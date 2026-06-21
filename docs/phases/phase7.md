# Phase 7 — AI integration

An optional, opt-in AI assistant built into RedTrace. It helps with the parts of
testing that are about judgement rather than mechanics: understanding an
exchange, triaging a finding, and proposing payloads — and a free-form chat for
anything else. It is **off by default** and only ever talks to the provider the
operator configures.

## How it works

1. Configure a provider in **Settings → AI assistant** (or via flags/env): a
   provider, a model, and either an API key (Anthropic / OpenAI) or a base URL
   (any OpenAI-compatible endpoint, including a local model).
2. From anywhere in the app, hand context to the assistant:
   - **Proxy / history → Send to AI** — explain a captured request/response.
   - **Scanner → Triage with AI** — assess a finding (true/false positive,
     impact, exploitation, remediation).
   - **Intruder → Suggest payloads (AI)** — propose payloads for the marked
     insertion points.
   - **AI page → New chat** — ask anything; paste traffic as needed.
3. The assistant replies **streaming token-by-token**. Conversations persist and
   can be reopened, continued, or deleted.

## Providers

| Provider | Endpoint | Auth | Notes |
|---|---|---|---|
| `anthropic` | `https://api.anthropic.com/v1/messages` | `x-api-key` | Claude models; native streaming. |
| `openai` | `<base-url>/chat/completions` | `Authorization: Bearer` (optional) | OpenAI, OpenRouter, or a **local** model (Ollama / LM Studio / vLLM) via the base URL — the key is optional for local runtimes. |

The model is operator-chosen (e.g. `claude-sonnet-4-6`, `gpt-4o-mini`). Both
paths parse Server-Sent Events; RedTrace adds no third-party SDK.

## Design notes

- **Opt-in, no surprise egress.** The assistant is disabled until configured,
  and a captured request is sent to the provider only when the operator
  explicitly invokes an AI action. RedTrace itself still phones nowhere; the
  operator chooses the destination (and can keep everything on-network with a
  local model).
- **Local key storage.** When set via the Settings UI the API key is stored in
  RedTrace's local SQLite database (under the `0700` data dir) and is **never
  returned to the UI** — only whether a key is set. Flags/env keys are kept in
  memory and not written to disk.
- **Streaming over SSE.** Replies stream from `POST
  /api/ai/conversations/{id}/messages` as `text/event-stream`; a disconnect
  cancels the upstream request. The disabled case is a clean `400` before the
  stream starts.
- **Safe rendering.** Assistant Markdown is rendered without
  `dangerouslySetInnerHTML`; only fenced code blocks get special (monospace)
  layout, everything else is escaped text.
- Per-message context is capped so a large paste cannot blow up the request.

## API surface (Phase 7)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/ai/config` | AI status (provider, model, base URL, whether enabled / a key is set) |
| `PUT` | `/api/ai/config` | Set provider/model/base URL/API key (persisted locally) |
| `GET` | `/api/ai/conversations` | List conversations |
| `POST` | `/api/ai/conversations` | Create a conversation (kind + optional seeded context) |
| `GET` | `/api/ai/conversations/{id}` | One conversation with its messages |
| `DELETE` | `/api/ai/conversations/{id}` | Delete a conversation |
| `DELETE` | `/api/ai/conversations` | Clear all conversations |
| `POST` | `/api/ai/conversations/{id}/messages` | Append a user turn and **stream** the reply (SSE) |

Conversations and messages persist (`ai_conversations`, `ai_messages`); provider
settings persist in `ai_settings` (migration 0008).

## Configuration

| Flag | Env | Default |
|---|---|---|
| `--ai-provider` | `REDTRACE_AI_PROVIDER` | `anthropic` |
| `--ai-model` | `REDTRACE_AI_MODEL` | provider default |
| `--ai-api-key` | `REDTRACE_AI_API_KEY` | _(unset → disabled)_ |
| `--ai-base-url` | `REDTRACE_AI_BASE_URL` | provider default |

Flags/env take precedence at startup; any field they leave empty falls back to
the settings saved via the UI.

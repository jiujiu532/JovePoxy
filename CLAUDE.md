# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**JovePoxy** (`module jovepoxy`) — single-process gateway that turns OpenCode Zen **free models** + **paid Zen Key pool** into OpenAI / Anthropic compatible APIs, with an embedded admin SPA.

- Does **not** require a local OpenCode process
- Data plane `/v1/*` + control plane `/api/admin/*` + embedded SPA on one listener
- Product version: `internal/version.Current` (default `1.5.1`); `--version` / `/health` report `jovepoxy <version>`

## Commands

### Backend (repo root)

```bash
make test              # go test -shuffle=on -count=1 ./...
make vet               # go vet ./...
make build             # go build -o bin/jovepoxy.exe ./cmd/server
make embed-web-win     # build web + copy into internal/webui/dist (Windows)
make all               # embed-web-win + test + build
make smoke             # scripts/smoke.ps1 (local health/login/key create; no live Zen)
```

Single package / single test:

```bash
go test -shuffle=on -count=1 ./internal/httpserver/
go test -shuffle=on -count=1 ./internal/httpserver/ -run TestChat
```

Run binary (required env):

```bash
set ADMIN_PASSWORD=...
set ADMIN_SECRET=...          # ≥32 chars
set DATA_DIR=./data
set LISTEN=127.0.0.1:6446
bin\jovepoxy.exe
```

Windows convenience: `start.bat` / `stop.bat` (ASCII-only batch; default listen `127.0.0.1:6446`).

Makefile intentionally **omits** `go test -race` (Windows host limitation).

### Frontend (`web/`)

Package manager: **pnpm** (`packageManager: pnpm@10.11.0`).

```bash
cd web
pnpm install --frozen-lockfile
pnpm dev          # Vite :5173, proxies /api /v1 /health /metrics → :6446
pnpm build        # tsc -b && vite build → web/dist
pnpm test         # vitest run
pnpm test:watch
pnpm typecheck
```

After UI changes that should ship inside the binary: `make embed-web-win` then rebuild Go.

### Docker

`Dockerfile` + `docker-compose.yml` exist for delivery only. **Do not install Docker or run `docker build` / `docker compose` on this host** (project/global constraint). Prefer `make test` / `make smoke` for verification.

## Architecture

### Request routing (`internal/app`)

`cmd/server` → `app.Run` → `app.Bootstrap`:

| Path prefix | Handler | Role |
|-------------|---------|------|
| `/api/admin/*` | `adminapi` | Control plane (cookie session) |
| `/v1/*`, `/health`, `/metrics` | `httpserver` | Public data plane |
| everything else | `webui` (embed SPA) | Admin UI + client routes → `index.html` |

### Credential model (do not mix)

| Credential | Purpose | Chat path? |
|------------|---------|------------|
| Local API Key `sk-oc-...` | Client → this gateway | Yes (inbound) |
| `Bearer public` | Zen free models | Yes (outbound free) |
| Zen API Key pool | Zen paid models | Yes (outbound paid) |
| OpenCode `auth_cookie` | Quota / usage scrape | **No** (control plane only) |
| `ADMIN_PASSWORD` | Admin UI login | No |

Local keys: SHA-256 only in SQLite; full secret returned once on create.  
Zen keys / cookies / proxy URLs: AES-GCM via `crypto.Box` keyed by `ADMIN_SECRET`.

### Free vs paid chat path

1. Client hits `POST /v1/chat/completions` or `POST /v1/messages` with local key (`httpserver` auth).
2. Model catalog (`models.Catalog`) merges three sources:
   - Public Zen (`ZEN_BASE`, `Bearer public`) → **free only** (`*-free` / `big-pickle` / allowlist)
   - OpenCode Go (`ZEN_GO_BASE` + `/v1`, pool OpenCode key) → paid OpenCode
   - Ollama Cloud (`OLLAMA_BASE` + `/v1`, pool Ollama key) → paid Ollama
3. Free → `proxypool.ProxyFree` → public Zen with `PublicAuth()`; optional egress proxy rotation on 429/5xx/connect fail (one retry).
4. Paid OpenCode → `zenpool.ProxyPaid` → **Go** dialer (`/zen/go/v1`) with pooled OpenCode key + failover.
5. Paid Ollama → `zenpool.ProxyPaid` → Ollama plain dialer with pooled Ollama key.
6. Zen HTTP client: `internal/zen` (compat headers on OpenCode paths; plain auth on Ollama).
7. Anthropic shape: `internal/anthropic` convert request/response/SSE; OpenAI shape mostly pass-through + observe/log.

Key vs proxy are independent: **Key = identity, Proxy = egress IP**.

### Control plane packages

- `adminapi` — HTTP surface for login, local keys, zen keys, proxies, OpenCode/Ollama accounts, quotas, usage, logs, settings
- `keys` — local API keys + optional concurrent session limits
- `zenpool` / `proxypool` — pool CRUD + selection/failover
- `quota` / `ollama` — account credential store + HTML/API scrapers (cookies encrypted)
- `usage` / `analytics` / `reqlog` — usage sync, overview metrics, request logs (**no prompt/response bodies**)
- `auth` — admin password + httpOnly cookie `jovepoxy_admin` + login rate limit
- `db` — SQLite (`modernc.org/sqlite`), schema + migrations under `internal/db`
- `config` — env-only config (`LISTEN`, `DATA_DIR`, `ADMIN_*`, `ZEN_BASE`, `ZEN_GO_BASE`, `OLLAMA_BASE`, TTLs, proxies, `SHOW_ALL_MODELS`, `COOKIE_SECURE`, …)

### Frontend (`web/src`)

React 19 + Vite 7 + Tailwind 4 + react-router. Alias `@` → `src/`.

- Pages under `pages/features/*` map to `lib/routes.ts` nav (overview, models, key-pool, accounts, quotas, local-keys, proxies, logs, settings)
- API client: `lib/api.ts` (credentials/cookies to `/api/admin`)
- Design system source of truth: `web/DESIGN.md` (Neo-Brutalist Playful: hard black/white geometry, five-color accents, offset shadows; no kraft paper)
- Embed path: `web/dist` → copied to `internal/webui/dist` → `//go:embed` in `webui/embed.go`

### Reference tree

`参考/` is **read-only comparison material**. Do not modify it.

## Working conventions

- Global interaction language: **简体中文** for user-facing replies (code/paths/errors stay as-is). See root `AGENTS.md` (Trellis block) and user global AGENTS for process rules.
- Trellis workflow lives under `.trellis/` (`workflow.md`, `tasks/`, `spec/`). Prefer Trellis skills/commands when managing multi-step work. Spec files under `.trellis/spec/backend/` are mostly templates — prefer code + this file for real structure.
- Prefer package layout already under `internal/<domain>/` (service + store + tests) over new top-level packages.
- After product code changes: run the relevant `go test` / `pnpm test`; use `make smoke` for end-to-end binary smoke without live Zen.
- Git: commit when a local change set is verified; do not `git push` unless explicitly asked.
- Secrets: never log or return full Zen keys, cookies, admin password, or local key secrets after create.

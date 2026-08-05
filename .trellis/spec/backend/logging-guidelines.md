# Logging Guidelines

> Request observation and secret redaction for JovePoxy.

---

## Scope

Apply to `slog`, request observation, metrics, admin log views, and any diagnostic field.

## Must obey

- Keep `reqlog.Entry` metadata-only and make persistence best-effort.
- Use structured fields without secret-bearing request/response contents.

## Do not

- Do not log prompts, completions, Authorization headers, tokens, cookies, passwords, or credential-bearing proxy URLs.

## Verify

```bash
go test -shuffle=on -count=1 ./internal/reqlog/ ./internal/httpserver/
make smoke
```

## Overview

- Runtime process logs: Go `log/slog` for fatal/startup (keep free of secrets).
- Data-plane observation: `internal/reqlog` — **metadata only**, async best-effort SQLite + in-memory ring/counters.
- Product rule (README): request logs do **not** store prompt/response bodies.

---

## What to log (reqlog.Entry)

| Field | Type | Meaning |
|-------|------|---------|
| `ID` | string | row id |
| `KeyID` | string | local key id (not secret) |
| `Model` | string | model id |
| `Route` | string | e.g. `/v1/chat/completions` |
| `Status` | int | HTTP status observed |
| `LatencyMS` | int64 | duration |
| `Stream` | bool | streaming request |
| `ErrorClass` | string | coarse class, not stack+body |
| `MaxTokens` | int | request-side max_tokens if known |
| `ReasoningEffort` | string | mapped effort (none/low/medium/high/xhigh) |
| `ThinkingType` | string | request thinking mode if present |
| `BudgetTokens` | int | Anthropic budget_tokens if present |
| `InputTokens` | int | upstream `prompt_tokens` (0 if missing) |
| `OutputTokens` | int | upstream `completion_tokens` (0 if missing) |
| `CacheReadTokens` | int | cache hit tokens (0 if missing) |
| `CacheCreationTokens` | int | cache write tokens (0 if missing) |
| `CreatedAt` | time | UTC |

Counters snapshot (`reqlog.Snapshot`): `total_requests`, `status_429`, `status_5xx`, `status_2xx`, `stream_requests`.

`Record` must never fail the request path if SQLite insert fails.

Token fields are **integers only** — never store SSE text, prompt, or completion bodies. Missing upstream usage → all zeros (do not invent cache hits).

---

## What NOT to log

| Data | Reason |
|------|--------|
| Prompt / completion text | Privacy + size |
| Local API key secret / full `sk-oc-...` | One-time reveal only at create |
| Zen API keys | Ciphertext only at rest |
| OpenCode/Ollama cookies | Ciphertext only |
| Proxy URLs with credentials | Ciphertext + mask in admin list |
| `ADMIN_PASSWORD` / `ADMIN_SECRET` | Env only |
| Upstream error bodies that may echo credentials | Strip / generic message |

Admin list endpoints return **masked** secrets only.

---

## Log Levels (process slog)

| Level | Use |
|-------|-----|
| Error | process cannot continue / listen failures |
| Info | rare operational lifecycle if needed |
| Debug | local dev only; still no secrets |

Prefer structured key-value fields (`"err", err`) over interpolated secret-bearing strings.

---

## Scenario: Extending request observation

### 1. Scope / Trigger
Adding fields to request logs, metrics, or observe middleware.

### 2. Signatures

```go
// internal/reqlog
type Entry struct {
    ID, KeyID, Model, Route string
    Status int; LatencyMS int64; Stream bool; ErrorClass string
    MaxTokens int; ReasoningEffort, ThinkingType string; BudgetTokens int
    InputTokens, OutputTokens, CacheReadTokens, CacheCreationTokens int
    CreatedAt time.Time
}
func (s *Service) Record(ctx context.Context, entry Entry)
func (s *Service) Snapshot() Snapshot

// internal/usageparse — OpenAI-shaped usage only (no bodies)
type UsageSnapshot struct {
    PromptTokens, CompletionTokens, CacheReadTokens, CacheCreationTokens int
}
func ParseOpenAIUsage(body []byte) UsageSnapshot
// stream: side-scan SSE data lines; last non-zero-ish usage wins; scan fail → zeros
```

Admin `GET /api/admin/logs` `logDTO` always emits the four token fields as numbers (including `0`).

### 3. Contracts
- New columns require **db migration** + admin DTO + frontend `LogDTO` update if exposed.
- No `TEXT` column for raw request/response payload or full SSE.
- **Non-stream**: parse upstream OpenAI JSON usage **before** Anthropic/Responses convert when possible.
- **Stream**: tee/pass-through with **line-level side-scan** only; never buffer the full stream into logs.
- Anthropic client-visible `cache_read_input_tokens` / `cache_creation_input_tokens` must map from real OpenAI usage when present — **never hardcode 0** if source data exists.
- OpenAI/Zen often sends `usage` on a **trailing frame after** `finish_reason`; Anthropic convert must delay final `message_delta` until EOF (or after that usage frame) so cache fields are not dropped.

### 4. Validation & Error Matrix
- Insert failure → ignored for client; counters still updated.
- Missing model/route → still record with empty/defaults only if observer guarantees non-panic.
- Usage parse / SSE scan failure → token fields `0`; stream and status recording continue.

### 5. Good/Base/Bad Cases
- **Good**: status + latency + model + input/output/cache integers for dashboard and log expand panel.
- **Base**: stream flag increments `stream_requests`; free path without usage still logs zeros.
- **Bad**: `entry.Prompt = string(body)` or storing SSE blob for “later parse”.

### 6. Tests Required
- `usageparse`: cached_tokens, missing usage, malformed JSON, SSE tail usage.
- `reqlog` service tests: counters + ring + new columns Insert/List.
- `anthropic`: usageMap non-zero cache; stream trailing usage frame.
- Smoke: metrics JSON must not match admin password or created local secret.
- Handler observe tests if middleware changes.

### 7. Wrong vs Correct

#### Wrong
```go
log.Printf("chat body=%s key=%s", body, rawKey)
// hardcode Anthropic cache always 0 while UI claims cache hits
// finish Anthropic stream on finish_reason before reading next-frame usage
```

#### Correct
```go
snap := usageparse.ParseOpenAIUsage(body) // or side-scan during copyStream
logs.Record(ctx, reqlog.Entry{
    KeyID: keyID, Model: model, Route: route, Status: status, LatencyMS: ms, Stream: stream,
    InputTokens: snap.PromptTokens, OutputTokens: snap.CompletionTokens,
    CacheReadTokens: snap.CacheReadTokens, CacheCreationTokens: snap.CacheCreationTokens,
})
```

---

## Scenario: Admin list with time window (logs / usage)

### 1. Scope / Trigger
Range-scoped overview analytics or any admin list filtered by time; cross-layer contract on `from`/`to` query params and SQLite string timestamps.

### 2. Signatures

```go
// reqlog
type ListFilter struct { From, To time.Time; Limit, Offset int } // zero From/To = open
func (s *Service) List(ctx context.Context, limit, offset int) ([]Entry, error) // unfiltered wrapper
func (s *Service) ListFiltered(ctx context.Context, filter ListFilter) ([]Entry, error)

// usage (same idea)
type ListFilter struct { AccountID string; From, To time.Time; Limit, Offset int }

// admin
// GET /api/admin/logs?from=&to=&limit=&offset=
// GET /api/admin/usage?from=&to=&limit=&offset=&account_id=
```

### 3. Contracts
- Query `from`/`to`: optional RFC3339 or RFC3339Nano; empty = open bound; closed interval `[from, to]` on `created_at` / `recorded_at`.
- Response: `{ "logs"|"records": [...], "truncated": bool, "limit": int }` — `truncated` when `len(rows) >= limit` (omitempty ok for false).
- Persist timestamps as **fixed-width fractional UTC** for lexicographic = chronological order:
  - reqlog: layout `2006-01-02T15:04:05.000000000Z07:00` (do **not** use bare `time.RFC3339Nano` for filter bounds — it strips trailing zeros and breaks string compares, e.g. `...00.5Z` < `...00Z`).
  - usage `recorded_at`: normalize to a fixed millisecond form (see `normalizeRecordedAt`).
- Still metadata-only; no bodies/secrets in list rows.

### 4. Validation & Error Matrix
- Invalid `from`/`to` → **400** `invalid from` / `invalid to`.
- SQLite list failure on logs → may fall back to in-memory `Recent` (no time filter); still set limit/truncated.
- Missing bounds → same as pre-filter list (newest first, limit only).

### 5. Good/Base/Bad Cases
- **Good**: overview passes `from`/`to` ISO strings + high limit; UI shows non-blocking truncated hint.
- **Base**: Logs page calls without from/to; default limit; backward compatible.
- **Bad**: client filters only the newest 2000 unscoped rows for a multi-month range.

### 6. Tests Required
- `reqlog`: multi-timestamp ListFiltered bounds + fractional-second order/filter.
- `usage`: recorded_at range filter.
- `adminapi`: valid window, truncated flag, invalid time → 400.

### 7. Wrong vs Correct

#### Wrong
```go
created := t.UTC().Format(time.RFC3339Nano) // variable fractional width
// ... WHERE created_at >= ?  // string order ≠ time order
```

#### Correct
```go
const createdAtLayout = "2006-01-02T15:04:05.000000000Z07:00"
created := t.UTC().Format(createdAtLayout)
// filter bounds use the same layout
```

---

## Scenario: Overview final-upstream routing KPIs

### 1. Scope / Trigger
`GET /api/admin/overview?window=1h|24h|7d` 需要把请求日志的最终上游通道作为时间窗口运维指标返回，供管理台比较 paid OpenCode 与 paid Ollama 的实际结果。

### 2. Signatures

```go
// internal/analytics
const UnknownUpstream = "unknown"

type UpstreamKPI struct {
    Upstream string `json:"upstream"`
    Requests int64 `json:"requests"`
    SuccessRate *float64 `json:"success_rate"`
    LatencyP50MS, LatencyP95MS *int64
    Status2xx, Status429, Status4xx, Status5xx int64
}
type RoutingKPIs struct {
    Window string `json:"window"`
    Requests int64 `json:"requests"`
    ByUpstream []UpstreamKPI `json:"by_upstream"`
}
func AggregateRoutingKPIs(entries []reqlog.Entry, window string, now time.Time) RoutingKPIs

// internal/adminapi overviewResponse
RoutingKPIs *analytics.RoutingKPIs `json:"routing_kpis,omitempty"`
```

### 3. Contracts
- `routing_kpis` is an additive field on the existing authenticated overview response; request query `window` is normalized with the existing `1h|24h|7d` rules.
- Aggregate only `Entry.Upstream`, `Status`, `LatencyMS`, and `CreatedAt`; no prompt, completion, error body, cookie, local key secret, or upstream Key is read or returned.
- The label is the **final provider that served or finally failed the request**. It does not represent every attempted provider, a cross-pool failover count, or a per-Zen-Key hit count.
- Blank/whitespace legacy upstream values normalize to `unknown`. `unknown` and `opencode_free` are separate buckets and must not be folded into `opencode_paid` or `ollama_paid` shares.
- Use one bounded `List(5000)` / `Recent(5000)` load to derive both `ops_kpis` and `routing_kpis`; no schema or `/metrics` semantic change is required.

### 4. Validation & Error Matrix
- Empty log window -> `requests: 0`, non-nil empty `by_upstream: []`, and nil success/latency values; clients render an empty state, not `0%` success.
- Old row with empty upstream -> `unknown` bucket.
- Persistent log-list failure -> use the existing bounded in-memory `Recent` fallback; response remains metadata-only.
- Invalid/unknown `window` -> existing normalized default `24h`.

### 5. Good/Base/Bad Cases
- **Good**: `opencode_paid` and `ollama_paid` are compared from their final status/latency buckets in the same requested window.
- **Base**: a window contains only free or unknown rows; paid channel UI remains empty and does not invent a distribution.
- **Bad**: infer a failover hop because the final upstream is `opencode_paid`, or merge unknown historical rows into OpenCode paid.

### 6. Tests Required
- `internal/analytics/routing_kpis_test.go`: window filter, all status buckets, p50/p95, deterministic group order, empty input, and unknown normalization.
- `internal/adminapi/server_test.go`: authenticated overview response contains `routing_kpis`, respects the requested window, emits empty arrays, and keeps unknown independent.
- Frontend DTO/helper tests: unknown/free excluded from paid share and an empty channel shows `-`, not a fabricated rate.

### 7. Wrong vs Correct

#### Wrong
```go
// Treat final upstream as every provider tried, and serialize untrusted body details.
summary.FailoverCount++
summary.LastErrorBody = string(responseBody)
```

#### Correct
```go
routing := analytics.AggregateRoutingKPIs(entries, window, now)
resp.RoutingKPIs = &routing // final metadata only
```

---

## Structured Logging

- Use attribute names stable for grepping: `err`, `version`, not free-form essays.
- Config errors: expose env **key names** only (`config.EnvironmentError.Variable`).

---

## Common Mistakes

1. Dumping entire upstream HTTP response into `error_class`.
2. Logging `Authorization` headers in debug middleware.
3. Returning full secrets in `/metrics` or admin list after a "helpful" debug field.

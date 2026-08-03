package db

var initialSchemaTables = []string{
	"local_api_keys",
	"zen_keys",
	"opencode_accounts",
	"usage_records",
	"usage_sync_state",
	"request_logs",
	"settings",
	"admin_sessions",
}

var usageSchemaTables = []string{"local_key_usage"}

var opencodeAccountVisibilityColumns = []string{"show_rolling", "show_weekly", "show_monthly"}

const initialSchema = `
CREATE TABLE local_api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    prefix TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    rpm_limit INTEGER NOT NULL DEFAULT 0,
    daily_limit INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT
);

CREATE TABLE zen_keys (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    key_ciphertext TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    cooldown_until TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE opencode_accounts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    workspace_id TEXT,
    auth_cookie_ciphertext TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE usage_records (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    usg_id TEXT NOT NULL UNIQUE,
    model TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    recorded_at TEXT NOT NULL,
    FOREIGN KEY (account_id) REFERENCES opencode_accounts(id)
);

CREATE TABLE usage_sync_state (
    account_id TEXT PRIMARY KEY,
    cursor TEXT,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES opencode_accounts(id)
);

CREATE TABLE request_logs (
    id TEXT PRIMARY KEY,
    key_id TEXT,
    model TEXT NOT NULL,
    route TEXT NOT NULL,
    status INTEGER NOT NULL,
    latency_ms INTEGER NOT NULL,
    stream INTEGER NOT NULL DEFAULT 0,
    error_class TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (key_id) REFERENCES local_api_keys(id)
);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE admin_sessions (
    token_hash TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT
);
`

const localKeyUsageSchema = `
CREATE TABLE local_key_usage (
    key_id TEXT NOT NULL,
    window_kind TEXT NOT NULL,
    window_started_at TEXT NOT NULL,
    request_count INTEGER NOT NULL,
    PRIMARY KEY (key_id, window_kind),
    FOREIGN KEY (key_id) REFERENCES local_api_keys(id) ON DELETE CASCADE
);
`

// adminSessionsHashOnlySchema invalidates legacy plaintext-token sessions.
const adminSessionsHashOnlySchema = `
DROP TABLE admin_sessions;
CREATE TABLE admin_sessions (
    token_hash TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TEXT
);
`

const opencodeAccountVisibilitySchema = `
ALTER TABLE opencode_accounts ADD COLUMN show_rolling INTEGER NOT NULL DEFAULT 1;
ALTER TABLE opencode_accounts ADD COLUMN show_weekly INTEGER NOT NULL DEFAULT 1;
ALTER TABLE opencode_accounts ADD COLUMN show_monthly INTEGER NOT NULL DEFAULT 1;
`

const ollamaAccountsSchema = `
CREATE TABLE ollama_accounts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    session_cookie_ciphertext TEXT NOT NULL,
    show_session INTEGER NOT NULL DEFAULT 1,
    show_weekly INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

const egressProxiesSchema = `
CREATE TABLE egress_proxies (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    url_ciphertext TEXT NOT NULL,
    scheme TEXT NOT NULL,
    host TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    cooldown_until TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// zen_keys.provider distinguishes OpenCode vs Ollama upstream key pools.
const zenKeyProviderSchema = `
ALTER TABLE zen_keys ADD COLUMN provider TEXT NOT NULL DEFAULT 'opencode';
`

// zen_keys.key_prefix stores a secret-free display prefix so List never decrypts.
const zenKeyPrefixSchema = `
ALTER TABLE zen_keys ADD COLUMN key_prefix TEXT NOT NULL DEFAULT '';
`

// request_logs request-side generation metadata (no prompt/response bodies).
const requestLogMetaSchema = `
ALTER TABLE request_logs ADD COLUMN max_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE request_logs ADD COLUMN reasoning_effort TEXT NOT NULL DEFAULT '';
ALTER TABLE request_logs ADD COLUMN thinking_type TEXT NOT NULL DEFAULT '';
ALTER TABLE request_logs ADD COLUMN budget_tokens INTEGER NOT NULL DEFAULT 0;
`

// request_logs response-side usage/cache counters (no prompt/response bodies).
const requestLogUsageSchema = `
ALTER TABLE request_logs ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE request_logs ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE request_logs ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE request_logs ADD COLUMN cache_creation_tokens INTEGER NOT NULL DEFAULT 0;
`

// request_logs time-to-first-byte (ms from handler start to first body write).
const requestLogTTFTSchema = `
ALTER TABLE request_logs ADD COLUMN ttft_ms INTEGER NOT NULL DEFAULT 0;
`

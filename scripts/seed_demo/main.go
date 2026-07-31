// seed_demo inserts local demo rows so admin UI can be previewed without live Zen traffic.
// Usage (from repo root):
//
//	go run ./scripts/seed_demo -db ./data/jovepoxy.db -secret 0123456789abcdef0123456789abcdef
//
// Safe to re-run: deletes only rows with demo_* / seed_* ids first.
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	secretcrypto "jovepoxy/internal/crypto"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "./data/jovepoxy.db", "sqlite path")
	secret := flag.String("secret", envOr("ADMIN_SECRET", "0123456789abcdef0123456789abcdef"), "ADMIN_SECRET for AES-GCM")
	flag.Parse()

	if *secret == "" {
		log.Fatal("ADMIN_SECRET is required")
	}
	box, err := secretcrypto.NewBox(*secret)
	if err != nil {
		log.Fatalf("crypto box: %v", err)
	}

	db, err := sql.Open("sqlite", *dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping %s: %v", *dbPath, err)
	}

	if err := cleanDemo(db); err != nil {
		log.Fatalf("clean: %v", err)
	}
	if err := seed(db, box); err != nil {
		log.Fatalf("seed: %v", err)
	}
	fmt.Printf("demo seed ok → %s\n", *dbPath)
	fmt.Println("refresh admin UI: overview / key-pool / accounts / logs / quotas")
	fmt.Println("note: demo_* quota scrapes return synthetic windows (no live cookie needed); process 429 is in-memory only")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func cleanDemo(db *sql.DB) error {
	stmts := []string{
		`DELETE FROM usage_records WHERE id LIKE 'seed_%' OR account_id LIKE 'demo_%'`,
		`DELETE FROM usage_sync_state WHERE account_id LIKE 'demo_%'`,
		`DELETE FROM request_logs WHERE id LIKE 'seed_rl_%'`,
		`DELETE FROM local_api_keys WHERE id LIKE 'demo_%'`,
		`DELETE FROM zen_keys WHERE id LIKE 'demo_%'`,
		`DELETE FROM opencode_accounts WHERE id LIKE 'demo_%'`,
		`DELETE FROM ollama_accounts WHERE id LIKE 'demo_%'`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return nil
}

func seed(db *sql.DB, box *secretcrypto.Box) error {
	now := time.Now().UTC()
	rng := rand.New(rand.NewSource(now.UnixNano()))

	// --- local API keys (for FK on request_logs if needed) ---
	localKeyID := "demo_local_key_01"
	hash := sha256.Sum256([]byte("sk-oc-demo-local-secret-not-real"))
	if _, err := db.Exec(`
		INSERT INTO local_api_keys (id, name, key_hash, prefix, enabled, rpm_limit, daily_limit, created_at)
		VALUES (?, 'Demo Client', ?, 'sk-oc-demo', 1, 60, 10000, ?)
	`, localKeyID, hex.EncodeToString(hash[:]), now.Add(-48*time.Hour).Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("local key: %w", err)
	}

	// --- zen keys (opencode + ollama providers) ---
	type zk struct {
		id, label, provider string
		weight              int
		enabled             int
		cooldown            *time.Time
	}
	cool := now.Add(12 * time.Minute)
	zenKeys := []zk{
		{"demo_zk_oc_alpha", "OC Alpha · prod", "opencode", 3, 1, nil},
		{"demo_zk_oc_beta", "OC Beta · backup", "opencode", 2, 1, &cool},
		{"demo_zk_oc_gamma", "OC Gamma · off", "opencode", 1, 0, nil},
		{"demo_zk_ol_main", "Ollama Main", "ollama", 2, 1, nil},
		{"demo_zk_ol_spare", "Ollama Spare", "ollama", 1, 1, nil},
	}
	for i, k := range zenKeys {
		ct, err := box.Seal(fmt.Sprintf("sk-zen-demo-%s-not-a-real-key", k.id))
		if err != nil {
			return err
		}
		var coolAny any
		if k.cooldown != nil {
			coolAny = k.cooldown.UTC().Format(time.RFC3339Nano)
		}
		created := now.Add(-time.Duration(72-i*8) * time.Hour).Format(time.RFC3339Nano)
		if _, err := db.Exec(`
			INSERT INTO zen_keys (id, label, key_ciphertext, weight, enabled, cooldown_until, created_at, provider)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, k.id, k.label, ct, k.weight, k.enabled, coolAny, created, k.provider); err != nil {
			return fmt.Errorf("zen key %s: %w", k.id, err)
		}
	}

	// --- OpenCode accounts ---
	type acc struct {
		id, name, workspace string
	}
	accounts := []acc{
		{"demo_oc_acc_alice", "alice-workspace", "wrk_demoAlice01"},
		{"demo_oc_acc_bob", "bob-dev", "wrk_demoBob02xx"},
		{"demo_oc_acc_carol", "carol-batch", "wrk_demoCarol3"},
	}
	for i, a := range accounts {
		ct, err := box.Seal("auth=demo_cookie_value_not_real_" + a.id)
		if err != nil {
			return err
		}
		created := now.Add(-time.Duration(10-i) * 24 * time.Hour).Format(time.RFC3339Nano)
		if _, err := db.Exec(`
			INSERT INTO opencode_accounts
			  (id, name, workspace_id, auth_cookie_ciphertext, enabled, created_at, show_rolling, show_weekly, show_monthly)
			VALUES (?, ?, ?, ?, 1, ?, 1, 1, 1)
		`, a.id, a.name, a.workspace, ct, created); err != nil {
			return fmt.Errorf("account %s: %w", a.id, err)
		}
		if _, err := db.Exec(`
			INSERT INTO usage_sync_state (account_id, cursor, updated_at) VALUES (?, '3', ?)
		`, a.id, now.Add(-30*time.Minute).Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("sync state %s: %w", a.id, err)
		}
	}

	// --- Ollama accounts (quota UI Ollama tab) ---
	type olAcc struct {
		id, name string
	}
	ollamaAccounts := []olAcc{
		{"demo_ol_acc_nova", "nova-cloud"},
		{"demo_ol_acc_orbit", "orbit-dev"},
	}
	for i, a := range ollamaAccounts {
		ct, err := box.Seal("demo_session=not_real_" + a.id)
		if err != nil {
			return err
		}
		created := now.Add(-time.Duration(7-i) * 24 * time.Hour).Format(time.RFC3339Nano)
		if _, err := db.Exec(`
			INSERT INTO ollama_accounts
			  (id, name, session_cookie_ciphertext, show_session, show_weekly, enabled, created_at)
			VALUES (?, ?, ?, 1, 1, 1, ?)
		`, a.id, a.name, ct, created); err != nil {
			return fmt.Errorf("ollama account %s: %w", a.id, err)
		}
	}

	// --- model catalog (usage + request_logs share the same weighted pool) ---
	// Weights bias free/popular models so overview legends show many series, not 5 equal blobs.
	type modelWeight struct {
		name   string
		weight int
	}
	modelWeights := []modelWeight{
		{"deepseek-v4-flash-free", 14},
		{"big-pickle", 12},
		{"claude-sonnet-4-5", 11},
		{"claude-haiku-4-5", 9},
		{"claude-opus-4", 5},
		{"gpt-5", 7},
		{"gpt-5-mini", 10},
		{"gpt-4.1", 6},
		{"gemini-2.5-flash", 10},
		{"gemini-2.5-pro", 6},
		{"llama-4-maverick", 5},
		{"qwen3-235b", 5},
		{"kimi-k2", 4},
		{"grok-3-mini", 4},
		{"mistral-small-3.1", 4},
		{"glm-4.5", 3},
	}
	pickModel := func() string {
		total := 0
		for _, m := range modelWeights {
			total += m.weight
		}
		r := rng.Intn(total)
		for _, m := range modelWeights {
			r -= m.weight
			if r < 0 {
				return m.name
			}
		}
		return modelWeights[0].name
	}
	// fixed-width timestamps: bare RFC3339Nano varies fractional width and breaks
	// lexicographic closed-interval filters on created_at / recorded_at.
	const tsLayout = "2006-01-02T15:04:05.000000000Z07:00"

	// --- usage_records (drives overview tokens/requests + OpenCode usage tab) ---
	nUsage := 0
	for day := 44; day >= 0; day-- {
		dayBase := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -day)
		count := 4 + rng.Intn(6)
		if day <= 1 {
			count = 18 + rng.Intn(12)
		} else if day <= 7 {
			count = 10 + rng.Intn(8)
		}
		for j := 0; j < count; j++ {
			nUsage++
			acc := accounts[rng.Intn(len(accounts))]
			model := pickModel()
			inTok := 400 + rng.Intn(6000)
			outTok := 80 + rng.Intn(2500)
			ts := dayBase.Add(time.Duration(rng.Intn(23))*time.Hour + time.Duration(rng.Intn(3600))*time.Second)
			id := fmt.Sprintf("seed_usg_%04d", nUsage)
			usgID := fmt.Sprintf("usg_demo_%04d", nUsage)
			if _, err := db.Exec(`
				INSERT INTO usage_records (id, account_id, usg_id, model, input_tokens, output_tokens, recorded_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`, id, acc.id, usgID, model, inTok, outTok, ts.UTC().Format(tsLayout)); err != nil {
				return fmt.Errorf("usage %s: %w", id, err)
			}
		}
	}

	// --- request_logs (ops KPI + model analytics charts) ---
	routes := []string{"/v1/chat/completions", "/v1/messages", "/v1/responses"}
	nLog := 0
	// denser near now; cover ~45d so custom multi-week ranges have series
	type span struct {
		from, to time.Duration
		n        int
	}
	spans := []span{
		{45 * 24 * time.Hour, 21 * 24 * time.Hour, 70},
		{21 * 24 * time.Hour, 7 * 24 * time.Hour, 80},
		{7 * 24 * time.Hour, 2 * 24 * time.Hour, 70},
		{2 * 24 * time.Hour, 24 * time.Hour, 50},
		{24 * time.Hour, time.Hour, 70},
		{time.Hour, time.Minute, 32},
	}
	for _, s := range spans {
		for j := 0; j < s.n; j++ {
			nLog++
			// weighted status mix ~88% 2xx, 5% 429, 5% 5xx, rest 4xx
			roll := rng.Intn(100)
			status := 200
			errClass := ""
			switch {
			case roll < 5:
				status = 429
				errClass = "rate_limited"
			case roll < 10:
				status = 502
				errClass = "upstream_error"
			case roll < 12:
				status = 401
				errClass = "auth"
			default:
				if rng.Intn(3) == 0 {
					status = 201
				}
			}
			lat := int64(180 + rng.Intn(2200))
			if status >= 500 {
				lat = int64(800 + rng.Intn(8000))
			}
			window := s.from - s.to
			if window <= 0 {
				window = time.Minute
			}
			delta := s.to + time.Duration(rng.Int63n(int64(window)))
			ts := now.Add(-delta)
			stream := 0
			if rng.Intn(3) == 0 {
				stream = 1
			}
			id := fmt.Sprintf("seed_rl_%04d", nLog)
			route := routes[rng.Intn(len(routes))]
			model := pickModel()
			var errAny any
			if errClass != "" {
				errAny = errClass
			}
			if _, err := db.Exec(`
				INSERT INTO request_logs (id, key_id, model, route, status, latency_ms, stream, error_class, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, id, localKeyID, model, route, status, lat, stream, errAny, ts.UTC().Format(tsLayout)); err != nil {
				return fmt.Errorf("log %s: %w", id, err)
			}
		}
	}

	fmt.Printf("  zen_keys: %d  accounts: %d  ollama_accounts: %d  usage: %d  request_logs: %d\n",
		len(zenKeys), len(accounts), len(ollamaAccounts), nUsage, nLog)
	return nil
}

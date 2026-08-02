package adminapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/adminapi"
	"jovepoxy/internal/auth"
	"jovepoxy/internal/config"
	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/usage"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

func TestAdminAPI_login_and_protect_routes(t *testing.T) {
	// Given
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	authService, err := auth.NewService(auth.Config{Database: database, Password: "secret-admin"})
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	box, _ := crypto.NewBox(strings.Repeat("s", 32))
	client, _ := zen.NewClient(config.Config{ZenBase: "http://127.0.0.1:9", OCVersion: "t", UpstreamTimeout: time.Second})
	catalog, _ := models.NewCatalog(client, models.Settings{TTL: time.Hour})
	handler := adminapi.New(adminapi.Dependencies{
		Auth: authService, Keys: keys.NewService(database, nil), Pool: zenpool.NewService(database, box, nil), Catalog: catalog,
		Config: config.Config{Listen: "127.0.0.1:6446", ModelCacheTTL: 5 * time.Minute, OCVersion: "t", CookieSecure: false},
	})

	// When: unauthenticated
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/local-keys", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d", recorder.Code)
	}

	// When: login
	loginBody := bytes.NewBufferString(`{"password":"secret-admin"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookie := loginRec.Result().Cookies()
	if len(cookie) == 0 {
		t.Fatal("expected session cookie")
	}

	// When: authenticated list
	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/local-keys", nil)
	listReq.AddCookie(cookie[0])
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	var payload struct {
		Keys []struct {
			ID     string `json:"id"`
			Label  string `json:"label"`
			Prefix string `json:"prefix"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v body=%s", err, listRec.Body.String())
	}
	// Contract: snake_case keys field present
	if payload.Keys == nil {
		t.Fatalf("expected keys array, body=%s", listRec.Body.String())
	}

	// When: create local key returns snake_case secret once
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/local-keys", bytes.NewBufferString(`{"label":"demo"}`))
	createReq.AddCookie(cookie[0])
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
		Prefix string `json:"prefix"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil || created.Secret == "" || created.ID == "" {
		t.Fatalf("created dto = %+v err=%v body=%s", created, err, createRec.Body.String())
	}
}

func TestAdminAPI_logs_time_window_and_truncated(t *testing.T) {
	// Given
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	authService, err := auth.NewService(auth.Config{Database: database, Password: "secret-admin"})
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	logs := reqlog.NewService(database, nil)
	t1 := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 30, 14, 0, 0, 0, time.UTC)
	logs.Record(ctx, reqlog.Entry{ID: "rl_a", Model: "m1", Route: "/v1/chat/completions", Status: 200, CreatedAt: t1})
	logs.Record(ctx, reqlog.Entry{ID: "rl_b", Model: "m2", Route: "/v1/chat/completions", Status: 200, CreatedAt: t2})
	logs.Record(ctx, reqlog.Entry{ID: "rl_c", Model: "m3", Route: "/v1/chat/completions", Status: 200, CreatedAt: t3})

	handler := adminapi.New(adminapi.Dependencies{
		Auth: authService, Logs: logs,
		Config: config.Config{Listen: "127.0.0.1:6446", CookieSecure: false},
	})
	cookie := loginCookie(t, handler, "secret-admin")

	// When: invalid from
	badReq := httptest.NewRequest(http.MethodGet, "/api/admin/logs?from=not-a-time", nil)
	badReq.AddCookie(cookie)
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid from status = %d body=%s", badRec.Code, badRec.Body.String())
	}

	// When: closed window covering mid+late
	from := t2.Format(time.RFC3339)
	to := t3.Format(time.RFC3339)
	url := "/api/admin/logs?from=" + from + "&to=" + to + "&limit=10"
	listReq := httptest.NewRequest(http.MethodGet, url, nil)
	listReq.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	var payload struct {
		Logs []struct {
			ID string `json:"id"`
		} `json:"logs"`
		Truncated bool `json:"truncated"`
		Limit     int  `json:"limit"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v body=%s", err, listRec.Body.String())
	}
	if len(payload.Logs) != 2 {
		t.Fatalf("logs len = %d body=%s", len(payload.Logs), listRec.Body.String())
	}
	if payload.Truncated {
		t.Fatalf("expected truncated=false when under limit, body=%s", listRec.Body.String())
	}
	if payload.Limit != 10 {
		t.Fatalf("limit = %d", payload.Limit)
	}

	// When: limit hits truncation signal
	truncReq := httptest.NewRequest(http.MethodGet, "/api/admin/logs?limit=2", nil)
	truncReq.AddCookie(cookie)
	truncRec := httptest.NewRecorder()
	handler.ServeHTTP(truncRec, truncReq)
	if truncRec.Code != http.StatusOK {
		t.Fatalf("trunc status = %d body=%s", truncRec.Code, truncRec.Body.String())
	}
	if err := json.Unmarshal(truncRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode trunc: %v", err)
	}
	if len(payload.Logs) != 2 || !payload.Truncated || payload.Limit != 2 {
		t.Fatalf("truncated payload = %+v body=%s", payload, truncRec.Body.String())
	}
}

func TestAdminAPI_usage_time_window_and_truncated(t *testing.T) {
	// Given
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	authService, err := auth.NewService(auth.Config{Database: database, Password: "secret-admin"})
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO opencode_accounts (id, name, workspace_id, auth_cookie_ciphertext, enabled)
		VALUES ('acct_u', 'u', 'wrk_u', 'cipher', 1)
	`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	store := usage.NewSQLiteStore(database)
	if _, err := store.InsertIgnore(ctx, "acct_u", []usage.Record{
		{USGID: "usg_a", CreatedAt: "2026-07-30T10:00:00.000Z", Model: "m1", InputTokens: 1, OutputTokens: 1},
		{USGID: "usg_b", CreatedAt: "2026-07-30T12:00:00.000Z", Model: "m2", InputTokens: 2, OutputTokens: 2},
		{USGID: "usg_c", CreatedAt: "2026-07-30T14:00:00.000Z", Model: "m3", InputTokens: 3, OutputTokens: 3},
	}); err != nil {
		t.Fatalf("insert usage: %v", err)
	}
	usageService := usage.NewService(store, nil)
	handler := adminapi.New(adminapi.Dependencies{
		Auth: authService, Usage: usageService,
		Config: config.Config{Listen: "127.0.0.1:6446", CookieSecure: false},
	})
	cookie := loginCookie(t, handler, "secret-admin")

	// When: invalid to
	badReq := httptest.NewRequest(http.MethodGet, "/api/admin/usage?to=bad", nil)
	badReq.AddCookie(cookie)
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid to status = %d body=%s", badRec.Code, badRec.Body.String())
	}

	// When: window mid..late
	url := "/api/admin/usage?from=2026-07-30T12:00:00Z&to=2026-07-30T14:00:00Z&limit=10&account_id=acct_u"
	listReq := httptest.NewRequest(http.MethodGet, url, nil)
	listReq.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	var payload struct {
		Records []struct {
			USGID string `json:"usg_id"`
		} `json:"records"`
		Truncated bool `json:"truncated"`
		Limit     int  `json:"limit"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v body=%s", err, listRec.Body.String())
	}
	if len(payload.Records) != 2 {
		t.Fatalf("records len = %d body=%s", len(payload.Records), listRec.Body.String())
	}
	if payload.Truncated || payload.Limit != 10 {
		t.Fatalf("payload flags = truncated=%v limit=%d", payload.Truncated, payload.Limit)
	}

	// When: truncated
	truncReq := httptest.NewRequest(http.MethodGet, "/api/admin/usage?limit=2&account_id=acct_u", nil)
	truncReq.AddCookie(cookie)
	truncRec := httptest.NewRecorder()
	handler.ServeHTTP(truncRec, truncReq)
	if err := json.Unmarshal(truncRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode trunc: %v", err)
	}
	if len(payload.Records) != 2 || !payload.Truncated || payload.Limit != 2 {
		t.Fatalf("truncated payload = %+v body=%s", payload, truncRec.Body.String())
	}
}

func TestAdminAPI_login_rate_limits_same_ip_across_ports(t *testing.T) {
	// Given
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	authService, err := auth.NewService(auth.Config{Database: database, Password: "secret-admin"})
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	handler := adminapi.New(adminapi.Dependencies{
		Auth:   authService,
		Config: config.Config{Listen: "127.0.0.1:6446", CookieSecure: false},
	})

	// When: fail DefaultLoginAttemptLimit times from same IP, different ports
	for i := 0; i < auth.DefaultLoginAttemptLimit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewBufferString(`{"password":"wrong"}`))
		req.RemoteAddr = "1.2.3.4:" + strconv.Itoa(10000+i)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d body=%s, want 401", i, rec.Code, rec.Body.String())
		}
	}
	// Another port on the same IP should be rate-limited (429)
	limitedReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewBufferString(`{"password":"wrong"}`))
	limitedReq.RemoteAddr = "1.2.3.4:19999"
	limitedRec := httptest.NewRecorder()
	handler.ServeHTTP(limitedRec, limitedReq)

	// Different IP still gets ordinary 401
	otherReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewBufferString(`{"password":"wrong"}`))
	otherReq.RemoteAddr = "5.6.7.8:10000"
	otherRec := httptest.NewRecorder()
	handler.ServeHTTP(otherRec, otherReq)

	// Then
	if limitedRec.Code != http.StatusTooManyRequests {
		t.Fatalf("same-IP different-port status = %d body=%s, want 429", limitedRec.Code, limitedRec.Body.String())
	}
	if otherRec.Code != http.StatusUnauthorized {
		t.Fatalf("different IP status = %d body=%s, want 401", otherRec.Code, otherRec.Body.String())
	}
}

func loginCookie(t *testing.T, handler http.Handler, password string) *http.Cookie {
	t.Helper()
	loginBody := bytes.NewBufferString(`{"password":"` + password + `"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/admin/login", loginBody)
	loginRec := httptest.NewRecorder()
	handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	return cookies[0]
}

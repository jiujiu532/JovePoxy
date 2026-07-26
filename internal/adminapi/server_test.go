package adminapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/adminapi"
	"jovepoxy/internal/auth"
	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
	"jovepoxy/internal/config"
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

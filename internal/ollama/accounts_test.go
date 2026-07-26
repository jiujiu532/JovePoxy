package ollama_test

import (
	"context"
	"strings"
	"testing"

	"jovepoxy/internal/crypto"
	"jovepoxy/internal/db"
	"jovepoxy/internal/ollama"
)

func TestAccountService_create_list_delete_masks_cookie(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	box, err := crypto.NewBox(strings.Repeat("s", 32))
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	service, err := ollama.NewAccountService(database, box)
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	created, err := service.Create(ctx, ollama.CreateInput{
		Name: "primary", SessionCookie: "token-abc", ShowSession: true, ShowWeekly: true, Enabled: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.MaskedCookie == "token-abc" || strings.Contains(created.MaskedCookie, "token-abc") {
		t.Fatalf("leaked cookie: %q", created.MaskedCookie)
	}

	list, err := service.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v err=%v", list, err)
	}
	cookie, err := service.GetCookie(ctx, created.ID)
	if err != nil || cookie != "__Secure-session=token-abc" {
		t.Fatalf("cookie = %q err=%v", cookie, err)
	}
	var ciphertext string
	if err := database.QueryRowContext(ctx, `SELECT session_cookie_ciphertext FROM ollama_accounts WHERE id = ?`, created.ID).Scan(&ciphertext); err != nil {
		t.Fatalf("read cipher: %v", err)
	}
	if strings.Contains(ciphertext, "token-abc") {
		t.Fatal("ciphertext contains plaintext")
	}
	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

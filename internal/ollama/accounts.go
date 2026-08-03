package ollama

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	secretcrypto "jovepoxy/internal/crypto"
	"jovepoxy/internal/idgen"
)

var (
	ErrInvalidInput    = errors.New("invalid Ollama account input")
	ErrAccountNotFound = errors.New("Ollama account not found")
)

// AccountID identifies a stored Ollama dashboard account.
type AccountID string

// Account is the masked control-plane view.
type Account struct {
	ID           AccountID `json:"id"`
	Name         string    `json:"name"`
	MaskedCookie string    `json:"masked_cookie"`
	ShowSession  bool      `json:"show_session"`
	ShowWeekly   bool      `json:"show_weekly"`
	Enabled      bool      `json:"enabled"`
}

// CreateInput creates an encrypted Ollama session account.
type CreateInput struct {
	Name          string
	SessionCookie string
	ShowSession   bool
	ShowWeekly    bool
	Enabled       bool
}

// UpdateInput patches optional Ollama account fields.
type UpdateInput struct {
	Name          *string
	SessionCookie *string
	ShowSession   *bool
	ShowWeekly    *bool
	Enabled       *bool
}

// AccountService stores encrypted Ollama session cookies.
type AccountService struct {
	database *sql.DB
	box      *secretcrypto.Box
}

// NewAccountService constructs the Ollama account store.
func NewAccountService(database *sql.DB, box *secretcrypto.Box) (*AccountService, error) {
	if database == nil || box == nil {
		return nil, ErrInvalidInput
	}
	return &AccountService{database: database, box: box}, nil
}

// Create encrypts and stores a session cookie. Plaintext is never returned later.
func (service *AccountService) Create(ctx context.Context, input CreateInput) (Account, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 200 {
		return Account{}, ErrInvalidInput
	}
	cookie, err := NormalizeSessionCookie(input.SessionCookie)
	if err != nil {
		return Account{}, err
	}
	ciphertext, err := service.box.Seal(cookie)
	if err != nil {
		return Account{}, fmt.Errorf("encrypt ollama cookie: %w", err)
	}
	id, err := newAccountID()
	if err != nil {
		return Account{}, err
	}
	if _, err := service.database.ExecContext(ctx, `
		INSERT INTO ollama_accounts (id, name, session_cookie_ciphertext, show_session, show_weekly, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`, string(id), name, ciphertext, boolInt(input.ShowSession), boolInt(input.ShowWeekly), boolInt(input.Enabled)); err != nil {
		return Account{}, fmt.Errorf("insert ollama account: %w", err)
	}
	return Account{
		ID: id, Name: name, MaskedCookie: maskCookie(cookie),
		ShowSession: input.ShowSession, ShowWeekly: input.ShowWeekly, Enabled: input.Enabled,
	}, nil
}

// List returns masked accounts.
func (service *AccountService) List(ctx context.Context) ([]Account, error) {
	rows, err := service.database.QueryContext(ctx, `
		SELECT id, name, session_cookie_ciphertext, show_session, show_weekly, enabled
		FROM ollama_accounts ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list ollama accounts: %w", err)
	}
	defer rows.Close()
	out := make([]Account, 0)
	for rows.Next() {
		var account Account
		var ciphertext string
		var showSession, showWeekly, enabled int
		if err := rows.Scan(&account.ID, &account.Name, &ciphertext, &showSession, &showWeekly, &enabled); err != nil {
			return nil, fmt.Errorf("scan ollama account: %w", err)
		}
		cookie, err := service.box.Open(ciphertext)
		if err != nil {
			account.MaskedCookie = "session=***"
		} else {
			account.MaskedCookie = maskCookie(cookie)
		}
		account.ShowSession = showSession != 0
		account.ShowWeekly = showWeekly != 0
		account.Enabled = enabled != 0
		out = append(out, account)
	}
	return out, rows.Err()
}

// Delete removes an account.
func (service *AccountService) Delete(ctx context.Context, id AccountID) error {
	if id == "" {
		return ErrInvalidInput
	}
	result, err := service.database.ExecContext(ctx, `DELETE FROM ollama_accounts WHERE id = ?`, string(id))
	if err != nil {
		return fmt.Errorf("delete ollama account: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ErrAccountNotFound
	}
	return nil
}

// SetEnabled toggles whether the account participates in quota scrape.
func (service *AccountService) SetEnabled(ctx context.Context, id AccountID, enabled bool) error {
	if id == "" {
		return ErrInvalidInput
	}
	result, err := service.database.ExecContext(ctx, `
		UPDATE ollama_accounts SET enabled = ? WHERE id = ?
	`, boolInt(enabled), string(id))
	if err != nil {
		return fmt.Errorf("set ollama account enabled: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ErrAccountNotFound
	}
	return nil
}

// Update patches mutable Ollama account fields.
func (service *AccountService) Update(ctx context.Context, id AccountID, input UpdateInput) (Account, error) {
	if id == "" {
		return Account{}, ErrInvalidInput
	}
	if input.Name == nil && input.SessionCookie == nil && input.ShowSession == nil &&
		input.ShowWeekly == nil && input.Enabled == nil {
		return Account{}, ErrInvalidInput
	}
	current, err := service.GetAccount(ctx, id)
	if err != nil {
		return Account{}, err
	}
	name := current.Name
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
		if name == "" || len(name) > 200 {
			return Account{}, ErrInvalidInput
		}
	}
	showSession := current.ShowSession
	if input.ShowSession != nil {
		showSession = *input.ShowSession
	}
	showWeekly := current.ShowWeekly
	if input.ShowWeekly != nil {
		showWeekly = *input.ShowWeekly
	}
	enabled := current.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	var ciphertext string
	if input.SessionCookie != nil {
		cookie, cookieErr := NormalizeSessionCookie(*input.SessionCookie)
		if cookieErr != nil {
			return Account{}, cookieErr
		}
		sealed, sealErr := service.box.Seal(cookie)
		if sealErr != nil {
			return Account{}, fmt.Errorf("encrypt ollama cookie: %w", sealErr)
		}
		ciphertext = sealed
	} else {
		loadErr := service.database.QueryRowContext(ctx, `
			SELECT session_cookie_ciphertext FROM ollama_accounts WHERE id = ?
		`, string(id)).Scan(&ciphertext)
		if errors.Is(loadErr, sql.ErrNoRows) {
			return Account{}, ErrAccountNotFound
		}
		if loadErr != nil {
			return Account{}, fmt.Errorf("load ollama cookie: %w", loadErr)
		}
	}

	result, err := service.database.ExecContext(ctx, `
		UPDATE ollama_accounts SET
			name = ?, session_cookie_ciphertext = ?, show_session = ?, show_weekly = ?, enabled = ?
		WHERE id = ?
	`, name, ciphertext, boolInt(showSession), boolInt(showWeekly), boolInt(enabled), string(id))
	if err != nil {
		return Account{}, fmt.Errorf("update ollama account: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return Account{}, ErrAccountNotFound
	}
	cookie, _ := service.box.Open(ciphertext)
	return Account{
		ID: id, Name: name, MaskedCookie: maskCookie(cookie),
		ShowSession: showSession, ShowWeekly: showWeekly, Enabled: enabled,
	}, nil
}

// GetCookie decrypts the session cookie for scrape callers.
func (service *AccountService) GetCookie(ctx context.Context, id AccountID) (string, error) {
	if id == "" {
		return "", ErrInvalidInput
	}
	var ciphertext string
	err := service.database.QueryRowContext(ctx, `
		SELECT session_cookie_ciphertext FROM ollama_accounts WHERE id = ?
	`, string(id)).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAccountNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load ollama cookie: %w", err)
	}
	return service.box.Open(ciphertext)
}

// GetAccount loads one masked account row.
func (service *AccountService) GetAccount(ctx context.Context, id AccountID) (Account, error) {
	list, err := service.List(ctx)
	if err != nil {
		return Account{}, err
	}
	for _, account := range list {
		if account.ID == id {
			return account, nil
		}
	}
	return Account{}, ErrAccountNotFound
}

func maskCookie(cookie string) string {
	if strings.Contains(cookie, "__Secure-session=") {
		return "__Secure-session=***"
	}
	if i := strings.Index(cookie, "="); i > 0 {
		return cookie[:i+1] + "***"
	}
	return "session=***"
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func newAccountID() (AccountID, error) {
	id, err := idgen.Prefixed("ollama_", 16)
	if err != nil {
		return "", err
	}
	return AccountID(id), nil
}

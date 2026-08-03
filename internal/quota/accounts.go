package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	secretcrypto "jovepoxy/internal/crypto"
	"jovepoxy/internal/idgen"
)

var (
	ErrInvalidAccountInput = errors.New("invalid OpenCode account input")
	ErrInvalidCookie       = errors.New("invalid OpenCode auth cookie")
	ErrAccountNotFound     = errors.New("OpenCode account not found")
)

type AccountID string

type Account struct {
	ID           AccountID
	Name         string
	WorkspaceID  string
	MaskedCookie string
	ShowRolling  bool
	ShowWeekly   bool
	ShowMonthly  bool
	Enabled      bool
}

type CreateAccountInput struct {
	Name        string
	WorkspaceID string
	AuthCookie  string
	ShowRolling bool
	ShowWeekly  bool
	ShowMonthly bool
	Enabled     bool
}

type UpdateAccountInput struct {
	Name        *string
	WorkspaceID *string
	AuthCookie  *string
	ShowRolling *bool
	ShowWeekly  *bool
	ShowMonthly *bool
	Enabled     *bool
}

type Credential struct {
	WorkspaceID string
	AuthCookie  string
}

type AccountService struct {
	database *sql.DB
	box      *secretcrypto.Box
}

func NewAccountService(database *sql.DB, box *secretcrypto.Box) (*AccountService, error) {
	if database == nil || box == nil {
		return nil, ErrInvalidAccountInput
	}
	return &AccountService{database: database, box: box}, nil
}

func (service *AccountService) Create(ctx context.Context, rawInput CreateAccountInput) (Account, error) {
	input, err := parseCreateAccountInput(rawInput)
	if err != nil {
		return Account{}, err
	}
	id, err := newAccountID()
	if err != nil {
		return Account{}, fmt.Errorf("generate OpenCode account identifier: %w", err)
	}
	ciphertext, err := service.box.Seal(input.authCookie)
	if err != nil {
		return Account{}, fmt.Errorf("encrypt OpenCode account credential: %w", err)
	}
	if _, err := service.database.ExecContext(ctx, `
		INSERT INTO opencode_accounts (
			id, name, workspace_id, auth_cookie_ciphertext, show_rolling, show_weekly, show_monthly, enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, input.name, input.workspaceID, ciphertext, input.showRolling, input.showWeekly, input.showMonthly, input.enabled,
	); err != nil {
		return Account{}, fmt.Errorf("insert OpenCode account: %w", err)
	}
	return input.account(id), nil
}

func (service *AccountService) List(ctx context.Context) ([]Account, error) {
	rows, err := service.database.QueryContext(ctx, `
		SELECT id, name, workspace_id, show_rolling, show_weekly, show_monthly, enabled
		FROM opencode_accounts ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list OpenCode accounts: %w", err)
	}
	defer rows.Close()
	accounts := make([]Account, 0)
	for rows.Next() {
		account, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate OpenCode accounts: %w", err)
	}
	return accounts, nil
}

func (service *AccountService) Update(ctx context.Context, id AccountID, rawInput UpdateAccountInput) (Account, error) {
	if id == "" || rawInput.empty() {
		return Account{}, ErrInvalidAccountInput
	}
	stored, err := service.loadStored(ctx, id)
	if err != nil {
		return Account{}, err
	}
	updated, err := stored.withUpdate(rawInput, service.box)
	if err != nil {
		return Account{}, err
	}
	result, err := service.database.ExecContext(ctx, `
		UPDATE opencode_accounts SET
			name = ?, workspace_id = ?, auth_cookie_ciphertext = ?,
			show_rolling = ?, show_weekly = ?, show_monthly = ?, enabled = ?
		WHERE id = ?`,
		updated.name, updated.workspaceID, updated.ciphertext,
		updated.showRolling, updated.showWeekly, updated.showMonthly, updated.enabled, id,
	)
	if err != nil {
		return Account{}, fmt.Errorf("update OpenCode account: %w", err)
	}
	if err := requireAccountAffected(result); err != nil {
		return Account{}, err
	}
	return updated.account(id), nil
}

func (service *AccountService) Delete(ctx context.Context, id AccountID) error {
	if id == "" {
		return ErrInvalidAccountInput
	}
	result, err := service.database.ExecContext(ctx, "DELETE FROM opencode_accounts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete OpenCode account: %w", err)
	}
	return requireAccountAffected(result)
}

func (service *AccountService) GetCredential(ctx context.Context, id AccountID) (Credential, error) {
	if id == "" {
		return Credential{}, ErrInvalidAccountInput
	}
	stored, err := service.loadStored(ctx, id)
	if err != nil {
		return Credential{}, err
	}
	cookie, err := service.box.Open(stored.ciphertext)
	if err != nil {
		return Credential{}, fmt.Errorf("decrypt OpenCode account credential: %w", err)
	}
	return Credential{WorkspaceID: stored.workspaceID, AuthCookie: cookie}, nil
}

func (service *AccountService) loadStored(ctx context.Context, id AccountID) (storedAccount, error) {
	var account storedAccount
	var showRolling, showWeekly, showMonthly, enabled int
	err := service.database.QueryRowContext(ctx, `
		SELECT name, workspace_id, auth_cookie_ciphertext, show_rolling, show_weekly, show_monthly, enabled
		FROM opencode_accounts WHERE id = ?`, id,
	).Scan(
		&account.name, &account.workspaceID, &account.ciphertext,
		&showRolling, &showWeekly, &showMonthly, &enabled,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storedAccount{}, ErrAccountNotFound
	}
	if err != nil {
		return storedAccount{}, fmt.Errorf("load OpenCode account: %w", err)
	}
	account.showRolling = showRolling != 0
	account.showWeekly = showWeekly != 0
	account.showMonthly = showMonthly != 0
	account.enabled = enabled != 0
	return account, nil
}

func newAccountID() (AccountID, error) {
	id, err := idgen.Prefixed("acct_", 16)
	if err != nil {
		return "", err
	}
	return AccountID(id), nil
}

package quota

import (
	"database/sql"
	"fmt"
	"strings"

	secretcrypto "jovepoxy/internal/crypto"
)

type accountInput struct {
	name        string
	workspaceID string
	authCookie  string
	showRolling bool
	showWeekly  bool
	showMonthly bool
	enabled     bool
}

type storedAccount struct {
	name        string
	workspaceID string
	ciphertext  string
	showRolling bool
	showWeekly  bool
	showMonthly bool
	enabled     bool
}

func parseCreateAccountInput(raw CreateAccountInput) (accountInput, error) {
	name, err := parseAccountName(raw.Name)
	if err != nil {
		return accountInput{}, err
	}
	workspaceID, err := parseWorkspaceID(raw.WorkspaceID)
	if err != nil {
		return accountInput{}, err
	}
	cookie, err := normalizeAuthCookie(raw.AuthCookie)
	if err != nil {
		return accountInput{}, err
	}
	return accountInput{
		name: name, workspaceID: workspaceID, authCookie: cookie,
		showRolling: raw.ShowRolling, showWeekly: raw.ShowWeekly, showMonthly: raw.ShowMonthly, enabled: raw.Enabled,
	}, nil
}

func parseAccountName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > 200 {
		return "", ErrInvalidAccountInput
	}
	return name, nil
}

func parseWorkspaceID(raw string) (string, error) {
	workspaceID := strings.TrimSpace(raw)
	if !strings.HasPrefix(workspaceID, "wrk_") || len(workspaceID) == len("wrk_") {
		return "", ErrInvalidAccountInput
	}
	for _, character := range workspaceID[len("wrk_"):] {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')) {
			return "", ErrInvalidAccountInput
		}
	}
	return workspaceID, nil
}

// NormalizeAuthCookie accepts auth=... or Cookie: auth=... and returns the
// strict Cookie header value used by scrape/usage callers.
func NormalizeAuthCookie(raw string) (string, error) {
	cookie := strings.TrimSpace(raw)
	if len(cookie) >= len("Cookie:") && strings.EqualFold(cookie[:len("Cookie:")], "Cookie:") {
		cookie = strings.TrimSpace(cookie[len("Cookie:"):])
	}
	if !strings.HasPrefix(cookie, "auth=") || len(cookie) == len("auth=") {
		return "", ErrInvalidCookie
	}
	for _, character := range cookie[len("auth="):] {
		if character <= 0x20 || character >= 0x7f || character == ';' {
			return "", ErrInvalidCookie
		}
	}
	return cookie, nil
}

func normalizeAuthCookie(raw string) (string, error) {
	return NormalizeAuthCookie(raw)
}

func (input accountInput) account(id AccountID) Account {
	return Account{
		ID: id, Name: input.name, WorkspaceID: input.workspaceID, MaskedCookie: "auth=***",
		ShowRolling: input.showRolling, ShowWeekly: input.showWeekly, ShowMonthly: input.showMonthly, Enabled: input.enabled,
	}
}

func (input UpdateAccountInput) empty() bool {
	return input.Name == nil && input.WorkspaceID == nil && input.AuthCookie == nil &&
		input.ShowRolling == nil && input.ShowWeekly == nil && input.ShowMonthly == nil && input.Enabled == nil
}

func (account storedAccount) withUpdate(input UpdateAccountInput, box *secretcrypto.Box) (storedAccount, error) {
	if input.Name != nil {
		name, err := parseAccountName(*input.Name)
		if err != nil {
			return storedAccount{}, err
		}
		account.name = name
	}
	if input.WorkspaceID != nil {
		workspaceID, err := parseWorkspaceID(*input.WorkspaceID)
		if err != nil {
			return storedAccount{}, err
		}
		account.workspaceID = workspaceID
	}
	if input.AuthCookie != nil {
		cookie, err := normalizeAuthCookie(*input.AuthCookie)
		if err != nil {
			return storedAccount{}, err
		}
		ciphertext, err := box.Seal(cookie)
		if err != nil {
			return storedAccount{}, fmt.Errorf("encrypt OpenCode account credential: %w", err)
		}
		account.ciphertext = ciphertext
	}
	if input.ShowRolling != nil {
		account.showRolling = *input.ShowRolling
	}
	if input.ShowWeekly != nil {
		account.showWeekly = *input.ShowWeekly
	}
	if input.ShowMonthly != nil {
		account.showMonthly = *input.ShowMonthly
	}
	if input.Enabled != nil {
		account.enabled = *input.Enabled
	}
	return account, nil
}

func (account storedAccount) account(id AccountID) Account {
	return Account{
		ID: id, Name: account.name, WorkspaceID: account.workspaceID, MaskedCookie: "auth=***",
		ShowRolling: account.showRolling, ShowWeekly: account.showWeekly, ShowMonthly: account.showMonthly, Enabled: account.enabled,
	}
}

type accountScanner interface {
	Scan(...any) error
}

func scanAccount(scanner accountScanner) (Account, error) {
	var account Account
	var showRolling, showWeekly, showMonthly, enabled int
	if err := scanner.Scan(
		&account.ID, &account.Name, &account.WorkspaceID,
		&showRolling, &showWeekly, &showMonthly, &enabled,
	); err != nil {
		return Account{}, fmt.Errorf("scan OpenCode account: %w", err)
	}
	account.MaskedCookie = "auth=***"
	account.ShowRolling = showRolling != 0
	account.ShowWeekly = showWeekly != 0
	account.ShowMonthly = showMonthly != 0
	account.Enabled = enabled != 0
	return account, nil
}

func requireAccountAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read OpenCode account affected rows: %w", err)
	}
	if affected != 1 {
		return ErrAccountNotFound
	}
	return nil
}

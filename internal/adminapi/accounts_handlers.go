package adminapi

import (
	"encoding/json"
	"net/http"

	"jovepoxy/internal/quota"
)

func (server server) listAccounts(writer http.ResponseWriter, request *http.Request) {
	if server.accounts == nil {
		writeJSON(writer, http.StatusOK, accountsResponse{Accounts: []accountDTO{}})
		return
	}
	list, err := server.accounts.List(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list accounts failed")
		return
	}
	writeJSON(writer, http.StatusOK, mapAccounts(list))
}

func (server server) createAccount(writer http.ResponseWriter, request *http.Request) {
	var body createAccountRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := server.accounts.Create(request.Context(), quota.CreateAccountInput{
		Name: body.Name, WorkspaceID: body.WorkspaceID, AuthCookie: body.AuthCookie,
		ShowRolling: body.ShowRolling, ShowWeekly: body.ShowWeekly, ShowMonthly: body.ShowMonthly, Enabled: body.Enabled,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, mapAccount(created))
}

func (server server) deleteAccount(writer http.ResponseWriter, request *http.Request) {
	if err := server.accounts.Delete(request.Context(), quota.AccountID(request.PathValue("id"))); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) enableAccount(writer http.ResponseWriter, request *http.Request) {
	server.setAccountEnabled(writer, request, true)
}

func (server server) disableAccount(writer http.ResponseWriter, request *http.Request) {
	server.setAccountEnabled(writer, request, false)
}

func (server server) setAccountEnabled(writer http.ResponseWriter, request *http.Request, enabled bool) {
	if server.accounts == nil {
		writeError(writer, http.StatusServiceUnavailable, "accounts unavailable")
		return
	}
	enabledCopy := enabled
	updated, err := server.accounts.Update(request.Context(), quota.AccountID(request.PathValue("id")), quota.UpdateAccountInput{
		Enabled: &enabledCopy,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, mapAccount(updated))
}

func (server server) getAccountCredential(writer http.ResponseWriter, request *http.Request) {
	if server.accounts == nil {
		writeError(writer, http.StatusServiceUnavailable, "accounts unavailable")
		return
	}
	credential, err := server.accounts.GetCredential(request.Context(), quota.AccountID(request.PathValue("id")))
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{
		"workspace_id": credential.WorkspaceID,
		"auth_cookie":  credential.AuthCookie,
	})
}

func (server server) listQuotas(writer http.ResponseWriter, request *http.Request) {
	if server.quotas == nil {
		writeJSON(writer, http.StatusOK, quotasResponse{Quotas: []accountQuotaDTO{}})
		return
	}
	results, err := server.quotas.Snapshot(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list quotas failed")
		return
	}
	writeJSON(writer, http.StatusOK, mapQuotas(results))
}

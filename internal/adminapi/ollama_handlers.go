package adminapi

import (
	"encoding/json"
	"net/http"

	"jovepoxy/internal/ollama"
)

func (server server) listOllamaAccounts(writer http.ResponseWriter, request *http.Request) {
	if server.ollamaAccounts == nil {
		writeJSON(writer, http.StatusOK, map[string]any{"accounts": []any{}})
		return
	}
	list, err := server.ollamaAccounts.List(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list ollama accounts failed")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"accounts": list})
}

func (server server) createOllamaAccount(writer http.ResponseWriter, request *http.Request) {
	if server.ollamaAccounts == nil {
		writeError(writer, http.StatusServiceUnavailable, "ollama accounts unavailable")
		return
	}
	var body struct {
		Name          string `json:"name"`
		SessionCookie string `json:"session_cookie"`
		ShowSession   *bool  `json:"show_session"`
		ShowWeekly    *bool  `json:"show_weekly"`
		Enabled       *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	showSession := true
	showWeekly := true
	enabled := true
	if body.ShowSession != nil {
		showSession = *body.ShowSession
	}
	if body.ShowWeekly != nil {
		showWeekly = *body.ShowWeekly
	}
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	created, err := server.ollamaAccounts.Create(request.Context(), ollama.CreateInput{
		Name: body.Name, SessionCookie: body.SessionCookie,
		ShowSession: showSession, ShowWeekly: showWeekly, Enabled: enabled,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, created)
}

func (server server) deleteOllamaAccount(writer http.ResponseWriter, request *http.Request) {
	if server.ollamaAccounts == nil {
		writeError(writer, http.StatusServiceUnavailable, "ollama accounts unavailable")
		return
	}
	if err := server.ollamaAccounts.Delete(request.Context(), ollama.AccountID(request.PathValue("id"))); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) enableOllamaAccount(writer http.ResponseWriter, request *http.Request) {
	server.setOllamaAccountEnabled(writer, request, true)
}

func (server server) disableOllamaAccount(writer http.ResponseWriter, request *http.Request) {
	server.setOllamaAccountEnabled(writer, request, false)
}

func (server server) setOllamaAccountEnabled(writer http.ResponseWriter, request *http.Request, enabled bool) {
	if server.ollamaAccounts == nil {
		writeError(writer, http.StatusServiceUnavailable, "ollama accounts unavailable")
		return
	}
	if err := server.ollamaAccounts.SetEnabled(request.Context(), ollama.AccountID(request.PathValue("id")), enabled); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	account, err := server.ollamaAccounts.GetAccount(request.Context(), ollama.AccountID(request.PathValue("id")))
	if err != nil {
		writeJSON(writer, http.StatusOK, okResponse{OK: true})
		return
	}
	writeJSON(writer, http.StatusOK, account)
}

func (server server) getOllamaAccountCredential(writer http.ResponseWriter, request *http.Request) {
	if server.ollamaAccounts == nil {
		writeError(writer, http.StatusServiceUnavailable, "ollama accounts unavailable")
		return
	}
	cookie, err := server.ollamaAccounts.GetCookie(request.Context(), ollama.AccountID(request.PathValue("id")))
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{
		"session_cookie": cookie,
	})
}

func (server server) listOllamaQuotas(writer http.ResponseWriter, request *http.Request) {
	if server.ollamaAccounts == nil || server.ollamaScraper == nil {
		writeJSON(writer, http.StatusOK, map[string]any{"quotas": []any{}})
		return
	}
	accounts, err := server.ollamaAccounts.List(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list ollama accounts failed")
		return
	}
	enabled := make([]ollama.Account, 0)
	cookies := make(map[ollama.AccountID]string)
	for _, account := range accounts {
		if !account.Enabled {
			continue
		}
		cookie, err := server.ollamaAccounts.GetCookie(request.Context(), account.ID)
		if err != nil {
			continue
		}
		enabled = append(enabled, account)
		cookies[account.ID] = cookie
	}
	results := server.ollamaScraper.FetchAll(request.Context(), enabled, cookies)
	writeJSON(writer, http.StatusOK, map[string]any{"quotas": mapOllamaQuotas(results)})
}

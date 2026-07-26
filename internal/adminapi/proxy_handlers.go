package adminapi

import (
	"encoding/json"
	"net/http"

	"jovepoxy/internal/proxypool"
)

func (server server) listProxies(writer http.ResponseWriter, request *http.Request) {
	if server.proxies == nil {
		writeJSON(writer, http.StatusOK, map[string]any{"proxies": []any{}})
		return
	}
	list, err := server.proxies.List(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list proxies failed")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"proxies": list})
}

func (server server) createProxy(writer http.ResponseWriter, request *http.Request) {
	if server.proxies == nil {
		writeError(writer, http.StatusServiceUnavailable, "proxy pool unavailable")
		return
	}
	var body struct {
		Label  string `json:"label"`
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := server.proxies.Create(request.Context(), proxypool.CreateInput{
		Label: body.Label, URL: body.URL, Weight: body.Weight,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, created)
}

func (server server) enableProxy(writer http.ResponseWriter, request *http.Request) {
	if err := server.proxies.SetEnabled(request.Context(), proxypool.ProxyID(request.PathValue("id")), true); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) disableProxy(writer http.ResponseWriter, request *http.Request) {
	if err := server.proxies.SetEnabled(request.Context(), proxypool.ProxyID(request.PathValue("id")), false); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) deleteProxy(writer http.ResponseWriter, request *http.Request) {
	if err := server.proxies.Delete(request.Context(), proxypool.ProxyID(request.PathValue("id"))); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) updateProxy(writer http.ResponseWriter, request *http.Request) {
	if server.proxies == nil {
		writeError(writer, http.StatusServiceUnavailable, "proxy pool unavailable")
		return
	}
	var body struct {
		Label  string `json:"label"`
		URL    string `json:"url"`
		Weight int    `json:"weight"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	updated, err := server.proxies.Update(request.Context(), proxypool.ProxyID(request.PathValue("id")), proxypool.UpdateInput{
		Label: body.Label, URL: body.URL, Weight: body.Weight,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, updated)
}

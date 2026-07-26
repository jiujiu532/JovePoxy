package adminapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"jovepoxy/internal/zenpool"
)

func (server server) listZenKeys(writer http.ResponseWriter, request *http.Request) {
	if server.pool == nil {
		writeJSON(writer, http.StatusOK, zenKeysResponse{Keys: []zenKeyDTO{}})
		return
	}
	provider := zenpool.Provider(strings.TrimSpace(request.URL.Query().Get("provider")))
	var list []zenpool.Metadata
	var err error
	if provider == "" {
		list, err = server.pool.List(request.Context())
	} else {
		list, err = server.pool.ListByProvider(request.Context(), provider)
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list keys failed")
		return
	}
	writeJSON(writer, http.StatusOK, mapZenKeys(list))
}

func (server server) createZenKey(writer http.ResponseWriter, request *http.Request) {
	var body createZenKeyRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	provider := zenpool.Provider(strings.TrimSpace(body.Provider))
	if provider == "" {
		provider = zenpool.ProviderOpenCode
	}
	created, err := server.pool.Create(request.Context(), zenpool.CreateInput{
		Label: body.Label, Secret: body.Secret, Weight: body.Weight, Provider: provider,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, mapZenKeys([]zenpool.Metadata{created}).Keys[0])
}

func (server server) enableZenKey(writer http.ResponseWriter, request *http.Request) {
	if err := server.pool.SetEnabled(request.Context(), zenpool.KeyID(request.PathValue("id")), true); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) disableZenKey(writer http.ResponseWriter, request *http.Request) {
	if err := server.pool.SetEnabled(request.Context(), zenpool.KeyID(request.PathValue("id")), false); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) deleteZenKey(writer http.ResponseWriter, request *http.Request) {
	if err := server.pool.Delete(request.Context(), zenpool.KeyID(request.PathValue("id"))); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) updateZenKey(writer http.ResponseWriter, request *http.Request) {
	if server.pool == nil {
		writeError(writer, http.StatusServiceUnavailable, "zen pool unavailable")
		return
	}
	var body updateZenKeyRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	updated, err := server.pool.Update(request.Context(), zenpool.KeyID(request.PathValue("id")), zenpool.UpdateInput{
		Label: body.Label, Secret: body.Secret, Weight: body.Weight,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, mapZenKeys([]zenpool.Metadata{updated}).Keys[0])
}

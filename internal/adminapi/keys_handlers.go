package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"jovepoxy/internal/keys"
)

// writeLocalKeyError maps keys package errors to HTTP status codes.
func writeLocalKeyError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, keys.ErrNotFound):
		writeError(writer, http.StatusNotFound, "key not found")
	case errors.Is(err, keys.ErrInvalidInput):
		writeError(writer, http.StatusBadRequest, err.Error())
	default:
		writeError(writer, http.StatusBadRequest, err.Error())
	}
}

func (server server) listLocalKeys(writer http.ResponseWriter, request *http.Request) {
	if server.keys == nil {
		writeJSON(writer, http.StatusOK, localKeysResponse{Keys: []localKeyDTO{}})
		return
	}
	list, err := server.keys.List(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "list keys failed")
		return
	}
	writeJSON(writer, http.StatusOK, mapLocalKeys(list))
}

func (server server) createLocalKey(writer http.ResponseWriter, request *http.Request) {
	var body createLocalKeyRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := server.keys.Create(request.Context(), keys.CreateInput{
		Label: body.Label, RPMLimit: body.RPMLimit, DailyLimit: body.DailyLimit,
	})
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusCreated, mapLocalKeyCreated(created))
}

func (server server) revokeLocalKey(writer http.ResponseWriter, request *http.Request) {
	if err := server.keys.Revoke(request.Context(), keys.KeyID(request.PathValue("id"))); err != nil {
		writeLocalKeyError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) updateLocalKey(writer http.ResponseWriter, request *http.Request) {
	if server.keys == nil {
		writeError(writer, http.StatusServiceUnavailable, "keys unavailable")
		return
	}
	var body updateLocalKeyRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := server.keys.Update(request.Context(), keys.KeyID(request.PathValue("id")), keys.UpdateInput{
		Label: body.Label, RPMLimit: body.RPMLimit, DailyLimit: body.DailyLimit,
	}); err != nil {
		writeLocalKeyError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) enableLocalKey(writer http.ResponseWriter, request *http.Request) {
	if server.keys == nil {
		writeError(writer, http.StatusServiceUnavailable, "keys unavailable")
		return
	}
	if err := server.keys.SetEnabled(request.Context(), keys.KeyID(request.PathValue("id")), true); err != nil {
		writeLocalKeyError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) disableLocalKey(writer http.ResponseWriter, request *http.Request) {
	if server.keys == nil {
		writeError(writer, http.StatusServiceUnavailable, "keys unavailable")
		return
	}
	if err := server.keys.SetEnabled(request.Context(), keys.KeyID(request.PathValue("id")), false); err != nil {
		writeLocalKeyError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

// revealLocalKey returns the full secret once for clipboard copy (admin session required).
func (server server) revealLocalKey(writer http.ResponseWriter, request *http.Request) {
	if server.keys == nil {
		writeError(writer, http.StatusServiceUnavailable, "keys unavailable")
		return
	}
	secret, err := server.keys.Reveal(request.Context(), keys.KeyID(request.PathValue("id")))
	if err != nil {
		switch {
		case errors.Is(err, keys.ErrNotFound):
			writeError(writer, http.StatusNotFound, "key not found")
		case errors.Is(err, keys.ErrSecretUnavailable):
			writeError(writer, http.StatusGone, "secret unavailable for this key; create a new key")
		case errors.Is(err, keys.ErrInvalidInput):
			writeError(writer, http.StatusBadRequest, err.Error())
		default:
			writeError(writer, http.StatusInternalServerError, "reveal key failed")
		}
		return
	}
	writeJSON(writer, http.StatusOK, localKeyRevealDTO{Secret: secret})
}

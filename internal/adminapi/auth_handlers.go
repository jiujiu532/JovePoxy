package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"jovepoxy/internal/auth"
)

func (server server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if server.auth == nil {
			writeError(writer, http.StatusServiceUnavailable, "admin auth unavailable")
			return
		}
		token := sessionTokenFrom(request, server.cookieName())
		if err := server.auth.Verify(request.Context(), token); err != nil {
			writeError(writer, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(writer, request)
	}
}

func sessionTokenFrom(request *http.Request, cookieName string) string {
	if cookie, err := request.Cookie(cookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	header := request.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func (server server) login(writer http.ResponseWriter, request *http.Request) {
	if server.auth == nil {
		writeError(writer, http.StatusServiceUnavailable, "admin auth unavailable")
		return
	}
	var body loginRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	session, err := server.auth.Login(request.Context(), auth.LoginInput{
		Password: body.Password, Source: request.RemoteAddr,
	})
	if errors.Is(err, auth.ErrRateLimited) {
		writeError(writer, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "invalid password")
		return
	}
	server.setSessionCookie(writer, session.Token, session.ExpiresAt)
	writeJSON(writer, http.StatusOK, loginResponse{OK: true, ExpiresAt: session.ExpiresAt.UTC()})
}

func (server server) logout(writer http.ResponseWriter, request *http.Request) {
	token := sessionTokenFrom(request, server.cookieName())
	if token != "" {
		_ = server.auth.Logout(request.Context(), token)
	}
	server.clearSessionCookie(writer)
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (server server) me(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, meResponse{OK: true, Role: "admin"})
}

func (server server) changePassword(writer http.ResponseWriter, request *http.Request) {
	if server.auth == nil {
		writeError(writer, http.StatusServiceUnavailable, "admin auth unavailable")
		return
	}
	var body changePasswordRequest
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON")
		return
	}
	err := server.auth.ChangePassword(request.Context(), body.CurrentPassword, body.NewPassword)
	if errors.Is(err, auth.ErrUnauthorized) {
		writeError(writer, http.StatusUnauthorized, "当前密码不正确")
		return
	}
	if errors.Is(err, auth.ErrWeakPassword) {
		writeError(writer, http.StatusBadRequest, "新密码至少 8 位")
		return
	}
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	// Sessions already revoked in ChangePassword; clear this browser cookie too.
	server.clearSessionCookie(writer)
	writeJSON(writer, http.StatusOK, okResponse{OK: true})
}

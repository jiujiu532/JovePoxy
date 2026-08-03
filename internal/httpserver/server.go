// Package httpserver exposes the public OpenAI-compatible HTTP surface.
package httpserver

import (
	"net/http"

	"jovepoxy/internal/keys"
	"jovepoxy/internal/models"
	"jovepoxy/internal/proxypool"
	"jovepoxy/internal/reqlog"
	"jovepoxy/internal/zen"
	"jovepoxy/internal/zenpool"
)

// Dependencies are the already-constructed services required by the data plane.
type Dependencies struct {
	Keys          *keys.Service
	Catalog       *models.Catalog
	Zen           *zen.Client // free OpenCode Zen (Bearer public)
	ZenGo         *zen.Client // paid OpenCode Go suite
	Ollama        *zen.Client
	Pool          *zenpool.Service
	Proxies       *proxypool.Service
	Logs          *reqlog.Service
	Version       string
	ShowAllModels bool
}

type server struct {
	keys          *keys.Service
	catalog       *models.Catalog
	zen           *zen.Client
	zenGo         *zen.Client
	ollama        *zen.Client
	pool          *zenpool.Service
	proxies       *proxypool.Service
	logs          *reqlog.Service
	version       string
	showAllModels bool
}

// New constructs the public data-plane handler. Models and health are public;
// chat completions authenticate only local API keys.
func New(dependencies Dependencies) http.Handler {
	version := dependencies.Version
	if version == "" {
		version = "jovepoxy"
	}
	server := server{
		keys: dependencies.Keys, catalog: dependencies.Catalog, zen: dependencies.Zen,
		zenGo: dependencies.ZenGo, ollama: dependencies.Ollama, pool: dependencies.Pool,
		proxies: dependencies.Proxies, logs: dependencies.Logs, version: version,
		showAllModels: dependencies.ShowAllModels,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /metrics", server.metrics)
	mux.HandleFunc("GET /v1/models", server.listModels)
	mux.HandleFunc("POST /v1/chat/completions", server.observe("/v1/chat/completions", server.chatCompletions))
	mux.HandleFunc("POST /v1/messages", server.observe("/v1/messages", server.messages))
	mux.HandleFunc("POST /v1/responses", server.observe("/v1/responses", server.responsesHandler))
	return mux
}

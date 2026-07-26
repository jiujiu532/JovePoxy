package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// Dist holds the built SPA assets from web/dist.
//
//go:embed all:dist
var dist embed.FS

// Handler serves the embedded SPA and falls back to index.html for client routes.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := strings.TrimPrefix(request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			request.URL.Path = "/"
			fileServer.ServeHTTP(writer, request)
			return
		}
		fileServer.ServeHTTP(writer, request)
	}), nil
}

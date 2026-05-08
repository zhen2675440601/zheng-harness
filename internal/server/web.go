package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"zheng-harness/internal/config"
)

//go:embed web
var embeddedWebAssets embed.FS

func RegisterWebRoutes(r chi.Router, api *API) {
	RegisterWebRoutesWithFS(r, api, embeddedWebAssets)
}

func RegisterWebRoutesWithFS(r chi.Router, api *API, embeddedAssets fs.FS) {
	if r == nil || api == nil || !api.Config.Server.WebUIEnabled {
		return
	}
	assets := resolveWebAssets(api.Config, embeddedAssets)
	indexPath := strings.TrimSpace(api.Config.Server.WebUI.IndexPath)

	r.Route("/web", func(r chi.Router) {
		webFS, err := fs.Sub(assets, "web")
		if err != nil {
			return
		}
		fileServer := http.FileServerFS(webFS)
		r.Get("/*", http.StripPrefix("/web", fileServer).ServeHTTP)
	})

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		data, err := fs.ReadFile(assets, indexPath)
		if err != nil {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}

func resolveWebAssets(cfg config.Config, embeddedAssets fs.FS) fs.FS {
	_ = cfg
	return embeddedAssets
}

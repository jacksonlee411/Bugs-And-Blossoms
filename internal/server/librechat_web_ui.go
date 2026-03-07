package server

import (
	"bytes"
	"io/fs"
	"net/http"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const (
	libreChatFormalEntryPrefix = "/app/assistant/librechat"
	libreChatStaticPrefix      = "/assets/librechat-web"
	libreChatWebUIIndexPath    = "assets/librechat-web/index.html"
)

func newLibreChatWebUIHandler(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		serveLibreChatWebUIIndex(w, r, assets)
	})
}

func serveLibreChatWebUIIndex(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	b, err := fs.ReadFile(assets, libreChatWebUIIndexPath)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	b = bytes.ReplaceAll(b, []byte(`<base href="/" />`), []byte(`<base href="`+libreChatStaticPrefix+`/" />`))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(b)
}

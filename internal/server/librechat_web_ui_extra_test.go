package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLibreChatWebUIHandler_ExtraCoverage(t *testing.T) {
	assets := fstest.MapFS{
		libreChatWebUIIndexPath: &fstest.MapFile{Data: []byte(`<html><head><base href="/" /></head><body>LibreChat</body></html>`)},
	}
	h := newLibreChatWebUIHandler(assets)

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, libreChatFormalEntryPrefix, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("head request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, libreChatFormalEntryPrefix, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("expected empty body for HEAD, got=%q", rec.Body.String())
		}
	})

	t.Run("read error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, libreChatFormalEntryPrefix, nil)
		rec := httptest.NewRecorder()
		serveLibreChatWebUIIndex(rec, req, fstest.MapFS{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("body rewrite", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, libreChatFormalEntryPrefix, nil)
		rec := httptest.NewRecorder()
		serveLibreChatWebUIIndex(rec, req, fs.FS(assets))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), libreChatStaticPrefix+`/`) {
			t.Fatalf("unexpected body=%q", rec.Body.String())
		}
	})
}

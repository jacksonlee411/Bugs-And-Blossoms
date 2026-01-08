package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRenderTopbar_UsesHXCurrentURLAndPreservesQueryExceptAsOf(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ui/topbar?as_of=2026-01-01", nil)
	req.Header.Set("HX-Current-URL", "http://localhost/org/assignments?as_of=2026-01-01&pernr=123&pernr=456&q=abc")

	out := renderTopbar(req, "2026-01-01")

	if !strings.Contains(out, `action="/org/assignments"`) {
		t.Fatalf("unexpected target: %q", out)
	}
	if strings.Contains(out, `type="hidden" name="as_of"`) {
		t.Fatalf("unexpected as_of hidden input: %q", out)
	}
	if !strings.Contains(out, `type="hidden" name="pernr" value="123"`) || !strings.Contains(out, `type="hidden" name="pernr" value="456"`) {
		t.Fatalf("missing pernr inputs: %q", out)
	}
	if !strings.Contains(out, `type="hidden" name="q" value="abc"`) {
		t.Fatalf("missing q input: %q", out)
	}
	if !strings.Contains(out, `hx-target="#content"`) || !strings.Contains(out, `hx-push-url="true"`) {
		t.Fatalf("missing htmx attrs: %q", out)
	}
}

func TestRenderTopbar_FallsBackToRefererAndHandlesBadURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ui/topbar?as_of=2026-01-01", nil)
	req.Header.Set("Referer", "http://localhost/%zz")

	out := renderTopbar(req, "2026-01-01")
	if !strings.Contains(out, `action="/app/home"`) {
		t.Fatalf("unexpected target: %q", out)
	}
}

func TestRenderTopbar_RewritesAppToAppHome(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ui/topbar?as_of=2026-01-01", nil)
	req.Header.Set("HX-Current-URL", "http://localhost/app?as_of=2026-01-01&q=abc")

	out := renderTopbar(req, "2026-01-01")
	if !strings.Contains(out, `action="/app/home"`) {
		t.Fatalf("unexpected target: %q", out)
	}
	if !strings.Contains(out, `type="hidden" name="q" value="abc"`) {
		t.Fatalf("missing q input: %q", out)
	}
}

func TestRenderTopbar_ParsesURLWithEmptyPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ui/topbar?as_of=2026-01-01", nil)
	req.Header.Set("HX-Current-URL", "http://localhost?x=1")

	out := renderTopbar(req, "2026-01-01")
	if !strings.Contains(out, `action="/app/home"`) {
		t.Fatalf("unexpected target: %q", out)
	}
	if !strings.Contains(out, `type="hidden" name="x" value="1"`) {
		t.Fatalf("missing x input: %q", out)
	}
}

func TestReplaceMainContent(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		out, err := replaceMainContent(`<main id="content">OLD</main>`, "NEW")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "OLD") || !strings.Contains(out, ">NEW</main>") {
			t.Fatalf("unexpected output: %q", out)
		}
	})

	t.Run("missing mount", func(t *testing.T) {
		_, err := replaceMainContent(`<main>OLD</main>`, "NEW")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing main", func(t *testing.T) {
		_, err := replaceMainContent(`<div id="content"></div>`, "NEW")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unterminated main tag", func(t *testing.T) {
		_, err := replaceMainContent(`<main id="content"`, "NEW")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing closing main", func(t *testing.T) {
		_, err := replaceMainContent(`<main id="content">OLD`, "NEW")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRenderAstroShellFromTemplate(t *testing.T) {
	const tmpl = `<html><body>` +
		`<aside id="nav" hx-get="/ui/nav?as_of=__BB_AS_OF__" hx-trigger="load"></aside>` +
		`<main id="content"></main>` +
		`</body></html>`

	t.Run("anonymous removes hx-trigger", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app?as_of=2026-01-01", nil)
		out, err := renderAstroShellFromTemplate(tmpl, req, "2026-01-01", "")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, astroAsOfToken) {
			t.Fatalf("token not injected: %q", out)
		}
		if strings.Contains(out, `hx-trigger="load"`) {
			t.Fatalf("expected hx-trigger removed: %q", out)
		}
	})

	t.Run("logged-in keeps hx-trigger and injects body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app?as_of=2026-01-01", nil)
		req = req.WithContext(withPrincipal(req.Context(), Principal{ID: "p1", TenantID: "t1", Status: "active"}))
		out, err := renderAstroShellFromTemplate(tmpl, req, "2026-01-01", "<h1>X</h1>")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, `hx-trigger="load"`) {
			t.Fatalf("expected hx-trigger kept: %q", out)
		}
		if !strings.Contains(out, "<h1>X</h1>") {
			t.Fatalf("expected body injected: %q", out)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := renderAstroShellFromTemplate(`<main id="content"></main>`, httptest.NewRequest(http.MethodGet, "/app", nil), "2026-01-01", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("body injection error", func(t *testing.T) {
		_, err := renderAstroShellFromTemplate(`__BB_AS_OF__`, httptest.NewRequest(http.MethodGet, "/app", nil), "2026-01-01", "<h1>Y</h1>")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("token still present after injection", func(t *testing.T) {
		_, err := renderAstroShellFromTemplate(tmpl, httptest.NewRequest(http.MethodGet, "/app", nil), astroAsOfToken, "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRenderAstroShellFromAssets_ReadFileError(t *testing.T) {
	_, err := renderAstroShellFromAssets(fstest.MapFS{}, httptest.NewRequest(http.MethodGet, "/app", nil), "2026-01-01", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteShellWithStatusFromAssets(t *testing.T) {
	const tmpl = `<main id="content"></main><aside id="nav" hx-get="/ui/nav?as_of=__BB_AS_OF__" hx-trigger="load"></aside>`
	okFS := fstest.MapFS{
		"assets/astro/app.html": &fstest.MapFile{Data: []byte(tmpl)},
	}

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/login?as_of=2026-01-01", nil)
		writeShellWithStatusFromAssets(rec, req, http.StatusTeapot, "<h1>OK</h1>", okFS)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "<h1>OK</h1>") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("render error writes 500", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		writeShellWithStatusFromAssets(rec, req, http.StatusOK, "<h1>OK</h1>", fstest.MapFS{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

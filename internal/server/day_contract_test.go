package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestParseRequiredDay(t *testing.T) {
	t.Run("missing as_of", func(t *testing.T) {
		_, err := parseRequiredDay("", "as_of")
		if err == nil {
			t.Fatal("expected error")
		}
		code, message, ok := dayFieldErrorDetails(err)
		if !ok || code != "invalid_as_of" || message != "as_of required" {
			t.Fatalf("code=%q message=%q ok=%v", code, message, ok)
		}
	})

	t.Run("invalid effective_date", func(t *testing.T) {
		_, err := parseRequiredDay("bad", "effective_date")
		if err == nil {
			t.Fatal("expected error")
		}
		code, message, ok := dayFieldErrorDetails(err)
		if !ok || code != "invalid_effective_date" || message != "invalid effective_date" {
			t.Fatalf("code=%q message=%q ok=%v", code, message, ok)
		}
	})

	t.Run("valid query day", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x?as_of=2026-01-01", nil)
		got, err := parseRequiredQueryDay(req, "as_of")
		if err != nil || got != "2026-01-01" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})
}

func TestRequestDateFallbackStopline(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(asof|effectivedate)[a-z0-9_]*\s*:?=\s*time\.Now\(\)\.UTC\(\)\.Format\("2006-01-02"\)`),
		regexp.MustCompile(`\borgUnitDefaultDate\(`),
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(content)
		for _, pattern := range patterns {
			if pattern.MatchString(text) {
				t.Fatalf("stopline matched %q in %s", pattern.String(), file)
			}
		}
	}
}

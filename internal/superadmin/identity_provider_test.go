package superadmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKratosIdentityProvider_AuthenticatePassword(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"flow-1"}`))
	})

	mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("flow") != "flow-1" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad flow"))
			return
		}
		var body struct {
			Method     string `json:"method"`
			Identifier string `json:"identifier"`
			Password   string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Method != "password" || body.Identifier != "sa:admin@example.invalid" || body.Password != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_token":"tok-1"}`))
	})

	mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Session-Token") != "tok-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"identity":{"id":"kid-1","traits":{"email":"admin@example.invalid"}}}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("KRATOS_PUBLIC_URL", srv.URL)

	idp, err := newKratosIdentityProviderFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	ident, err := idp.AuthenticatePassword(context.Background(), "Admin@Example.invalid", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if ident.Email != "admin@example.invalid" || ident.KratosIdentityID != "kid-1" {
		t.Fatalf("ident=%+v", ident)
	}

	_, err = idp.AuthenticatePassword(context.Background(), "admin@example.invalid", "bad")
	if err == nil || err != errInvalidCredentials {
		t.Fatalf("expected invalid credentials, got: %v", err)
	}
}

func TestKratosIdentityProvider_EmailMismatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"flow-1"}`))
	})
	mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_token":"tok-1"}`))
	})
	mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"identity":{"id":"kid-1","traits":{"email":"other@example.invalid"}}}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("KRATOS_PUBLIC_URL", srv.URL)

	idp, err := newKratosIdentityProviderFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	_, err = idp.AuthenticatePassword(context.Background(), "admin@example.invalid", "pw")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestKratosIdentityProvider_MissingIdentityID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"flow-1"}`))
	})
	mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_token":"tok-1"}`))
	})
	mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"identity":{"id":"","traits":{"email":"admin@example.invalid"}}}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("KRATOS_PUBLIC_URL", srv.URL)

	idp, err := newKratosIdentityProviderFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	_, err = idp.AuthenticatePassword(context.Background(), "admin@example.invalid", "pw")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestKratosIdentityProvider_HTTP500_NotInvalidCredentials(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"flow-1"}`))
	})
	mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("KRATOS_PUBLIC_URL", srv.URL)

	idp, err := newKratosIdentityProviderFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	_, err = idp.AuthenticatePassword(context.Background(), "admin@example.invalid", "pw")
	if err == nil || err == errInvalidCredentials {
		t.Fatalf("err=%v", err)
	}
}

func TestStringTrait(t *testing.T) {
	if _, ok := stringTrait(map[string]any{}, "email"); ok {
		t.Fatal("expected missing")
	}
	if _, ok := stringTrait(map[string]any{"email": 123}, "email"); ok {
		t.Fatal("expected non-string")
	}
	if _, ok := stringTrait(map[string]any{"email": "   "}, "email"); ok {
		t.Fatal("expected empty")
	}
	if v, ok := stringTrait(map[string]any{"email": " a@b.com "}, "email"); !ok || strings.TrimSpace(v) != "a@b.com" {
		t.Fatalf("v=%q ok=%v", v, ok)
	}
}

func TestNewKratosIdentityProviderFromEnv_BadURL(t *testing.T) {
	t.Setenv("KRATOS_PUBLIC_URL", "ftp://x")
	if _, err := newKratosIdentityProviderFromEnv(); err == nil {
		t.Fatal("expected error")
	}
}

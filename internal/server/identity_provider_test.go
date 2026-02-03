package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/iam/infrastructure/kratos"
)

func TestNewKratosIdentityProviderFromEnv_DefaultAndInvalid(t *testing.T) {
	t.Setenv("KRATOS_PUBLIC_URL", "")
	if _, err := newKratosIdentityProviderFromEnv(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KRATOS_PUBLIC_URL", "ftp://localhost:4433")
	if _, err := newKratosIdentityProviderFromEnv(); err == nil {
		t.Fatal("expected error")
	}
}

func TestStringTrait(t *testing.T) {
	if _, ok := stringTrait(nil, "x"); ok {
		t.Fatal("expected ok=false")
	}
	if _, ok := stringTrait(map[string]any{}, "x"); ok {
		t.Fatal("expected ok=false")
	}
	if _, ok := stringTrait(map[string]any{"x": 1}, "x"); ok {
		t.Fatal("expected ok=false")
	}
	if _, ok := stringTrait(map[string]any{"x": "   "}, "x"); ok {
		t.Fatal("expected ok=false")
	}
	if got, ok := stringTrait(map[string]any{"x": " a "}, "x"); !ok || got != "a" {
		t.Fatalf("got=%q ok=%v", got, ok)
	}
}

func TestKratosIdentityProvider_AuthenticatePassword(t *testing.T) {
	type stubState struct {
		loginStatus int
		whoamiID    string
		tenantID    string
		email       string
		whoamiBody  string
	}

	st := &stubState{
		loginStatus: http.StatusOK,
		whoamiID:    "kid1",
		tenantID:    "t1",
		email:       "a@example.invalid",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "flow1"})
	})
	mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if st.loginStatus/100 != 2 {
			w.WriteHeader(st.loginStatus)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"session_token": "st1"})
	})
	mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		if r.Header.Get("X-Session-Token") != "st1" {
			t.Fatalf("missing session token header")
		}
		if st.whoamiBody != "" {
			_, _ = io.WriteString(w, st.whoamiBody)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{
				"id": st.whoamiID,
				"traits": map[string]any{
					"tenant_uuid": st.tenantID,
					"email":       st.email,
				},
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	kc, err := kratos.New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	p := &kratosIdentityProvider{client: kc}

	t.Run("success", func(t *testing.T) {
		st.loginStatus = http.StatusOK
		st.whoamiID = "kid1"
		st.tenantID = "t1"
		st.email = "a@example.invalid"
		st.whoamiBody = ""

		ident, err := p.AuthenticatePassword(context.Background(), Tenant{ID: "t1"}, "A@Example.Invalid", "pw")
		if err != nil {
			t.Fatal(err)
		}
		if ident.KratosIdentityID != "kid1" || ident.Email != "a@example.invalid" {
			t.Fatalf("ident=%+v", ident)
		}
	})

	t.Run("invalid_credentials", func(t *testing.T) {
		st.loginStatus = http.StatusUnauthorized
		st.whoamiBody = ""

		_, err := p.AuthenticatePassword(context.Background(), Tenant{ID: "t1"}, "a@example.invalid", "pw")
		if !errors.Is(err, errInvalidCredentials) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("tenant_mismatch", func(t *testing.T) {
		st.loginStatus = http.StatusOK
		st.tenantID = "t2"
		st.email = "a@example.invalid"
		st.whoamiBody = ""

		if _, err := p.AuthenticatePassword(context.Background(), Tenant{ID: "t1"}, "a@example.invalid", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("email_mismatch", func(t *testing.T) {
		st.loginStatus = http.StatusOK
		st.tenantID = "t1"
		st.email = "b@example.invalid"
		st.whoamiBody = ""

		if _, err := p.AuthenticatePassword(context.Background(), Tenant{ID: "t1"}, "a@example.invalid", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing_identity_id", func(t *testing.T) {
		st.loginStatus = http.StatusOK
		st.whoamiID = ""
		st.tenantID = "t1"
		st.email = "a@example.invalid"
		st.whoamiBody = ""

		if _, err := p.AuthenticatePassword(context.Background(), Tenant{ID: "t1"}, "a@example.invalid", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unexpected_kratos_error", func(t *testing.T) {
		st.loginStatus = http.StatusOK
		st.whoamiBody = "{"

		if _, err := p.AuthenticatePassword(context.Background(), Tenant{ID: "t1"}, "a@example.invalid", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})
}

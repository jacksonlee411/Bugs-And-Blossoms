package kratos

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type errRoundTripper struct{}

func (errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("boom")
}

func TestNew(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := New("   "); err == nil {
		t.Fatal("expected error")
	}
	if _, err := New("ftp://localhost:4433"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := New("http://"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := New("http://%zz"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := New("http://localhost:4433"); err != nil {
		t.Fatal(err)
	}
}

func TestClient_LoginPassword_Success(t *testing.T) {
	var gotLoginMethod string
	var gotIdentifier string
	var gotPassword string
	var gotWhoamiToken string

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
		if q := r.URL.Query().Get("flow"); q != "flow1" {
			t.Fatalf("flow=%q", q)
		}
		b, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(b, &req); err != nil {
			t.Fatal(err)
		}
		gotLoginMethod, _ = req["method"].(string)
		gotIdentifier, _ = req["identifier"].(string)
		gotPassword, _ = req["password"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{"session_token": "st1"})
	})
	mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		gotWhoamiToken = r.Header.Get("X-Session-Token")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{
				"id": "ident1",
				"traits": map[string]any{
					"tenant_id": "t1",
					"email":     "a@example.invalid",
				},
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	ident, err := c.LoginPassword(context.Background(), "t1:a@example.invalid", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if gotLoginMethod != "password" {
		t.Fatalf("method=%q", gotLoginMethod)
	}
	if gotIdentifier != "t1:a@example.invalid" {
		t.Fatalf("identifier=%q", gotIdentifier)
	}
	if gotPassword != "pw" {
		t.Fatalf("password=%q", gotPassword)
	}
	if gotWhoamiToken != "st1" {
		t.Fatalf("whoami token=%q", gotWhoamiToken)
	}
	if ident.ID != "ident1" {
		t.Fatalf("identity id=%q", ident.ID)
	}
	if ident.Traits["email"] != "a@example.invalid" {
		t.Fatalf("traits=%v", ident.Traits)
	}
}

func TestClient_Errors(t *testing.T) {
	t.Run("bad_base_url_request", func(t *testing.T) {
		c := &Client{publicBaseURL: "http:// bad", httpClient: http.DefaultClient}
		if _, err := c.createLoginFlow(context.Background()); err == nil {
			t.Fatal("expected error")
		}
		if _, err := c.submitLoginPassword(context.Background(), "flow1", "id", "pw"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := c.Whoami(context.Background(), "st"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("transport_error", func(t *testing.T) {
		c := &Client{
			publicBaseURL: "http://localhost",
			httpClient:    &http.Client{Transport: errRoundTripper{}},
		}
		if _, err := c.createLoginFlow(context.Background()); err == nil {
			t.Fatal("expected error")
		}
		if _, err := c.submitLoginPassword(context.Background(), "flow1", "id", "pw"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := c.Whoami(context.Background(), "st"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("flow_non_2xx", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("no"))
		}))
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		_, err := c.createLoginFlow(context.Background())
		var he *HTTPError
		if err == nil || !errors.As(err, &he) || he.StatusCode != http.StatusBadRequest {
			t.Fatalf("err=%T %v", err, err)
		}
	})

	t.Run("flow_invalid_json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("{"))
		}))
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		if _, err := c.createLoginFlow(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("flow_missing_id", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": ""})
		}))
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		if _, err := c.createLoginFlow(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_non_2xx_empty_body", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		_, err := c.submitLoginPassword(context.Background(), "flow1", "id", "pw")
		var he *HTTPError
		if err == nil || !errors.As(err, &he) || he.StatusCode != http.StatusUnauthorized || !strings.Contains(err.Error(), "401") {
			t.Fatalf("err=%T %v", err, err)
		}
	})

	t.Run("submit_invalid_json", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("{"))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		if _, err := c.submitLoginPassword(context.Background(), "flow1", "id", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_missing_token", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"session_token": ""})
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		if _, err := c.submitLoginPassword(context.Background(), "flow1", "id", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("whoami_non_2xx", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("nope"))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		_, err := c.Whoami(context.Background(), "st")
		var he *HTTPError
		if err == nil || !errors.As(err, &he) || he.StatusCode != http.StatusForbidden {
			t.Fatalf("err=%T %v", err, err)
		}
	})

	t.Run("whoami_non_2xx_empty_body", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		_, err := c.Whoami(context.Background(), "st")
		var he *HTTPError
		if err == nil || !errors.As(err, &he) || he.StatusCode != http.StatusForbidden || !strings.Contains(err.Error(), "Forbidden") {
			t.Fatalf("err=%T %v", err, err)
		}
	})

	t.Run("whoami_invalid_json", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("{"))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		if _, err := c.Whoami(context.Background(), "st"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestClient_LoginPassword_ErrorPaths(t *testing.T) {
	t.Run("flow_error", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("no"))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		if _, err := c.LoginPassword(context.Background(), "id", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit_error", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "flow1"})
		})
		mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		if _, err := c.LoginPassword(context.Background(), "id", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("whoami_error", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "flow1"})
		})
		mux.HandleFunc("/self-service/login", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"session_token": "st1"})
		})
		mux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		c, _ := New(srv.URL)
		if _, err := c.LoginPassword(context.Background(), "id", "pw"); err == nil {
			t.Fatal("expected error")
		}
	})
}

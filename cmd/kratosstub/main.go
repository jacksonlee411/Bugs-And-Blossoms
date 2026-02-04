package main

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type identity struct {
	ID         string
	TenantUUID string
	Email      string
	RoleSlug   string
	Identifier string
	Password   string
}

type store struct {
	mu sync.Mutex

	byIdentifier map[string]identity
	byID         map[string]identity
	sessions     map[string]string // session_token -> identity_id
}

func newStore() *store {
	return &store{
		byIdentifier: map[string]identity{},
		byID:         map[string]identity{},
		sessions:     map[string]string{},
	}
}

func main() {
	publicAddr := getenvDefault("KRATOS_STUB_PUBLIC_ADDR", "127.0.0.1:4433")
	adminAddr := getenvDefault("KRATOS_STUB_ADMIN_ADDR", "127.0.0.1:4434")

	s := newStore()

	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	publicMux.HandleFunc("/self-service/login/api", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "flow1"})
	})
	publicMux.HandleFunc("/self-service/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("flow") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var req struct {
			Method     string `json:"method"`
			Identifier string `json:"identifier"`
			Password   string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Method != "password" || strings.TrimSpace(req.Identifier) == "" || req.Password == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		ident, ok := s.byIdentifier[req.Identifier]
		if !ok || ident.Password != req.Password {
			s.mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		token := newToken()
		s.sessions[token] = ident.ID
		s.mu.Unlock()

		_ = json.NewEncoder(w).Encode(map[string]any{"session_token": token})
	})
	publicMux.HandleFunc("/sessions/whoami", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		token := strings.TrimSpace(r.Header.Get("X-Session-Token"))
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		s.mu.Lock()
		identityID, ok := s.sessions[token]
		if !ok {
			s.mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		ident, ok := s.byID[identityID]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{
				"id": ident.ID,
				"traits": map[string]any{
					"tenant_uuid": ident.TenantUUID,
					"email":       ident.Email,
					"role_slug":   ident.RoleSlug,
				},
			},
		})
	})

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	adminMux.HandleFunc("/admin/identities", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			SchemaID string `json:"schema_id"`
			Traits   struct {
				TenantUUID string `json:"tenant_uuid"`
				Email      string `json:"email"`
				RoleSlug   string `json:"role_slug"`
			} `json:"traits"`
			Credentials struct {
				Password struct {
					Identifiers []string `json:"identifiers"`
					Config      struct {
						Password string `json:"password"`
					} `json:"config"`
				} `json:"password"`
			} `json:"credentials"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		tenantUUID := strings.TrimSpace(req.Traits.TenantUUID)
		email := strings.ToLower(strings.TrimSpace(req.Traits.Email))
		roleSlug := strings.ToLower(strings.TrimSpace(req.Traits.RoleSlug))
		password := req.Credentials.Password.Config.Password
		identifier := ""
		if len(req.Credentials.Password.Identifiers) > 0 {
			identifier = strings.TrimSpace(req.Credentials.Password.Identifiers[0])
		}
		isSuperadmin := strings.HasPrefix(identifier, "sa:")
		if req.SchemaID == "" || email == "" || password == "" || identifier == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if tenantUUID == "" && !isSuperadmin {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if strings.ContainsAny(email, " \t\r\n") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		if _, exists := s.byIdentifier[identifier]; exists {
			w.WriteHeader(http.StatusConflict)
			return
		}

		id := identityUUID(identifier)
		ident := identity{
			ID:         id,
			TenantUUID: tenantUUID,
			Email:      email,
			RoleSlug:   roleSlug,
			Identifier: identifier,
			Password:   password,
		}
		s.byIdentifier[identifier] = ident
		s.byID[id] = ident

		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": id,
			"traits": map[string]any{
				"tenant_uuid": tenantUUID,
				"email":       email,
				"role_slug":   roleSlug,
			},
		})
	})

	publicSrv := &http.Server{
		Addr:              publicAddr,
		Handler:           publicMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	adminSrv := &http.Server{
		Addr:              adminAddr,
		Handler:           adminMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() { errCh <- listenAndServe(publicSrv) }()
	go func() { errCh <- listenAndServe(adminSrv) }()

	if err := <-errCh; err != nil {
		log.Printf("kratosstub: server error: %v", err)
	}
	_ = publicSrv.Shutdown(context.Background())
	_ = adminSrv.Shutdown(context.Background())
}

func listenAndServe(srv *http.Server) error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return formatUUID(b)
}

func identityUUID(identifier string) string {
	sum := sha1.Sum([]byte("kratosstub:" + identifier))
	var b [16]byte
	copy(b[:], sum[:16])

	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	return formatUUID(b)
}

func formatUUID(b [16]byte) string {
	hex := "0123456789abcdef"
	out := make([]byte, 36)
	j := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[j] = '-'
			j++
		}
		out[j] = hex[v>>4]
		out[j+1] = hex[v&0x0f]
		j += 2
	}
	return string(out)
}

func getenvDefault(k string, def string) string {
	if v := os.Getenv(k); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

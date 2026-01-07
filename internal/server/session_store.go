package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sidCookieName = "sid"

var sidRandReader io.Reader = rand.Reader

type Session struct {
	TenantID    string
	PrincipalID string
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}

type sessionStore interface {
	Create(ctx context.Context, tenantID string, principalID string, expiresAt time.Time, ip string, userAgent string) (sid string, err error)
	Lookup(ctx context.Context, sid string) (Session, bool, error)
	Revoke(ctx context.Context, sid string) error
}

type principalStore interface {
	GetOrCreateTenantAdmin(ctx context.Context, tenantID string) (Principal, error)
	UpsertFromKratos(ctx context.Context, tenantID string, email string, roleSlug string, kratosIdentityID string) (Principal, error)
	GetByID(ctx context.Context, tenantID string, principalID string) (Principal, bool, error)
}

func sidTTLFromEnv() time.Duration {
	const defaultHours = 24 * 14

	v := os.Getenv("SID_TTL_HOURS")
	if v == "" {
		return time.Hour * defaultHours
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return time.Hour * defaultHours
	}
	return time.Hour * time.Duration(n)
}

func newSID() (sid string, tokenSha256 []byte, err error) {
	var b [32]byte
	if _, err := sidRandReader.Read(b[:]); err != nil {
		return "", nil, err
	}
	sid = base64.RawURLEncoding.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(sid))
	return sid, sum[:], nil
}

func readSID(r *http.Request) (string, bool) {
	c, err := r.Cookie(sidCookieName)
	if err != nil {
		return "", false
	}
	if c.Value == "" {
		return "", false
	}
	return c.Value, true
}

func setSIDCookie(w http.ResponseWriter, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sidCookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSIDCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sidCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

type memoryPrincipalStore struct {
	mu    sync.Mutex
	byKey map[string]Principal
	byID  map[string]Principal
}

func newMemoryPrincipalStore() *memoryPrincipalStore {
	return &memoryPrincipalStore{
		byKey: map[string]Principal{},
		byID:  map[string]Principal{},
	}
}

func (s *memoryPrincipalStore) GetOrCreateTenantAdmin(_ context.Context, tenantID string) (Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tenantID + "|tenant-admin@example.invalid"
	if p, ok := s.byKey[key]; ok {
		return p, nil
	}
	p, err := s.newPrincipalLocked(tenantID, "tenant-admin@example.invalid", "tenant-admin")
	if err != nil {
		return Principal{}, err
	}
	s.byKey[key] = p
	return p, nil
}

func (s *memoryPrincipalStore) UpsertFromKratos(_ context.Context, tenantID string, email string, roleSlug string, kratosIdentityID string) (Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tenantID + "|" + email
	if p, ok := s.byKey[key]; ok {
		if p.Status != "active" {
			return Principal{}, errors.New("server: principal is not active")
		}
		if p.KratosIdentityID != "" && p.KratosIdentityID != kratosIdentityID {
			return Principal{}, errors.New("server: principal kratos identity mismatch")
		}
		if p.KratosIdentityID == "" {
			p.KratosIdentityID = kratosIdentityID
			s.byKey[key] = p
			s.byID[p.ID] = p
		}
		return p, nil
	}

	p, err := s.newPrincipalLocked(tenantID, email, roleSlug)
	if err != nil {
		return Principal{}, err
	}
	p.KratosIdentityID = kratosIdentityID
	s.byKey[key] = p
	s.byID[p.ID] = p
	return p, nil
}

func (s *memoryPrincipalStore) GetByID(_ context.Context, tenantID string, principalID string) (Principal, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.byID[principalID]
	if !ok {
		return Principal{}, false, nil
	}
	if p.TenantID != tenantID {
		return Principal{}, false, nil
	}
	return p, true, nil
}

func (s *memoryPrincipalStore) newPrincipalLocked(tenantID string, email string, roleSlug string) (Principal, error) {
	var idb [16]byte
	if _, err := sidRandReader.Read(idb[:]); err != nil {
		return Principal{}, err
	}
	id := base64.RawURLEncoding.EncodeToString(idb[:])
	p := Principal{
		ID:       id,
		TenantID: tenantID,
		RoleSlug: roleSlug,
		Status:   "active",
		Email:    email,
	}
	s.byID[id] = p
	return p, nil
}

type memorySessionStore struct {
	mu    sync.Mutex
	bySID map[string]Session
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{
		bySID: map[string]Session{},
	}
}

func (s *memorySessionStore) Create(_ context.Context, tenantID string, principalID string, expiresAt time.Time, _ string, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sid, _, err := newSID()
	if err != nil {
		return "", err
	}
	s.bySID[sid] = Session{
		TenantID:    tenantID,
		PrincipalID: principalID,
		ExpiresAt:   expiresAt,
	}
	return sid, nil
}

func (s *memorySessionStore) Lookup(_ context.Context, sid string) (Session, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.bySID[sid]
	if !ok {
		return Session{}, false, nil
	}
	if v.RevokedAt != nil {
		return Session{}, false, nil
	}
	if time.Now().After(v.ExpiresAt) {
		return Session{}, false, nil
	}
	return v, true, nil
}

func (s *memorySessionStore) Revoke(_ context.Context, sid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.bySID, sid)
	return nil
}

type pgPrincipalStore struct {
	q queryExecer
}

type queryExecer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func newPrincipalStore(pool *pgxpool.Pool) principalStore {
	if pool == nil {
		return newMemoryPrincipalStore()
	}
	return &pgPrincipalStore{q: pool}
}

func (s *pgPrincipalStore) GetOrCreateTenantAdmin(ctx context.Context, tenantID string) (Principal, error) {
	const email = "tenant-admin@example.invalid"
	const roleSlug = "tenant-admin"

	var p Principal
	p.Email = email
	p.TenantID = tenantID
	p.RoleSlug = roleSlug

	err := s.q.QueryRow(ctx, `
INSERT INTO iam.principals (tenant_id, email, role_slug, status)
VALUES ($1, $2, $3, 'active')
ON CONFLICT (tenant_id, email) DO UPDATE SET
  role_slug = EXCLUDED.role_slug,
  status = EXCLUDED.status,
  updated_at = now()
RETURNING id::text, status;
`, tenantID, email, roleSlug).Scan(&p.ID, &p.Status)
	if err != nil {
		return Principal{}, err
	}
	if p.Status != "active" {
		return Principal{}, errors.New("server: principal is not active")
	}
	return p, nil
}

func (s *pgPrincipalStore) UpsertFromKratos(ctx context.Context, tenantID string, email string, roleSlug string, kratosIdentityID string) (Principal, error) {
	var p Principal
	p.Email = email
	p.TenantID = tenantID
	p.RoleSlug = roleSlug
	p.Status = "active"
	p.KratosIdentityID = kratosIdentityID

	err := s.q.QueryRow(ctx, `
	INSERT INTO iam.principals (tenant_id, email, role_slug, status, kratos_identity_id)
	VALUES ($1, $2, $3, 'active', $4::uuid)
	ON CONFLICT (tenant_id, email) DO UPDATE SET
	  kratos_identity_id = COALESCE(iam.principals.kratos_identity_id, EXCLUDED.kratos_identity_id),
	  updated_at = now()
	RETURNING id::text, role_slug, status, COALESCE(kratos_identity_id::text, '');
	`, tenantID, email, roleSlug, kratosIdentityID).Scan(&p.ID, &p.RoleSlug, &p.Status, &p.KratosIdentityID)
	if err != nil {
		return Principal{}, err
	}
	if p.Status != "active" {
		return Principal{}, errors.New("server: principal is not active")
	}
	if p.KratosIdentityID == "" {
		return Principal{}, errors.New("server: principal missing kratos identity")
	}
	if p.KratosIdentityID != kratosIdentityID {
		return Principal{}, errors.New("server: principal kratos identity mismatch")
	}
	return p, nil
}

func (s *pgPrincipalStore) GetByID(ctx context.Context, tenantID string, principalID string) (Principal, bool, error) {
	var p Principal
	err := s.q.QueryRow(ctx, `
SELECT id::text, tenant_id::text, email, role_slug, status
FROM iam.principals
WHERE tenant_id = $1 AND id = $2;
	`, tenantID, principalID).Scan(&p.ID, &p.TenantID, &p.Email, &p.RoleSlug, &p.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Principal{}, false, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Principal{}, false, err
		}
		return Principal{}, false, err
	}
	return p, true, nil
}

type pgSessionStore struct {
	q queryExecer
}

func newSessionStore(pool *pgxpool.Pool) sessionStore {
	if pool == nil {
		return newMemorySessionStore()
	}
	return &pgSessionStore{q: pool}
}

func (s *pgSessionStore) Create(ctx context.Context, tenantID string, principalID string, expiresAt time.Time, ip string, userAgent string) (string, error) {
	sid, tokenSha256, err := newSID()
	if err != nil {
		return "", err
	}
	_, err = s.q.Exec(ctx, `
INSERT INTO iam.sessions (token_sha256, tenant_id, principal_id, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6);
`, tokenSha256, tenantID, principalID, expiresAt, ip, userAgent)
	if err != nil {
		return "", err
	}
	return sid, nil
}

func (s *pgSessionStore) Lookup(ctx context.Context, sid string) (Session, bool, error) {
	sum := sha256.Sum256([]byte(sid))
	var out Session
	out.RevokedAt = nil
	var revokedAt *time.Time
	err := s.q.QueryRow(ctx, `
SELECT tenant_id::text, principal_id::text, expires_at, revoked_at
FROM iam.sessions
WHERE token_sha256 = $1;
	`, sum[:]).Scan(&out.TenantID, &out.PrincipalID, &out.ExpiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, false, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Session{}, false, err
		}
		return Session{}, false, err
	}
	out.RevokedAt = revokedAt
	if out.RevokedAt != nil {
		return Session{}, false, nil
	}
	if time.Now().After(out.ExpiresAt) {
		return Session{}, false, nil
	}
	return out, true, nil
}

func (s *pgSessionStore) Revoke(ctx context.Context, sid string) error {
	if sid == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(sid))
	_, err := s.q.Exec(ctx, `DELETE FROM iam.sessions WHERE token_sha256 = $1;`, sum[:])
	return err
}

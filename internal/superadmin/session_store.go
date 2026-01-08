package superadmin

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
)

const saSidCookieName = "sa_sid"

var saSidRandReader io.Reader = rand.Reader

type superadminSession struct {
	PrincipalID string
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}

type sessionStore interface {
	Create(ctx context.Context, principalID string, expiresAt time.Time, ip string, userAgent string) (saSid string, err error)
	Lookup(ctx context.Context, saSid string) (superadminSession, bool, error)
	Revoke(ctx context.Context, saSid string) error
}

type queryExecer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func saSidTTLFromEnv() time.Duration {
	const defaultHours = 8

	v := os.Getenv("SA_SID_TTL_HOURS")
	if v == "" {
		return time.Hour * defaultHours
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return time.Hour * defaultHours
	}
	return time.Hour * time.Duration(n)
}

func newSASID() (saSid string, tokenSha256 []byte, err error) {
	var b [32]byte
	if _, err := saSidRandReader.Read(b[:]); err != nil {
		return "", nil, err
	}
	saSid = base64.RawURLEncoding.EncodeToString(b[:])
	sum := sha256.Sum256([]byte(saSid))
	return saSid, sum[:], nil
}

func readSASID(r *http.Request) (string, bool) {
	c, err := r.Cookie(saSidCookieName)
	if err != nil {
		return "", false
	}
	if c.Value == "" {
		return "", false
	}
	return c.Value, true
}

func setSASIDCookie(w http.ResponseWriter, saSid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     saSidCookieName,
		Value:    saSid,
		Path:     "/superadmin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSASIDCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     saSidCookieName,
		Value:    "",
		Path:     "/superadmin",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

type memorySessionStore struct {
	mu    sync.Mutex
	bySID map[string]superadminSession
}

func newMemorySessionStore() *memorySessionStore {
	return &memorySessionStore{
		bySID: map[string]superadminSession{},
	}
}

func (s *memorySessionStore) Create(_ context.Context, principalID string, expiresAt time.Time, _ string, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	saSid, _, err := newSASID()
	if err != nil {
		return "", err
	}
	s.bySID[saSid] = superadminSession{
		PrincipalID: principalID,
		ExpiresAt:   expiresAt,
	}
	return saSid, nil
}

func (s *memorySessionStore) Lookup(_ context.Context, saSid string) (superadminSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.bySID[saSid]
	if !ok {
		return superadminSession{}, false, nil
	}
	if v.RevokedAt != nil {
		return superadminSession{}, false, nil
	}
	if time.Now().After(v.ExpiresAt) {
		return superadminSession{}, false, nil
	}
	return v, true, nil
}

func (s *memorySessionStore) Revoke(_ context.Context, saSid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.bySID, saSid)
	return nil
}

type pgSessionStore struct {
	q queryExecer
}

func newSessionStoreFromDB(db queryExecer) sessionStore {
	if db == nil {
		return newMemorySessionStore()
	}
	return &pgSessionStore{q: db}
}

func (s *pgSessionStore) Create(ctx context.Context, principalID string, expiresAt time.Time, ip string, userAgent string) (string, error) {
	saSid, tokenSha256, err := newSASID()
	if err != nil {
		return "", err
	}
	_, err = s.q.Exec(ctx, `
INSERT INTO iam.superadmin_sessions (token_sha256, principal_id, expires_at, ip, user_agent)
VALUES ($1, $2::uuid, $3, $4, $5);
`, tokenSha256, principalID, expiresAt, ip, userAgent)
	if err != nil {
		return "", err
	}
	return saSid, nil
}

func (s *pgSessionStore) Lookup(ctx context.Context, saSid string) (superadminSession, bool, error) {
	sum := sha256.Sum256([]byte(saSid))
	var out superadminSession
	out.RevokedAt = nil
	var revokedAt *time.Time
	err := s.q.QueryRow(ctx, `
SELECT principal_id::text, expires_at, revoked_at
FROM iam.superadmin_sessions
WHERE token_sha256 = $1;
`, sum[:]).Scan(&out.PrincipalID, &out.ExpiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return superadminSession{}, false, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return superadminSession{}, false, err
		}
		return superadminSession{}, false, err
	}
	out.RevokedAt = revokedAt
	if out.RevokedAt != nil {
		return superadminSession{}, false, nil
	}
	if time.Now().After(out.ExpiresAt) {
		return superadminSession{}, false, nil
	}
	return out, true, nil
}

func (s *pgSessionStore) Revoke(ctx context.Context, saSid string) error {
	if saSid == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(saSid))
	_, err := s.q.Exec(ctx, `DELETE FROM iam.superadmin_sessions WHERE token_sha256 = $1;`, sum[:])
	return err
}

type superadminPrincipal struct {
	ID               string
	Email            string
	Status           string
	KratosIdentityID string
}

type principalStore interface {
	UpsertFromKratos(ctx context.Context, email string, kratosIdentityID string) (superadminPrincipal, error)
	GetByID(ctx context.Context, principalID string) (superadminPrincipal, bool, error)
}

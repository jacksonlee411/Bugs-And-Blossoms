package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, os.ErrInvalid }

type scanRow struct {
	scan func(dest ...any) error
}

func (r scanRow) Scan(dest ...any) error { return r.scan(dest...) }

type stubQ struct {
	row     pgx.Row
	rowErr  error
	execErr error
}

func (s *stubQ) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if s.rowErr != nil {
		return scanRow{scan: func(_ ...any) error { return s.rowErr }}
	}
	return s.row
}

func (s *stubQ) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.CommandTag{}, nil
}

func TestSIDTTLFromEnv_DefaultAndOverride(t *testing.T) {
	t.Setenv("SID_TTL_HOURS", "")
	if got := sidTTLFromEnv(); got != 14*24*time.Hour {
		t.Fatalf("default ttl=%v", got)
	}

	t.Setenv("SID_TTL_HOURS", "1")
	if got := sidTTLFromEnv(); got != time.Hour {
		t.Fatalf("override ttl=%v", got)
	}

	t.Setenv("SID_TTL_HOURS", "bad")
	if got := sidTTLFromEnv(); got != 14*24*time.Hour {
		t.Fatalf("bad ttl=%v", got)
	}

	t.Setenv("SID_TTL_HOURS", "0")
	if got := sidTTLFromEnv(); got != 14*24*time.Hour {
		t.Fatalf("zero ttl=%v", got)
	}
}

func TestNewSID_SuccessAndError(t *testing.T) {
	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })

	sidRandReader = bytes.NewReader(bytes.Repeat([]byte{0xAB}, 64))
	sid, sum, err := newSID()
	if err != nil {
		t.Fatal(err)
	}
	if sid == "" {
		t.Fatal("empty sid")
	}
	if len(sum) != 32 {
		t.Fatalf("sha len=%d", len(sum))
	}

	sidRandReader = errReader{}
	if _, _, err := newSID(); err == nil {
		t.Fatal("expected error")
	}
}

func TestReadSID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := readSID(req); ok {
		t.Fatal("expected ok=false")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: sidCookieName, Value: ""})
	if _, ok := readSID(req2); ok {
		t.Fatal("expected ok=false")
	}

	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.AddCookie(&http.Cookie{Name: sidCookieName, Value: "x"})
	if got, ok := readSID(req3); !ok || got != "x" {
		t.Fatalf("sid=%q ok=%v", got, ok)
	}
}

func TestSIDCookieHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	setSIDCookie(rec, "abc")
	resp := rec.Result()
	defer resp.Body.Close()
	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == sidCookieName && c.Value == "abc" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("sid cookie not set")
	}

	rec2 := httptest.NewRecorder()
	clearSIDCookie(rec2)
	resp2 := rec2.Result()
	defer resp2.Body.Close()
	found = false
	for _, c := range resp2.Cookies() {
		if c.Name == sidCookieName && c.MaxAge < 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("sid cookie not cleared")
	}
}

func TestMemoryPrincipalStore_ErrOnRand(t *testing.T) {
	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })
	sidRandReader = errReader{}

	s := newMemoryPrincipalStore()
	if _, err := s.GetOrCreateTenantAdmin(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemoryPrincipalStore_GetOrCreateTenantAdmin_CacheHit(t *testing.T) {
	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })
	sidRandReader = bytes.NewReader(bytes.Repeat([]byte{0xAB}, 16))

	s := newMemoryPrincipalStore()
	p1, err := s.GetOrCreateTenantAdmin(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.GetOrCreateTenantAdmin(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if p1.ID == "" || p2.ID == "" || p1.ID != p2.ID {
		t.Fatalf("cache mismatch p1=%+v p2=%+v", p1, p2)
	}
}

func TestMemoryPrincipalStore_GetByID_Branches(t *testing.T) {
	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })
	sidRandReader = bytes.NewReader(bytes.Repeat([]byte{0xCD}, 16))

	s := newMemoryPrincipalStore()
	p, err := s.GetOrCreateTenantAdmin(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok, err := s.GetByID(context.Background(), "t1", "nope"); err != nil || ok {
		t.Fatalf("not-found ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.GetByID(context.Background(), "t2", p.ID); err != nil || ok {
		t.Fatalf("tenant-mismatch ok=%v err=%v", ok, err)
	}
	if got, ok, err := s.GetByID(context.Background(), "t1", p.ID); err != nil || !ok || got.ID != p.ID {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
}

func TestMemoryPrincipalStore_UpsertFromKratos_Branches(t *testing.T) {
	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })
	sidRandReader = bytes.NewReader(bytes.Repeat([]byte{0xAB}, 16))

	s := newMemoryPrincipalStore()
	p, err := s.GetOrCreateTenantAdmin(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if p.KratosIdentityID != "" {
		t.Fatalf("unexpected kratos id: %q", p.KratosIdentityID)
	}

	p2, err := s.UpsertFromKratos(context.Background(), "t1", "tenant-admin@example.invalid", "tenant-admin", "kid1")
	if err != nil {
		t.Fatal(err)
	}
	if p2.ID != p.ID || p2.KratosIdentityID != "kid1" {
		t.Fatalf("p2=%+v p=%+v", p2, p)
	}

	p3, err := s.UpsertFromKratos(context.Background(), "t1", "tenant-admin@example.invalid", "tenant-admin", "kid1")
	if err != nil {
		t.Fatal(err)
	}
	if p3.ID != p.ID {
		t.Fatalf("p3=%+v", p3)
	}

	if _, err := s.UpsertFromKratos(context.Background(), "t1", "tenant-admin@example.invalid", "tenant-admin", "kid2"); err == nil {
		t.Fatal("expected error")
	}

	pDisabled := p3
	pDisabled.Status = "disabled"
	s.byKey["t1|tenant-admin@example.invalid"] = pDisabled
	s.byID[pDisabled.ID] = pDisabled
	if _, err := s.UpsertFromKratos(context.Background(), "t1", "tenant-admin@example.invalid", "tenant-admin", "kid1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMemorySessionStore_RevokedOrExpired(t *testing.T) {
	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })

	s := newMemorySessionStore()
	ctx := context.Background()

	sidRandReader = errReader{}
	if _, err := s.Create(ctx, "t1", "p1", time.Now().Add(time.Hour), "", ""); err == nil {
		t.Fatal("expected error")
	}

	sidRandReader = bytes.NewReader(bytes.Repeat([]byte{0xAA}, 64))
	sid, err := s.Create(ctx, "t1", "p1", time.Now().Add(time.Hour), "", "")
	if err != nil {
		t.Fatal(err)
	}

	revokedAt := time.Now()
	s.bySID[sid] = Session{
		TenantID:    "t1",
		PrincipalID: "p1",
		ExpiresAt:   time.Now().Add(time.Hour),
		RevokedAt:   &revokedAt,
	}
	if _, ok, err := s.Lookup(ctx, sid); err != nil || ok {
		t.Fatalf("revoked ok=%v err=%v", ok, err)
	}

	s.bySID[sid] = Session{
		TenantID:    "t1",
		PrincipalID: "p1",
		ExpiresAt:   time.Now().Add(-time.Hour),
	}
	if _, ok, err := s.Lookup(ctx, sid); err != nil || ok {
		t.Fatalf("expired ok=%v err=%v", ok, err)
	}
}

func TestPGPrincipalStore_GetOrCreateAndGetByID(t *testing.T) {
	q := &stubQ{}
	s := &pgPrincipalStore{q: q}

	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "p1"
		*(dest[1].(*string)) = "active"
		return nil
	}}
	p, err := s.GetOrCreateTenantAdmin(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "p1" || p.Status != "active" || p.RoleSlug == "" {
		t.Fatalf("principal=%+v", p)
	}

	q.rowErr = os.ErrInvalid
	if _, err := s.GetOrCreateTenantAdmin(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}
	q.rowErr = nil

	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "p1"
		*(dest[1].(*string)) = "disabled"
		return nil
	}}
	if _, err := s.GetOrCreateTenantAdmin(context.Background(), "t1"); err == nil {
		t.Fatal("expected error")
	}

	q.rowErr = pgx.ErrNoRows
	if _, ok, err := s.GetByID(context.Background(), "t1", "p1"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	q.rowErr = context.Canceled
	if _, _, err := s.GetByID(context.Background(), "t1", "p1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}

	q.rowErr = os.ErrInvalid
	if _, _, err := s.GetByID(context.Background(), "t1", "p1"); err == nil {
		t.Fatal("expected error")
	}

	q.rowErr = nil
	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "p1"
		*(dest[1].(*string)) = "t1"
		*(dest[2].(*string)) = "a@example.invalid"
		*(dest[3].(*string)) = "tenant-admin"
		*(dest[4].(*string)) = "active"
		return nil
	}}
	got, ok, err := s.GetByID(context.Background(), "t1", "p1")
	if err != nil || !ok || got.ID != "p1" {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
}

func TestPGPrincipalStore_UpsertFromKratos(t *testing.T) {
	q := &stubQ{}
	s := &pgPrincipalStore{q: q}

	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "p1"
		*(dest[1].(*string)) = "tenant-admin"
		*(dest[2].(*string)) = "active"
		*(dest[3].(*string)) = "kid1"
		return nil
	}}
	p, err := s.UpsertFromKratos(context.Background(), "t1", "a@example.invalid", "tenant-admin", "kid1")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "p1" || p.KratosIdentityID != "kid1" {
		t.Fatalf("p=%+v", p)
	}

	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "p1"
		*(dest[1].(*string)) = "tenant-admin"
		*(dest[2].(*string)) = "disabled"
		*(dest[3].(*string)) = "kid1"
		return nil
	}}
	if _, err := s.UpsertFromKratos(context.Background(), "t1", "a@example.invalid", "tenant-admin", "kid1"); err == nil {
		t.Fatal("expected error")
	}

	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "p1"
		*(dest[1].(*string)) = "tenant-admin"
		*(dest[2].(*string)) = "active"
		*(dest[3].(*string)) = ""
		return nil
	}}
	if _, err := s.UpsertFromKratos(context.Background(), "t1", "a@example.invalid", "tenant-admin", "kid1"); err == nil {
		t.Fatal("expected error")
	}

	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "p1"
		*(dest[1].(*string)) = "tenant-admin"
		*(dest[2].(*string)) = "active"
		*(dest[3].(*string)) = "kid2"
		return nil
	}}
	if _, err := s.UpsertFromKratos(context.Background(), "t1", "a@example.invalid", "tenant-admin", "kid1"); err == nil {
		t.Fatal("expected error")
	}

	q.rowErr = os.ErrInvalid
	if _, err := s.UpsertFromKratos(context.Background(), "t1", "a@example.invalid", "tenant-admin", "kid1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPGSessionStore_CreateLookupRevoke(t *testing.T) {
	old := sidRandReader
	t.Cleanup(func() { sidRandReader = old })
	sidRandReader = bytes.NewReader(bytes.Repeat([]byte{0xCD}, 64))

	q := &stubQ{}
	s := &pgSessionStore{q: q}

	q.execErr = os.ErrInvalid
	if _, err := s.Create(context.Background(), "t1", "p1", time.Now().Add(time.Hour), "", ""); err == nil {
		t.Fatal("expected error")
	}

	q.execErr = nil
	sid, err := s.Create(context.Background(), "t1", "p1", time.Now().Add(time.Hour), "", "")
	if err != nil || sid == "" {
		t.Fatalf("sid=%q err=%v", sid, err)
	}

	sidRandReader = errReader{}
	if _, err := s.Create(context.Background(), "t1", "p1", time.Now().Add(time.Hour), "", ""); err == nil {
		t.Fatal("expected error")
	}
	sidRandReader = bytes.NewReader(bytes.Repeat([]byte{0xCD}, 64))

	q.rowErr = pgx.ErrNoRows
	if _, ok, err := s.Lookup(context.Background(), sid); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	q.rowErr = context.DeadlineExceeded
	if _, _, err := s.Lookup(context.Background(), sid); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}

	q.rowErr = os.ErrInvalid
	if _, _, err := s.Lookup(context.Background(), sid); err == nil {
		t.Fatal("expected error")
	}

	q.rowErr = nil
	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "t1"
		*(dest[1].(*string)) = "p1"
		*(dest[2].(*time.Time)) = time.Now().Add(time.Hour)
		*(dest[3].(**time.Time)) = nil
		return nil
	}}
	if _, ok, err := s.Lookup(context.Background(), sid); err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}

	revokedAt := time.Now()
	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "t1"
		*(dest[1].(*string)) = "p1"
		*(dest[2].(*time.Time)) = time.Now().Add(time.Hour)
		*(dest[3].(**time.Time)) = &revokedAt
		return nil
	}}
	if _, ok, err := s.Lookup(context.Background(), sid); err != nil || ok {
		t.Fatalf("revoked ok=%v err=%v", ok, err)
	}

	q.row = scanRow{scan: func(dest ...any) error {
		*(dest[0].(*string)) = "t1"
		*(dest[1].(*string)) = "p1"
		*(dest[2].(*time.Time)) = time.Now().Add(-time.Hour)
		*(dest[3].(**time.Time)) = nil
		return nil
	}}
	if _, ok, err := s.Lookup(context.Background(), sid); err != nil || ok {
		t.Fatalf("expired ok=%v err=%v", ok, err)
	}

	q.execErr = os.ErrInvalid
	if err := s.Revoke(context.Background(), sid); err == nil {
		t.Fatal("expected error")
	}
	q.execErr = nil
	if err := s.Revoke(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Revoke(context.Background(), sid); err != nil {
		t.Fatal(err)
	}
}

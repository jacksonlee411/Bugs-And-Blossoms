package uuidv7

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestNew(t *testing.T) {
	u, err := New()
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if u.Version() != 7 {
		t.Fatalf("expected version 7, got %d", u.Version())
	}
	if u.Variant() != uuid.RFC4122 {
		t.Fatalf("expected RFC4122 variant, got %v", u.Variant())
	}
}

func TestNewString(t *testing.T) {
	got, err := NewString()
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty string")
	}
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("expected parseable uuid, got %v", err)
	}
}

func TestNewReadError(t *testing.T) {
	orig := rand.Reader
	rand.Reader = errReader{}
	defer func() { rand.Reader = orig }()

	if _, err := New(); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewStringReadError(t *testing.T) {
	orig := rand.Reader
	rand.Reader = errReader{}
	defer func() { rand.Reader = orig }()

	if _, err := NewString(); err == nil {
		t.Fatal("expected error")
	}
}

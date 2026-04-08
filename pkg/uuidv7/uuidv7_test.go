package uuidv7_test

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/google/uuid"
	uuidv7 "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestNew_BlackBox(t *testing.T) {
	u, err := uuidv7.New()
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

func TestNewString_BlackBox(t *testing.T) {
	got, err := uuidv7.NewString()
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

func TestNew_ReadError_BlackBox(t *testing.T) {
	orig := rand.Reader
	rand.Reader = errReader{}
	defer func() { rand.Reader = orig }()

	if _, err := uuidv7.New(); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewString_ReadError_BlackBox(t *testing.T) {
	orig := rand.Reader
	rand.Reader = errReader{}
	defer func() { rand.Reader = orig }()

	if _, err := uuidv7.NewString(); err == nil {
		t.Fatal("expected error")
	}
}

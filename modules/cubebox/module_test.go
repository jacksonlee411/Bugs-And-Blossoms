package cubebox

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeBeginner struct{}

func (fakeBeginner) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

func TestNewPGStore(t *testing.T) {
	store := NewPGStore(fakeBeginner{})
	if store == nil {
		t.Fatal("expected pg store")
	}
}

func TestNewLocalFileService(t *testing.T) {
	svc := NewLocalFileService(t.TempDir())
	if svc == nil {
		t.Fatal("expected file service")
	}
}

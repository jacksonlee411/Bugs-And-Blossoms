package cubebox

import (
	"context"
	"os"
	"path/filepath"
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

func TestNewPGFileService(t *testing.T) {
	svc := NewPGFileService(fakeBeginner{}, t.TempDir())
	if svc == nil {
		t.Fatal("expected pg file service")
	}
}

func TestDefaultLocalFileRoot(t *testing.T) {
	t.Run("prefers environment override", func(t *testing.T) {
		t.Setenv("CUBEBOX_FILE_ROOT", " /tmp/cubebox-root ")
		if got := DefaultLocalFileRoot(); got != "/tmp/cubebox-root" {
			t.Fatalf("unexpected root: %q", got)
		}
	})

	t.Run("falls back to working directory", func(t *testing.T) {
		t.Setenv("CUBEBOX_FILE_ROOT", "")
		wd := t.TempDir()
		oldWD, err := filepath.Abs(".")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(oldWD)
		})
		want := filepath.Join(wd, ".local", "cubebox", "files")
		if got := DefaultLocalFileRoot(); got != want {
			t.Fatalf("unexpected root: got %q want %q", got, want)
		}
	})
}

func TestNewDefaultLocalFileService(t *testing.T) {
	t.Setenv("CUBEBOX_FILE_ROOT", t.TempDir())
	svc := NewDefaultLocalFileService()
	if svc == nil {
		t.Fatal("expected default file service")
	}
}

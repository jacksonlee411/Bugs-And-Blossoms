package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFileStoreObjectLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewLocalFileStore(t.TempDir())

	object, err := store.SaveObject(ctx, "tenant-a", "file_test", " notes.txt ", "", strings.NewReader("hello cubebox"))
	if err != nil {
		t.Fatalf("save object: %v", err)
	}
	if object.StorageProvider != localFileStorageProvider {
		t.Fatalf("unexpected provider: %q", object.StorageProvider)
	}
	if object.Filename != "notes.txt" {
		t.Fatalf("unexpected filename: %q", object.Filename)
	}
	if object.ContentType != "application/octet-stream" {
		t.Fatalf("unexpected content type: %q", object.ContentType)
	}
	if object.StorageKey != "tenant-a/file_test/notes.txt" {
		t.Fatalf("unexpected storage key: %q", object.StorageKey)
	}

	fullPath := filepath.Join(store.filesDir(), filepath.FromSlash(object.StorageKey))
	raw, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	sum := sha256.Sum256([]byte("hello cubebox"))
	if got := hex.EncodeToString(sum[:]); object.SHA256 != got {
		t.Fatalf("sha mismatch: got %q want %q", object.SHA256, got)
	}
	if string(raw) != "hello cubebox" {
		t.Fatalf("unexpected object content: %q", string(raw))
	}

	if err := store.DeleteObject(ctx, object.StorageKey); err != nil {
		t.Fatalf("delete object: %v", err)
	}
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, stat err=%v", err)
	}
}

func TestLocalFileStoreValidationAndHelpers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewLocalFileStore(t.TempDir())

	if _, err := store.SaveObject(ctx, "", "file_test", "a.txt", "", strings.NewReader("x")); err == nil {
		t.Fatal("expected tenant error")
	}
	if _, err := store.SaveObject(ctx, "tenant-a", "", "a.txt", "", strings.NewReader("x")); err == nil {
		t.Fatal("expected file id error")
	}
	if _, err := store.SaveObject(ctx, "tenant-a", "file_test", "", "", strings.NewReader("x")); err == nil {
		t.Fatal("expected filename error")
	}

	tooLarge := bytes.NewReader(make([]byte, maxFileSizeBytes+1))
	if _, err := store.SaveObject(ctx, "tenant-a", "file_test", "huge.bin", "", tooLarge); err == nil || !strings.Contains(err.Error(), "file exceeds") {
		t.Fatalf("expected size error, got %v", err)
	}

	if got := sanitizeFileName(" "); got != "upload.bin" {
		t.Fatalf("expected upload.bin, got %q", got)
	}
	if got := sanitizeFileName("../folder/report.txt"); got != "report.txt" {
		t.Fatalf("expected sanitized base name, got %q", got)
	}
	if got := defaultMediaType(""); got != "application/octet-stream" {
		t.Fatalf("expected default media type, got %q", got)
	}
	if got := defaultMediaType("text/plain"); got != "text/plain" {
		t.Fatalf("expected explicit media type, got %q", got)
	}

	if err := store.Healthy(ctx); err != nil {
		t.Fatalf("healthy: %v", err)
	}
	blankStore := NewLocalFileStore(" ")
	if err := blankStore.Healthy(ctx); err == nil || !strings.Contains(err.Error(), "cubebox file root missing") {
		t.Fatalf("expected missing root error, got %v", err)
	}
}

func TestLocalFileStoreIOFailurePaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("save fails when root path is an existing file", func(t *testing.T) {
		rootFile := filepath.Join(t.TempDir(), "root-file")
		if err := os.WriteFile(rootFile, []byte("not a directory"), 0o644); err != nil {
			t.Fatalf("write root file: %v", err)
		}
		store := NewLocalFileStore(rootFile)
		if _, err := store.SaveObject(ctx, "tenant-a", "file_test", "a.txt", "", strings.NewReader("payload")); err == nil {
			t.Fatal("expected save root mkdir error")
		}
	})

	t.Run("delete ignores missing object", func(t *testing.T) {
		store := NewLocalFileStore(t.TempDir())
		if err := store.DeleteObject(ctx, "tenant-a/file_x/missing.txt"); err != nil {
			t.Fatalf("expected missing delete ignored, got %v", err)
		}
	})
}

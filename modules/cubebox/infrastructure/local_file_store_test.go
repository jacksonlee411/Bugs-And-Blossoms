package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFileStoreLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewLocalFileStore(t.TempDir())

	first, err := store.Save(ctx, "tenant-a", "actor-1", "conversation-a", " notes.txt ", "", strings.NewReader("hello cubebox"))
	if err != nil {
		t.Fatalf("save first: %v", err)
	}
	second, err := store.Save(ctx, "tenant-a", "actor-1", "conversation-b", "report.pdf", "application/pdf", strings.NewReader("second file"))
	if err != nil {
		t.Fatalf("save second: %v", err)
	}
	if !strings.HasPrefix(first.FileID, "file_") {
		t.Fatalf("expected file id prefix, got %q", first.FileID)
	}
	if first.FileName != "notes.txt" {
		t.Fatalf("expected sanitized file name, got %q", first.FileName)
	}
	if first.MediaType != "application/octet-stream" {
		t.Fatalf("expected default media type, got %q", first.MediaType)
	}
	if second.MediaType != "application/pdf" {
		t.Fatalf("expected explicit media type, got %q", second.MediaType)
	}
	if first.UploadedAt == "" || second.UploadedAt == "" {
		t.Fatal("expected uploaded_at")
	}

	rawPath := filepath.Join(store.filesDir(), filepath.FromSlash(first.StorageKey))
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	sum := sha256.Sum256([]byte("hello cubebox"))
	if got := hex.EncodeToString(sum[:]); first.SHA256 != got {
		t.Fatalf("sha mismatch: got %q want %q", first.SHA256, got)
	}
	if string(raw) != "hello cubebox" {
		t.Fatalf("unexpected file content: %q", string(raw))
	}

	list, err := store.List(ctx, "tenant-a", "")
	if err != nil {
		t.Fatalf("list tenant: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 files, got %d", len(list))
	}
	if list[0].UploadedAt < list[1].UploadedAt {
		t.Fatalf("expected descending uploaded_at order: %+v", list)
	}

	conversationList, err := store.List(ctx, "tenant-a", "conversation-a")
	if err != nil {
		t.Fatalf("list conversation: %v", err)
	}
	if len(conversationList) != 1 || conversationList[0].FileID != first.FileID {
		t.Fatalf("unexpected conversation list: %+v", conversationList)
	}

	otherTenant, err := store.List(ctx, "tenant-b", "")
	if err != nil {
		t.Fatalf("list other tenant: %v", err)
	}
	if len(otherTenant) != 0 {
		t.Fatalf("expected empty list, got %+v", otherTenant)
	}

	removed, err := store.Delete(ctx, "tenant-a", first.FileID)
	if err != nil {
		t.Fatalf("delete first: %v", err)
	}
	if !removed {
		t.Fatal("expected first file deleted")
	}
	if _, err := os.Stat(rawPath); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, stat err=%v", err)
	}

	missingPath := filepath.Join(store.filesDir(), filepath.FromSlash(second.StorageKey))
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("remove underlying file before delete: %v", err)
	}
	removed, err = store.Delete(ctx, "tenant-a", second.FileID)
	if err != nil {
		t.Fatalf("delete second after manual remove: %v", err)
	}
	if !removed {
		t.Fatal("expected second file deleted")
	}

	removed, err = store.Delete(ctx, "tenant-a", "missing")
	if err != nil {
		t.Fatalf("delete missing: %v", err)
	}
	if removed {
		t.Fatal("expected missing delete to return false")
	}

	index, err := store.readIndex()
	if err != nil {
		t.Fatalf("read index after deletes: %v", err)
	}
	if len(index.Items) != 0 {
		t.Fatalf("expected empty index, got %+v", index.Items)
	}
}

func TestLocalFileStoreValidationAndHelpers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewLocalFileStore(t.TempDir())

	if _, err := store.Save(ctx, "", "actor-1", "", "a.txt", "", strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "tenant_id required") {
		t.Fatalf("expected tenant_id required, got %v", err)
	}
	if _, err := store.Save(ctx, "tenant-a", "", "", "a.txt", "", strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "uploaded_by required") {
		t.Fatalf("expected uploaded_by required, got %v", err)
	}
	if _, err := store.Save(ctx, "tenant-a", "actor-1", "", "", "", strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "file_name required") {
		t.Fatalf("expected file_name required, got %v", err)
	}

	tooLarge := bytes.NewReader(make([]byte, maxFileSizeBytes+1))
	if _, err := store.Save(ctx, "tenant-a", "actor-1", "", "huge.bin", "", tooLarge); err == nil || !strings.Contains(err.Error(), "file exceeds") {
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

func TestLocalFileStoreReadIndexHandlesEmptyAndInvalidJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	store := NewLocalFileStore(root)

	if err := os.WriteFile(store.indexPath(), nil, 0o644); err != nil {
		t.Fatalf("write empty index: %v", err)
	}
	items, err := store.List(ctx, "tenant-a", "")
	if err != nil {
		t.Fatalf("list empty index: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %+v", items)
	}

	if err := os.WriteFile(store.indexPath(), []byte("{"), 0o644); err != nil {
		t.Fatalf("write invalid index: %v", err)
	}
	if _, err := store.List(ctx, "tenant-a", ""); err == nil {
		t.Fatal("expected invalid json error")
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
		if _, err := store.Save(ctx, "tenant-a", "actor-1", "", "a.txt", "", strings.NewReader("payload")); err == nil {
			t.Fatal("expected save root mkdir error")
		}
	})

	t.Run("save fails when files dir parent is blocked by file", func(t *testing.T) {
		root := t.TempDir()
		store := NewLocalFileStore(root)
		if err := os.WriteFile(store.indexPath(), []byte("{}"), 0o644); err != nil {
			t.Fatalf("seed index: %v", err)
		}
		if err := os.RemoveAll(store.filesDir()); err != nil {
			t.Fatalf("remove files dir: %v", err)
		}
		if err := os.WriteFile(store.filesDir(), []byte("blocked"), 0o644); err != nil {
			t.Fatalf("write blocking file: %v", err)
		}
		if _, err := store.Save(ctx, "tenant-a", "actor-1", "", "a.txt", "", strings.NewReader("payload")); err == nil {
			t.Fatal("expected save mkdir error")
		}
	})

	t.Run("delete fails when index path parent is blocked by file", func(t *testing.T) {
		root := t.TempDir()
		store := NewLocalFileStore(root)
		record, err := store.Save(ctx, "tenant-a", "actor-1", "", "a.txt", "", strings.NewReader("payload"))
		if err != nil {
			t.Fatalf("save file: %v", err)
		}
		if err := os.Remove(store.indexPath()); err != nil {
			t.Fatalf("remove index: %v", err)
		}
		blockerDir := filepath.Dir(store.indexPath())
		if err := os.RemoveAll(blockerDir); err != nil {
			t.Fatalf("remove blocker dir: %v", err)
		}
		if err := os.WriteFile(blockerDir, []byte("blocked"), 0o644); err != nil {
			t.Fatalf("write blocker file: %v", err)
		}
		removed, err := store.Delete(ctx, "tenant-a", record.FileID)
		if err == nil {
			t.Fatal("expected delete index write error")
		}
		if removed {
			t.Fatal("expected delete failure to report not removed")
		}
	})

	t.Run("read index propagates read file error", func(t *testing.T) {
		root := t.TempDir()
		store := NewLocalFileStore(root)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("mkdir root: %v", err)
		}
		if err := os.Remove(store.indexPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove index: %v", err)
		}
		if err := os.Mkdir(store.indexPath(), 0o755); err != nil {
			t.Fatalf("mkdir at index path: %v", err)
		}
		if _, err := store.readIndex(); err == nil {
			t.Fatal("expected read index error")
		}
	})
}

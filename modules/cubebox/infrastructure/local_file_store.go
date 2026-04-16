package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
)

const (
	maxFileSizeBytes         int64 = 20 << 20
	localFileStorageProvider       = "localfs"
)

type FileObject = cubeboxdomain.FileObject

type LocalFileStore struct {
	rootDir string
}

func NewLocalFileStore(rootDir string) *LocalFileStore {
	return &LocalFileStore{rootDir: strings.TrimSpace(rootDir)}
}

func (s *LocalFileStore) SaveObject(
	_ context.Context,
	tenantID string,
	fileID string,
	filename string,
	mediaType string,
	body io.Reader,
) (FileObject, error) {
	tenantID = strings.TrimSpace(tenantID)
	fileID = strings.TrimSpace(fileID)
	filename = strings.TrimSpace(filename)
	mediaType = defaultMediaType(mediaType)
	if tenantID == "" {
		return FileObject{}, cubeboxdomain.ErrFileUploadInvalid
	}
	if fileID == "" {
		return FileObject{}, cubeboxdomain.ErrFileUploadInvalid
	}
	if filename == "" {
		return FileObject{}, cubeboxdomain.ErrFileUploadInvalid
	}
	if err := s.ensureRoot(); err != nil {
		return FileObject{}, err
	}

	limited := io.LimitReader(body, maxFileSizeBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return FileObject{}, err
	}
	if int64(len(raw)) > maxFileSizeBytes {
		return FileObject{}, fmt.Errorf("%w: file exceeds %d bytes", cubeboxdomain.ErrFileUploadInvalid, maxFileSizeBytes)
	}

	safeName := sanitizeFileName(filename)
	storageKey := filepath.ToSlash(filepath.Join(tenantID, fileID, safeName))
	fullPath := filepath.Join(s.filesDir(), filepath.FromSlash(storageKey))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return FileObject{}, err
	}
	if err := os.WriteFile(fullPath, raw, 0o644); err != nil {
		return FileObject{}, err
	}

	sum := sha256.Sum256(raw)
	return FileObject{
		StorageProvider: localFileStorageProvider,
		StorageKey:      storageKey,
		Filename:        safeName,
		ContentType:     mediaType,
		SizeBytes:       int64(len(raw)),
		SHA256:          hex.EncodeToString(sum[:]),
	}, nil
}

func (s *LocalFileStore) DeleteObject(_ context.Context, storageKey string) error {
	if err := s.ensureRoot(); err != nil {
		return err
	}
	fullPath := filepath.Join(s.filesDir(), filepath.FromSlash(strings.TrimSpace(storageKey)))
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalFileStore) Healthy(_ context.Context) error {
	return s.ensureRoot()
}

func (s *LocalFileStore) ensureRoot() error {
	if s.rootDir == "" {
		return errors.New("cubebox file root missing")
	}
	return os.MkdirAll(s.filesDir(), 0o755)
}

func (s *LocalFileStore) filesDir() string {
	return filepath.Join(s.rootDir, "objects")
}

func sanitizeFileName(name string) string {
	trimmed := strings.TrimSpace(filepath.Base(name))
	if trimmed == "" || trimmed == "." || trimmed == string(filepath.Separator) {
		return "upload.bin"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "..", "_")
	return replacer.Replace(trimmed)
}

func defaultMediaType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "application/octet-stream"
	}
	return value
}

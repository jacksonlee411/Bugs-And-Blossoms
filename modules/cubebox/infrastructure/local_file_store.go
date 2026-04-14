package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const maxFileSizeBytes int64 = 20 << 20

type FileRecord struct {
	FileID         string `json:"file_id"`
	TenantID       string `json:"tenant_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	FileName       string `json:"file_name"`
	MediaType      string `json:"media_type"`
	SizeBytes      int64  `json:"size_bytes"`
	SHA256         string `json:"sha256"`
	StorageKey     string `json:"storage_key"`
	UploadedBy     string `json:"uploaded_by"`
	UploadedAt     string `json:"uploaded_at"`
}

type fileIndex struct {
	Items []FileRecord `json:"items"`
}

type LocalFileStore struct {
	rootDir string
	mu      sync.Mutex
}

func NewLocalFileStore(rootDir string) *LocalFileStore {
	return &LocalFileStore{rootDir: strings.TrimSpace(rootDir)}
}

func (s *LocalFileStore) List(_ context.Context, tenantID string, conversationID string) ([]FileRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.readIndex()
	if err != nil {
		return nil, err
	}

	items := make([]FileRecord, 0, len(index.Items))
	for _, item := range index.Items {
		if item.TenantID != tenantID {
			continue
		}
		if conversationID != "" && item.ConversationID != conversationID {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UploadedAt > items[j].UploadedAt
	})
	return items, nil
}

func (s *LocalFileStore) Save(
	_ context.Context,
	tenantID string,
	actorID string,
	conversationID string,
	filename string,
	mediaType string,
	body io.Reader,
) (FileRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tenantID == "" {
		return FileRecord{}, errors.New("tenant_id required")
	}
	if actorID == "" {
		return FileRecord{}, errors.New("uploaded_by required")
	}
	if filename == "" {
		return FileRecord{}, errors.New("file_name required")
	}

	if err := s.ensureRoot(); err != nil {
		return FileRecord{}, err
	}

	limited := io.LimitReader(body, maxFileSizeBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return FileRecord{}, err
	}
	if int64(len(raw)) > maxFileSizeBytes {
		return FileRecord{}, fmt.Errorf("file exceeds %d bytes", maxFileSizeBytes)
	}

	fileID := "file_" + uuid.NewString()
	sum := sha256.Sum256(raw)
	safeName := sanitizeFileName(filename)
	storageKey := filepath.ToSlash(filepath.Join(tenantID, fileID, safeName))
	fullPath := filepath.Join(s.filesDir(), filepath.FromSlash(storageKey))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return FileRecord{}, err
	}
	if err := os.WriteFile(fullPath, raw, 0o644); err != nil {
		return FileRecord{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := FileRecord{
		FileID:         fileID,
		TenantID:       tenantID,
		ConversationID: conversationID,
		FileName:       safeName,
		MediaType:      defaultMediaType(mediaType),
		SizeBytes:      int64(len(raw)),
		SHA256:         hex.EncodeToString(sum[:]),
		StorageKey:     storageKey,
		UploadedBy:     actorID,
		UploadedAt:     now,
	}

	index, err := s.readIndex()
	if err != nil {
		return FileRecord{}, err
	}
	index.Items = append(index.Items, record)
	if err := s.writeIndex(index); err != nil {
		return FileRecord{}, err
	}
	return record, nil
}

func (s *LocalFileStore) Delete(_ context.Context, tenantID string, fileID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.readIndex()
	if err != nil {
		return false, err
	}

	next := make([]FileRecord, 0, len(index.Items))
	var removed *FileRecord
	for idx := range index.Items {
		item := index.Items[idx]
		if item.TenantID == tenantID && item.FileID == fileID {
			copyItem := item
			removed = &copyItem
			continue
		}
		next = append(next, item)
	}
	if removed == nil {
		return false, nil
	}

	index.Items = next
	if err := s.writeIndex(index); err != nil {
		return false, err
	}

	fullPath := filepath.Join(s.filesDir(), filepath.FromSlash(removed.StorageKey))
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, nil
}

func (s *LocalFileStore) Healthy(_ context.Context) error {
	return s.ensureRoot()
}

func (s *LocalFileStore) ensureRoot() error {
	if s.rootDir == "" {
		return errors.New("cubebox file root missing")
	}
	if err := os.MkdirAll(s.filesDir(), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(s.indexPath()); errors.Is(err, os.ErrNotExist) {
		return s.writeIndex(fileIndex{})
	}
	return nil
}

func (s *LocalFileStore) readIndex() (fileIndex, error) {
	if err := s.ensureRoot(); err != nil {
		return fileIndex{}, err
	}
	raw, err := os.ReadFile(s.indexPath())
	if err != nil {
		return fileIndex{}, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fileIndex{}, nil
	}
	var out fileIndex
	if err := json.Unmarshal(raw, &out); err != nil {
		return fileIndex{}, err
	}
	return out, nil
}

func (s *LocalFileStore) writeIndex(index fileIndex) error {
	if err := os.MkdirAll(filepath.Dir(s.indexPath()), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.indexPath(), raw, 0o644)
}

func (s *LocalFileStore) filesDir() string {
	return filepath.Join(s.rootDir, "objects")
}

func (s *LocalFileStore) indexPath() string {
	return filepath.Join(s.rootDir, "index.json")
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

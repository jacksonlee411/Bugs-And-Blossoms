package services

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type stubFileStore struct {
	listTenant       string
	listConversation string
	deleteTenant     string
	deleteFile       string
	saveTenant       string
	saveActor        string
	saveConversation string
	saveFilename     string
	saveMediaType    string
	saveBody         string

	listResult   []FileRecord
	saveResult   FileRecord
	deleteResult bool
	healthyErr   error
	listErr      error
	saveErr      error
	deleteErr    error
}

func (s *stubFileStore) List(_ context.Context, tenantID string, conversationID string) ([]FileRecord, error) {
	s.listTenant = tenantID
	s.listConversation = conversationID
	return s.listResult, s.listErr
}

func (s *stubFileStore) Save(_ context.Context, tenantID string, actorID string, conversationID string, filename string, mediaType string, body io.Reader) (FileRecord, error) {
	s.saveTenant = tenantID
	s.saveActor = actorID
	s.saveConversation = conversationID
	s.saveFilename = filename
	s.saveMediaType = mediaType
	raw, _ := io.ReadAll(body)
	s.saveBody = string(raw)
	return s.saveResult, s.saveErr
}

func (s *stubFileStore) Delete(_ context.Context, tenantID string, fileID string) (bool, error) {
	s.deleteTenant = tenantID
	s.deleteFile = fileID
	return s.deleteResult, s.deleteErr
}

func (s *stubFileStore) Healthy(context.Context) error {
	return s.healthyErr
}

func TestFileServiceNilBranches(t *testing.T) {
	t.Parallel()

	var nilSvc *FileService
	items, err := nilSvc.ListFiles(context.Background(), "tenant", "conversation")
	if err != nil {
		t.Fatalf("nil list err: %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items, got %+v", items)
	}
	deleted, err := nilSvc.DeleteFile(context.Background(), "tenant", "file")
	if err != nil {
		t.Fatalf("nil delete err: %v", err)
	}
	if deleted {
		t.Fatal("expected false delete on nil service")
	}
	if err := nilSvc.Healthy(context.Background()); err != nil {
		t.Fatalf("nil healthy err: %v", err)
	}

	emptySvc := &FileService{}
	items, err = emptySvc.ListFiles(context.Background(), "tenant", "conversation")
	if err != nil {
		t.Fatalf("empty list err: %v", err)
	}
	if items != nil {
		t.Fatalf("expected nil items from empty store, got %+v", items)
	}
	deleted, err = emptySvc.DeleteFile(context.Background(), "tenant", "file")
	if err != nil {
		t.Fatalf("empty delete err: %v", err)
	}
	if deleted {
		t.Fatal("expected false delete from empty store")
	}
	if err := emptySvc.Healthy(context.Background()); err != nil {
		t.Fatalf("empty healthy err: %v", err)
	}
}

func TestFileServiceDelegatesAndTrims(t *testing.T) {
	t.Parallel()

	store := &stubFileStore{
		listResult:   []FileRecord{{FileID: "file-1"}},
		saveResult:   FileRecord{FileID: "file-2"},
		deleteResult: true,
		healthyErr:   errors.New("unhealthy"),
	}
	svc := NewFileService(store)
	if svc == nil {
		t.Fatal("expected service")
	}

	items, err := svc.ListFiles(context.Background(), " tenant-a ", " conversation-a ")
	if err != nil {
		t.Fatalf("list err: %v", err)
	}
	if len(items) != 1 || items[0].FileID != "file-1" {
		t.Fatalf("unexpected list result: %+v", items)
	}
	if store.listTenant != "tenant-a" || store.listConversation != "conversation-a" {
		t.Fatalf("unexpected trimmed list args: tenant=%q conversation=%q", store.listTenant, store.listConversation)
	}

	record, err := svc.SaveFile(context.Background(), " tenant-a ", " actor-1 ", " conversation-a ", " notes.txt ", " text/plain ", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("save err: %v", err)
	}
	if record.FileID != "file-2" {
		t.Fatalf("unexpected save result: %+v", record)
	}
	if store.saveTenant != "tenant-a" || store.saveActor != "actor-1" || store.saveConversation != "conversation-a" {
		t.Fatalf("unexpected save tenant/actor/conversation args: %+v", store)
	}
	if store.saveFilename != "notes.txt" || store.saveMediaType != "text/plain" || store.saveBody != "payload" {
		t.Fatalf("unexpected save args filename=%q media=%q body=%q", store.saveFilename, store.saveMediaType, store.saveBody)
	}

	deleted, err := svc.DeleteFile(context.Background(), " tenant-a ", " file-2 ")
	if err != nil {
		t.Fatalf("delete err: %v", err)
	}
	if !deleted {
		t.Fatal("expected delete true")
	}
	if store.deleteTenant != "tenant-a" || store.deleteFile != "file-2" {
		t.Fatalf("unexpected delete args tenant=%q file=%q", store.deleteTenant, store.deleteFile)
	}

	if err := svc.Healthy(context.Background()); err == nil || err.Error() != "unhealthy" {
		t.Fatalf("expected unhealthy error, got %v", err)
	}
}

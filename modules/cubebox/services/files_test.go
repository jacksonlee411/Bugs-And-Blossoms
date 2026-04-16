package services

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
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

	listResult []FileRecord
	saveResult FileRecord
	healthyErr error
	listErr    error
	saveErr    error
	deleteErr  error
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
	if errors.Is(s.deleteErr, ErrFileNotFound) {
		return false, s.deleteErr
	}
	return true, s.deleteErr
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
	if err := emptySvc.Healthy(context.Background()); !errors.Is(err, ErrFileUnavailable) {
		t.Fatalf("expected file unavailable, got %v", err)
	}
}

func TestFileServiceDelegatesAndTrims(t *testing.T) {
	t.Parallel()

	store := &stubFileStore{
		listResult: []FileRecord{{FileID: "file-1"}},
		saveResult: FileRecord{FileID: "file-2"},
		healthyErr: errors.New("unhealthy"),
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

	store.deleteErr = ErrFileNotFound
	deleted, err = svc.DeleteFile(context.Background(), "tenant-a", "missing")
	if err != nil {
		t.Fatalf("delete missing err: %v", err)
	}
	if deleted {
		t.Fatal("expected missing delete to map to false")
	}

	if err := svc.Healthy(context.Background()); err == nil || err.Error() != "unhealthy" {
		t.Fatalf("expected unhealthy error, got %v", err)
	}
}

type stubFileRepo struct {
	conversationExists    bool
	conversationExistsErr error
	createFileErr         error
	cleanupErr            error
	healthyErr            error
	listFilesResult       []FileMetadata
	listFilesErr          error
	listTenantLinksResult []FileLinkRef
	listTenantLinksErr    error
	listFileLinksByFileID map[string][]FileLinkRef
	listFileLinksErr      error
	getFileResult         FileMetadata
	getFileErr            error
	countFileLinks        int64
	countFileLinksErr     error
	deleteRows            int64
	deleteFileErr         error

	healthyTenant string
	createdFileID string
	createdObject FileObject
	cleanupJobs   []FileCleanupJob
	listedFileIDs []string
}

func (s *stubFileRepo) ListFiles(context.Context, string, string, int32) ([]FileMetadata, error) {
	return s.listFilesResult, s.listFilesErr
}

func (s *stubFileRepo) ListFileLinks(_ context.Context, _ string, fileID string) ([]FileLinkRef, error) {
	s.listedFileIDs = append(s.listedFileIDs, fileID)
	if s.listFileLinksErr != nil {
		return nil, s.listFileLinksErr
	}
	if s.listFileLinksByFileID == nil {
		return nil, nil
	}
	return s.listFileLinksByFileID[fileID], nil
}

func (s *stubFileRepo) ListTenantFileLinks(context.Context, string) ([]FileLinkRef, error) {
	return s.listTenantLinksResult, s.listTenantLinksErr
}

func (s *stubFileRepo) GetFile(context.Context, string, string) (FileMetadata, error) {
	if s.getFileErr != nil {
		return FileMetadata{}, s.getFileErr
	}
	return s.getFileResult, nil
}

func (s *stubFileRepo) ConversationExists(context.Context, string, string) (bool, error) {
	return s.conversationExists, s.conversationExistsErr
}

func (s *stubFileRepo) CreateFile(_ context.Context, _ string, record FileObject, fileID string, actorID string, conversationID string, now time.Time) (FileMetadata, []FileLinkRef, error) {
	s.createdFileID = fileID
	s.createdObject = record
	if s.createFileErr != nil {
		return FileMetadata{}, nil, s.createFileErr
	}
	return FileMetadata{
			FileID:          fileID,
			Filename:        record.Filename,
			ContentType:     record.ContentType,
			SizeBytes:       record.SizeBytes,
			SHA256:          record.SHA256,
			StorageProvider: record.StorageProvider,
			StorageKey:      record.StorageKey,
			ScanStatus:      "ready",
			UploadedBy:      actorID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}, []FileLinkRef{{
			FileID:         fileID,
			ConversationID: conversationID,
			LinkRole:       "conversation_attachment",
		}}, nil
}

func (s *stubFileRepo) CountFileLinks(context.Context, string, string) (int64, error) {
	return s.countFileLinks, s.countFileLinksErr
}

func (s *stubFileRepo) DeleteFile(context.Context, string, string) (int64, error) {
	return s.deleteRows, s.deleteFileErr
}

func (s *stubFileRepo) InsertFileCleanupJob(_ context.Context, _ string, job FileCleanupJob, _ time.Time) error {
	s.cleanupJobs = append(s.cleanupJobs, job)
	return s.cleanupErr
}

func (s *stubFileRepo) Healthy(_ context.Context, tenantID string) error {
	s.healthyTenant = tenantID
	return s.healthyErr
}

type stubFileObjectStore struct {
	saveObject     FileObject
	saveErr        error
	deleteErr      error
	healthyErr     error
	savedTenant    string
	savedFileID    string
	savedFilename  string
	savedMediaType string
	savedBody      string
	deletedKey     string
}

func (s *stubFileObjectStore) SaveObject(_ context.Context, tenantID string, fileID string, filename string, mediaType string, body io.Reader) (FileObject, error) {
	s.savedTenant = tenantID
	s.savedFileID = fileID
	s.savedFilename = filename
	s.savedMediaType = mediaType
	if body != nil {
		raw, _ := io.ReadAll(body)
		s.savedBody = string(raw)
	}
	if s.saveErr != nil {
		return FileObject{}, s.saveErr
	}
	object := s.saveObject
	if object.StorageProvider == "" {
		object.StorageProvider = "localfs"
	}
	if object.StorageKey == "" {
		object.StorageKey = tenantID + "/" + fileID + "/notes.txt"
	}
	if object.Filename == "" {
		object.Filename = strings.TrimSpace(filename)
	}
	if object.ContentType == "" {
		object.ContentType = strings.TrimSpace(mediaType)
	}
	if object.SizeBytes == 0 {
		object.SizeBytes = int64(len(s.savedBody))
	}
	if object.SHA256 == "" {
		object.SHA256 = "sha256"
	}
	return object, nil
}

func (s *stubFileObjectStore) DeleteObject(_ context.Context, storageKey string) error {
	s.deletedKey = storageKey
	return s.deleteErr
}

func (s *stubFileObjectStore) Healthy(context.Context) error {
	return s.healthyErr
}

func TestFileServiceSaveFileUsesStableFileID(t *testing.T) {
	t.Parallel()

	repo := &stubFileRepo{conversationExists: true}
	objectStore := &stubFileObjectStore{}
	svc := NewFileService(repo, objectStore)
	record, err := svc.SaveFile(context.Background(), "tenant-a", "actor-1", "conversation-a", "notes.txt", "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("save file: %v", err)
	}
	if repo.createdFileID == "" || !strings.HasPrefix(repo.createdFileID, "file_") {
		t.Fatalf("expected stable file id, got %q", repo.createdFileID)
	}
	if objectStore.savedFileID != repo.createdFileID {
		t.Fatalf("object store file id mismatch: store=%q repo=%q", objectStore.savedFileID, repo.createdFileID)
	}
	if repo.createdObject.StorageKey != "tenant-a/"+repo.createdFileID+"/notes.txt" {
		t.Fatalf("unexpected storage key: %q", repo.createdObject.StorageKey)
	}
	if record.FileID != repo.createdFileID {
		t.Fatalf("record=%+v repoFileID=%q", record, repo.createdFileID)
	}
}

func TestFileServiceSaveFileMetadataFailureCompensatesBeforeCleanup(t *testing.T) {
	t.Parallel()

	t.Run("delete succeeds so no cleanup job is recorded", func(t *testing.T) {
		repo := &stubFileRepo{
			conversationExists: true,
			createFileErr:      errors.New("metadata write failed"),
		}
		objectStore := &stubFileObjectStore{}
		svc := NewFileService(repo, objectStore)

		if _, err := svc.SaveFile(context.Background(), "tenant-a", "actor-1", "conversation-a", "notes.txt", "text/plain", strings.NewReader("payload")); err == nil {
			t.Fatal("expected save failure")
		}
		if objectStore.deletedKey == "" {
			t.Fatal("expected object compensation delete")
		}
		if len(repo.cleanupJobs) != 0 {
			t.Fatalf("expected no cleanup job, got %+v", repo.cleanupJobs)
		}
	})

	t.Run("delete fails so cleanup job is recorded", func(t *testing.T) {
		repo := &stubFileRepo{
			conversationExists: true,
			createFileErr:      errors.New("metadata write failed"),
		}
		objectStore := &stubFileObjectStore{
			deleteErr: errors.New("disk unavailable"),
		}
		svc := NewFileService(repo, objectStore)

		if _, err := svc.SaveFile(context.Background(), "tenant-a", "actor-1", "conversation-a", "notes.txt", "text/plain", strings.NewReader("payload")); err == nil {
			t.Fatal("expected save failure")
		}
		if len(repo.cleanupJobs) != 1 {
			t.Fatalf("expected one cleanup job, got %+v", repo.cleanupJobs)
		}
		if repo.cleanupJobs[0].CleanupReason != "metadata_write_failed" {
			t.Fatalf("unexpected cleanup job: %+v", repo.cleanupJobs[0])
		}
		if repo.cleanupJobs[0].LastError != "disk unavailable" {
			t.Fatalf("unexpected cleanup error: %+v", repo.cleanupJobs[0])
		}
	})
}

func TestFileServiceHealthyChecksRepoAndObjectStore(t *testing.T) {
	t.Parallel()

	repo := &stubFileRepo{}
	objectStore := &stubFileObjectStore{}
	svc := NewFileService(repo, objectStore)
	if err := svc.Healthy(context.Background()); err != nil {
		t.Fatalf("healthy err: %v", err)
	}
	if repo.healthyTenant != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("unexpected health tenant: %q", repo.healthyTenant)
	}

	repo.healthyErr = errors.New("repo down")
	if err := svc.Healthy(context.Background()); err == nil || err.Error() != "repo down" {
		t.Fatalf("expected repo down, got %v", err)
	}

	repo.healthyErr = nil
	objectStore.healthyErr = errors.New("disk down")
	if err := svc.Healthy(context.Background()); err == nil || err.Error() != "disk down" {
		t.Fatalf("expected disk down, got %v", err)
	}
}

func TestFileServiceListFilesFormalPath(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC)

	t.Run("tenant scope groups tenant links by file id", func(t *testing.T) {
		repo := &stubFileRepo{
			listFilesResult: []FileMetadata{
				{FileID: " file-1 ", Filename: "a.txt", ContentType: "text/plain", CreatedAt: now, UpdatedAt: now},
				{FileID: "file-2", Filename: "b.txt", ContentType: "text/plain", CreatedAt: now, UpdatedAt: now},
			},
			listTenantLinksResult: []FileLinkRef{
				{FileID: " file-1 ", LinkRole: " conversation_attachment ", ConversationID: " conv-1 ", TurnID: " turn-1 "},
				{FileID: "   ", LinkRole: "ignored", ConversationID: "ignored"},
			},
		}

		items, err := NewFileService(repo).ListFiles(context.Background(), " tenant-a ", " ")
		if err != nil {
			t.Fatalf("list files: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %+v", items)
		}
		if items[0].ConversationID != "conv-1" || len(items[0].Links) != 1 {
			t.Fatalf("expected first item to carry grouped link, got %+v", items[0])
		}
		if items[0].Links[0].TurnID != "turn-1" || items[0].Links[0].LinkRole != "conversation_attachment" {
			t.Fatalf("unexpected trimmed link: %+v", items[0].Links[0])
		}
		if items[1].ConversationID != "" || len(items[1].Links) != 0 {
			t.Fatalf("expected second item to remain unlinked, got %+v", items[1])
		}
	})

	t.Run("conversation scope loads links per file", func(t *testing.T) {
		repo := &stubFileRepo{
			listFilesResult: []FileMetadata{
				{FileID: "file-1", Filename: "a.txt", ContentType: "text/plain", CreatedAt: now, UpdatedAt: now},
				{FileID: "file-2", Filename: "b.txt", ContentType: "text/plain", CreatedAt: now, UpdatedAt: now},
			},
			listFileLinksByFileID: map[string][]FileLinkRef{
				"file-1": {{FileID: "file-1", ConversationID: "conv-9", LinkRole: "conversation_attachment"}},
				"file-2": {{FileID: "file-2", ConversationID: "conv-9", LinkRole: "conversation_attachment", TurnID: "turn-2"}},
			},
		}

		items, err := NewFileService(repo).ListFiles(context.Background(), "tenant-a", " conv-9 ")
		if err != nil {
			t.Fatalf("list conversation files: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %+v", items)
		}
		if len(repo.listedFileIDs) != 2 || repo.listedFileIDs[0] != "file-1" || repo.listedFileIDs[1] != "file-2" {
			t.Fatalf("expected per-file link lookups, got %+v", repo.listedFileIDs)
		}
		if items[1].Links[0].TurnID != "turn-2" {
			t.Fatalf("expected per-file turn id, got %+v", items[1].Links)
		}
	})

	t.Run("link lookup error stops aggregation", func(t *testing.T) {
		repo := &stubFileRepo{
			listFilesResult:  []FileMetadata{{FileID: "file-1", CreatedAt: now, UpdatedAt: now}},
			listFileLinksErr: errors.New("list links failed"),
		}

		if _, err := NewFileService(repo).ListFiles(context.Background(), "tenant-a", "conversation-a"); err == nil || err.Error() != "list links failed" {
			t.Fatalf("expected list links failure, got %v", err)
		}
	})
}

func TestFileServiceSaveFileValidationAndDeleteFormalPath(t *testing.T) {
	t.Parallel()

	t.Run("save file validates inputs and conversation checks", func(t *testing.T) {
		repo := &stubFileRepo{}
		objectStore := &stubFileObjectStore{}
		svc := NewFileService(repo, objectStore)

		if _, err := svc.SaveFile(context.Background(), "", "actor-1", "", "notes.txt", "text/plain", strings.NewReader("payload")); !errors.Is(err, ErrFileUploadInvalid) {
			t.Fatalf("expected invalid upload, got %v", err)
		}

		repo.conversationExistsErr = errors.New("repo unavailable")
		if _, err := svc.SaveFile(context.Background(), "tenant-a", "actor-1", "conversation-a", "notes.txt", "text/plain", strings.NewReader("payload")); err == nil || err.Error() != "repo unavailable" {
			t.Fatalf("expected conversation lookup failure, got %v", err)
		}

		repo.conversationExistsErr = nil
		if _, err := svc.SaveFile(context.Background(), "tenant-a", "actor-1", "conversation-a", "notes.txt", "text/plain", strings.NewReader("payload")); !errors.Is(err, ErrFileConversationNotFound) {
			t.Fatalf("expected conversation not found, got %v", err)
		}

		repo.conversationExists = true
		objectStore.saveErr = errors.New("disk full")
		if _, err := svc.SaveFile(context.Background(), "tenant-a", "actor-1", "conversation-a", "notes.txt", "text/plain", strings.NewReader("payload")); err == nil || err.Error() != "disk full" {
			t.Fatalf("expected object store error, got %v", err)
		}
	})

	t.Run("delete file formal path handles blocked and cleanup branches", func(t *testing.T) {
		repo := &stubFileRepo{
			getFileResult: FileMetadata{
				FileID:          "file-1",
				StorageProvider: "localfs",
				StorageKey:      "tenant-a/file-1/notes.txt",
			},
			deleteRows: 1,
		}
		objectStore := &stubFileObjectStore{}
		svc := NewFileService(repo, objectStore)

		repo.getFileErr = errors.New("missing")
		if deleted, err := svc.DeleteFile(context.Background(), "tenant-a", "file-1"); deleted || !errors.Is(err, ErrFileNotFound) {
			t.Fatalf("expected missing file, deleted=%v err=%v", deleted, err)
		}

		repo.getFileErr = nil
		repo.countFileLinksErr = errors.New("count failed")
		if deleted, err := svc.DeleteFile(context.Background(), "tenant-a", "file-1"); deleted || err == nil || err.Error() != "count failed" {
			t.Fatalf("expected count failure, deleted=%v err=%v", deleted, err)
		}

		repo.countFileLinksErr = nil
		repo.countFileLinks = 2
		if deleted, err := svc.DeleteFile(context.Background(), "tenant-a", "file-1"); deleted || !errors.Is(err, ErrFileDeleteBlocked) {
			t.Fatalf("expected delete blocked, deleted=%v err=%v", deleted, err)
		}

		repo.countFileLinks = 0
		repo.deleteRows = 0
		if deleted, err := svc.DeleteFile(context.Background(), "tenant-a", "file-1"); deleted || !errors.Is(err, ErrFileNotFound) {
			t.Fatalf("expected deleted row miss, deleted=%v err=%v", deleted, err)
		}

		repo.deleteRows = 1
		objectStore.deleteErr = errors.New("unlink failed")
		deleted, err := svc.DeleteFile(context.Background(), "tenant-a", "file-1")
		if err != nil || !deleted {
			t.Fatalf("expected delete success with cleanup compensation, deleted=%v err=%v", deleted, err)
		}
		if len(repo.cleanupJobs) != 1 || repo.cleanupJobs[0].CleanupReason != "object_delete_failed" {
			t.Fatalf("expected cleanup job after object delete failure, got %+v", repo.cleanupJobs)
		}
	})
}

func TestFileServiceHelpers(t *testing.T) {
	t.Parallel()

	if got := mapFileLinks(nil); got != nil {
		t.Fatalf("expected nil mapped links, got %+v", got)
	}
	if got := formatFileTime(time.Time{}); got != "" {
		t.Fatalf("expected empty zero timestamp, got %q", got)
	}

	svc := &FileService{}
	if svc.now().IsZero() {
		t.Fatal("expected fallback now timestamp")
	}

	svc.compensateCleanup(context.Background(), "tenant-a", FileCleanupJob{FileID: "file-1"})
}

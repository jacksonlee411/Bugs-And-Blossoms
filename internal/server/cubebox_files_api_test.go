package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cubeboxservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/services"
)

type stubCubeBoxFileStore struct {
	listResult   []cubeboxservices.FileRecord
	saveResult   cubeboxservices.FileRecord
	deleteResult bool
	listErr      error
	saveErr      error
	deleteErr    error

	listTenant       string
	listConversation string
	saveTenant       string
	saveActor        string
	saveConversation string
	saveFilename     string
	saveMediaType    string
	saveBody         string
	deleteTenant     string
	deleteFileID     string
}

func (s *stubCubeBoxFileStore) List(_ context.Context, tenantID string, conversationID string) ([]cubeboxservices.FileRecord, error) {
	s.listTenant = tenantID
	s.listConversation = conversationID
	return s.listResult, s.listErr
}

func (s *stubCubeBoxFileStore) Save(_ context.Context, tenantID string, actorID string, conversationID string, filename string, mediaType string, body io.Reader) (cubeboxservices.FileRecord, error) {
	s.saveTenant = tenantID
	s.saveActor = actorID
	s.saveConversation = conversationID
	s.saveFilename = filename
	s.saveMediaType = mediaType
	raw, _ := io.ReadAll(body)
	s.saveBody = string(raw)
	return s.saveResult, s.saveErr
}

func (s *stubCubeBoxFileStore) Delete(_ context.Context, tenantID string, fileID string) (bool, error) {
	s.deleteTenant = tenantID
	s.deleteFileID = fileID
	return s.deleteResult, s.deleteErr
}

func (s *stubCubeBoxFileStore) Healthy(context.Context) error { return nil }

func TestCubeBoxFilesAPI(t *testing.T) {
	t.Run("method routing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/internal/cubebox/files", nil)
		handleCubeBoxFilesAPI(rec, req, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("post routes to upload", func(t *testing.T) {
		store := &stubCubeBoxFileStore{
			saveResult: cubeboxservices.FileRecord{FileID: "file-routed", FileName: "hello.txt"},
		}
		svc := cubeboxservices.NewFileService(store)
		req := newCubeBoxUploadRequest(t, "conversation-2", "hello.txt", "", []byte("hello"))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-a"}))
		rec := httptest.NewRecorder()

		handleCubeBoxFilesAPI(rec, req, svc)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if store.saveFilename != "hello.txt" {
			t.Fatalf("unexpected save filename: %+v", store)
		}
	})

	t.Run("list files success", func(t *testing.T) {
		store := &stubCubeBoxFileStore{
			listResult: []cubeboxservices.FileRecord{{
				FileID:         "file-1",
				ConversationID: "conversation-1",
				FileName:       "doc.txt",
				MediaType:      "text/plain",
				SizeBytes:      42,
				SHA256:         "sha256",
				StorageKey:     "tenant-a/file-1/doc.txt",
				UploadedBy:     "actor-1",
				UploadedAt:     "2026-04-15T00:00:00Z",
			}},
		}
		svc := cubeboxservices.NewFileService(store)
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/files?conversation_id=conversation-1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
		rec := httptest.NewRecorder()

		handleCubeBoxFilesAPI(rec, req, svc)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var payload cubeboxFileListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Items) != 1 || payload.Items[0].FileID != "file-1" {
			t.Fatalf("payload=%+v", payload)
		}
		if store.listTenant != "tenant-a" || store.listConversation != "conversation-1" {
			t.Fatalf("unexpected list args tenant=%q conversation=%q", store.listTenant, store.listConversation)
		}
	})

	t.Run("list files failures", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/files", nil)
		handleCubeBoxFilesListAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		store := &stubCubeBoxFileStore{}
		svc := cubeboxservices.NewFileService(store)
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/files", nil)
		rec = httptest.NewRecorder()
		handleCubeBoxFilesListAPI(rec, req, svc)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		store.listErr = errors.New("boom")
		req = httptest.NewRequest(http.MethodGet, "/internal/cubebox/files", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
		rec = httptest.NewRecorder()
		handleCubeBoxFilesListAPI(rec, req, svc)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("upload success and helpers", func(t *testing.T) {
		store := &stubCubeBoxFileStore{
			saveResult: cubeboxservices.FileRecord{
				FileID:         "file-2",
				ConversationID: "conversation-2",
				FileName:       "hello.txt",
				MediaType:      "text/plain; charset=utf-8",
				SizeBytes:      5,
				SHA256:         "sum",
				StorageKey:     "tenant-a/file-2/hello.txt",
				UploadedBy:     "actor-1",
				UploadedAt:     "2026-04-15T00:00:00Z",
			},
		}
		svc := cubeboxservices.NewFileService(store)
		req := newCubeBoxUploadRequest(t, "conversation-2", "hello.txt", "", []byte("hello"))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-a"}))
		rec := httptest.NewRecorder()

		handleCubeBoxFilesUploadAPI(rec, req, svc)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if store.saveTenant != "tenant-a" || store.saveActor != "actor-1" || store.saveConversation != "conversation-2" {
			t.Fatalf("unexpected save args: %+v", store)
		}
		if store.saveFilename != "hello.txt" || store.saveBody != "hello" {
			t.Fatalf("unexpected save filename/body: %+v", store)
		}
		if store.saveMediaType != "application/octet-stream" {
			t.Fatalf("expected detected application/octet-stream, got %q", store.saveMediaType)
		}

		resp := cubeboxFileRecordResponse(store.saveResult)
		if resp.FileID != "file-2" || resp.StorageKey != "tenant-a/file-2/hello.txt" {
			t.Fatalf("unexpected record response: %+v", resp)
		}
		if got := detectUploadMediaType("application/pdf", bytes.NewReader([]byte("hello"))); got != "application/pdf" {
			t.Fatalf("unexpected explicit content type: %q", got)
		}
		if got := detectUploadMediaType("", nil); got != "application/octet-stream" {
			t.Fatalf("unexpected nil content type: %q", got)
		}
		if got := detectUploadMediaType("", bytes.NewReader(nil)); got != "application/octet-stream" {
			t.Fatalf("unexpected empty content type: %q", got)
		}
		if got := detectUploadMediaType("", readSeekErrorReader{}); got != "application/octet-stream" {
			t.Fatalf("unexpected seek error content type: %q", got)
		}
	})

	t.Run("upload failures", func(t *testing.T) {
		store := &stubCubeBoxFileStore{}
		svc := cubeboxservices.NewFileService(store)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/files", nil)
		handleCubeBoxFilesUploadAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/files", nil)
		rec = httptest.NewRecorder()
		handleCubeBoxFilesUploadAPI(rec, req, svc)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/files", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
		rec = httptest.NewRecorder()
		handleCubeBoxFilesUploadAPI(rec, req, svc)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/files", strings.NewReader("not multipart"))
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-a"}))
		rec = httptest.NewRecorder()
		handleCubeBoxFilesUploadAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		req = newCubeBoxUploadRequestWithoutFile(t)
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-a"}))
		rec = httptest.NewRecorder()
		handleCubeBoxFilesUploadAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/internal/cubebox/files", http.MaxBytesReader(httptest.NewRecorder(), io.NopCloser(strings.NewReader("x")), 1))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-a"}))
		rec = httptest.NewRecorder()
		handleCubeBoxFilesUploadAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		store.saveErr = errors.New("file_name required")
		req = newCubeBoxUploadRequest(t, "", "hello.txt", "text/plain", []byte("hello"))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-a"}))
		rec = httptest.NewRecorder()
		handleCubeBoxFilesUploadAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		store.saveErr = errors.New("storage failure")
		req = newCubeBoxUploadRequest(t, "", "hello.txt", "text/plain", []byte("hello"))
		req = req.WithContext(withTenant(withPrincipal(req.Context(), Principal{ID: "actor-1"}), Tenant{ID: "tenant-a"}))
		rec = httptest.NewRecorder()
		handleCubeBoxFilesUploadAPI(rec, req, svc)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("delete file branches", func(t *testing.T) {
		store := &stubCubeBoxFileStore{}
		svc := cubeboxservices.NewFileService(store)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/internal/cubebox/files/file-1", nil)
		handleCubeBoxFileDeleteAPI(rec, req, svc)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/internal/cubebox/files/file-1", nil)
		handleCubeBoxFileDeleteAPI(rec, req, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/internal/cubebox/files/file-1", nil)
		handleCubeBoxFileDeleteAPI(rec, req, svc)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/internal/cubebox/files", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
		handleCubeBoxFileDeleteAPI(rec, req, svc)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		store.deleteErr = errors.New("boom")
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/internal/cubebox/files/file-1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
		handleCubeBoxFileDeleteAPI(rec, req, svc)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		store.deleteErr = nil
		store.deleteResult = false
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/internal/cubebox/files/file-1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
		handleCubeBoxFileDeleteAPI(rec, req, svc)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		store.deleteResult = true
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodDelete, "/internal/cubebox/files/file-1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "tenant-a"}))
		handleCubeBoxFileDeleteAPI(rec, req, svc)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if store.deleteTenant != "tenant-a" || store.deleteFileID != "file-1" {
			t.Fatalf("unexpected delete args tenant=%q file=%q", store.deleteTenant, store.deleteFileID)
		}
	})
}

func TestExtractCubeBoxFileIDFromPath(t *testing.T) {
	t.Parallel()

	if fileID, ok := extractCubeBoxFileIDFromPath("/internal/cubebox/files/file-1"); !ok || fileID != "file-1" {
		t.Fatalf("expected file-1, got %q ok=%v", fileID, ok)
	}
	if _, ok := extractCubeBoxFileIDFromPath("/internal/cubebox/files/ "); ok {
		t.Fatal("expected blank file id rejected")
	}
	if _, ok := extractCubeBoxFileIDFromPath("/internal/assistant/files/file-1"); ok {
		t.Fatal("expected assistant path rejected")
	}
	if _, ok := extractCubeBoxFileIDFromPath("/internal/cubebox/files/file-1/extra"); ok {
		t.Fatal("expected extra segment rejected")
	}
}

func TestDefaultCubeBoxFileRoot(t *testing.T) {
	t.Run("prefers environment override", func(t *testing.T) {
		t.Setenv("CUBEBOX_FILE_ROOT", " /tmp/cubebox-root ")
		if got := defaultCubeBoxFileRoot(); got != "/tmp/cubebox-root" {
			t.Fatalf("unexpected root: %q", got)
		}
	})

	t.Run("falls back to working directory", func(t *testing.T) {
		t.Setenv("CUBEBOX_FILE_ROOT", "")
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(wd, ".local", "cubebox", "files")
		if got := defaultCubeBoxFileRoot(); got != want {
			t.Fatalf("unexpected root: got %q want %q", got, want)
		}
	})
}

type readSeekErrorReader struct{}

func (readSeekErrorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (readSeekErrorReader) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek failed")
}

func newCubeBoxUploadRequest(t *testing.T, conversationID string, filename string, contentType string, body []byte) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if conversationID != "" {
		if err := writer.WriteField("conversation_id", conversationID); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 0 {
		if _, err := part.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/files", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if contentType != "" {
		req.Header.Set("X-Content-Type-Override", contentType)
	}
	return req
}

func newCubeBoxUploadRequestWithoutFile(t *testing.T) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("conversation_id", "conversation-1"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/files", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

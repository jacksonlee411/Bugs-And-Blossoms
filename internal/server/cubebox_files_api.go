package server

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
)

const cubeboxFileUploadLimitBytes int64 = 20 << 20

type cubeboxFileResponse struct {
	FileID         string `json:"file_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	FileName       string `json:"file_name"`
	MediaType      string `json:"media_type"`
	SizeBytes      int64  `json:"size_bytes"`
	SHA256         string `json:"sha256"`
	StorageKey     string `json:"storage_key"`
	UploadedBy     string `json:"uploaded_by"`
	UploadedAt     string `json:"uploaded_at"`
}

type cubeboxFileListResponse struct {
	Items []cubeboxFileResponse `json:"items"`
}

type cubeBoxFileFacade interface {
	ListFiles(ctx context.Context, tenantID string, conversationID string) ([]cubeboxdomain.FileRecord, error)
	SaveFile(ctx context.Context, tenantID string, actorID string, conversationID string, filename string, mediaType string, body io.Reader) (cubeboxdomain.FileRecord, error)
	DeleteFile(ctx context.Context, tenantID string, fileID string) (bool, error)
}

func handleCubeBoxFilesAPI(w http.ResponseWriter, r *http.Request, svc cubeBoxFileFacade) {
	switch r.Method {
	case http.MethodGet:
		handleCubeBoxFilesListAPI(w, r, svc)
	case http.MethodPost:
		handleCubeBoxFilesUploadAPI(w, r, svc)
	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func handleCubeBoxFilesListAPI(w http.ResponseWriter, r *http.Request, svc cubeBoxFileFacade) {
	if svc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, "cubebox_files_unavailable", "cubebox files unavailable")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	items, err := svc.ListFiles(r.Context(), tenant.ID, strings.TrimSpace(r.URL.Query().Get("conversation_id")))
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_files_list_failed", "cubebox files list failed")
		return
	}
	resp := cubeboxFileListResponse{Items: make([]cubeboxFileResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, cubeboxFileRecordResponse(item))
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleCubeBoxFilesUploadAPI(w http.ResponseWriter, r *http.Request, svc cubeBoxFileFacade) {
	if svc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, "cubebox_files_unavailable", "cubebox files unavailable")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	principal, ok := currentPrincipal(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, cubeboxFileUploadLimitBytes+1024)
	if err := r.ParseMultipartForm(cubeboxFileUploadLimitBytes); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "file required")
		return
	}
	defer file.Close()

	record, err := svc.SaveFile(
		r.Context(),
		tenant.ID,
		principal.ID,
		strings.TrimSpace(r.FormValue("conversation_id")),
		strings.TrimSpace(header.Filename),
		detectUploadMediaType(header.Header.Get("Content-Type"), file),
		file,
	)
	if err != nil {
		status := http.StatusInternalServerError
		code := "cubebox_file_upload_failed"
		message := "cubebox file upload failed"
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "exceeds") {
			status = http.StatusBadRequest
			code = "invalid_request"
			message = err.Error()
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, status, code, message)
		return
	}
	writeJSON(w, http.StatusCreated, cubeboxFileRecordResponse(record))
}

func handleCubeBoxFileDeleteAPI(w http.ResponseWriter, r *http.Request, svc cubeBoxFileFacade) {
	if r.Method != http.MethodDelete {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if svc == nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusServiceUnavailable, "cubebox_files_unavailable", "cubebox files unavailable")
		return
	}
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	fileID, ok := extractCubeBoxFileIDFromPath(r.URL.Path)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "invalid file path")
		return
	}
	deleted, err := svc.DeleteFile(r.Context(), tenant.ID, fileID)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "cubebox_file_delete_failed", "cubebox file delete failed")
		return
	}
	if !deleted {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "cubebox_file_not_found", "cubebox file not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func cubeboxFileRecordResponse(item cubeboxdomain.FileRecord) cubeboxFileResponse {
	return cubeboxFileResponse{
		FileID:         item.FileID,
		ConversationID: item.ConversationID,
		FileName:       item.FileName,
		MediaType:      item.MediaType,
		SizeBytes:      item.SizeBytes,
		SHA256:         item.SHA256,
		StorageKey:     item.StorageKey,
		UploadedBy:     item.UploadedBy,
		UploadedAt:     item.UploadedAt,
	}
}

func detectUploadMediaType(contentType string, file io.ReadSeeker) string {
	if strings.TrimSpace(contentType) != "" {
		return contentType
	}
	if file == nil {
		return "application/octet-stream"
	}
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		return "application/octet-stream"
	}
	if err != nil && err != io.EOF {
		return "application/octet-stream"
	}
	if n <= 0 {
		return "application/octet-stream"
	}
	return http.DetectContentType(buf[:n])
}

func extractCubeBoxFileIDFromPath(path string) (string, bool) {
	parts := assistantSplitPathSegments(path)
	if len(parts) != 4 {
		return "", false
	}
	if parts[0] != "internal" || parts[1] != "cubebox" || parts[2] != "files" {
		return "", false
	}
	fileID := strings.TrimSpace(parts[3])
	if fileID == "" {
		return "", false
	}
	return fileID, true
}

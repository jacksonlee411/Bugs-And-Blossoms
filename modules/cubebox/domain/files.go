package domain

import "errors"

var (
	ErrFileUnavailable            = errors.New("cubebox_files_unavailable")
	ErrFileNotFound               = errors.New("cubebox_file_not_found")
	ErrFileDeleteBlocked          = errors.New("cubebox_file_delete_blocked")
	ErrFileUploadInvalid          = errors.New("cubebox_file_upload_invalid")
	ErrFileConversationNotFound   = errors.New("cubebox_file_conversation_not_found")
)

type FileLink struct {
	LinkRole       string `json:"link_role"`
	ConversationID string `json:"conversation_id"`
	TurnID         string `json:"turn_id,omitempty"`
}

type FileRecord struct {
	FileID          string     `json:"file_id"`
	TenantID        string     `json:"tenant_id"`
	Filename        string     `json:"filename"`
	ContentType     string     `json:"content_type"`
	SizeBytes       int64      `json:"size_bytes"`
	SHA256          string     `json:"sha256"`
	StorageProvider string     `json:"storage_provider"`
	StorageKey      string     `json:"storage_key"`
	ScanStatus      string     `json:"scan_status"`
	UploadedBy      string     `json:"uploaded_by"`
	CreatedAt       string     `json:"created_at"`
	UpdatedAt       string     `json:"updated_at,omitempty"`
	Links           []FileLink `json:"links,omitempty"`

	ConversationID string `json:"conversation_id,omitempty"`
	FileName       string `json:"file_name,omitempty"`
	MediaType      string `json:"media_type,omitempty"`
	UploadedAt     string `json:"uploaded_at,omitempty"`
}

type FileObject struct {
	StorageProvider string `json:"storage_provider"`
	StorageKey      string `json:"storage_key"`
	Filename        string `json:"filename"`
	ContentType     string `json:"content_type"`
	SizeBytes       int64  `json:"size_bytes"`
	SHA256          string `json:"sha256"`
}

type FileCleanupJob struct {
	FileID          string `json:"file_id"`
	StorageProvider string `json:"storage_provider"`
	StorageKey      string `json:"storage_key"`
	CleanupReason   string `json:"cleanup_reason"`
	LastError       string `json:"last_error,omitempty"`
}

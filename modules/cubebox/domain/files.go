package domain

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

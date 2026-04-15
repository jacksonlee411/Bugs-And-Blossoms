package services

import (
	"context"
	"io"
	"strings"

	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
)

type FileRecord = cubeboxdomain.FileRecord

type FileStore interface {
	List(ctx context.Context, tenantID string, conversationID string) ([]FileRecord, error)
	Save(ctx context.Context, tenantID string, actorID string, conversationID string, filename string, mediaType string, body io.Reader) (FileRecord, error)
	Delete(ctx context.Context, tenantID string, fileID string) (bool, error)
	Healthy(ctx context.Context) error
}

type FileService struct {
	store FileStore
}

func NewFileService(store FileStore) *FileService {
	return &FileService{store: store}
}

func (s *FileService) ListFiles(ctx context.Context, tenantID string, conversationID string) ([]FileRecord, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.List(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(conversationID))
}

func (s *FileService) SaveFile(
	ctx context.Context,
	tenantID string,
	actorID string,
	conversationID string,
	filename string,
	mediaType string,
	body io.Reader,
) (FileRecord, error) {
	return s.store.Save(
		ctx,
		strings.TrimSpace(tenantID),
		strings.TrimSpace(actorID),
		strings.TrimSpace(conversationID),
		strings.TrimSpace(filename),
		strings.TrimSpace(mediaType),
		body,
	)
}

func (s *FileService) DeleteFile(ctx context.Context, tenantID string, fileID string) (bool, error) {
	if s == nil || s.store == nil {
		return false, nil
	}
	return s.store.Delete(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(fileID))
}

func (s *FileService) Healthy(ctx context.Context) error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Healthy(ctx)
}

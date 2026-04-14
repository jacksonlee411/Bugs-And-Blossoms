package services

import (
	"context"
	"io"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure"
)

type FileRecord = infrastructure.FileRecord

type FileService struct {
	store *infrastructure.LocalFileStore
}

func NewFileService(rootDir string) *FileService {
	return &FileService{store: infrastructure.NewLocalFileStore(rootDir)}
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

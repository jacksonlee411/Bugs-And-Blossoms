package services

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxsqlc "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/gen"
)

type FileRecord = cubeboxdomain.FileRecord
type FileLink = cubeboxdomain.FileLink
type FileObject = cubeboxdomain.FileObject
type FileCleanupJob = cubeboxdomain.FileCleanupJob

var (
	ErrFileUnavailable          = cubeboxdomain.ErrFileUnavailable
	ErrFileNotFound             = cubeboxdomain.ErrFileNotFound
	ErrFileDeleteBlocked        = cubeboxdomain.ErrFileDeleteBlocked
	ErrFileUploadInvalid        = cubeboxdomain.ErrFileUploadInvalid
	ErrFileConversationNotFound = cubeboxdomain.ErrFileConversationNotFound
)

type FileRepository interface {
	ListFiles(ctx context.Context, tenantID string, conversationID string, limit int32) ([]cubeboxsqlc.IamCubeboxFile, error)
	ListFileLinks(ctx context.Context, tenantID string, fileID string) ([]cubeboxsqlc.IamCubeboxFileLink, error)
	ListTenantFileLinks(ctx context.Context, tenantID string) ([]cubeboxsqlc.IamCubeboxFileLink, error)
	GetFile(ctx context.Context, tenantID string, fileID string) (cubeboxsqlc.IamCubeboxFile, error)
	ConversationExists(ctx context.Context, tenantID string, conversationID string) (bool, error)
	CreateFile(ctx context.Context, tenantID string, record FileObject, fileID string, actorID string, conversationID string, now time.Time) (cubeboxsqlc.IamCubeboxFile, []cubeboxsqlc.IamCubeboxFileLink, error)
	CountFileLinks(ctx context.Context, tenantID string, fileID string) (int64, error)
	DeleteFile(ctx context.Context, tenantID string, fileID string) (int64, error)
	InsertFileCleanupJob(ctx context.Context, tenantID string, job FileCleanupJob, now time.Time) (cubeboxsqlc.IamCubeboxFileCleanupJob, error)
	Healthy(ctx context.Context, tenantID string) error
}

type FileStore interface {
	List(ctx context.Context, tenantID string, conversationID string) ([]FileRecord, error)
	Save(ctx context.Context, tenantID string, actorID string, conversationID string, filename string, mediaType string, body io.Reader) (FileRecord, error)
	Delete(ctx context.Context, tenantID string, fileID string) (bool, error)
	Healthy(ctx context.Context) error
}

type FileObjectStore interface {
	SaveObject(ctx context.Context, tenantID string, fileID string, filename string, mediaType string, body io.Reader) (FileObject, error)
	DeleteObject(ctx context.Context, storageKey string) error
	Healthy(ctx context.Context) error
}

type FileService struct {
	legacyStore FileStore
	repo        FileRepository
	objectStore FileObjectStore
	nowFn       func() time.Time
}

func NewFileService(primary any, extras ...any) *FileService {
	svc := &FileService{
		nowFn: func() time.Time {
			return time.Now().UTC()
		},
	}
	if legacyStore, ok := primary.(FileStore); ok {
		svc.legacyStore = legacyStore
		return svc
	}
	if repo, ok := primary.(FileRepository); ok {
		svc.repo = repo
	}
	if len(extras) > 0 {
		if objectStore, ok := extras[0].(FileObjectStore); ok {
			svc.objectStore = objectStore
		}
	}
	return svc
}

func (s *FileService) ListFiles(ctx context.Context, tenantID string, conversationID string) ([]FileRecord, error) {
	tenantID = strings.TrimSpace(tenantID)
	conversationID = strings.TrimSpace(conversationID)
	if s == nil || s.repo == nil {
		if s != nil && s.legacyStore != nil {
			return s.legacyStore.List(ctx, tenantID, conversationID)
		}
		return nil, nil
	}
	items, err := s.repo.ListFiles(ctx, tenantID, conversationID, 200)
	if err != nil {
		return nil, err
	}

	linksByFile := map[string][]FileLink{}
	var rawLinks []cubeboxsqlc.IamCubeboxFileLink
	if conversationID == "" {
		rawLinks, err = s.repo.ListTenantFileLinks(ctx, tenantID)
	} else {
		for _, item := range items {
			fileLinks, fileErr := s.repo.ListFileLinks(ctx, tenantID, item.FileID)
			if fileErr != nil {
				return nil, fileErr
			}
			rawLinks = append(rawLinks, fileLinks...)
		}
	}
	if err != nil {
		return nil, err
	}
	for _, link := range rawLinks {
		linksByFile[link.FileID] = append(linksByFile[link.FileID], mapFileLink(link))
	}

	out := make([]FileRecord, 0, len(items))
	for _, item := range items {
		out = append(out, mapFileRecord(item, linksByFile[item.FileID]))
	}
	return out, nil
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
	tenantID = strings.TrimSpace(tenantID)
	actorID = strings.TrimSpace(actorID)
	conversationID = strings.TrimSpace(conversationID)
	filename = strings.TrimSpace(filename)
	mediaType = strings.TrimSpace(mediaType)
	if s == nil || s.repo == nil || s.objectStore == nil {
		if s != nil && s.legacyStore != nil {
			return s.legacyStore.Save(ctx, tenantID, actorID, conversationID, filename, mediaType, body)
		}
		return FileRecord{}, ErrFileUnavailable
	}
	if tenantID == "" || actorID == "" || filename == "" {
		return FileRecord{}, ErrFileUploadInvalid
	}
	if conversationID != "" {
		exists, err := s.repo.ConversationExists(ctx, tenantID, conversationID)
		if err != nil {
			return FileRecord{}, err
		}
		if !exists {
			return FileRecord{}, ErrFileConversationNotFound
		}
	}

	fileID := "file_" + uuid.NewString()
	object, err := s.objectStore.SaveObject(ctx, tenantID, fileID, filename, mediaType, body)
	if err != nil {
		if errors.Is(err, ErrFileUploadInvalid) {
			return FileRecord{}, err
		}
		return FileRecord{}, err
	}
	now := s.now()
	inserted, rawLinks, err := s.repo.CreateFile(ctx, tenantID, object, fileID, actorID, conversationID, now)
	if err != nil {
		if deleteErr := s.objectStore.DeleteObject(ctx, object.StorageKey); deleteErr != nil {
			s.compensateCleanup(ctx, tenantID, FileCleanupJob{
				FileID:          fileID,
				StorageProvider: object.StorageProvider,
				StorageKey:      object.StorageKey,
				CleanupReason:   "metadata_write_failed",
				LastError:       deleteErr.Error(),
			})
		}
		return FileRecord{}, err
	}

	links := make([]FileLink, 0, len(rawLinks))
	for _, link := range rawLinks {
		links = append(links, mapFileLink(link))
	}

	return mapFileRecord(inserted, links), nil
}

func (s *FileService) DeleteFile(ctx context.Context, tenantID string, fileID string) (bool, error) {
	tenantID = strings.TrimSpace(tenantID)
	fileID = strings.TrimSpace(fileID)
	if s == nil || s.repo == nil || s.objectStore == nil {
		if s != nil && s.legacyStore != nil {
			deleted, err := s.legacyStore.Delete(ctx, tenantID, fileID)
			if errors.Is(err, ErrFileNotFound) {
				return false, nil
			}
			return deleted, err
		}
		return false, nil
	}
	if tenantID == "" || fileID == "" {
		return false, ErrFileNotFound
	}

	record, err := s.repo.GetFile(ctx, tenantID, fileID)
	if err != nil {
		return false, ErrFileNotFound
	}
	linkCount, err := s.repo.CountFileLinks(ctx, tenantID, fileID)
	if err != nil {
		return false, err
	}
	if linkCount > 0 {
		return false, ErrFileDeleteBlocked
	}
	rows, err := s.repo.DeleteFile(ctx, tenantID, fileID)
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return false, ErrFileNotFound
	}
	if err := s.objectStore.DeleteObject(ctx, record.StorageKey); err != nil {
		s.compensateCleanup(ctx, tenantID, FileCleanupJob{
			FileID:          fileID,
			StorageProvider: record.StorageProvider,
			StorageKey:      record.StorageKey,
			CleanupReason:   "object_delete_failed",
			LastError:       err.Error(),
		})
	}
	return true, nil
}

func (s *FileService) Healthy(ctx context.Context) error {
	if s != nil && s.legacyStore != nil {
		return s.legacyStore.Healthy(ctx)
	}
	if s == nil {
		return nil
	}
	if s.repo == nil || s.objectStore == nil {
		return ErrFileUnavailable
	}
	if err := s.repo.Healthy(ctx, uuid.Nil.String()); err != nil {
		return err
	}
	return s.objectStore.Healthy(ctx)
}

func (s *FileService) now() time.Time {
	if s != nil && s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now().UTC()
}

func (s *FileService) compensateCleanup(ctx context.Context, tenantID string, job FileCleanupJob) {
	if s == nil || s.repo == nil {
		return
	}
	_, _ = s.repo.InsertFileCleanupJob(ctx, tenantID, job, s.now())
}

func mapFileRecord(item cubeboxsqlc.IamCubeboxFile, links []FileLink) FileRecord {
	record := FileRecord{
		FileID:          strings.TrimSpace(item.FileID),
		Filename:        strings.TrimSpace(item.FileName),
		ContentType:     strings.TrimSpace(item.MediaType),
		SizeBytes:       item.SizeBytes,
		SHA256:          strings.TrimSpace(item.Sha256),
		StorageProvider: strings.TrimSpace(item.StorageProvider),
		StorageKey:      strings.TrimSpace(item.StorageKey),
		ScanStatus:      strings.TrimSpace(item.ScanStatus),
		UploadedBy:      strings.TrimSpace(item.UploadedBy),
		CreatedAt:       item.UploadedAt.Time.UTC().Format(time.RFC3339Nano),
		UpdatedAt:       item.UpdatedAt.Time.UTC().Format(time.RFC3339Nano),
		Links:           links,
	}
	record.FileName = record.Filename
	record.MediaType = record.ContentType
	record.UploadedAt = record.CreatedAt
	if len(links) > 0 {
		record.ConversationID = strings.TrimSpace(links[0].ConversationID)
	}
	return record
}

func mapFileLink(link cubeboxsqlc.IamCubeboxFileLink) FileLink {
	return FileLink{
		LinkRole:       strings.TrimSpace(link.LinkRole),
		ConversationID: strings.TrimSpace(link.ConversationID),
		TurnID:         strings.TrimSpace(fileStringValue(link.TurnID)),
	}
}

func fileStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

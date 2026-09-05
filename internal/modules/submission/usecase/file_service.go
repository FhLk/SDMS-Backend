package usecase

import (
	"context"
	"errors"
	"io"
	"mime"
	"path/filepath"
	"strings"

	submissiondomain "sdms/internal/modules/submission/domain"
	topicdomain "sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

const DefaultMaxUploadSize int64 = 20 * 1024 * 1024

type SubmissionFileStorage interface {
	Save(ctx context.Context, relativePath string, src io.Reader) error
	Open(ctx context.Context, relativePath string) (io.ReadCloser, error)
	Delete(ctx context.Context, relativePath string) error
}

type UploadSubmissionFileInput struct {
	FieldUID         uuid.UUID
	OriginalFilename string
	ContentType      string
	Size             int64
	Reader           io.Reader
}

type SubmissionFileService struct {
	submissionRepo submissiondomain.SubmissionRepository
	fileRepo       submissiondomain.SubmissionFileRepository
	fieldRepo      topicdomain.FieldRepository
	storage        SubmissionFileStorage
	maxUploadSize  int64
}

func NewSubmissionFileService(
	submissionRepo submissiondomain.SubmissionRepository,
	fileRepo submissiondomain.SubmissionFileRepository,
	fieldRepo topicdomain.FieldRepository,
	storage SubmissionFileStorage,
	maxUploadSize int64,
) *SubmissionFileService {
	if maxUploadSize <= 0 {
		maxUploadSize = DefaultMaxUploadSize
	}

	return &SubmissionFileService{
		submissionRepo: submissionRepo,
		fileRepo:       fileRepo,
		fieldRepo:      fieldRepo,
		storage:        storage,
		maxUploadSize:  maxUploadSize,
	}
}

func (s *SubmissionFileService) Upload(
	ctx context.Context,
	topicUID uuid.UUID,
	submissionUID uuid.UUID,
	input UploadSubmissionFileInput,
) (*submissiondomain.SubmissionFile, error) {
	if input.Reader == nil {
		return nil, submissiondomain.ErrSubmissionFileEmpty
	}
	if input.Size <= 0 {
		return nil, submissiondomain.ErrSubmissionFileEmpty
	}
	if input.Size > s.maxUploadSize {
		return nil, submissiondomain.ErrSubmissionFileTooLarge
	}

	if _, err := s.submissionRepo.FindByIDAndTopicID(ctx, submissionUID, topicUID); err != nil {
		return nil, err
	}

	field, err := s.fieldRepo.FindByID(ctx, input.FieldUID)
	if err != nil {
		return nil, err
	}
	if field.TopicUID != topicUID {
		return nil, submissiondomain.NewFieldError(
			submissiondomain.ErrSubmissionFileFieldTopicMismatch,
			field.UID,
			field.Label,
		)
	}
	if field.Type != topicdomain.FieldTypeFile {
		return nil, submissiondomain.NewFieldError(
			submissiondomain.ErrSubmissionFileFieldNotFile,
			field.UID,
			field.Label,
		)
	}

	_, err = s.fileRepo.FindBySubmissionIDAndFieldID(ctx, submissionUID, field.UID)
	switch {
	case err == nil:
		return nil, submissiondomain.NewFieldError(
			submissiondomain.ErrSubmissionFileAlreadyExists,
			field.UID,
			field.Label,
		)
	case !errors.Is(err, submissiondomain.ErrSubmissionFileNotFound):
		return nil, err
	}

	originalFilename := strings.TrimSpace(input.OriginalFilename)
	originalFilename = strings.ReplaceAll(originalFilename, "\\", "/")
	originalFilename = filepath.Base(originalFilename)
	if originalFilename == "" || originalFilename == "." {
		return nil, submissiondomain.ErrSubmissionFileNameRequired
	}

	ext := strings.ToLower(filepath.Ext(originalFilename))
	if !isAllowedUploadExtension(ext) {
		return nil, submissiondomain.ErrSubmissionFileTypeNotAllowed
	}

	storedFilename := uuid.NewString() + ext
	storagePath := filepath.Join("submissions", submissionUID.String(), storedFilename)
	contentType := strings.TrimSpace(input.ContentType)
	if inferred := mime.TypeByExtension(ext); inferred != "" {
		contentType = inferred
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := s.storage.Save(ctx, storagePath, input.Reader); err != nil {
		return nil, err
	}

	file, err := submissiondomain.NewSubmissionFile(
		submissionUID,
		field.UID,
		originalFilename,
		storedFilename,
		storagePath,
		contentType,
		input.Size,
	)
	if err != nil {
		_ = s.storage.Delete(ctx, storagePath)
		return nil, err
	}

	if err := s.fileRepo.Create(ctx, file); err != nil {
		_ = s.storage.Delete(ctx, storagePath)
		return nil, err
	}
	return file, nil
}

func (s *SubmissionFileService) FindAll(
	ctx context.Context,
	topicUID uuid.UUID,
	submissionUID uuid.UUID,
) ([]submissiondomain.SubmissionFile, error) {
	if _, err := s.submissionRepo.FindByIDAndTopicID(ctx, submissionUID, topicUID); err != nil {
		return nil, err
	}
	return s.fileRepo.FindAllBySubmissionID(ctx, submissionUID)
}

func (s *SubmissionFileService) FindByID(
	ctx context.Context,
	fileUID uuid.UUID,
) (*submissiondomain.SubmissionFile, error) {
	return s.fileRepo.FindByID(ctx, fileUID)
}

func (s *SubmissionFileService) Open(
	ctx context.Context,
	fileUID uuid.UUID,
) (*submissiondomain.SubmissionFile, io.ReadCloser, error) {
	file, err := s.fileRepo.FindByID(ctx, fileUID)
	if err != nil {
		return nil, nil, err
	}

	reader, err := s.storage.Open(ctx, file.StoragePath)
	if err != nil {
		return nil, nil, err
	}
	return file, reader, nil
}

func (s *SubmissionFileService) Delete(
	ctx context.Context,
	fileUID uuid.UUID,
) error {
	file, err := s.fileRepo.FindByID(ctx, fileUID)
	if err != nil {
		return err
	}

	if err := s.fileRepo.Delete(ctx, fileUID); err != nil {
		return err
	}
	return s.storage.Delete(ctx, file.StoragePath)
}

func isAllowedUploadExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".pdf", ".xls", ".xlsx", ".png", ".jpg", ".jpeg", ".mp4", "mov":
		return true
	default:
		return false
	}
}

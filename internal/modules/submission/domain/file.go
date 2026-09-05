package domain

import (
	"time"

	"github.com/google/uuid"
)

type SubmissionFile struct {
	UID              uuid.UUID
	SubmissionUID    uuid.UUID
	FieldUID         uuid.UUID
	OriginalFilename string
	StoredFilename   string
	StoragePath      string
	ContentType      string
	Size             int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewSubmissionFile(
	submissionUID uuid.UUID,
	fieldUID uuid.UUID,
	originalFilename string,
	storedFilename string,
	storagePath string,
	contentType string,
	size int64,
) (*SubmissionFile, error) {
	if submissionUID == uuid.Nil {
		return nil, ErrSubmissionFileSubmissionUIDRequired
	}
	if fieldUID == uuid.Nil {
		return nil, ErrSubmissionFileFieldUIDRequired
	}
	if originalFilename == "" {
		return nil, ErrSubmissionFileNameRequired
	}
	if storedFilename == "" || storagePath == "" {
		return nil, ErrSubmissionFileStoragePathRequired
	}
	if size <= 0 {
		return nil, ErrSubmissionFileEmpty
	}

	return &SubmissionFile{
		UID:              uuid.New(),
		SubmissionUID:    submissionUID,
		FieldUID:         fieldUID,
		OriginalFilename: originalFilename,
		StoredFilename:   storedFilename,
		StoragePath:      storagePath,
		ContentType:      contentType,
		Size:             size,
	}, nil
}

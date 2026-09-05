package postgres

import (
	"time"

	"sdms/internal/modules/submission/domain"
	topicpostgres "sdms/internal/modules/topic/repository/postgres"

	"github.com/google/uuid"
)

type SubmissionFileModel struct {
	UID              uuid.UUID                     `gorm:"type:uuid;primaryKey"`
	SubmissionUID    uuid.UUID                     `gorm:"type:uuid;not null;uniqueIndex:idx_submission_file_field;index"`
	FieldUID         uuid.UUID                     `gorm:"type:uuid;not null;uniqueIndex:idx_submission_file_field;index"`
	Submission       SubmissionModel               `gorm:"foreignKey:SubmissionUID;references:UID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Field            topicpostgres.TopicFieldModel `gorm:"foreignKey:FieldUID;references:UID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	OriginalFilename string                        `gorm:"type:text;not null"`
	StoredFilename   string                        `gorm:"type:text;not null;uniqueIndex"`
	StoragePath      string                        `gorm:"type:text;not null;uniqueIndex"`
	ContentType      string                        `gorm:"type:varchar(255);not null"`
	Size             int64                         `gorm:"not null"`
	CreatedAt        time.Time                     `gorm:"not null"`
	UpdatedAt        time.Time                     `gorm:"not null"`
}

func (SubmissionFileModel) TableName() string {
	return "submission_files"
}

func fromFileDomain(file domain.SubmissionFile) SubmissionFileModel {
	return SubmissionFileModel{
		UID:              file.UID,
		SubmissionUID:    file.SubmissionUID,
		FieldUID:         file.FieldUID,
		OriginalFilename: file.OriginalFilename,
		StoredFilename:   file.StoredFilename,
		StoragePath:      file.StoragePath,
		ContentType:      file.ContentType,
		Size:             file.Size,
		CreatedAt:        file.CreatedAt,
		UpdatedAt:        file.UpdatedAt,
	}
}

func toFileDomain(model SubmissionFileModel) domain.SubmissionFile {
	return domain.SubmissionFile{
		UID:              model.UID,
		SubmissionUID:    model.SubmissionUID,
		FieldUID:         model.FieldUID,
		OriginalFilename: model.OriginalFilename,
		StoredFilename:   model.StoredFilename,
		StoragePath:      model.StoragePath,
		ContentType:      model.ContentType,
		Size:             model.Size,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

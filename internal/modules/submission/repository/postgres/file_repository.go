package postgres

import (
	"context"
	"errors"

	"sdms/internal/modules/submission/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type submissionFileRepository struct {
	db *gorm.DB
}

func NewSubmissionFileRepository(db *gorm.DB) domain.SubmissionFileRepository {
	return &submissionFileRepository{db: db}
}

func (r *submissionFileRepository) Create(
	ctx context.Context,
	file *domain.SubmissionFile,
) error {
	model := fromFileDomain(*file)

	if err := r.db.WithContext(ctx).
		Omit("Submission", "Field").
		Create(&model).Error; err != nil {
		return err
	}

	file.CreatedAt = model.CreatedAt
	file.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *submissionFileRepository) FindAllBySubmissionID(
	ctx context.Context,
	submissionUID uuid.UUID,
) ([]domain.SubmissionFile, error) {
	var models []SubmissionFileModel

	if err := r.db.WithContext(ctx).
		Where("submission_uid = ?", submissionUID).
		Order("created_at ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	files := make([]domain.SubmissionFile, 0, len(models))
	for _, model := range models {
		files = append(files, toFileDomain(model))
	}
	return files, nil
}

func (r *submissionFileRepository) FindByID(
	ctx context.Context,
	fileUID uuid.UUID,
) (*domain.SubmissionFile, error) {
	var model SubmissionFileModel

	err := r.db.WithContext(ctx).
		First(&model, "uid = ?", fileUID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSubmissionFileNotFound
	}
	if err != nil {
		return nil, err
	}

	file := toFileDomain(model)
	return &file, nil
}

func (r *submissionFileRepository) FindBySubmissionIDAndFieldID(
	ctx context.Context,
	submissionUID uuid.UUID,
	fieldUID uuid.UUID,
) (*domain.SubmissionFile, error) {
	var model SubmissionFileModel

	err := r.db.WithContext(ctx).
		Where("submission_uid = ? AND field_uid = ?", submissionUID, fieldUID).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSubmissionFileNotFound
	}
	if err != nil {
		return nil, err
	}

	file := toFileDomain(model)
	return &file, nil
}

func (r *submissionFileRepository) Delete(
	ctx context.Context,
	fileUID uuid.UUID,
) error {
	result := r.db.WithContext(ctx).
		Delete(&SubmissionFileModel{}, "uid = ?", fileUID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrSubmissionFileNotFound
	}
	return nil
}

package postgres

import (
	"context"
	"errors"

	"sdms/internal/modules/submission/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type repository struct {
	db *gorm.DB
}

func NewSubmissionRepository(db *gorm.DB) domain.SubmissionRepository {
	return &repository{
		db: db,
	}
}

func (r *repository) Create(
	ctx context.Context,
	submission *domain.Submission,
) error {
	return r.db.WithContext(ctx).Transaction(
		func(tx *gorm.DB) error {
			model := fromDomain(*submission)

			values := model.Values
			model.Values = nil

			if err := tx.Create(&model).Error; err != nil {
				return err
			}

			if len(values) > 0 {
				if err := tx.Create(&values).Error; err != nil {
					return err
				}
			}

			submission.CreatedAt = model.CreatedAt
			submission.UpdatedAt = model.UpdatedAt

			for i := range submission.Values {
				submission.Values[i].CreatedAt =
					values[i].CreatedAt

				submission.Values[i].UpdatedAt =
					values[i].UpdatedAt
			}

			return nil
		},
	)
}

func (r *repository) FindAllByTopicID(
	ctx context.Context,
	topicUID uuid.UUID,
) ([]domain.Submission, error) {
	var models []SubmissionModel

	if err := r.db.
		WithContext(ctx).
		Where("topic_uid = ?", topicUID).
		Order("created_at DESC").
		Find(&models).
		Error; err != nil {
		return nil, err
	}

	submissions := make(
		[]domain.Submission,
		0,
		len(models),
	)

	for _, model := range models {
		submissions = append(
			submissions,
			toDomain(model),
		)
	}

	return submissions, nil
}

func (r *repository) FindByIDAndTopicID(
	ctx context.Context,
	submissionUID uuid.UUID,
	topicUID uuid.UUID,
) (*domain.Submission, error) {
	var model SubmissionModel

	err := r.db.
		WithContext(ctx).
		Preload("Values").
		Where(
			"uid = ? AND topic_uid = ?",
			submissionUID,
			topicUID,
		).
		First(&model).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSubmissionNotFound
	}

	if err != nil {
		return nil, err
	}

	submission := toDomain(model)

	return &submission, nil
}

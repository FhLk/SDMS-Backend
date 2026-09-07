package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"sdms/internal/modules/submission/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type repository struct{ db *gorm.DB }

func NewSubmissionRepository(db *gorm.DB) domain.SubmissionRepository { return &repository{db: db} }

func (r *repository) Create(ctx context.Context, submission *domain.Submission) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := fromDomain(*submission)
		values := model.Values
		model.Values = nil
		if err := tx.Omit("Topic", "Submitter", "Files").Create(&model).Error; err != nil {
			return err
		}
		if len(values) > 0 {
			if err := tx.Omit("Field").Create(&values).Error; err != nil {
				return err
			}
		}
		submission.CreatedAt = model.CreatedAt
		submission.UpdatedAt = model.UpdatedAt
		for i := range submission.Values {
			submission.Values[i].CreatedAt = values[i].CreatedAt
			submission.Values[i].UpdatedAt = values[i].UpdatedAt
		}
		return nil
	})
}

func (r *repository) UpdateValues(ctx context.Context, submission *domain.Submission) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		snapshot, _ := json.Marshal(submission.FormSnapshot)
		result := tx.Model(&SubmissionModel{}).Where("uid = ?", submission.UID).Updates(map[string]interface{}{
			"form_version":  submission.FormVersion,
			"form_snapshot": string(snapshot),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrSubmissionNotFound
		}
		if err := tx.Where("submission_uid = ?", submission.UID).Delete(&SubmissionValueModel{}).Error; err != nil {
			return err
		}
		values := make([]SubmissionValueModel, 0, len(submission.Values))
		for i := range submission.Values {
			submission.Values[i].UID = uuid.New()
			submission.Values[i].SubmissionUID = submission.UID
			values = append(values, fromValueDomain(submission.Values[i]))
		}
		if len(values) > 0 {
			if err := tx.Omit("Field").Create(&values).Error; err != nil {
				return err
			}
		}
		var updated SubmissionModel
		if err := tx.First(&updated, "uid = ?", submission.UID).Error; err != nil {
			return err
		}
		submission.UpdatedAt = updated.UpdatedAt
		return nil
	})
}

func (r *repository) Delete(ctx context.Context, submissionUID uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&SubmissionModel{}, "uid = ?", submissionUID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrSubmissionNotFound
	}
	return nil
}

func (r *repository) FindAllByTopicID(ctx context.Context, topicUID uuid.UUID) ([]domain.Submission, error) {
	return r.findAll(r.db.WithContext(ctx).Where("topic_uid = ?", topicUID))
}

func (r *repository) FindAllByTopicIDAndSubmittedBy(ctx context.Context, topicUID, submittedBy uuid.UUID) ([]domain.Submission, error) {
	return r.findAll(r.db.WithContext(ctx).Where("topic_uid = ? AND submitted_by = ?", topicUID, submittedBy))
}

func (r *repository) findAll(query *gorm.DB) ([]domain.Submission, error) {
	var models []SubmissionModel
	if err := query.Preload("Values.Field").Preload("Files").Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Submission, 0, len(models))
	for _, model := range models {
		result = append(result, toDomain(model))
	}
	return result, nil
}

func (r *repository) FindByIDAndTopicID(ctx context.Context, submissionUID, topicUID uuid.UUID) (*domain.Submission, error) {
	return r.findOne(r.db.WithContext(ctx).Where("uid = ? AND topic_uid = ?", submissionUID, topicUID))
}

func (r *repository) FindByIDAndTopicIDAndSubmittedBy(ctx context.Context, submissionUID, topicUID, submittedBy uuid.UUID) (*domain.Submission, error) {
	return r.findOne(r.db.WithContext(ctx).Where("uid = ? AND topic_uid = ? AND submitted_by = ?", submissionUID, topicUID, submittedBy))
}

func (r *repository) findOne(query *gorm.DB) (*domain.Submission, error) {
	var model SubmissionModel
	err := query.Preload("Values.Field").Preload("Files").First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSubmissionNotFound
	}
	if err != nil {
		return nil, err
	}
	submission := toDomain(model)
	return &submission, nil
}

func (r *repository) HasAnyByTopicID(ctx context.Context, topicUID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&SubmissionModel{}).Where("topic_uid = ?", topicUID).Limit(1).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

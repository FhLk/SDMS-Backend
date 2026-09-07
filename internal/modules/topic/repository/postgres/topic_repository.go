package postgres

import (
	"context"
	"errors"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TopicRepository struct{ db *gorm.DB }

func NewTopicRepository(db *gorm.DB) *TopicRepository { return &TopicRepository{db: db} }

func (r *TopicRepository) Create(ctx context.Context, topic *domain.Topic) error {
	model := toModel(*topic)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return err
	}
	*topic = toDomain(model)
	return nil
}

func (r *TopicRepository) FindAll(ctx context.Context) ([]domain.Topic, error) {
	return r.findAll(r.db.WithContext(ctx))
}

func (r *TopicRepository) FindAllByAcademicYear(ctx context.Context, academicYear string) ([]domain.Topic, error) {
	return r.findAll(r.db.WithContext(ctx).Where("academic_year = ?", academicYear))
}

func (r *TopicRepository) findAll(query *gorm.DB) ([]domain.Topic, error) {
	var models []TopicModel
	if err := query.Order("academic_year DESC").Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	topics := make([]domain.Topic, len(models))
	for i, model := range models {
		topics[i] = toDomain(model)
	}
	return topics, nil
}

func (r *TopicRepository) FindByID(ctx context.Context, topicID uuid.UUID) (*domain.Topic, error) {
	var model TopicModel
	err := r.db.WithContext(ctx).Where("uid = ?", topicID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTopicNotFound
	}
	if err != nil {
		return nil, err
	}
	topic := toDomain(model)
	return &topic, nil
}

func (r *TopicRepository) Update(ctx context.Context, topic *domain.Topic) error {
	model := toModel(*topic)
	result := r.db.WithContext(ctx).Model(&TopicModel{}).Where("uid = ?", model.UID).Updates(map[string]interface{}{
		"academic_year": model.AcademicYear,
		"name":          model.Name,
		"description":   model.Description,
		"is_active":     model.IsActive,
		"form_version":  model.FormVersion,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrTopicNotFound
	}
	return nil
}

func (r *TopicRepository) Delete(ctx context.Context, topicID uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("uid = ?", topicID).Delete(&TopicModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrTopicNotFound
	}
	return nil
}

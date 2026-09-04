package postgres

import (
	"context"
	"errors"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fieldRepository struct {
	db *gorm.DB
}

func NewFieldRepository(db *gorm.DB) domain.FieldRepository {
	return &fieldRepository{
		db: db,
	}
}

func (r *fieldRepository) Create(
	ctx context.Context,
	field *domain.TopicField,
) error {
	model := toTopicFieldModel(*field)

	if err := r.db.
		WithContext(ctx).
		Create(&model).
		Error; err != nil {
		return err
	}

	field.CreatedAt = model.CreatedAt
	field.UpdatedAt = model.UpdatedAt

	return nil
}

func (r *fieldRepository) FindAllByTopicID(ctx context.Context, topicUID uuid.UUID) ([]domain.TopicField, error) {
	var models []TopicFieldModel

	if err := r.db.
		WithContext(ctx).
		Where("topic_uid = ?", topicUID).
		Order("position ASC").
		Order("created_at ASC").
		Find(&models).
		Error; err != nil {
		return nil, err
	}

	fields := make([]domain.TopicField, 0, len(models))

	for _, model := range models {
		fields = append(
			fields,
			toTopicFieldDomain(model),
		)
	}

	return fields, nil
}

func (r *fieldRepository) FindByID(ctx context.Context, fieldUID uuid.UUID) (*domain.TopicField, error) {
	var model TopicFieldModel

	err := r.db.
		WithContext(ctx).
		First(&model, "uid = ?", fieldUID).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTopicFieldNotFound
	}

	if err != nil {
		return nil, err
	}

	field := toTopicFieldDomain(model)

	return &field, nil
}

func (r *fieldRepository) Update(
	ctx context.Context,
	field *domain.TopicField,
) error {
	model := toTopicFieldModel(*field)

	if err := r.db.
		WithContext(ctx).
		Save(&model).
		Error; err != nil {
		return err
	}

	field.UpdatedAt = model.UpdatedAt

	return nil
}

func (r *fieldRepository) Delete(
	ctx context.Context,
	fieldUID uuid.UUID,
) error {
	return r.db.
		WithContext(ctx).
		Delete(
			&TopicFieldModel{},
			"uid = ?",
			fieldUID,
		).
		Error
}

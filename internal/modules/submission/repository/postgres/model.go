package postgres

import (
	"time"

	"sdms/internal/modules/submission/domain"
	topicpostgres "sdms/internal/modules/topic/repository/postgres"
	userpostgres "sdms/internal/modules/user/repository/postgres"

	"github.com/google/uuid"
)

type SubmissionModel struct {
	UID         uuid.UUID                `gorm:"type:uuid;primaryKey"`
	TopicUID    uuid.UUID                `gorm:"type:uuid;not null;index"`
	SubmittedBy uuid.UUID                `gorm:"type:uuid;not null;index"`
	Topic       topicpostgres.TopicModel `gorm:"foreignKey:TopicUID;references:UID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Submitter   userpostgres.UserModel   `gorm:"foreignKey:SubmittedBy;references:UID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Values      []SubmissionValueModel   `gorm:"foreignKey:SubmissionUID;references:UID;constraint:OnDelete:CASCADE;"`
	CreatedAt   time.Time                `gorm:"not null"`
	UpdatedAt   time.Time                `gorm:"not null"`
}

func (SubmissionModel) TableName() string {
	return "submissions"
}

type SubmissionValueModel struct {
	UID           uuid.UUID                     `gorm:"type:uuid;primaryKey"`
	SubmissionUID uuid.UUID                     `gorm:"type:uuid;not null;uniqueIndex:idx_submission_field"`
	FieldUID      uuid.UUID                     `gorm:"type:uuid;not null;uniqueIndex:idx_submission_field;index"`
	Field         topicpostgres.TopicFieldModel `gorm:"foreignKey:FieldUID;references:UID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	TextValue     *string                       `gorm:"type:text"`
	NumberValue   *float64                      `gorm:"type:double precision"`
	DateValue     *time.Time                    `gorm:"type:date"`
	CreatedAt     time.Time                     `gorm:"not null"`
	UpdatedAt     time.Time                     `gorm:"not null"`
}

func (SubmissionValueModel) TableName() string {
	return "submission_values"
}

func fromDomain(submission domain.Submission) SubmissionModel {
	values := make(
		[]SubmissionValueModel,
		0,
		len(submission.Values),
	)

	for _, value := range submission.Values {
		values = append(
			values,
			fromValueDomain(value),
		)
	}

	return SubmissionModel{
		UID:         submission.UID,
		TopicUID:    submission.TopicUID,
		SubmittedBy: submission.SubmittedBy,
		Values:      values,
		CreatedAt:   submission.CreatedAt,
		UpdatedAt:   submission.UpdatedAt,
	}
}

func fromValueDomain(
	value domain.SubmissionValue,
) SubmissionValueModel {
	return SubmissionValueModel{
		UID:           value.UID,
		SubmissionUID: value.SubmissionUID,
		FieldUID:      value.FieldUID,
		TextValue:     value.TextValue,
		NumberValue:   value.NumberValue,
		DateValue:     value.DateValue,
		CreatedAt:     value.CreatedAt,
		UpdatedAt:     value.UpdatedAt,
	}
}

func toDomain(model SubmissionModel) domain.Submission {
	values := make(
		[]domain.SubmissionValue,
		0,
		len(model.Values),
	)

	for _, value := range model.Values {
		values = append(
			values,
			toValueDomain(value),
		)
	}

	return domain.Submission{
		UID:         model.UID,
		TopicUID:    model.TopicUID,
		SubmittedBy: model.SubmittedBy,
		Values:      values,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

func toValueDomain(
	model SubmissionValueModel,
) domain.SubmissionValue {
	return domain.SubmissionValue{
		UID:            model.UID,
		SubmissionUID:  model.SubmissionUID,
		FieldUID:       model.FieldUID,
		FieldLabel:     model.Field.Label,
		FieldType:      model.Field.Type,
		FieldIsPreview: model.Field.IsPreview,
		FieldPosition:  model.Field.Position,
		TextValue:      model.TextValue,
		NumberValue:    model.NumberValue,
		DateValue:      model.DateValue,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}

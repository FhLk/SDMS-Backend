package usecase

import (
	"context"
	"errors"
	"testing"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

func TestPhase1TopicCannotBeDeletedAfterSubmission(t *testing.T) {
	topicUID := uuid.New()
	service := NewTopicService(
		&topicRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) {
			return &domain.Topic{UID: topicUID, AcademicYear: "2569", Name: "งาน", FormVersion: 1}, nil
		}},
		&fieldRepositoryStub{},
		&submissionLookupStub{hasAnyFn: func(context.Context, uuid.UUID) (bool, error) { return true, nil }},
	)

	if err := service.Delete(context.Background(), topicUID); !errors.Is(err, domain.ErrTopicDeleteLocked) {
		t.Fatalf("Delete() error = %v, want ErrTopicDeleteLocked", err)
	}
}

func TestPhase1RequiredFieldCannotBeAddedAfterSubmission(t *testing.T) {
	topicUID := uuid.New()
	service := NewTopicService(
		&topicRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) {
			return &domain.Topic{UID: topicUID, AcademicYear: "2569", Name: "งาน", FormVersion: 1}, nil
		}},
		&fieldRepositoryStub{},
		&submissionLookupStub{hasAnyFn: func(context.Context, uuid.UUID) (bool, error) { return true, nil }},
	)

	_, err := service.CreateField(context.Background(), topicUID, CreateFieldInput{
		Label: "หลักฐานเพิ่ม", Type: domain.FieldTypeFile, Required: true,
	})
	if !errors.Is(err, domain.ErrTopicRequiredFieldAddLocked) {
		t.Fatalf("CreateField() error = %v, want ErrTopicRequiredFieldAddLocked", err)
	}
}

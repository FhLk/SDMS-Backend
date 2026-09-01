package usecase

import (
	"context"
	"errors"
	"testing"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

type fakeRepository struct {
	createFn   func(ctx context.Context, topic *domain.Topic) error
	findAllFn  func(ctx context.Context) ([]domain.Topic, error)
	findByIDFn func(ctx context.Context, id uuid.UUID) (*domain.Topic, error)
	updateFn   func(ctx context.Context, topic *domain.Topic) error
	deleteFn   func(ctx context.Context, id uuid.UUID) error
}

func (f *fakeRepository) Create(
	ctx context.Context,
	topic *domain.Topic,
) error {
	if f.createFn != nil {
		return f.createFn(ctx, topic)
	}

	return nil
}

func (f *fakeRepository) FindAll(
	ctx context.Context,
) ([]domain.Topic, error) {
	if f.findAllFn != nil {
		return f.findAllFn(ctx)
	}

	return nil, nil
}

func (f *fakeRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*domain.Topic, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, id)
	}

	return nil, nil
}

func (f *fakeRepository) Update(
	ctx context.Context,
	topic *domain.Topic,
) error {
	if f.updateFn != nil {
		return f.updateFn(ctx, topic)
	}

	return nil
}

func (f *fakeRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}

	return nil
}

func TestService_Create_Success(t *testing.T) {
	repo := &fakeRepository{
		createFn: func(
			ctx context.Context,
			topic *domain.Topic,
		) error {

			if topic.UID == uuid.Nil {
				t.Error("expected topic ID to be generated")
			}

			if topic.Name != "งานวิชาการ" {
				t.Errorf(
					"expected name %q, got %q",
					"งานวิชาการ",
					topic.Name,
				)
			}

			if topic.Description != "เอกสารงานวิชาการ" {
				t.Errorf(
					"expected description %q, got %q",
					"เอกสารงานวิชาการ",
					topic.Description,
				)
			}

			if !topic.IsActive {
				t.Error("expected topic to be active")
			}

			return nil
		},
	}

	service := NewService(repo)

	topic, err := service.Create(
		context.Background(),
		"งานวิชาการ",
		"เอกสารงานวิชาการ",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topic == nil {
		t.Fatal("expected topic, got nil")
	}

	if topic.UID == uuid.Nil {
		t.Error("expected UUID to be generated")
	}

	if topic.Name != "งานวิชาการ" {
		t.Errorf(
			"expected %q, got %q",
			"งานวิชาการ",
			topic.Name,
		)
	}

	if !topic.IsActive {
		t.Error("expected IsActive to be true")
	}
}

func TestService_Create_TrimWhitespace(t *testing.T) {
	repo := &fakeRepository{}

	service := NewService(repo)

	topic, err := service.Create(
		context.Background(),
		"   งานวิชาการ   ",
		"   เอกสารงานวิชาการ   ",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topic.Name != "งานวิชาการ" {
		t.Errorf(
			"expected trimmed name %q, got %q",
			"งานวิชาการ",
			topic.Name,
		)
	}

	if topic.Description != "เอกสารงานวิชาการ" {
		t.Errorf(
			"expected trimmed description %q, got %q",
			"เอกสารงานวิชาการ",
			topic.Description,
		)
	}
}

func TestService_Create_EmptyName(t *testing.T) {
	createCalled := false

	repo := &fakeRepository{
		createFn: func(
			ctx context.Context,
			topic *domain.Topic,
		) error {

			createCalled = true
			return nil
		},
	}

	service := NewService(repo)

	topic, err := service.Create(
		context.Background(),
		"   ",
		"description",
	)

	if !errors.Is(err, domain.ErrTopicNameEmpty) {
		t.Errorf(
			"expected ErrTopicNameEmpty, got %v",
			err,
		)
	}

	if topic != nil {
		t.Errorf(
			"expected nil topic, got %+v",
			topic,
		)
	}

	if createCalled {
		t.Error(
			"repository Create should not be called when name is empty",
		)
	}
}

func TestService_Create_RepositoryError(t *testing.T) {
	expectedErr := errors.New("database unavailable")

	repo := &fakeRepository{
		createFn: func(
			ctx context.Context,
			topic *domain.Topic,
		) error {

			return expectedErr
		},
	}

	service := NewService(repo)

	topic, err := service.Create(
		context.Background(),
		"งานวิชาการ",
		"description",
	)

	if !errors.Is(err, expectedErr) {
		t.Errorf(
			"expected error %v, got %v",
			expectedErr,
			err,
		)
	}

	if topic != nil {
		t.Errorf(
			"expected nil topic, got %+v",
			topic,
		)
	}
}

func TestService_FindAll_Success(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()

	repo := &fakeRepository{
		findAllFn: func(
			ctx context.Context,
		) ([]domain.Topic, error) {

			return []domain.Topic{
				{
					UID:      id1,
					Name:     "งานวิชาการ",
					IsActive: true,
				},
				{
					UID:      id2,
					Name:     "งานบุคคล",
					IsActive: true,
				},
			}, nil
		},
	}

	service := NewService(repo)

	topics, err := service.FindAll(
		context.Background(),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(topics) != 2 {
		t.Fatalf(
			"expected 2 topics, got %d",
			len(topics),
		)
	}

	if topics[0].UID != id1 {
		t.Errorf(
			"expected ID %s, got %s",
			id1,
			topics[0].UID,
		)
	}

	if topics[1].Name != "งานบุคคล" {
		t.Errorf(
			"expected %q, got %q",
			"งานบุคคล",
			topics[1].Name,
		)
	}
}

func TestService_FindAll_RepositoryError(t *testing.T) {
	expectedErr := errors.New("database error")

	repo := &fakeRepository{
		findAllFn: func(
			ctx context.Context,
		) ([]domain.Topic, error) {

			return nil, expectedErr
		},
	}

	service := NewService(repo)

	topics, err := service.FindAll(
		context.Background(),
	)

	if !errors.Is(err, expectedErr) {
		t.Errorf(
			"expected %v, got %v",
			expectedErr,
			err,
		)
	}

	if topics != nil {
		t.Errorf(
			"expected nil topics, got %+v",
			topics,
		)
	}
}

func TestService_FindByID_Success(t *testing.T) {
	topicID := uuid.New()

	repo := &fakeRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*domain.Topic, error) {

			if id != topicID {
				t.Errorf(
					"expected ID %s, got %s",
					topicID,
					id,
				)
			}

			return &domain.Topic{
				UID:      id,
				Name:     "งานวิชาการ",
				IsActive: true,
			}, nil
		},
	}

	service := NewService(repo)

	topic, err := service.FindByID(
		context.Background(),
		topicID,
	)

	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if topic == nil {
		t.Fatal("expected topic, got nil")
	}

	if topic.UID != topicID {
		t.Errorf(
			"expected ID %s, got %s",
			topicID,
			topic.UID,
		)
	}
}

func TestService_FindByID_NotFound(t *testing.T) {
	topicID := uuid.New()

	repo := &fakeRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*domain.Topic, error) {

			return nil, domain.ErrTopicNotFound
		},
	}

	service := NewService(repo)

	topic, err := service.FindByID(
		context.Background(),
		topicID,
	)

	if !errors.Is(err, domain.ErrTopicNotFound) {
		t.Errorf(
			"expected ErrTopicNotFound, got %v",
			err,
		)
	}

	if topic != nil {
		t.Errorf(
			"expected nil topic, got %+v",
			topic,
		)
	}
}

func TestService_Update_Success(t *testing.T) {
	topicID := uuid.New()

	existingTopic := &domain.Topic{
		UID:         topicID,
		Name:        "ชื่อเดิม",
		Description: "รายละเอียดเดิม",
		IsActive:    true,
	}

	updateCalled := false

	repo := &fakeRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*domain.Topic, error) {

			return existingTopic, nil
		},

		updateFn: func(
			ctx context.Context,
			topic *domain.Topic,
		) error {

			updateCalled = true

			if topic.UID != topicID {
				t.Errorf(
					"expected ID %s, got %s",
					topicID,
					topic.UID,
				)
			}

			if topic.Name != "ชื่อใหม่" {
				t.Errorf(
					"expected name %q, got %q",
					"ชื่อใหม่",
					topic.Name,
				)
			}

			if topic.Description != "รายละเอียดใหม่" {
				t.Errorf(
					"expected description %q, got %q",
					"รายละเอียดใหม่",
					topic.Description,
				)
			}

			if topic.IsActive {
				t.Error(
					"expected IsActive to be false",
				)
			}

			return nil
		},
	}

	service := NewService(repo)

	topic, err := service.Update(
		context.Background(),
		topicID,
		"  ชื่อใหม่  ",
		"  รายละเอียดใหม่  ",
		false,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !updateCalled {
		t.Error("expected repository Update to be called")
	}

	if topic.Name != "ชื่อใหม่" {
		t.Errorf(
			"expected trimmed name, got %q",
			topic.Name,
		)
	}

	if topic.IsActive {
		t.Error(
			"expected topic to be inactive",
		)
	}
}

func TestService_Update_EmptyName(t *testing.T) {
	findCalled := false
	updateCalled := false

	repo := &fakeRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*domain.Topic, error) {

			findCalled = true
			return nil, nil
		},

		updateFn: func(
			ctx context.Context,
			topic *domain.Topic,
		) error {

			updateCalled = true
			return nil
		},
	}

	service := NewService(repo)

	topic, err := service.Update(
		context.Background(),
		uuid.New(),
		"   ",
		"description",
		true,
	)

	if !errors.Is(err, domain.ErrTopicNameEmpty) {
		t.Errorf(
			"expected ErrTopicNameEmpty, got %v",
			err,
		)
	}

	if topic != nil {
		t.Errorf(
			"expected nil topic, got %+v",
			topic,
		)
	}

	if findCalled {
		t.Error(
			"FindByID should not be called when validation fails",
		)
	}

	if updateCalled {
		t.Error(
			"Update should not be called when validation fails",
		)
	}
}

func TestService_Update_NotFound(t *testing.T) {
	topicID := uuid.New()

	updateCalled := false

	repo := &fakeRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*domain.Topic, error) {

			return nil, domain.ErrTopicNotFound
		},

		updateFn: func(
			ctx context.Context,
			topic *domain.Topic,
		) error {

			updateCalled = true
			return nil
		},
	}

	service := NewService(repo)

	topic, err := service.Update(
		context.Background(),
		topicID,
		"งานวิชาการ",
		"description",
		true,
	)

	if !errors.Is(err, domain.ErrTopicNotFound) {
		t.Errorf(
			"expected ErrTopicNotFound, got %v",
			err,
		)
	}

	if topic != nil {
		t.Errorf(
			"expected nil topic, got %+v",
			topic,
		)
	}

	if updateCalled {
		t.Error(
			"Update should not be called when topic is not found",
		)
	}
}

func TestService_Update_RepositoryError(t *testing.T) {
	topicID := uuid.New()
	expectedErr := errors.New("update failed")

	repo := &fakeRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*domain.Topic, error) {

			return &domain.Topic{
				UID:      topicID,
				Name:     "เดิม",
				IsActive: true,
			}, nil
		},

		updateFn: func(
			ctx context.Context,
			topic *domain.Topic,
		) error {

			return expectedErr
		},
	}

	service := NewService(repo)

	topic, err := service.Update(
		context.Background(),
		topicID,
		"ใหม่",
		"description",
		true,
	)

	if !errors.Is(err, expectedErr) {
		t.Errorf(
			"expected %v, got %v",
			expectedErr,
			err,
		)
	}

	if topic != nil {
		t.Errorf(
			"expected nil topic, got %+v",
			topic,
		)
	}
}

func TestService_Delete_Success(t *testing.T) {
	topicID := uuid.New()

	deleteCalled := false

	repo := &fakeRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*domain.Topic, error) {

			return &domain.Topic{
				UID:  topicID,
				Name: "งานวิชาการ",
			}, nil
		},

		deleteFn: func(
			ctx context.Context,
			id uuid.UUID,
		) error {

			deleteCalled = true

			if id != topicID {
				t.Errorf(
					"expected ID %s, got %s",
					topicID,
					id,
				)
			}

			return nil
		},
	}

	service := NewService(repo)

	err := service.Delete(
		context.Background(),
		topicID,
	)

	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if !deleteCalled {
		t.Error(
			"expected Delete to be called",
		)
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	topicID := uuid.New()

	deleteCalled := false

	repo := &fakeRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*domain.Topic, error) {

			return nil, domain.ErrTopicNotFound
		},

		deleteFn: func(
			ctx context.Context,
			id uuid.UUID,
		) error {

			deleteCalled = true
			return nil
		},
	}

	service := NewService(repo)

	err := service.Delete(
		context.Background(),
		topicID,
	)

	if !errors.Is(err, domain.ErrTopicNotFound) {
		t.Errorf(
			"expected ErrTopicNotFound, got %v",
			err,
		)
	}

	if deleteCalled {
		t.Error(
			"Delete should not be called when topic is not found",
		)
	}
}

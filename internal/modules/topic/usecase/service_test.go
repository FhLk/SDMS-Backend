package usecase

import (
	"context"
	"errors"
	"testing"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

type topicRepositoryStub struct {
	createFn   func(context.Context, *domain.Topic) error
	findAllFn  func(context.Context) ([]domain.Topic, error)
	findByIDFn func(context.Context, uuid.UUID) (*domain.Topic, error)
	updateFn   func(context.Context, *domain.Topic) error
	deleteFn   func(context.Context, uuid.UUID) error
}

func (s *topicRepositoryStub) Create(ctx context.Context, topic *domain.Topic) error {
	if s.createFn != nil {
		return s.createFn(ctx, topic)
	}
	return nil
}

func (s *topicRepositoryStub) FindAll(ctx context.Context) ([]domain.Topic, error) {
	if s.findAllFn != nil {
		return s.findAllFn(ctx)
	}
	return []domain.Topic{}, nil
}

func (s *topicRepositoryStub) FindByID(ctx context.Context, id uuid.UUID) (*domain.Topic, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return &domain.Topic{UID: id}, nil
}

func (s *topicRepositoryStub) Update(ctx context.Context, topic *domain.Topic) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, topic)
	}
	return nil
}

func (s *topicRepositoryStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

type fieldRepositoryStub struct {
	createFn           func(context.Context, *domain.TopicField) error
	findAllByTopicIDFn func(context.Context, uuid.UUID) ([]domain.TopicField, error)
	findByIDFn         func(context.Context, uuid.UUID) (*domain.TopicField, error)
	updateFn           func(context.Context, *domain.TopicField) error
	deleteFn           func(context.Context, uuid.UUID) error
}

func (s *fieldRepositoryStub) Create(ctx context.Context, field *domain.TopicField) error {
	if s.createFn != nil {
		return s.createFn(ctx, field)
	}
	return nil
}

func (s *fieldRepositoryStub) FindAllByTopicID(ctx context.Context, id uuid.UUID) ([]domain.TopicField, error) {
	if s.findAllByTopicIDFn != nil {
		return s.findAllByTopicIDFn(ctx, id)
	}
	return []domain.TopicField{}, nil
}

func (s *fieldRepositoryStub) FindByID(ctx context.Context, id uuid.UUID) (*domain.TopicField, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *fieldRepositoryStub) Update(ctx context.Context, field *domain.TopicField) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, field)
	}
	return nil
}

func (s *fieldRepositoryStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

type submissionLookupStub struct {
	hasAnyFn func(context.Context, uuid.UUID) (bool, error)
}

func (s *submissionLookupStub) HasAnyByTopicID(ctx context.Context, topicID uuid.UUID) (bool, error) {
	if s.hasAnyFn != nil {
		return s.hasAnyFn(ctx, topicID)
	}
	return false, nil
}

func TestCreateTopic(t *testing.T) {
	t.Run("trims and persists a new active topic", func(t *testing.T) {
		var persisted *domain.Topic
		repo := &topicRepositoryStub{
			createFn: func(_ context.Context, topic *domain.Topic) error {
				persisted = topic
				return nil
			},
		}
		service := NewTopicService(repo, &fieldRepositoryStub{})

		topic, err := service.CreateTopic(context.Background(), "  งานวิชาการ  ", "  รายละเอียด  ")
		if err != nil {
			t.Fatalf("CreateTopic() error = %v", err)
		}
		if topic != persisted {
			t.Fatal("repository did not receive the returned topic")
		}
		if topic.UID == uuid.Nil || topic.Name != "งานวิชาการ" || topic.Description != "รายละเอียด" || !topic.IsActive {
			t.Errorf("topic = %+v", topic)
		}
	})

	t.Run("rejects a blank name before repository call", func(t *testing.T) {
		called := false
		repo := &topicRepositoryStub{createFn: func(context.Context, *domain.Topic) error {
			called = true
			return nil
		}}
		service := NewTopicService(repo, &fieldRepositoryStub{})

		topic, err := service.CreateTopic(context.Background(), " \t ", "description")
		if !errors.Is(err, domain.ErrTopicNameEmpty) || topic != nil {
			t.Fatalf("topic = %+v, error = %v", topic, err)
		}
		if called {
			t.Error("Create repository method was called")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		wantErr := errors.New("create failed")
		service := NewTopicService(&topicRepositoryStub{
			createFn: func(context.Context, *domain.Topic) error { return wantErr },
		}, &fieldRepositoryStub{})

		topic, err := service.CreateTopic(context.Background(), "topic", "description")
		if !errors.Is(err, wantErr) || topic != nil {
			t.Fatalf("topic = %+v, error = %v", topic, err)
		}
	})
}

func TestCreateField(t *testing.T) {
	topicID := uuid.New()

	t.Run("validates topic then creates a trimmed field", func(t *testing.T) {
		lookupCalled := false
		var persisted *domain.TopicField
		topicRepo := &topicRepositoryStub{findByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Topic, error) {
			lookupCalled = true
			if id != topicID {
				t.Errorf("FindByID() id = %s, want %s", id, topicID)
			}
			return &domain.Topic{UID: id}, nil
		}}
		fieldRepo := &fieldRepositoryStub{createFn: func(_ context.Context, field *domain.TopicField) error {
			persisted = field
			return nil
		}}
		service := NewTopicService(topicRepo, fieldRepo)

		field, err := service.CreateField(context.Background(), topicID, CreateFieldInput{
			Label: "  วันที่ส่ง  ", Type: domain.FieldTypeDate, Required: true, Position: 1,
		})
		if err != nil {
			t.Fatalf("CreateField() error = %v", err)
		}
		if !lookupCalled || field != persisted {
			t.Fatal("topic lookup or field persistence was not performed")
		}
		if field.UID == uuid.Nil || field.TopicUID != topicID || field.Label != "วันที่ส่ง" ||
			field.Type != domain.FieldTypeDate || !field.Required || field.Position != 1 {
			t.Errorf("field = %+v", field)
		}
	})

	t.Run("stops when topic lookup fails", func(t *testing.T) {
		createCalled := false
		service := NewTopicService(&topicRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) {
				return nil, domain.ErrTopicNotFound
			},
		}, &fieldRepositoryStub{createFn: func(context.Context, *domain.TopicField) error {
			createCalled = true
			return nil
		}})

		field, err := service.CreateField(context.Background(), topicID, CreateFieldInput{Label: "label", Type: domain.FieldTypeText})
		if !errors.Is(err, domain.ErrTopicNotFound) || field != nil {
			t.Fatalf("field = %+v, error = %v", field, err)
		}
		if createCalled {
			t.Error("field repository was called")
		}
	})

	validationTests := []struct {
		name    string
		id      uuid.UUID
		input   CreateFieldInput
		wantErr error
	}{
		{"missing topic ID", uuid.Nil, CreateFieldInput{Label: "label", Type: domain.FieldTypeText}, domain.ErrTopicFieldInvalidTopicUID},
		{"blank label", topicID, CreateFieldInput{Label: "   ", Type: domain.FieldTypeText}, domain.ErrTopicFieldLabelRequired},
		{"invalid type", topicID, CreateFieldInput{Label: "label", Type: "checkbox"}, domain.ErrTopicFieldInvalidType},
		{"negative position", topicID, CreateFieldInput{Label: "label", Type: domain.FieldTypeText, Position: -1}, domain.ErrTopicFieldInvalidPosition},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			createCalled := false
			service := NewTopicService(&topicRepositoryStub{}, &fieldRepositoryStub{
				createFn: func(context.Context, *domain.TopicField) error {
					createCalled = true
					return nil
				},
			})

			field, err := service.CreateField(context.Background(), tt.id, tt.input)
			if !errors.Is(err, tt.wantErr) || field != nil {
				t.Fatalf("field = %+v, error = %v, want %v", field, err, tt.wantErr)
			}
			if createCalled {
				t.Error("field repository was called")
			}
		})
	}

	t.Run("propagates field repository error", func(t *testing.T) {
		wantErr := errors.New("field create failed")
		service := NewTopicService(&topicRepositoryStub{}, &fieldRepositoryStub{
			createFn: func(context.Context, *domain.TopicField) error { return wantErr },
		})

		field, err := service.CreateField(context.Background(), topicID, CreateFieldInput{Label: "label", Type: domain.FieldTypeFile})
		if !errors.Is(err, wantErr) || field != nil {
			t.Fatalf("field = %+v, error = %v", field, err)
		}
	})
}

func TestTopicQueries(t *testing.T) {
	wantErr := errors.New("query failed")
	topicID := uuid.New()
	wantTopic := &domain.Topic{UID: topicID, Name: "topic"}
	wantTopics := []domain.Topic{*wantTopic}

	t.Run("FindAll success", func(t *testing.T) {
		service := NewTopicService(&topicRepositoryStub{
			findAllFn: func(context.Context) ([]domain.Topic, error) { return wantTopics, nil },
		}, &fieldRepositoryStub{})
		got, err := service.FindAll(context.Background())
		if err != nil || len(got) != 1 || got[0] != *wantTopic {
			t.Fatalf("FindAll() = %+v, %v", got, err)
		}
	})

	t.Run("FindAll error", func(t *testing.T) {
		service := NewTopicService(&topicRepositoryStub{
			findAllFn: func(context.Context) ([]domain.Topic, error) { return nil, wantErr },
		}, &fieldRepositoryStub{})
		got, err := service.FindAll(context.Background())
		if !errors.Is(err, wantErr) || got != nil {
			t.Fatalf("FindAll() = %+v, %v", got, err)
		}
	})

	t.Run("FindByID success", func(t *testing.T) {
		service := NewTopicService(&topicRepositoryStub{
			findByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Topic, error) {
				if id != topicID {
					t.Errorf("id = %s, want %s", id, topicID)
				}
				return wantTopic, nil
			},
		}, &fieldRepositoryStub{})
		got, err := service.FindByID(context.Background(), topicID)
		if err != nil || got != wantTopic {
			t.Fatalf("FindByID() = %+v, %v", got, err)
		}
	})

	t.Run("FindByID error", func(t *testing.T) {
		service := NewTopicService(&topicRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) { return nil, wantErr },
		}, &fieldRepositoryStub{})
		got, err := service.FindByID(context.Background(), topicID)
		if !errors.Is(err, wantErr) || got != nil {
			t.Fatalf("FindByID() = %+v, %v", got, err)
		}
	})
}

func TestUpdateTopic(t *testing.T) {
	topicID := uuid.New()

	t.Run("trims and persists all mutable fields", func(t *testing.T) {
		existing := &domain.Topic{UID: topicID, Name: "old", IsActive: true}
		updateCalled := false
		service := NewTopicService(&topicRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) { return existing, nil },
			updateFn: func(_ context.Context, topic *domain.Topic) error {
				updateCalled = true
				if topic != existing {
					t.Error("Update received a different topic pointer")
				}
				return nil
			},
		}, &fieldRepositoryStub{})

		got, err := service.Update(context.Background(), topicID, "  new  ", "  detail  ", false)
		if err != nil || got != existing || !updateCalled {
			t.Fatalf("Update() = %+v, %v", got, err)
		}
		if got.Name != "new" || got.Description != "detail" || got.IsActive {
			t.Errorf("updated topic = %+v", got)
		}
	})

	t.Run("rejects blank name before lookup", func(t *testing.T) {
		lookupCalled := false
		service := NewTopicService(&topicRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) {
			lookupCalled = true
			return nil, nil
		}}, &fieldRepositoryStub{})
		got, err := service.Update(context.Background(), topicID, " ", "detail", true)
		if !errors.Is(err, domain.ErrTopicNameEmpty) || got != nil || lookupCalled {
			t.Fatalf("Update() = %+v, %v, lookupCalled=%v", got, err, lookupCalled)
		}
	})

	t.Run("propagates lookup error", func(t *testing.T) {
		wantErr := errors.New("lookup failed")
		service := NewTopicService(&topicRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) { return nil, wantErr },
		}, &fieldRepositoryStub{})
		got, err := service.Update(context.Background(), topicID, "new", "detail", true)
		if !errors.Is(err, wantErr) || got != nil {
			t.Fatalf("Update() = %+v, %v", got, err)
		}
	})

	t.Run("propagates update error", func(t *testing.T) {
		wantErr := errors.New("update failed")
		service := NewTopicService(&topicRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) { return &domain.Topic{UID: topicID}, nil },
			updateFn:   func(context.Context, *domain.Topic) error { return wantErr },
		}, &fieldRepositoryStub{})
		got, err := service.Update(context.Background(), topicID, "new", "detail", true)
		if !errors.Is(err, wantErr) || got != nil {
			t.Fatalf("Update() = %+v, %v", got, err)
		}
	})
}

func TestDeleteTopic(t *testing.T) {
	topicID := uuid.New()

	t.Run("looks up then deletes", func(t *testing.T) {
		deleteCalled := false
		service := NewTopicService(&topicRepositoryStub{
			deleteFn: func(_ context.Context, id uuid.UUID) error {
				deleteCalled = true
				if id != topicID {
					t.Errorf("Delete() id = %s, want %s", id, topicID)
				}
				return nil
			},
		}, &fieldRepositoryStub{})

		if err := service.Delete(context.Background(), topicID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if !deleteCalled {
			t.Error("repository Delete was not called")
		}
	})

	t.Run("stops on lookup error", func(t *testing.T) {
		deleteCalled := false
		service := NewTopicService(&topicRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) { return nil, domain.ErrTopicNotFound },
			deleteFn: func(context.Context, uuid.UUID) error {
				deleteCalled = true
				return nil
			},
		}, &fieldRepositoryStub{})
		err := service.Delete(context.Background(), topicID)
		if !errors.Is(err, domain.ErrTopicNotFound) || deleteCalled {
			t.Fatalf("Delete() error = %v, deleteCalled=%v", err, deleteCalled)
		}
	})

	t.Run("propagates delete error", func(t *testing.T) {
		wantErr := errors.New("delete failed")
		service := NewTopicService(&topicRepositoryStub{
			deleteFn: func(context.Context, uuid.UUID) error { return wantErr },
		}, &fieldRepositoryStub{})
		if err := service.Delete(context.Background(), topicID); !errors.Is(err, wantErr) {
			t.Fatalf("Delete() error = %v, want %v", err, wantErr)
		}
	})
}

func TestFindTopicWithFields(t *testing.T) {
	topicID := uuid.New()
	topic := &domain.Topic{UID: topicID, Name: "topic"}
	fields := []domain.TopicField{{UID: uuid.New(), TopicUID: topicID, Label: "one", Position: 0}}

	service := NewTopicService(&topicRepositoryStub{
		findByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Topic, error) {
			if id != topicID {
				t.Errorf("topic id = %s, want %s", id, topicID)
			}
			return topic, nil
		},
	}, &fieldRepositoryStub{
		findAllByTopicIDFn: func(_ context.Context, id uuid.UUID) ([]domain.TopicField, error) {
			if id != topicID {
				t.Errorf("field topic id = %s, want %s", id, topicID)
			}
			return fields, nil
		},
	})

	gotTopic, gotFields, err := service.FindTopicWithFields(context.Background(), topicID)
	if err != nil {
		t.Fatalf("FindTopicWithFields() error = %v", err)
	}
	if gotTopic != topic || len(gotFields) != 1 || gotFields[0].UID != fields[0].UID {
		t.Fatalf("FindTopicWithFields() = %+v, %+v", gotTopic, gotFields)
	}
}

func TestFindFieldsByTopicID(t *testing.T) {
	topicID := uuid.New()
	fields := []domain.TopicField{{UID: uuid.New(), TopicUID: topicID, Position: 1}}
	service := NewTopicService(&topicRepositoryStub{}, &fieldRepositoryStub{
		findAllByTopicIDFn: func(context.Context, uuid.UUID) ([]domain.TopicField, error) { return fields, nil },
	})

	got, err := service.FindFieldsByTopicID(context.Background(), topicID)
	if err != nil || len(got) != 1 || got[0].UID != fields[0].UID {
		t.Fatalf("FindFieldsByTopicID() = %+v, %v", got, err)
	}

	service = NewTopicService(&topicRepositoryStub{
		findByIDFn: func(context.Context, uuid.UUID) (*domain.Topic, error) { return nil, domain.ErrTopicNotFound },
	}, &fieldRepositoryStub{})
	if got, err := service.FindFieldsByTopicID(context.Background(), topicID); !errors.Is(err, domain.ErrTopicNotFound) || got != nil {
		t.Fatalf("FindFieldsByTopicID() = %+v, %v", got, err)
	}
}

func TestFindFieldByIDRequiresFieldToBelongToTopic(t *testing.T) {
	topicID := uuid.New()
	fieldID := uuid.New()

	t.Run("success", func(t *testing.T) {
		field := &domain.TopicField{UID: fieldID, TopicUID: topicID}
		service := NewTopicService(&topicRepositoryStub{}, &fieldRepositoryStub{
			findByIDFn: func(_ context.Context, id uuid.UUID) (*domain.TopicField, error) {
				if id != fieldID {
					t.Errorf("field id = %s, want %s", id, fieldID)
				}
				return field, nil
			},
		})
		got, err := service.FindFieldByID(context.Background(), topicID, fieldID)
		if err != nil || got != field {
			t.Fatalf("FindFieldByID() = %+v, %v", got, err)
		}
	})

	t.Run("field from another topic is hidden", func(t *testing.T) {
		service := NewTopicService(&topicRepositoryStub{}, &fieldRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.TopicField, error) {
				return &domain.TopicField{UID: fieldID, TopicUID: uuid.New()}, nil
			},
		})
		got, err := service.FindFieldByID(context.Background(), topicID, fieldID)
		if !errors.Is(err, domain.ErrTopicFieldNotFound) || got != nil {
			t.Fatalf("FindFieldByID() = %+v, %v", got, err)
		}
	})
}

func TestUpdateField(t *testing.T) {
	topicID := uuid.New()
	fieldID := uuid.New()
	field := &domain.TopicField{UID: fieldID, TopicUID: topicID, Label: "old", Type: domain.FieldTypeText}
	updateCalled := false

	service := NewTopicService(&topicRepositoryStub{}, &fieldRepositoryStub{
		findByIDFn: func(context.Context, uuid.UUID) (*domain.TopicField, error) { return field, nil },
		updateFn: func(_ context.Context, got *domain.TopicField) error {
			updateCalled = true
			if got != field {
				t.Error("repository received a different field pointer")
			}
			return nil
		},
	})

	got, err := service.UpdateField(context.Background(), topicID, fieldID, UpdateFieldInput{
		Label: "  updated  ", Type: domain.FieldTypeDate, Required: true, Position: 2,
	})
	if err != nil || got != field || !updateCalled {
		t.Fatalf("UpdateField() = %+v, %v, updateCalled=%v", got, err, updateCalled)
	}
	if got.Label != "updated" || got.Type != domain.FieldTypeDate || !got.Required || got.Position != 2 {
		t.Errorf("updated field = %+v", got)
	}

	got, err = service.UpdateField(context.Background(), topicID, fieldID, UpdateFieldInput{Label: " ", Type: domain.FieldTypeText})
	if !errors.Is(err, domain.ErrTopicFieldLabelRequired) || got != nil {
		t.Fatalf("validation UpdateField() = %+v, %v", got, err)
	}
}

func TestDeleteField(t *testing.T) {
	topicID := uuid.New()
	fieldID := uuid.New()
	deleteCalled := false
	service := NewTopicService(&topicRepositoryStub{}, &fieldRepositoryStub{
		findByIDFn: func(context.Context, uuid.UUID) (*domain.TopicField, error) {
			return &domain.TopicField{UID: fieldID, TopicUID: topicID}, nil
		},
		deleteFn: func(_ context.Context, id uuid.UUID) error {
			deleteCalled = true
			if id != fieldID {
				t.Errorf("delete id = %s, want %s", id, fieldID)
			}
			return nil
		},
	})

	if err := service.DeleteField(context.Background(), topicID, fieldID); err != nil {
		t.Fatalf("DeleteField() error = %v", err)
	}
	if !deleteCalled {
		t.Error("field repository Delete was not called")
	}
}

func TestUpdateFieldBlocksTypeChangeAfterSubmission(t *testing.T) {
	topicID := uuid.New()
	fieldID := uuid.New()
	field := &domain.TopicField{UID: fieldID, TopicUID: topicID, Label: "เดิม", Type: domain.FieldTypeText}
	updateCalled := false

	service := NewTopicService(
		&topicRepositoryStub{},
		&fieldRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.TopicField, error) { return field, nil },
			updateFn: func(context.Context, *domain.TopicField) error {
				updateCalled = true
				return nil
			},
		},
		&submissionLookupStub{hasAnyFn: func(_ context.Context, got uuid.UUID) (bool, error) {
			if got != topicID {
				t.Errorf("topic id = %s, want %s", got, topicID)
			}
			return true, nil
		}},
	)

	got, err := service.UpdateField(context.Background(), topicID, fieldID, UpdateFieldInput{
		Label: "ใหม่", Type: domain.FieldTypeDate, Position: 1,
	})
	if !errors.Is(err, domain.ErrTopicFieldTypeLocked) || got != nil {
		t.Fatalf("UpdateField() = %+v, %v", got, err)
	}
	if updateCalled {
		t.Fatal("field repository Update should not be called")
	}
	if field.Type != domain.FieldTypeText {
		t.Fatalf("field type mutated despite lock: %s", field.Type)
	}
}

func TestDeleteFieldBlocksDeleteAfterSubmission(t *testing.T) {
	topicID := uuid.New()
	fieldID := uuid.New()
	deleteCalled := false

	service := NewTopicService(
		&topicRepositoryStub{},
		&fieldRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.TopicField, error) {
				return &domain.TopicField{UID: fieldID, TopicUID: topicID, Type: domain.FieldTypeText}, nil
			},
			deleteFn: func(context.Context, uuid.UUID) error {
				deleteCalled = true
				return nil
			},
		},
		&submissionLookupStub{hasAnyFn: func(context.Context, uuid.UUID) (bool, error) { return true, nil }},
	)

	err := service.DeleteField(context.Background(), topicID, fieldID)
	if !errors.Is(err, domain.ErrTopicFieldDeleteLocked) {
		t.Fatalf("expected ErrTopicFieldDeleteLocked, got %v", err)
	}
	if deleteCalled {
		t.Fatal("field repository Delete should not be called")
	}
}

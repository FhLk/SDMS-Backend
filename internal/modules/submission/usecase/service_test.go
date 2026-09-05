package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	submissiondomain "sdms/internal/modules/submission/domain"
	topicdomain "sdms/internal/modules/topic/domain"
	userdomain "sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type fakeSubmissionRepository struct {
	createFn func(
		context.Context,
		*submissiondomain.Submission,
	) error

	findAllByTopicIDFn func(
		context.Context,
		uuid.UUID,
	) ([]submissiondomain.Submission, error)

	findAllByTopicIDAndSubmittedByFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
	) ([]submissiondomain.Submission, error)

	findByIDAndTopicIDFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
	) (*submissiondomain.Submission, error)

	findByIDAndTopicIDAndSubmittedByFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		uuid.UUID,
	) (*submissiondomain.Submission, error)

	hasAnyByTopicIDFn func(
		context.Context,
		uuid.UUID,
	) (bool, error)
}

func (f *fakeSubmissionRepository) Create(
	ctx context.Context,
	submission *submissiondomain.Submission,
) error {
	if f.createFn != nil {
		return f.createFn(ctx, submission)
	}

	return nil
}

func (f *fakeSubmissionRepository) FindAllByTopicID(
	ctx context.Context,
	topicUID uuid.UUID,
) ([]submissiondomain.Submission, error) {
	if f.findAllByTopicIDFn != nil {
		return f.findAllByTopicIDFn(
			ctx,
			topicUID,
		)
	}

	return []submissiondomain.Submission{}, nil
}

func (f *fakeSubmissionRepository) FindAllByTopicIDAndSubmittedBy(
	ctx context.Context,
	topicUID uuid.UUID,
	submittedBy uuid.UUID,
) ([]submissiondomain.Submission, error) {
	if f.findAllByTopicIDAndSubmittedByFn != nil {
		return f.findAllByTopicIDAndSubmittedByFn(
			ctx,
			topicUID,
			submittedBy,
		)
	}

	return []submissiondomain.Submission{}, nil
}

func (f *fakeSubmissionRepository) FindByIDAndTopicID(
	ctx context.Context,
	submissionUID uuid.UUID,
	topicUID uuid.UUID,
) (*submissiondomain.Submission, error) {
	if f.findByIDAndTopicIDFn != nil {
		return f.findByIDAndTopicIDFn(
			ctx,
			submissionUID,
			topicUID,
		)
	}

	return nil, submissiondomain.ErrSubmissionNotFound
}

func (f *fakeSubmissionRepository) FindByIDAndTopicIDAndSubmittedBy(
	ctx context.Context,
	submissionUID uuid.UUID,
	topicUID uuid.UUID,
	submittedBy uuid.UUID,
) (*submissiondomain.Submission, error) {
	if f.findByIDAndTopicIDAndSubmittedByFn != nil {
		return f.findByIDAndTopicIDAndSubmittedByFn(
			ctx,
			submissionUID,
			topicUID,
			submittedBy,
		)
	}

	return nil, submissiondomain.ErrSubmissionNotFound
}

func (f *fakeSubmissionRepository) HasAnyByTopicID(
	ctx context.Context,
	topicUID uuid.UUID,
) (bool, error) {
	if f.hasAnyByTopicIDFn != nil {
		return f.hasAnyByTopicIDFn(ctx, topicUID)
	}

	return false, nil
}

type fakeTopicRepository struct {
	createFn func(
		context.Context,
		*topicdomain.Topic,
	) error

	findAllFn func(
		context.Context,
	) ([]topicdomain.Topic, error)

	findByIDFn func(
		context.Context,
		uuid.UUID,
	) (*topicdomain.Topic, error)

	updateFn func(
		context.Context,
		*topicdomain.Topic,
	) error

	deleteFn func(
		context.Context,
		uuid.UUID,
	) error
}

func (f *fakeTopicRepository) Create(
	ctx context.Context,
	topic *topicdomain.Topic,
) error {
	if f.createFn != nil {
		return f.createFn(ctx, topic)
	}

	return nil
}

func (f *fakeTopicRepository) FindAll(
	ctx context.Context,
) ([]topicdomain.Topic, error) {
	if f.findAllFn != nil {
		return f.findAllFn(ctx)
	}

	return []topicdomain.Topic{}, nil
}

func (f *fakeTopicRepository) FindByID(
	ctx context.Context,
	uid uuid.UUID,
) (*topicdomain.Topic, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, uid)
	}

	return nil, topicdomain.ErrTopicNotFound
}

func (f *fakeTopicRepository) Update(
	ctx context.Context,
	topic *topicdomain.Topic,
) error {
	if f.updateFn != nil {
		return f.updateFn(ctx, topic)
	}

	return nil
}

func (f *fakeTopicRepository) Delete(
	ctx context.Context,
	uid uuid.UUID,
) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, uid)
	}

	return nil
}

type fakeFieldRepository struct {
	createFn func(
		context.Context,
		*topicdomain.TopicField,
	) error

	findAllByTopicIDFn func(
		context.Context,
		uuid.UUID,
	) ([]topicdomain.TopicField, error)

	findByIDFn func(
		context.Context,
		uuid.UUID,
	) (*topicdomain.TopicField, error)

	updateFn func(
		context.Context,
		*topicdomain.TopicField,
	) error

	deleteFn func(
		context.Context,
		uuid.UUID,
	) error
}

func (f *fakeFieldRepository) Create(
	ctx context.Context,
	field *topicdomain.TopicField,
) error {
	if f.createFn != nil {
		return f.createFn(ctx, field)
	}

	return nil
}

func (f *fakeFieldRepository) FindAllByTopicID(
	ctx context.Context,
	topicUID uuid.UUID,
) ([]topicdomain.TopicField, error) {
	if f.findAllByTopicIDFn != nil {
		return f.findAllByTopicIDFn(
			ctx,
			topicUID,
		)
	}

	return []topicdomain.TopicField{}, nil
}

func (f *fakeFieldRepository) FindByID(
	ctx context.Context,
	fieldUID uuid.UUID,
) (*topicdomain.TopicField, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(
			ctx,
			fieldUID,
		)
	}

	return nil, nil
}

func (f *fakeFieldRepository) Update(
	ctx context.Context,
	field *topicdomain.TopicField,
) error {
	if f.updateFn != nil {
		return f.updateFn(ctx, field)
	}

	return nil
}

func (f *fakeFieldRepository) Delete(
	ctx context.Context,
	fieldUID uuid.UUID,
) error {
	if f.deleteFn != nil {
		return f.deleteFn(
			ctx,
			fieldUID,
		)
	}

	return nil
}

type fakeUserLookupRepository struct {
	findByIDFn func(context.Context, uuid.UUID) (*userdomain.User, error)
}

func (f *fakeUserLookupRepository) FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, id)
	}

	return nil, userdomain.ErrUserNotFound
}

func raw(value string) json.RawMessage {
	return json.RawMessage(value)
}

func TestSubmissionServiceCreateSuccess(t *testing.T) {
	topicUID := uuid.New()
	userUID := uuid.New()

	nameFieldUID := uuid.New()
	budgetFieldUID := uuid.New()
	dateFieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
				Name:     "รายงานโครงการ",
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      nameFieldUID,
					TopicUID: topicUID,
					Label:    "ชื่อโครงการ",
					Type:     topicdomain.FieldTypeText,
					Required: true,
				},
				{
					UID:      budgetFieldUID,
					TopicUID: topicUID,
					Label:    "งบประมาณ",
					Type:     topicdomain.FieldTypeNumber,
					Required: true,
				},
				{
					UID:      dateFieldUID,
					TopicUID: topicUID,
					Label:    "วันที่ดำเนินงาน",
					Type:     topicdomain.FieldTypeDate,
					Required: false,
				},
			}, nil
		},
	}

	createCalled := false

	submissionRepo := &fakeSubmissionRepository{
		createFn: func(
			ctx context.Context,
			submission *submissiondomain.Submission,
		) error {
			createCalled = true
			return nil
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		fieldRepo,
	)

	submission, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: userUID,
			Values: []SubmissionValueInput{
				{
					FieldUID: nameFieldUID,
					Value: raw(
						`"โครงการห้องเรียนสีเขียว"`,
					),
				},
				{
					FieldUID: budgetFieldUID,
					Value:    raw(`50000`),
				},
				{
					FieldUID: dateFieldUID,
					Value:    raw(`"2026-09-03"`),
				},
			},
		},
	)

	if err != nil {
		t.Fatalf(
			"expected nil error, got %v",
			err,
		)
	}

	if !createCalled {
		t.Fatal(
			"expected repository Create to be called",
		)
	}

	if submission == nil {
		t.Fatal("expected submission, got nil")
	}

	if submission.TopicUID != topicUID {
		t.Errorf(
			"expected topic UID %s, got %s",
			topicUID,
			submission.TopicUID,
		)
	}

	if submission.SubmittedBy != userUID {
		t.Errorf(
			"expected submitted by %s, got %s",
			userUID,
			submission.SubmittedBy,
		)
	}

	if len(submission.Values) != 3 {
		t.Fatalf(
			"expected 3 values, got %d",
			len(submission.Values),
		)
	}

	nameValue := submission.Values[0]

	if nameValue.TextValue == nil {
		t.Fatal("expected text value")
	}

	if *nameValue.TextValue != "โครงการห้องเรียนสีเขียว" {
		t.Errorf(
			"unexpected text value %q",
			*nameValue.TextValue,
		)
	}

	budgetValue := submission.Values[1]

	if budgetValue.NumberValue == nil {
		t.Fatal("expected number value")
	}

	if *budgetValue.NumberValue != 50000 {
		t.Errorf(
			"expected 50000, got %f",
			*budgetValue.NumberValue,
		)
	}

	dateValue := submission.Values[2]

	if dateValue.DateValue == nil {
		t.Fatal("expected date value")
	}

	if dateValue.DateValue.Format("2006-01-02") != "2026-09-03" {
		t.Errorf(
			"unexpected date %s",
			dateValue.DateValue.Format("2006-01-02"),
		)
	}
}

func TestSubmissionServiceCreateTopicNotFound(t *testing.T) {
	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return nil, topicdomain.ErrTopicNotFound
		},
	}

	createCalled := false

	submissionRepo := &fakeSubmissionRepository{
		createFn: func(
			ctx context.Context,
			submission *submissiondomain.Submission,
		) error {
			createCalled = true
			return nil
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		&fakeFieldRepository{},
	)

	_, err := service.Create(
		context.Background(),
		uuid.New(),
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
		},
	)

	if !errors.Is(
		err,
		topicdomain.ErrTopicNotFound,
	) {
		t.Fatalf(
			"expected ErrTopicNotFound, got %v",
			err,
		)
	}

	if createCalled {
		t.Error(
			"repository Create should not be called",
		)
	}
}

func TestSubmissionServiceCreateRequiredFieldMissing(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "ชื่อโครงการ",
					Type:     topicdomain.FieldTypeText,
					Required: true,
				},
			}, nil
		},
	}

	service := NewSubmissionService(
		&fakeSubmissionRepository{},
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
			Values:      nil,
		},
	)

	if !errors.Is(
		err,
		submissiondomain.ErrSubmissionRequiredFieldMissing,
	) {
		t.Fatalf(
			"expected ErrSubmissionRequiredFieldMissing, got %v",
			err,
		)
	}
}

func TestSubmissionServiceCreateInvalidField(t *testing.T) {
	topicUID := uuid.New()
	validFieldUID := uuid.New()
	invalidFieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      validFieldUID,
					TopicUID: topicUID,
					Label:    "ชื่อโครงการ",
					Type:     topicdomain.FieldTypeText,
				},
			}, nil
		},
	}

	service := NewSubmissionService(
		&fakeSubmissionRepository{},
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
			Values: []SubmissionValueInput{
				{
					FieldUID: invalidFieldUID,
					Value:    raw(`"Hello"`),
				},
			},
		},
	)

	if !errors.Is(
		err,
		submissiondomain.ErrSubmissionInvalidField,
	) {
		t.Fatalf(
			"expected ErrSubmissionInvalidField, got %v",
			err,
		)
	}
}

func TestSubmissionServiceCreateDuplicateField(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "ชื่อโครงการ",
					Type:     topicdomain.FieldTypeText,
				},
			}, nil
		},
	}

	service := NewSubmissionService(
		&fakeSubmissionRepository{},
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
			Values: []SubmissionValueInput{
				{
					FieldUID: fieldUID,
					Value:    raw(`"A"`),
				},
				{
					FieldUID: fieldUID,
					Value:    raw(`"B"`),
				},
			},
		},
	)

	if !errors.Is(
		err,
		submissiondomain.ErrSubmissionDuplicateField,
	) {
		t.Fatalf(
			"expected ErrSubmissionDuplicateField, got %v",
			err,
		)
	}
}

func TestSubmissionServiceCreateInvalidNumber(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "งบประมาณ",
					Type:     topicdomain.FieldTypeNumber,
					Required: true,
				},
			}, nil
		},
	}

	service := NewSubmissionService(
		&fakeSubmissionRepository{},
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
			Values: []SubmissionValueInput{
				{
					FieldUID: fieldUID,
					Value: raw(
						`"ห้าหมื่นบาท"`,
					),
				},
			},
		},
	)

	if !errors.Is(
		err,
		submissiondomain.ErrSubmissionInvalidValue,
	) {
		t.Fatalf(
			"expected ErrSubmissionInvalidValue, got %v",
			err,
		)
	}
}

func TestSubmissionServiceCreateInvalidDate(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "วันที่ดำเนินงาน",
					Type:     topicdomain.FieldTypeDate,
					Required: true,
				},
			}, nil
		},
	}

	service := NewSubmissionService(
		&fakeSubmissionRepository{},
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
			Values: []SubmissionValueInput{
				{
					FieldUID: fieldUID,
					Value: raw(
						`"03/09/2026"`,
					),
				},
			},
		},
	)

	if !errors.Is(
		err,
		submissiondomain.ErrSubmissionInvalidValue,
	) {
		t.Fatalf(
			"expected ErrSubmissionInvalidValue, got %v",
			err,
		)
	}
}

func TestSubmissionServiceCreateAllowsRequiredFileToBeUploadedAfterward(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "เอกสารโครงการ",
					Type:     topicdomain.FieldTypeFile,
					Required: true,
				},
			}, nil
		},
	}

	created := false
	service := NewSubmissionService(
		&fakeSubmissionRepository{
			createFn: func(ctx context.Context, submission *submissiondomain.Submission) error {
				created = true
				return nil
			},
		},
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
		},
	)

	if err != nil {
		t.Fatalf("expected create to succeed before file upload, got %v", err)
	}
	if !created {
		t.Fatal("expected repository Create to be called")
	}
}

func TestSubmissionServiceCreateRejectsFileContentInJSONValues(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{UID: topicUID, IsActive: true}, nil
		},
	}
	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(ctx context.Context, id uuid.UUID) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{{
				UID: fieldUID, TopicUID: topicUID, Label: "เอกสารโครงการ",
				Type: topicdomain.FieldTypeFile, Required: true,
			}}, nil
		},
	}

	service := NewSubmissionService(&fakeSubmissionRepository{}, topicRepo, fieldRepo)
	_, err := service.Create(context.Background(), topicUID, CreateSubmissionInput{
		SubmittedBy: uuid.New(),
		Values:      []SubmissionValueInput{{FieldUID: fieldUID, Value: raw(`"file.pdf"`)}},
	})

	if !errors.Is(err, submissiondomain.ErrSubmissionFileFieldUnsupported) {
		t.Fatalf("expected ErrSubmissionFileFieldUnsupported, got %v", err)
	}
}

func TestSubmissionServiceCreateRepositoryError(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	expectedErr := errors.New("database error")

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "ชื่อโครงการ",
					Type:     topicdomain.FieldTypeText,
					Required: true,
				},
			}, nil
		},
	}

	submissionRepo := &fakeSubmissionRepository{
		createFn: func(
			ctx context.Context,
			submission *submissiondomain.Submission,
		) error {
			return expectedErr
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
			Values: []SubmissionValueInput{
				{
					FieldUID: fieldUID,
					Value: raw(
						`"โครงการ A"`,
					),
				},
			},
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected repository error, got %v",
			err,
		)
	}
}

func TestSubmissionServiceFindAllByTopicIDSuccess(t *testing.T) {
	topicUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	expected := []submissiondomain.Submission{
		{
			UID:      uuid.New(),
			TopicUID: topicUID,
		},
		{
			UID:      uuid.New(),
			TopicUID: topicUID,
		},
	}

	submissionRepo := &fakeSubmissionRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]submissiondomain.Submission, error) {
			return expected, nil
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		&fakeFieldRepository{},
	)

	result, err := service.FindAllByTopicID(
		context.Background(),
		topicUID,
	)

	if err != nil {
		t.Fatalf(
			"expected nil error, got %v",
			err,
		)
	}

	if len(result) != 2 {
		t.Fatalf(
			"expected 2 submissions, got %d",
			len(result),
		)
	}
}

func TestSubmissionServiceFindAllByTopicIDAndSubmittedBySuccess(t *testing.T) {
	topicUID := uuid.New()
	teacherUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	expected := []submissiondomain.Submission{
		{
			UID:         uuid.New(),
			TopicUID:    topicUID,
			SubmittedBy: teacherUID,
		},
	}

	submissionRepo := &fakeSubmissionRepository{
		findAllByTopicIDAndSubmittedByFn: func(
			ctx context.Context,
			tID uuid.UUID,
			submittedBy uuid.UUID,
		) ([]submissiondomain.Submission, error) {
			if tID != topicUID {
				t.Fatalf("expected topic UID %s, got %s", topicUID, tID)
			}
			if submittedBy != teacherUID {
				t.Fatalf("expected teacher UID %s, got %s", teacherUID, submittedBy)
			}
			return expected, nil
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		&fakeFieldRepository{},
	)

	result, err := service.FindAllByTopicIDAndSubmittedBy(
		context.Background(),
		topicUID,
		teacherUID,
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(result) != 1 || result[0].SubmittedBy != teacherUID {
		t.Fatalf("unexpected filtered submissions: %+v", result)
	}
}

func TestSubmissionServiceFindByIDSuccess(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	submissionRepo := &fakeSubmissionRepository{
		findByIDAndTopicIDFn: func(
			ctx context.Context,
			sID uuid.UUID,
			tID uuid.UUID,
		) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{
				UID:      submissionUID,
				TopicUID: topicUID,
			}, nil
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		&fakeFieldRepository{},
	)

	result, err := service.FindByID(
		context.Background(),
		topicUID,
		submissionUID,
	)

	if err != nil {
		t.Fatalf(
			"expected nil error, got %v",
			err,
		)
	}

	if result.UID != submissionUID {
		t.Errorf(
			"expected UID %s, got %s",
			submissionUID,
			result.UID,
		)
	}
}

func TestSubmissionServiceFindByIDForSubmitterSuccess(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	teacherUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	submissionRepo := &fakeSubmissionRepository{
		findByIDAndTopicIDAndSubmittedByFn: func(
			ctx context.Context,
			sID uuid.UUID,
			tID uuid.UUID,
			submittedBy uuid.UUID,
		) (*submissiondomain.Submission, error) {
			if sID != submissionUID || tID != topicUID || submittedBy != teacherUID {
				t.Fatalf("unexpected filter values")
			}
			return &submissiondomain.Submission{
				UID:         submissionUID,
				TopicUID:    topicUID,
				SubmittedBy: teacherUID,
			}, nil
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		&fakeFieldRepository{},
	)

	result, err := service.FindByIDForSubmitter(
		context.Background(),
		topicUID,
		submissionUID,
		teacherUID,
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.SubmittedBy != teacherUID {
		t.Fatalf("expected submitted_by %s, got %s", teacherUID, result.SubmittedBy)
	}
}

func TestSubmissionServiceFindByIDForSubmitterNotFoundWhenOwnerDoesNotMatch(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	teacherUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{UID: topicUID}, nil
		},
	}

	submissionRepo := &fakeSubmissionRepository{
		findByIDAndTopicIDAndSubmittedByFn: func(
			context.Context,
			uuid.UUID,
			uuid.UUID,
			uuid.UUID,
		) (*submissiondomain.Submission, error) {
			return nil, submissiondomain.ErrSubmissionNotFound
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		&fakeFieldRepository{},
	)

	_, err := service.FindByIDForSubmitter(
		context.Background(),
		topicUID,
		submissionUID,
		teacherUID,
	)
	if !errors.Is(err, submissiondomain.ErrSubmissionNotFound) {
		t.Fatalf("expected submission not found, got %v", err)
	}
}

func TestSubmissionServiceCreateValidSelectValue(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()
	userUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "ฝ่ายงาน",
					Type:     topicdomain.FieldTypeSelect,
					Required: true,
					Options: []topicdomain.SelectOption{
						{
							Label: "วิชาการ",
							Value: "academic",
						},
						{
							Label: "บุคคล",
							Value: "hr",
						},
					},
				},
			}, nil
		},
	}

	createCalled := false

	submissionRepo := &fakeSubmissionRepository{
		createFn: func(
			ctx context.Context,
			submission *submissiondomain.Submission,
		) error {
			createCalled = true
			return nil
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		fieldRepo,
	)

	submission, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: userUID,
			Values: []SubmissionValueInput{
				{
					FieldUID: fieldUID,
					Value:    raw(`"academic"`),
				},
			},
		},
	)

	if err != nil {
		t.Fatalf(
			"expected nil error, got %v",
			err,
		)
	}

	if !createCalled {
		t.Fatal(
			"expected repository Create to be called",
		)
	}

	if submission == nil {
		t.Fatal("expected submission, got nil")
	}

	if len(submission.Values) != 1 {
		t.Fatalf(
			"expected 1 value, got %d",
			len(submission.Values),
		)
	}

	value := submission.Values[0]

	if value.TextValue == nil {
		t.Fatal("expected TextValue")
	}

	if *value.TextValue != "academic" {
		t.Errorf(
			"expected academic, got %q",
			*value.TextValue,
		)
	}
}

func TestSubmissionServiceCreateInvalidSelectValue(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "ฝ่ายงาน",
					Type:     topicdomain.FieldTypeSelect,
					Required: true,
					Options: []topicdomain.SelectOption{
						{
							Label: "วิชาการ",
							Value: "academic",
						},
						{
							Label: "บุคคล",
							Value: "hr",
						},
					},
				},
			}, nil
		},
	}

	createCalled := false

	submissionRepo := &fakeSubmissionRepository{
		createFn: func(
			ctx context.Context,
			submission *submissiondomain.Submission,
		) error {
			createCalled = true
			return nil
		},
	}

	service := NewSubmissionService(
		submissionRepo,
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
			Values: []SubmissionValueInput{
				{
					FieldUID: fieldUID,
					Value:    raw(`"finance"`),
				},
			},
		},
	)

	if !errors.Is(
		err,
		submissiondomain.ErrSubmissionInvalidValue,
	) {
		t.Fatalf(
			"expected ErrSubmissionInvalidValue, got %v",
			err,
		)
	}

	if createCalled {
		t.Error(
			"repository Create should not be called",
		)
	}
}

func TestSubmissionServiceCreateSelectValueMustBeString(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID:      topicUID,
				IsActive: true,
			}, nil
		},
	}

	fieldRepo := &fakeFieldRepository{
		findAllByTopicIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{
					UID:      fieldUID,
					TopicUID: topicUID,
					Label:    "ฝ่ายงาน",
					Type:     topicdomain.FieldTypeSelect,
					Required: true,
					Options: []topicdomain.SelectOption{
						{
							Label: "วิชาการ",
							Value: "academic",
						},
					},
				},
			}, nil
		},
	}

	service := NewSubmissionService(
		&fakeSubmissionRepository{},
		topicRepo,
		fieldRepo,
	)

	_, err := service.Create(
		context.Background(),
		topicUID,
		CreateSubmissionInput{
			SubmittedBy: uuid.New(),
			Values: []SubmissionValueInput{
				{
					FieldUID: fieldUID,

					// ผิด เพราะ select ต้องเป็น string
					Value: raw(`123`),
				},
			},
		},
	)

	if !errors.Is(
		err,
		submissiondomain.ErrSubmissionInvalidValue,
	) {
		t.Fatalf(
			"expected ErrSubmissionInvalidValue, got %v",
			err,
		)
	}
}

func TestSubmissionServiceCreateRejectsInactiveTopic(t *testing.T) {
	topicUID := uuid.New()
	createCalled := false

	service := NewSubmissionService(
		&fakeSubmissionRepository{createFn: func(context.Context, *submissiondomain.Submission) error {
			createCalled = true
			return nil
		}},
		&fakeTopicRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{UID: topicUID, IsActive: false}, nil
		}},
		&fakeFieldRepository{},
	)

	_, err := service.Create(context.Background(), topicUID, CreateSubmissionInput{SubmittedBy: uuid.New()})
	if !errors.Is(err, submissiondomain.ErrSubmissionTopicInactive) {
		t.Fatalf("expected ErrSubmissionTopicInactive, got %v", err)
	}
	if createCalled {
		t.Fatal("repository Create should not be called")
	}
}

func TestSubmissionServiceCreateAcceptsEmptyOptionalValues(t *testing.T) {
	topicUID := uuid.New()
	dateUID := uuid.New()
	numberUID := uuid.New()
	selectUID := uuid.New()

	service := NewSubmissionService(
		&fakeSubmissionRepository{},
		&fakeTopicRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{UID: topicUID, IsActive: true}, nil
		}},
		&fakeFieldRepository{findAllByTopicIDFn: func(context.Context, uuid.UUID) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{
				{UID: dateUID, TopicUID: topicUID, Label: "วันที่", Type: topicdomain.FieldTypeDate},
				{UID: numberUID, TopicUID: topicUID, Label: "จำนวน", Type: topicdomain.FieldTypeNumber},
				{UID: selectUID, TopicUID: topicUID, Label: "ประเภท", Type: topicdomain.FieldTypeSelect, Options: []topicdomain.SelectOption{{Label: "A", Value: "a"}}},
			}, nil
		}},
	)

	submission, err := service.Create(context.Background(), topicUID, CreateSubmissionInput{
		SubmittedBy: uuid.New(),
		Values: []SubmissionValueInput{
			{FieldUID: dateUID, Value: raw(`""`)},
			{FieldUID: numberUID, Value: raw(`"   "`)},
			{FieldUID: selectUID, Value: raw(`""`)},
		},
	})
	if err != nil {
		t.Fatalf("expected empty optional values to be accepted, got %v", err)
	}
	if len(submission.Values) != 0 {
		t.Fatalf("expected empty optional fields not to be stored, got %d values", len(submission.Values))
	}
}

func TestSubmissionServiceCreateValidatesSubmitter(t *testing.T) {
	topicUID := uuid.New()
	teacherUID := uuid.New()
	topicRepo := &fakeTopicRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.Topic, error) {
		return &topicdomain.Topic{UID: topicUID, IsActive: true}, nil
	}}

	tests := []struct {
		name    string
		userFn  func(context.Context, uuid.UUID) (*userdomain.User, error)
		wantErr error
	}{
		{
			name: "user not found",
			userFn: func(context.Context, uuid.UUID) (*userdomain.User, error) {
				return nil, userdomain.ErrUserNotFound
			},
			wantErr: submissiondomain.ErrSubmissionSubmitterNotFound,
		},
		{
			name: "director cannot submit as teacher",
			userFn: func(context.Context, uuid.UUID) (*userdomain.User, error) {
				return &userdomain.User{UID: teacherUID, Role: userdomain.RoleDirector, Status: userdomain.StatusActive}, nil
			},
			wantErr: submissiondomain.ErrSubmissionSubmitterMustBeTeacher,
		},
		{
			name: "inactive teacher",
			userFn: func(context.Context, uuid.UUID) (*userdomain.User, error) {
				return &userdomain.User{UID: teacherUID, Role: userdomain.RoleTeacher, Status: userdomain.StatusInactive}, nil
			},
			wantErr: submissiondomain.ErrSubmissionSubmitterInactive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewSubmissionService(
				&fakeSubmissionRepository{},
				topicRepo,
				&fakeFieldRepository{},
				&fakeUserLookupRepository{findByIDFn: tt.userFn},
			)
			_, err := service.Create(context.Background(), topicUID, CreateSubmissionInput{SubmittedBy: teacherUID})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSubmissionServiceFieldErrorContainsFieldUID(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()
	service := NewSubmissionService(
		&fakeSubmissionRepository{},
		&fakeTopicRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{UID: topicUID, IsActive: true}, nil
		}},
		&fakeFieldRepository{findAllByTopicIDFn: func(context.Context, uuid.UUID) ([]topicdomain.TopicField, error) {
			return []topicdomain.TopicField{{UID: fieldUID, TopicUID: topicUID, Label: "วันที่", Type: topicdomain.FieldTypeDate, Required: true}}, nil
		}},
	)

	_, err := service.Create(context.Background(), topicUID, CreateSubmissionInput{
		SubmittedBy: uuid.New(),
		Values:      []SubmissionValueInput{{FieldUID: fieldUID, Value: raw(`"not-a-date"`)}},
	})
	var fieldErr *submissiondomain.FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected FieldError, got %T: %v", err, err)
	}
	if fieldErr.FieldUID != fieldUID || fieldErr.FieldLabel != "วันที่" {
		t.Fatalf("unexpected field metadata: %+v", fieldErr)
	}
}

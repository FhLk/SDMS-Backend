package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	submissiondomain "sdms/internal/modules/submission/domain"
	topicdomain "sdms/internal/modules/topic/domain"

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

	findByIDAndTopicIDFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
	) (*submissiondomain.Submission, error)
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
				UID:  topicUID,
				Name: "รายงานโครงการ",
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
				UID: topicUID,
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
				UID: topicUID,
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
				UID: topicUID,
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
				UID: topicUID,
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
				UID: topicUID,
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

func TestSubmissionServiceCreateFileFieldUnsupported(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID: topicUID,
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
		},
	)

	if !errors.Is(
		err,
		submissiondomain.ErrSubmissionFileFieldUnsupported,
	) {
		t.Fatalf(
			"expected ErrSubmissionFileFieldUnsupported, got %v",
			err,
		)
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
				UID: topicUID,
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
				UID: topicUID,
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

func TestSubmissionServiceFindByIDSuccess(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()

	topicRepo := &fakeTopicRepository{
		findByIDFn: func(
			ctx context.Context,
			id uuid.UUID,
		) (*topicdomain.Topic, error) {
			return &topicdomain.Topic{
				UID: topicUID,
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
				UID: topicUID,
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
				UID: topicUID,
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
				UID: topicUID,
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

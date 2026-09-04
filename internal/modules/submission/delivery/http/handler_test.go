package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	submissiondomain "sdms/internal/modules/submission/domain"
	"sdms/internal/modules/submission/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeSubmissionService struct {
	createFn func(
		context.Context,
		uuid.UUID,
		usecase.CreateSubmissionInput,
	) (*submissiondomain.Submission, error)

	findAllFn func(
		context.Context,
		uuid.UUID,
	) ([]submissiondomain.Submission, error)

	findAllBySubmitterFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
	) ([]submissiondomain.Submission, error)

	findByIDFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
	) (*submissiondomain.Submission, error)

	findByIDForSubmitterFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		uuid.UUID,
	) (*submissiondomain.Submission, error)
}

func (f *fakeSubmissionService) Create(
	ctx context.Context,
	topicUID uuid.UUID,
	input usecase.CreateSubmissionInput,
) (*submissiondomain.Submission, error) {
	if f.createFn != nil {
		return f.createFn(
			ctx,
			topicUID,
			input,
		)
	}

	return nil, nil
}

func (f *fakeSubmissionService) FindAllByTopicID(
	ctx context.Context,
	topicUID uuid.UUID,
) ([]submissiondomain.Submission, error) {
	if f.findAllFn != nil {
		return f.findAllFn(ctx, topicUID)
	}

	return nil, nil
}

func (f *fakeSubmissionService) FindAllByTopicIDAndSubmittedBy(
	ctx context.Context,
	topicUID uuid.UUID,
	submittedBy uuid.UUID,
) ([]submissiondomain.Submission, error) {
	if f.findAllBySubmitterFn != nil {
		return f.findAllBySubmitterFn(
			ctx,
			topicUID,
			submittedBy,
		)
	}

	return nil, nil
}

func (f *fakeSubmissionService) FindByID(
	ctx context.Context,
	topicUID uuid.UUID,
	submissionUID uuid.UUID,
) (*submissiondomain.Submission, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(
			ctx,
			topicUID,
			submissionUID,
		)
	}

	return nil, nil
}

func (f *fakeSubmissionService) FindByIDForSubmitter(
	ctx context.Context,
	topicUID uuid.UUID,
	submissionUID uuid.UUID,
	submittedBy uuid.UUID,
) (*submissiondomain.Submission, error) {
	if f.findByIDForSubmitterFn != nil {
		return f.findByIDForSubmitterFn(
			ctx,
			topicUID,
			submissionUID,
			submittedBy,
		)
	}

	return nil, nil
}

func TestSubmissionHandlerCreateSuccess(t *testing.T) {
	topicUID := uuid.New()
	userUID := uuid.New()
	fieldUID := uuid.New()

	service := &fakeSubmissionService{
		createFn: func(
			ctx context.Context,
			id uuid.UUID,
			input usecase.CreateSubmissionInput,
		) (*submissiondomain.Submission, error) {

			if id != topicUID {
				t.Errorf(
					"expected topic UID %s, got %s",
					topicUID,
					id,
				)
			}

			return &submissiondomain.Submission{
				UID:         uuid.New(),
				TopicUID:    topicUID,
				SubmittedBy: userUID,
			}, nil
		},
	}

	handler := NewSubmissionHandler(service)

	app := fiber.New()

	app.Post(
		"/topics/:id/submissions",
		handler.Create,
	)

	body := `{
		"submitted_by": "` + userUID.String() + `",
		"values": [
			{
				"field_uid": "` + fieldUID.String() + `",
				"value": "โครงการ A"
			}
		]
	}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/topics/"+topicUID.String()+"/submissions",
		bytes.NewBufferString(body),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf(
			"request failed: %v",
			err,
		)
	}

	defer res.Body.Close()

	if res.StatusCode != fiber.StatusCreated {
		t.Fatalf(
			"expected %d, got %d",
			fiber.StatusCreated,
			res.StatusCode,
		)
	}
}

func TestSubmissionHandlerCreateInvalidTopicID(t *testing.T) {
	handler := NewSubmissionHandler(
		&fakeSubmissionService{},
	)

	app := fiber.New()

	app.Post(
		"/topics/:id/submissions",
		handler.Create,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/topics/not-a-uuid/submissions",
		bytes.NewBufferString(`{}`),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	defer res.Body.Close()

	if res.StatusCode != fiber.StatusBadRequest {
		t.Fatalf(
			"expected 400, got %d",
			res.StatusCode,
		)
	}
}

func TestSubmissionHandlerCreateInvalidValue(t *testing.T) {
	service := &fakeSubmissionService{
		createFn: func(
			ctx context.Context,
			topicUID uuid.UUID,
			input usecase.CreateSubmissionInput,
		) (*submissiondomain.Submission, error) {
			return nil,
				submissiondomain.ErrSubmissionInvalidValue
		},
	}

	handler := NewSubmissionHandler(service)

	app := fiber.New()

	app.Post(
		"/topics/:id/submissions",
		handler.Create,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/topics/"+uuid.NewString()+"/submissions",
		bytes.NewBufferString(`{
			"submitted_by": "`+uuid.NewString()+`",
			"values": []
		}`),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	defer res.Body.Close()

	if res.StatusCode != fiber.StatusBadRequest {
		t.Fatalf(
			"expected 400, got %d",
			res.StatusCode,
		)
	}
}

func TestSubmissionHandlerCreateInvalidBody(t *testing.T) {
	handler := NewSubmissionHandler(
		&fakeSubmissionService{},
	)

	app := fiber.New()

	app.Post(
		"/topics/:id/submissions",
		handler.Create,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/topics/"+uuid.NewString()+"/submissions",
		bytes.NewBufferString(`{
			"submitted_by":
		}`),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	defer res.Body.Close()

	if res.StatusCode != fiber.StatusBadRequest {
		t.Fatalf(
			"expected 400, got %d",
			res.StatusCode,
		)
	}
}

func TestSubmissionHandlerFieldValidationErrorIncludesFieldMetadata(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()
	service := &fakeSubmissionService{
		createFn: func(context.Context, uuid.UUID, usecase.CreateSubmissionInput) (*submissiondomain.Submission, error) {
			return nil, submissiondomain.NewFieldError(
				submissiondomain.ErrSubmissionInvalidValue,
				fieldUID,
				"วันที่ดำเนินงาน",
			)
		},
	}

	handler := NewSubmissionHandler(service)
	app := fiber.New()
	app.Post("/topics/:id/submissions", handler.Create)

	req := httptest.NewRequest(
		http.MethodPost,
		"/topics/"+topicUID.String()+"/submissions",
		bytes.NewBufferString(`{"submitted_by":"`+uuid.NewString()+`","values":[]}`),
	)
	req.Header.Set("Content-Type", "application/json")

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}

	body := make([]byte, 2048)
	n, _ := res.Body.Read(body)
	payload := string(body[:n])
	if !bytes.Contains([]byte(payload), []byte(`"code":"INVALID_SUBMISSION_VALUE"`)) ||
		!bytes.Contains([]byte(payload), []byte(`"field_uid":"`+fieldUID.String()+`"`)) ||
		!bytes.Contains([]byte(payload), []byte(`"field_label":"วันที่ดำเนินงาน"`)) {
		t.Fatalf("unexpected response body: %s", payload)
	}
}

func TestSubmissionHandlerFindAllFiltersBySubmittedBy(t *testing.T) {
	topicUID := uuid.New()
	teacherUID := uuid.New()
	called := false

	service := &fakeSubmissionService{
		findAllBySubmitterFn: func(
			ctx context.Context,
			tID uuid.UUID,
			submittedBy uuid.UUID,
		) ([]submissiondomain.Submission, error) {
			called = true
			if tID != topicUID {
				t.Fatalf("expected topic UID %s, got %s", topicUID, tID)
			}
			if submittedBy != teacherUID {
				t.Fatalf("expected submitted_by %s, got %s", teacherUID, submittedBy)
			}
			return []submissiondomain.Submission{
				{
					UID:         uuid.New(),
					TopicUID:    topicUID,
					SubmittedBy: teacherUID,
				},
			}, nil
		},
	}

	handler := NewSubmissionHandler(service)
	app := fiber.New()
	app.Get("/topics/:id/submissions", handler.FindAll)

	req := httptest.NewRequest(
		http.MethodGet,
		"/topics/"+topicUID.String()+"/submissions?submitted_by="+teacherUID.String(),
		nil,
	)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if !called {
		t.Fatal("expected filtered service method to be called")
	}
}

func TestSubmissionHandlerFindAllRejectsInvalidSubmittedBy(t *testing.T) {
	handler := NewSubmissionHandler(&fakeSubmissionService{})
	app := fiber.New()
	app.Get("/topics/:id/submissions", handler.FindAll)

	req := httptest.NewRequest(
		http.MethodGet,
		"/topics/"+uuid.NewString()+"/submissions?submitted_by=not-a-uuid",
		nil,
	)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

func TestSubmissionHandlerFindByIDFiltersBySubmittedBy(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	teacherUID := uuid.New()
	called := false

	service := &fakeSubmissionService{
		findByIDForSubmitterFn: func(
			ctx context.Context,
			tID uuid.UUID,
			sID uuid.UUID,
			submittedBy uuid.UUID,
		) (*submissiondomain.Submission, error) {
			called = true
			if tID != topicUID || sID != submissionUID || submittedBy != teacherUID {
				t.Fatal("unexpected detail filter values")
			}
			return &submissiondomain.Submission{
				UID:         submissionUID,
				TopicUID:    topicUID,
				SubmittedBy: teacherUID,
			}, nil
		},
	}

	handler := NewSubmissionHandler(service)
	app := fiber.New()
	app.Get("/topics/:id/submissions/:submissionID", handler.FindByID)

	req := httptest.NewRequest(
		http.MethodGet,
		"/topics/"+topicUID.String()+"/submissions/"+submissionUID.String()+"?submitted_by="+teacherUID.String(),
		nil,
	)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if !called {
		t.Fatal("expected submitter-filtered detail service method to be called")
	}
}

func TestSubmissionHandlerFindByIDReturnsNotFoundForDifferentSubmitter(t *testing.T) {
	service := &fakeSubmissionService{
		findByIDForSubmitterFn: func(
			context.Context,
			uuid.UUID,
			uuid.UUID,
			uuid.UUID,
		) (*submissiondomain.Submission, error) {
			return nil, submissiondomain.ErrSubmissionNotFound
		},
	}

	handler := NewSubmissionHandler(service)
	app := fiber.New()
	app.Get("/topics/:id/submissions/:submissionID", handler.FindByID)

	req := httptest.NewRequest(
		http.MethodGet,
		"/topics/"+uuid.NewString()+"/submissions/"+uuid.NewString()+"?submitted_by="+uuid.NewString(),
		nil,
	)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}
}

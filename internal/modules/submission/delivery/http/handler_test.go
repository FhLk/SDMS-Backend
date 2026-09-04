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

	findByIDFn func(
		context.Context,
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

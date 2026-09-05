package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	submissiondomain "sdms/internal/modules/submission/domain"
	"sdms/internal/modules/submission/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeSubmissionFileService struct {
	uploadFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
		usecase.UploadSubmissionFileInput,
	) (*submissiondomain.SubmissionFile, error)

	findAllFn func(
		context.Context,
		uuid.UUID,
		uuid.UUID,
	) ([]submissiondomain.SubmissionFile, error)

	findByIDFn func(
		context.Context,
		uuid.UUID,
	) (*submissiondomain.SubmissionFile, error)

	openFn func(
		context.Context,
		uuid.UUID,
	) (*submissiondomain.SubmissionFile, io.ReadCloser, error)

	deleteFn func(
		context.Context,
		uuid.UUID,
	) error
}

func (f *fakeSubmissionFileService) Upload(
	ctx context.Context,
	topicUID uuid.UUID,
	submissionUID uuid.UUID,
	input usecase.UploadSubmissionFileInput,
) (*submissiondomain.SubmissionFile, error) {
	if f.uploadFn != nil {
		return f.uploadFn(ctx, topicUID, submissionUID, input)
	}
	return nil, nil
}

func (f *fakeSubmissionFileService) FindAll(
	ctx context.Context,
	topicUID uuid.UUID,
	submissionUID uuid.UUID,
) ([]submissiondomain.SubmissionFile, error) {
	if f.findAllFn != nil {
		return f.findAllFn(ctx, topicUID, submissionUID)
	}
	return nil, nil
}

func (f *fakeSubmissionFileService) FindByID(
	ctx context.Context,
	fileUID uuid.UUID,
) (*submissiondomain.SubmissionFile, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, fileUID)
	}
	return nil, nil
}

func (f *fakeSubmissionFileService) Open(
	ctx context.Context,
	fileUID uuid.UUID,
) (*submissiondomain.SubmissionFile, io.ReadCloser, error) {
	if f.openFn != nil {
		return f.openFn(ctx, fileUID)
	}
	return nil, nil, nil
}

func (f *fakeSubmissionFileService) Delete(
	ctx context.Context,
	fileUID uuid.UUID,
) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, fileUID)
	}
	return nil
}

func newSubmissionFileViewTestApp(
	service SubmissionFileService,
) *fiber.App {
	app := fiber.New()
	handler := NewSubmissionFileHandler(service)

	app.Get(
		"/api/v1/submission-files/:fileID/view",
		handler.View,
	)

	return app
}

func TestSubmissionFileHandlerView(t *testing.T) {
	const content = "0123456789"

	newFile := func(fileUID uuid.UUID) *submissiondomain.SubmissionFile {
		return &submissiondomain.SubmissionFile{
			UID:              fileUID,
			SubmissionUID:    uuid.New(),
			FieldUID:         uuid.New(),
			OriginalFilename: "clip.mp4",
			ContentType:      "video/mp4",
			Size:             int64(len(content)),
		}
	}

	t.Run("streams whole file inline", func(t *testing.T) {
		fileUID := uuid.New()
		service := &fakeSubmissionFileService{
			openFn: func(
				_ context.Context,
				gotFileUID uuid.UUID,
			) (*submissiondomain.SubmissionFile, io.ReadCloser, error) {
				if gotFileUID != fileUID {
					t.Fatalf("file UID = %s, want %s", gotFileUID, fileUID)
				}

				return newFile(fileUID), io.NopCloser(strings.NewReader(content)), nil
			},
		}

		app := newSubmissionFileViewTestApp(service)
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/submission-files/"+fileUID.String()+"/view",
			nil,
		)

		res, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != fiber.StatusOK {
			t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusOK)
		}
		if got := res.Header.Get("Content-Type"); got != "video/mp4" {
			t.Errorf("Content-Type = %q, want %q", got, "video/mp4")
		}
		if got := res.Header.Get("Accept-Ranges"); got != "bytes" {
			t.Errorf("Accept-Ranges = %q, want %q", got, "bytes")
		}

		disposition := res.Header.Get("Content-Disposition")
		if !strings.HasPrefix(disposition, "inline") {
			t.Errorf("Content-Disposition = %q, want inline", disposition)
		}
		if !strings.Contains(disposition, "clip.mp4") {
			t.Errorf("Content-Disposition = %q, want filename clip.mp4", disposition)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		if got := string(body); got != content {
			t.Errorf("body = %q, want %q", got, content)
		}
	})

	t.Run("streams requested byte range", func(t *testing.T) {
		fileUID := uuid.New()
		service := &fakeSubmissionFileService{
			openFn: func(
				_ context.Context,
				gotFileUID uuid.UUID,
			) (*submissiondomain.SubmissionFile, io.ReadCloser, error) {
				if gotFileUID != fileUID {
					t.Fatalf("file UID = %s, want %s", gotFileUID, fileUID)
				}

				return newFile(fileUID), io.NopCloser(strings.NewReader(content)), nil
			},
		}

		app := newSubmissionFileViewTestApp(service)
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/submission-files/"+fileUID.String()+"/view",
			nil,
		)
		req.Header.Set("Range", "bytes=2-5")

		res, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != fiber.StatusPartialContent {
			t.Fatalf(
				"status = %d, want %d",
				res.StatusCode,
				fiber.StatusPartialContent,
			)
		}
		if got := res.Header.Get("Content-Range"); got != "bytes 2-5/10" {
			t.Errorf("Content-Range = %q, want %q", got, "bytes 2-5/10")
		}
		if got := res.Header.Get("Content-Length"); got != "4" {
			t.Errorf("Content-Length = %q, want %q", got, "4")
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		if got := string(body); got != "2345" {
			t.Errorf("body = %q, want %q", got, "2345")
		}
	})

	t.Run("returns 416 for invalid range", func(t *testing.T) {
		fileUID := uuid.New()
		service := &fakeSubmissionFileService{
			openFn: func(
				_ context.Context,
				_ uuid.UUID,
			) (*submissiondomain.SubmissionFile, io.ReadCloser, error) {
				return newFile(fileUID), io.NopCloser(strings.NewReader(content)), nil
			},
		}

		app := newSubmissionFileViewTestApp(service)
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/submission-files/"+fileUID.String()+"/view",
			nil,
		)
		req.Header.Set("Range", "bytes=10-")

		res, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != fiber.StatusRequestedRangeNotSatisfiable {
			t.Fatalf(
				"status = %d, want %d",
				res.StatusCode,
				fiber.StatusRequestedRangeNotSatisfiable,
			)
		}
		if got := res.Header.Get("Content-Range"); got != "bytes */10" {
			t.Errorf("Content-Range = %q, want %q", got, "bytes */10")
		}
	})

	t.Run("returns 400 for invalid file id", func(t *testing.T) {
		openCalled := false
		service := &fakeSubmissionFileService{
			openFn: func(
				context.Context,
				uuid.UUID,
			) (*submissiondomain.SubmissionFile, io.ReadCloser, error) {
				openCalled = true
				return nil, nil, nil
			},
		}

		app := newSubmissionFileViewTestApp(service)
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/submission-files/not-a-uuid/view",
			nil,
		)

		res, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusBadRequest)
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		if !strings.Contains(string(body), "invalid file id") {
			t.Fatalf("body = %q, want invalid file id message", string(body))
		}
		if openCalled {
			t.Fatal("service.Open must not be called for invalid file ID")
		}
	})
}

func TestSubmissionFileHandlerFindAllRejectsInvalidPathIDs(t *testing.T) {
	tests := []struct {
		name         string
		topicID      string
		submissionID string
		wantMessage  string
	}{
		{
			name:         "invalid topic id",
			topicID:      "not-a-uuid",
			submissionID: uuid.NewString(),
			wantMessage:  "invalid topic id",
		},
		{
			name:         "invalid submission id",
			topicID:      uuid.NewString(),
			submissionID: "not-a-uuid",
			wantMessage:  "invalid submission id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findAllCalled := false
			service := &fakeSubmissionFileService{
				findAllFn: func(
					context.Context,
					uuid.UUID,
					uuid.UUID,
				) ([]submissiondomain.SubmissionFile, error) {
					findAllCalled = true
					return nil, nil
				},
			}

			app := fiber.New()
			handler := NewSubmissionFileHandler(service)
			app.Get(
				"/api/v1/topics/:id/submissions/:submissionID/files",
				handler.FindAll,
			)

			req := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/topics/"+tt.topicID+"/submissions/"+tt.submissionID+"/files",
				nil,
			)

			res, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer res.Body.Close()

			if res.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusBadRequest)
			}

			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			if !strings.Contains(string(body), tt.wantMessage) {
				t.Fatalf("body = %q, want %q", string(body), tt.wantMessage)
			}
			if findAllCalled {
				t.Fatal("service.FindAll must not be called for invalid path IDs")
			}
		})
	}
}

func TestParseByteRange(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		size      int64
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{name: "closed", value: "bytes=0-99", size: 1000, wantStart: 0, wantEnd: 99},
		{name: "open ended", value: "bytes=500-", size: 1000, wantStart: 500, wantEnd: 999},
		{name: "suffix", value: "bytes=-100", size: 1000, wantStart: 900, wantEnd: 999},
		{name: "suffix larger than file", value: "bytes=-2000", size: 1000, wantStart: 0, wantEnd: 999},
		{name: "end clipped to size", value: "bytes=900-2000", size: 1000, wantStart: 900, wantEnd: 999},
		{name: "invalid prefix", value: "items=0-99", size: 1000, wantErr: true},
		{name: "invalid start", value: "bytes=1000-", size: 1000, wantErr: true},
		{name: "invalid end before start", value: "bytes=100-99", size: 1000, wantErr: true},
		{name: "empty suffix", value: "bytes=-", size: 1000, wantErr: true},
		{name: "multiple ranges", value: "bytes=0-1,3-4", size: 1000, wantErr: true},
		{name: "zero size", value: "bytes=0-1", size: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseByteRange(tt.value, tt.size)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseByteRange() expected error, got %d-%d", start, end)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseByteRange() error = %v", err)
			}
			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf(
					"parseByteRange() = %d-%d, want %d-%d",
					start,
					end,
					tt.wantStart,
					tt.wantEnd,
				)
			}
		})
	}
}

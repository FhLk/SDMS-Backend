package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/upload"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeUploadService struct {
	upload func(context.Context, upload.UploadInput) (model.File, error)
	open   func(context.Context, uuid.UUID, auth.Profile) (upload.OpenedFile, error)
	delete func(context.Context, uuid.UUID, uuid.UUID) error
}

func (f fakeUploadService) Upload(ctx context.Context, input upload.UploadInput) (model.File, error) {
	return f.upload(ctx, input)
}

func (f fakeUploadService) Open(ctx context.Context, fileUUID uuid.UUID, profile auth.Profile) (upload.OpenedFile, error) {
	return f.open(ctx, fileUUID, profile)
}

func (f fakeUploadService) Delete(ctx context.Context, fileUUID, userUUID uuid.UUID) error {
	return f.delete(ctx, fileUUID, userUUID)
}

func TestUploadCreatesStagedFileForAuthenticatedUser(t *testing.T) {
	topicUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb51001")
	fieldUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb51002")
	userUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb51003")
	fileUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb51004")
	createdAt := time.Date(2026, time.October, 13, 7, 0, 0, 0, time.UTC)
	var received upload.UploadInput
	service := fakeUploadService{
		upload: func(_ context.Context, input upload.UploadInput) (model.File, error) {
			received = input
			content, err := io.ReadAll(input.Content)
			if err != nil {
				t.Fatalf("read upload content: %v", err)
			}
			if got, want := string(content), "document"; got != want {
				t.Fatalf("content = %q, want %q", got, want)
			}
			return model.File{
				UUIDModel:      model.UUIDModel{UUID: fileUUID},
				TopicUUID:      input.TopicUUID,
				FieldUUID:      input.FieldUUID,
				UploadedByUUID: input.UploadedByUUID,
				OriginalName:   input.OriginalName,
				MimeType:       input.MimeType,
				SizeBytes:      int64(len(content)),
				Status:         model.FileStaged,
				CreatedAt:      createdAt,
			}, nil
		},
	}
	app := uploadTestApp(service, userUUID)
	req := multipartUploadRequest(t, topicUUID, fieldUUID, "report.pdf", "application/pdf", []byte("document"))

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if received.TopicUUID != topicUUID || received.FieldUUID != fieldUUID || received.UploadedByUUID != userUUID {
		t.Fatalf("Upload input = %+v", received)
	}
	if received.OriginalName != "report.pdf" || received.MimeType != "application/pdf" || received.SizeHint != 8 {
		t.Fatalf("file metadata input = %+v", received)
	}

	var response uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.UUID != fileUUID || response.Status != model.FileStaged || response.SizeBytes != 8 {
		t.Fatalf("response = %+v", response)
	}
}

func TestUploadMapsOversizedFileTo413(t *testing.T) {
	service := fakeUploadService{
		upload: func(context.Context, upload.UploadInput) (model.File, error) {
			return model.File{}, upload.ErrFileTooLarge
		},
	}
	app := uploadTestApp(service, uuid.New())
	req := multipartUploadRequest(t, uuid.New(), uuid.New(), "large.bin", "application/octet-stream", []byte("size-checked-by-service"))
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusRequestEntityTooLarge; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	assertErrorResponse(t, res, "FILE_TOO_LARGE")
}

func TestDeleteUploadUsesAuthenticatedOwner(t *testing.T) {
	fileUUID := uuid.New()
	userUUID := uuid.New()
	called := false
	service := fakeUploadService{
		delete: func(_ context.Context, gotFileUUID, gotUserUUID uuid.UUID) error {
			called = true
			if gotFileUUID != fileUUID || gotUserUUID != userUUID {
				t.Fatalf("Delete(%s, %s), want (%s, %s)", gotFileUUID, gotUserUUID, fileUUID, userUUID)
			}
			return nil
		},
	}
	app := uploadTestApp(service, userUUID)
	req := httptest.NewRequest(http.MethodDelete, "/api/uploads/"+fileUUID.String(), nil)
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "upload-session"})
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer res.Body.Close()
	if got, want := res.StatusCode, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if !called {
		t.Fatal("Delete service was not called")
	}
}

func TestDownloadFileStreamsWithSafeDisposition(t *testing.T) {
	userUUID := uuid.New()
	tests := []struct {
		name            string
		mimeType        string
		originalName    string
		wantDisposition string
		wantContentType string
	}{
		{name: "PDF opens inline", mimeType: "application/pdf", originalName: "รายงาน.pdf", wantDisposition: "inline", wantContentType: "application/pdf"},
		{name: "raster image opens inline", mimeType: "image/png", originalName: "ภาพ.png", wantDisposition: "inline", wantContentType: "image/png"},
		{name: "other file downloads", mimeType: "application/zip", originalName: "หลักฐาน.zip", wantDisposition: "attachment", wantContentType: "application/zip"},
		{name: "SVG downloads for safety", mimeType: "image/svg+xml", originalName: "vector.svg", wantDisposition: "attachment", wantContentType: "image/svg+xml"},
		{name: "invalid MIME downloads as binary", mimeType: "not a mime", originalName: "unknown.bin", wantDisposition: "attachment", wantContentType: "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileUUID := uuid.New()
			content := []byte("streamed-content")
			service := fakeUploadService{
				open: func(_ context.Context, gotFileUUID uuid.UUID, profile auth.Profile) (upload.OpenedFile, error) {
					if gotFileUUID != fileUUID || profile.User.UUID != userUUID {
						t.Fatalf("Open(%s, %s), want (%s, %s)", gotFileUUID, profile.User.UUID, fileUUID, userUUID)
					}
					return upload.OpenedFile{
						File: model.File{
							UUIDModel:    model.UUIDModel{UUID: fileUUID},
							OriginalName: tt.originalName,
							MimeType:     tt.mimeType,
							SizeBytes:    int64(len(content)),
						},
						Content: io.NopCloser(bytes.NewReader(content)),
					}, nil
				},
			}
			app := uploadTestApp(service, userUUID)
			req := httptest.NewRequest(http.MethodGet, "/api/files/"+fileUUID.String(), nil)
			req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "upload-session"})

			res, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer res.Body.Close()
			if got, want := res.StatusCode, http.StatusOK; got != want {
				t.Fatalf("status = %d, want %d", got, want)
			}
			if got := res.Header.Get("Content-Type"); got != tt.wantContentType {
				t.Fatalf("Content-Type = %q, want %q", got, tt.wantContentType)
			}
			disposition, parameters, err := mime.ParseMediaType(res.Header.Get("Content-Disposition"))
			if err != nil {
				t.Fatalf("parse Content-Disposition: %v", err)
			}
			if disposition != tt.wantDisposition || parameters["filename"] != tt.originalName {
				t.Fatalf("Content-Disposition = %q/%q, want %q/%q", disposition, parameters["filename"], tt.wantDisposition, tt.originalName)
			}
			if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			if !bytes.Equal(body, content) {
				t.Fatalf("body = %q, want %q", body, content)
			}
		})
	}
}

func TestDownloadFileMapsAccessErrors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantCode   string
	}{
		{name: "forbidden", serviceErr: upload.ErrFileForbidden, wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN"},
		{name: "missing", serviceErr: upload.ErrFileNotFound, wantStatus: http.StatusNotFound, wantCode: "FILE_NOT_FOUND"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := fakeUploadService{
				open: func(context.Context, uuid.UUID, auth.Profile) (upload.OpenedFile, error) {
					return upload.OpenedFile{}, tt.serviceErr
				},
			}
			app := uploadTestApp(service, uuid.New())
			req := httptest.NewRequest(http.MethodGet, "/api/files/"+uuid.NewString(), nil)
			req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "upload-session"})
			res, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer res.Body.Close()
			if res.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
			assertErrorResponse(t, res, tt.wantCode)
		})
	}
}

func uploadTestApp(service uploadService, userUUID uuid.UUID) *fiber.App {
	authService := fakeAuthService{
		currentUser: func(_ context.Context, token string) (auth.Profile, error) {
			if token != "upload-session" {
				return auth.Profile{}, auth.ErrUnauthenticated
			}
			return auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: userUUID}}}, nil
		},
	}
	return New(Dependencies{Auth: authService, AuthConfig: testAuthConfig(), Uploads: service})
}

func multipartUploadRequest(t *testing.T, topicUUID, fieldUUID uuid.UUID, filename, mimeType string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("topic_uuid", topicUUID.String()); err != nil {
		t.Fatalf("write topic_uuid: %v", err)
	}
	if err := writer.WriteField("field_uuid", fieldUUID.String()); err != nil {
		t.Fatalf("write field_uuid: %v", err)
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	partHeader.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "sdms_session", Value: "upload-session"})
	return req
}

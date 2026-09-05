package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	submissiondomain "sdms/internal/modules/submission/domain"
	topicdomain "sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
)

type fakeSubmissionFileRepository struct {
	createFn                   func(context.Context, *submissiondomain.SubmissionFile) error
	findAllFn                  func(context.Context, uuid.UUID) ([]submissiondomain.SubmissionFile, error)
	findByIDFn                 func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error)
	findBySubmissionAndFieldFn func(context.Context, uuid.UUID, uuid.UUID) (*submissiondomain.SubmissionFile, error)
	deleteFn                   func(context.Context, uuid.UUID) error
}

func (f *fakeSubmissionFileRepository) Create(ctx context.Context, file *submissiondomain.SubmissionFile) error {
	if f.createFn != nil {
		return f.createFn(ctx, file)
	}
	return nil
}

func (f *fakeSubmissionFileRepository) FindAllBySubmissionID(ctx context.Context, submissionUID uuid.UUID) ([]submissiondomain.SubmissionFile, error) {
	if f.findAllFn != nil {
		return f.findAllFn(ctx, submissionUID)
	}
	return []submissiondomain.SubmissionFile{}, nil
}

func (f *fakeSubmissionFileRepository) FindByID(ctx context.Context, fileUID uuid.UUID) (*submissiondomain.SubmissionFile, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, fileUID)
	}
	return nil, submissiondomain.ErrSubmissionFileNotFound
}

func (f *fakeSubmissionFileRepository) FindBySubmissionIDAndFieldID(ctx context.Context, submissionUID, fieldUID uuid.UUID) (*submissiondomain.SubmissionFile, error) {
	if f.findBySubmissionAndFieldFn != nil {
		return f.findBySubmissionAndFieldFn(ctx, submissionUID, fieldUID)
	}
	return nil, submissiondomain.ErrSubmissionFileNotFound
}

func (f *fakeSubmissionFileRepository) Delete(ctx context.Context, fileUID uuid.UUID) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, fileUID)
	}
	return nil
}

type fakeSubmissionFileStorage struct {
	saveFn   func(context.Context, string, io.Reader) error
	openFn   func(context.Context, string) (io.ReadCloser, error)
	deleteFn func(context.Context, string) error
}

func (f *fakeSubmissionFileStorage) Save(ctx context.Context, path string, src io.Reader) error {
	if f.saveFn != nil {
		return f.saveFn(ctx, path, src)
	}
	return nil
}

func (f *fakeSubmissionFileStorage) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	if f.openFn != nil {
		return f.openFn(ctx, path)
	}
	return io.NopCloser(strings.NewReader("file")), nil
}

func (f *fakeSubmissionFileStorage) Delete(ctx context.Context, path string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, path)
	}
	return nil
}

func TestSubmissionFileServiceUploadSuccess(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	fieldUID := uuid.New()

	submissionRepo := &fakeSubmissionRepository{
		findByIDAndTopicIDFn: func(ctx context.Context, gotSubmissionUID, gotTopicUID uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID}, nil
		},
	}
	fieldRepo := &fakeFieldRepository{
		findByIDFn: func(ctx context.Context, gotFieldUID uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{
				UID: fieldUID, TopicUID: topicUID, Label: "เอกสาร", Type: topicdomain.FieldTypeFile,
			}, nil
		},
	}

	var savedPath string
	storage := &fakeSubmissionFileStorage{
		saveFn: func(ctx context.Context, path string, src io.Reader) error {
			savedPath = path
			body, err := io.ReadAll(src)
			if err != nil {
				return err
			}
			if string(body) != "pdf-data" {
				t.Fatalf("unexpected stored body %q", string(body))
			}
			return nil
		},
	}

	var created *submissiondomain.SubmissionFile
	fileRepo := &fakeSubmissionFileRepository{
		createFn: func(ctx context.Context, file *submissiondomain.SubmissionFile) error {
			copy := *file
			created = &copy
			return nil
		},
	}

	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)
	file, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "../report.pdf", ContentType: "application/pdf",
		Size: 8, Reader: bytes.NewBufferString("pdf-data"),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if file.OriginalFilename != "report.pdf" {
		t.Fatalf("OriginalFilename = %q", file.OriginalFilename)
	}
	if created == nil || savedPath == "" || created.StoragePath != savedPath {
		t.Fatalf("file was not stored and persisted consistently")
	}
	if !strings.HasSuffix(savedPath, ".pdf") || !strings.Contains(savedPath, submissionUID.String()) {
		t.Fatalf("unexpected storage path %q", savedPath)
	}
}

func TestSubmissionFileServiceUploadRejectsNonFileField(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	fieldUID := uuid.New()

	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID}, nil
		}},
		&fakeSubmissionFileRepository{},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: fieldUID, TopicUID: topicUID, Type: topicdomain.FieldTypeText}, nil
		}},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, submissiondomain.ErrSubmissionFileFieldNotFile) {
		t.Fatalf("expected ErrSubmissionFileFieldNotFile, got %v", err)
	}
}

func TestSubmissionFileServiceUploadRejectsTooLargeFile(t *testing.T) {
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		5,
	)

	_, err := service.Upload(context.Background(), uuid.New(), uuid.New(), UploadSubmissionFileInput{
		FieldUID: uuid.New(), OriginalFilename: "report.pdf", Size: 6, Reader: strings.NewReader("123456"),
	})
	if !errors.Is(err, submissiondomain.ErrSubmissionFileTooLarge) {
		t.Fatalf("expected ErrSubmissionFileTooLarge, got %v", err)
	}
}

func TestAllowedUploadExtensionsIncludeBrowserPreviewMedia(t *testing.T) {
	allowed := []string{".jpg", ".png", ".webp", ".mp4", ".webm", ".mov", ".m4v", ".pdf"}
	for _, ext := range allowed {
		if !isAllowedUploadExtension(ext) {
			t.Fatalf("expected %s to be allowed", ext)
		}
	}

	if isAllowedUploadExtension(".exe") {
		t.Fatal("expected .exe to be rejected")
	}
}

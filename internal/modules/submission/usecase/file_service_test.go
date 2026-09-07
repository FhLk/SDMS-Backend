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

func newFileUploadFixture() (
	uuid.UUID,
	uuid.UUID,
	uuid.UUID,
	*fakeSubmissionRepository,
	*fakeSubmissionFileRepository,
	*fakeFieldRepository,
	*fakeSubmissionFileStorage,
) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	fieldUID := uuid.New()

	submissionRepo := &fakeSubmissionRepository{
		findByIDAndTopicIDFn: func(_ context.Context, gotSubmissionUID, gotTopicUID uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{
				UID:      gotSubmissionUID,
				TopicUID: gotTopicUID,
			}, nil
		},
	}
	fileRepo := &fakeSubmissionFileRepository{}
	fieldRepo := &fakeFieldRepository{
		findByIDFn: func(_ context.Context, gotFieldUID uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{
				UID:      gotFieldUID,
				TopicUID: topicUID,
				Label:    "เอกสาร",
				Type:     topicdomain.FieldTypeFile,
			}, nil
		},
	}
	storage := &fakeSubmissionFileStorage{}

	return topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage
}

func newStoredSubmissionFile(submissionUID, fieldUID uuid.UUID) *submissiondomain.SubmissionFile {
	return &submissiondomain.SubmissionFile{
		UID:              uuid.New(),
		SubmissionUID:    submissionUID,
		FieldUID:         fieldUID,
		OriginalFilename: "report.pdf",
		StoredFilename:   "stored.pdf",
		StoragePath:      "submissions/" + submissionUID.String() + "/stored.pdf",
		ContentType:      "application/pdf",
		Size:             10,
	}
}

func TestNewSubmissionFileServiceUsesDefaultMaxUploadSize(t *testing.T) {
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		0,
	)

	if service.maxUploadSize != DefaultMaxUploadSize {
		t.Fatalf("maxUploadSize = %d, want %d", service.maxUploadSize, DefaultMaxUploadSize)
	}
}

func TestSubmissionFileServiceUploadSuccess(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()

	var savedPath string
	storage.saveFn = func(_ context.Context, path string, src io.Reader) error {
		savedPath = path
		body, err := io.ReadAll(src)
		if err != nil {
			return err
		}
		if string(body) != "pdf-data" {
			t.Fatalf("unexpected stored body %q", string(body))
		}
		return nil
	}

	var created *submissiondomain.SubmissionFile
	fileRepo.createFn = func(_ context.Context, file *submissiondomain.SubmissionFile) error {
		copyFile := *file
		created = &copyFile
		return nil
	}

	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)
	file, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID:         fieldUID,
		OriginalFilename: `..\folder\report.PDF`,
		ContentType:      "text/plain",
		Size:             8,
		Reader:           bytes.NewBufferString("pdf-data"),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if file.OriginalFilename != "report.PDF" {
		t.Fatalf("OriginalFilename = %q, want report.PDF", file.OriginalFilename)
	}
	if file.ContentType != "application/pdf" {
		t.Fatalf("ContentType = %q, want application/pdf", file.ContentType)
	}
	if created == nil || savedPath == "" || created.StoragePath != savedPath {
		t.Fatal("file was not stored and persisted consistently")
	}
	if !strings.HasSuffix(savedPath, ".pdf") || !strings.Contains(savedPath, submissionUID.String()) {
		t.Fatalf("unexpected storage path %q", savedPath)
	}
}

func TestSubmissionFileServiceUploadRejectsEmptyInput(t *testing.T) {
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	tests := []struct {
		name  string
		input UploadSubmissionFileInput
	}{
		{
			name: "nil reader",
			input: UploadSubmissionFileInput{
				FieldUID: uuid.New(), OriginalFilename: "report.pdf", Size: 1,
			},
		},
		{
			name: "zero size",
			input: UploadSubmissionFileInput{
				FieldUID: uuid.New(), OriginalFilename: "report.pdf", Size: 0, Reader: strings.NewReader("x"),
			},
		},
		{
			name: "negative size",
			input: UploadSubmissionFileInput{
				FieldUID: uuid.New(), OriginalFilename: "report.pdf", Size: -1, Reader: strings.NewReader("x"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Upload(context.Background(), uuid.New(), uuid.New(), tt.input)
			if !errors.Is(err, submissiondomain.ErrSubmissionFileEmpty) {
				t.Fatalf("expected ErrSubmissionFileEmpty, got %v", err)
			}
		})
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

func TestSubmissionFileServiceUploadReturnsSubmissionLookupError(t *testing.T) {
	expectedErr := errors.New("submission lookup failed")
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return nil, expectedErr
		}},
		&fakeSubmissionFileRepository{},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	_, err := service.Upload(context.Background(), uuid.New(), uuid.New(), UploadSubmissionFileInput{
		FieldUID: uuid.New(), OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceUploadReturnsFieldLookupError(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, _, storage := newFileUploadFixture()
	expectedErr := errors.New("field lookup failed")
	fieldRepo := &fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
		return nil, expectedErr
	}}
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceUploadRejectsFieldFromOtherTopic(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()
	fieldRepo.findByIDFn = func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
		return &topicdomain.TopicField{
			UID: fieldUID, TopicUID: uuid.New(), Label: "เอกสาร", Type: topicdomain.FieldTypeFile,
		}, nil
	}
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, submissiondomain.ErrSubmissionFileFieldTopicMismatch) {
		t.Fatalf("expected ErrSubmissionFileFieldTopicMismatch, got %v", err)
	}
}

func TestSubmissionFileServiceUploadRejectsNonFileField(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()
	fieldRepo.findByIDFn = func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
		return &topicdomain.TopicField{
			UID: fieldUID, TopicUID: topicUID, Label: "ชื่อ", Type: topicdomain.FieldTypeText,
		}, nil
	}
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, submissiondomain.ErrSubmissionFileFieldNotFile) {
		t.Fatalf("expected ErrSubmissionFileFieldNotFile, got %v", err)
	}
}

func TestSubmissionFileServiceUploadRejectsExistingFileForField(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()
	fileRepo.findBySubmissionAndFieldFn = func(_ context.Context, gotSubmissionUID, gotFieldUID uuid.UUID) (*submissiondomain.SubmissionFile, error) {
		return newStoredSubmissionFile(gotSubmissionUID, gotFieldUID), nil
	}
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, submissiondomain.ErrSubmissionFileAlreadyExists) {
		t.Fatalf("expected ErrSubmissionFileAlreadyExists, got %v", err)
	}
}

func TestSubmissionFileServiceUploadReturnsDuplicateLookupError(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()
	expectedErr := errors.New("duplicate lookup failed")
	fileRepo.findBySubmissionAndFieldFn = func(context.Context, uuid.UUID, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
		return nil, expectedErr
	}
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceUploadRejectsMissingFilename(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "   ", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, submissiondomain.ErrSubmissionFileNameRequired) {
		t.Fatalf("expected ErrSubmissionFileNameRequired, got %v", err)
	}
}

func TestSubmissionFileServiceUploadRejectsUnsupportedExtension(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "malware.exe", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, submissiondomain.ErrSubmissionFileTypeNotAllowed) {
		t.Fatalf("expected ErrSubmissionFileTypeNotAllowed, got %v", err)
	}
}

func TestSubmissionFileServiceUploadReturnsStorageSaveError(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()
	expectedErr := errors.New("storage save failed")
	storage.saveFn = func(context.Context, string, io.Reader) error { return expectedErr }
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceUploadCleansUpStoredFileWhenDomainCreationFails(t *testing.T) {
	topicUID := uuid.New()
	fieldUID := uuid.New()
	var savedPath string
	var deletedPath string

	submissionRepo := &fakeSubmissionRepository{findByIDAndTopicIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
		return &submissiondomain.Submission{UID: uuid.Nil, TopicUID: topicUID}, nil
	}}
	fieldRepo := &fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
		return &topicdomain.TopicField{UID: fieldUID, TopicUID: topicUID, Type: topicdomain.FieldTypeFile}, nil
	}}
	storage := &fakeSubmissionFileStorage{
		saveFn: func(_ context.Context, path string, _ io.Reader) error {
			savedPath = path
			return nil
		},
		deleteFn: func(_ context.Context, path string) error {
			deletedPath = path
			return nil
		},
	}
	service := NewSubmissionFileService(submissionRepo, &fakeSubmissionFileRepository{}, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, uuid.Nil, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, submissiondomain.ErrSubmissionFileSubmissionUIDRequired) {
		t.Fatalf("expected ErrSubmissionFileSubmissionUIDRequired, got %v", err)
	}
	if savedPath == "" || deletedPath != savedPath {
		t.Fatalf("stored file was not cleaned up: saved=%q deleted=%q", savedPath, deletedPath)
	}
}

func TestSubmissionFileServiceUploadCleansUpStoredFileWhenRepositoryCreateFails(t *testing.T) {
	topicUID, submissionUID, fieldUID, submissionRepo, fileRepo, fieldRepo, storage := newFileUploadFixture()
	expectedErr := errors.New("create failed")
	var savedPath string
	var deletedPath string

	storage.saveFn = func(_ context.Context, path string, _ io.Reader) error {
		savedPath = path
		return nil
	}
	storage.deleteFn = func(_ context.Context, path string) error {
		deletedPath = path
		return nil
	}
	fileRepo.createFn = func(context.Context, *submissiondomain.SubmissionFile) error { return expectedErr }
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, storage, DefaultMaxUploadSize)

	_, err := service.Upload(context.Background(), topicUID, submissionUID, UploadSubmissionFileInput{
		FieldUID: fieldUID, OriginalFilename: "report.pdf", Size: 1, Reader: strings.NewReader("x"),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if savedPath == "" || deletedPath != savedPath {
		t.Fatalf("stored file was not cleaned up: saved=%q deleted=%q", savedPath, deletedPath)
	}
}

func TestAllowedUploadExtensions(t *testing.T) {
	allowed := []string{
		".pdf", ".xls", ".xlsx",
		".png", ".jpg", ".jpeg", ".webp",
		".mp4", ".webm", ".mov", ".m4v",
		".PDF", ".JPG", ".MP4",
	}
	for _, ext := range allowed {
		if !isAllowedUploadExtension(ext) {
			t.Fatalf("expected %s to be allowed", ext)
		}
	}

	disallowed := []string{"", ".exe", ".sh", ".zip", ".svg", ".html"}
	for _, ext := range disallowed {
		if isAllowedUploadExtension(ext) {
			t.Fatalf("expected %s to be rejected", ext)
		}
	}
}

func TestSubmissionFileServiceFindAllSuccess(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	fieldUID := uuid.New()
	expected := []submissiondomain.SubmissionFile{*newStoredSubmissionFile(submissionUID, fieldUID)}

	submissionRepo := &fakeSubmissionRepository{findByIDAndTopicIDFn: func(_ context.Context, gotSubmissionUID, gotTopicUID uuid.UUID) (*submissiondomain.Submission, error) {
		if gotSubmissionUID != submissionUID || gotTopicUID != topicUID {
			t.Fatalf("unexpected submission lookup args: %s %s", gotSubmissionUID, gotTopicUID)
		}
		return &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID}, nil
	}}
	fileRepo := &fakeSubmissionFileRepository{findAllFn: func(_ context.Context, gotSubmissionUID uuid.UUID) ([]submissiondomain.SubmissionFile, error) {
		if gotSubmissionUID != submissionUID {
			t.Fatalf("submissionUID = %s, want %s", gotSubmissionUID, submissionUID)
		}
		return expected, nil
	}}
	service := NewSubmissionFileService(submissionRepo, fileRepo, &fakeFieldRepository{}, &fakeSubmissionFileStorage{}, DefaultMaxUploadSize)

	files, err := service.FindAll(context.Background(), topicUID, submissionUID)
	if err != nil {
		t.Fatalf("FindAll() error = %v", err)
	}
	if len(files) != 1 || files[0].UID != expected[0].UID {
		t.Fatalf("FindAll() = %#v, want %#v", files, expected)
	}
}

func TestSubmissionFileServiceFindAllReturnsSubmissionLookupError(t *testing.T) {
	expectedErr := errors.New("submission lookup failed")
	fileRepoCalled := false
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return nil, expectedErr
		}},
		&fakeSubmissionFileRepository{findAllFn: func(context.Context, uuid.UUID) ([]submissiondomain.SubmissionFile, error) {
			fileRepoCalled = true
			return nil, nil
		}},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	_, err := service.FindAll(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if fileRepoCalled {
		t.Fatal("file repository should not be called after submission lookup error")
	}
}

func TestSubmissionFileServiceFindAllReturnsRepositoryError(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	expectedErr := errors.New("find all failed")
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID}, nil
		}},
		&fakeSubmissionFileRepository{findAllFn: func(context.Context, uuid.UUID) ([]submissiondomain.SubmissionFile, error) {
			return nil, expectedErr
		}},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	_, err := service.FindAll(context.Background(), topicUID, submissionUID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceFindByID(t *testing.T) {
	fileUID := uuid.New()
	file := &submissiondomain.SubmissionFile{UID: fileUID}
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{findByIDFn: func(_ context.Context, gotFileUID uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			if gotFileUID != fileUID {
				t.Fatalf("fileUID = %s, want %s", gotFileUID, fileUID)
			}
			return file, nil
		}},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	got, err := service.FindByID(context.Background(), fileUID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got != file {
		t.Fatalf("FindByID() returned unexpected file")
	}
}

func TestSubmissionFileServiceFindByIDReturnsRepositoryError(t *testing.T) {
	expectedErr := errors.New("find by id failed")
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			return nil, expectedErr
		}},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	_, err := service.FindByID(context.Background(), uuid.New())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceFindByIDForSubmitterSuccess(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	fieldUID := uuid.New()
	submitterUID := uuid.New()
	file := newStoredSubmissionFile(submissionUID, fieldUID)

	fileRepo := &fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
		return file, nil
	}}
	fieldRepo := &fakeFieldRepository{findByIDFn: func(_ context.Context, gotFieldUID uuid.UUID) (*topicdomain.TopicField, error) {
		if gotFieldUID != fieldUID {
			t.Fatalf("fieldUID = %s, want %s", gotFieldUID, fieldUID)
		}
		return &topicdomain.TopicField{UID: fieldUID, TopicUID: topicUID}, nil
	}}
	submissionRepo := &fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(_ context.Context, gotSubmissionUID, gotTopicUID, gotSubmitterUID uuid.UUID) (*submissiondomain.Submission, error) {
		if gotSubmissionUID != submissionUID || gotTopicUID != topicUID || gotSubmitterUID != submitterUID {
			t.Fatalf("unexpected ownership lookup args: submission=%s topic=%s submitter=%s", gotSubmissionUID, gotTopicUID, gotSubmitterUID)
		}
		return &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID, SubmittedBy: submitterUID}, nil
	}}
	service := NewSubmissionFileService(submissionRepo, fileRepo, fieldRepo, &fakeSubmissionFileStorage{}, DefaultMaxUploadSize)

	got, err := service.FindByIDForSubmitter(context.Background(), file.UID, submitterUID)
	if err != nil {
		t.Fatalf("FindByIDForSubmitter() error = %v", err)
	}
	if got != file {
		t.Fatal("FindByIDForSubmitter() returned unexpected file")
	}
}

func TestSubmissionFileServiceFindByIDForSubmitterReturnsFileLookupError(t *testing.T) {
	expectedErr := submissiondomain.ErrSubmissionFileNotFound
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	_, err := service.FindByIDForSubmitter(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceFindByIDForSubmitterReturnsFieldLookupError(t *testing.T) {
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	expectedErr := errors.New("field lookup failed")
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			return file, nil
		}},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return nil, expectedErr
		}},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	_, err := service.FindByIDForSubmitter(context.Background(), file.UID, uuid.New())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceFindByIDForSubmitterRejectsOtherSubmitter(t *testing.T) {
	topicUID := uuid.New()
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	fileOwnerUID := uuid.New()
	otherTeacherUID := uuid.New()

	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, gotSubmitterUID uuid.UUID) (*submissiondomain.Submission, error) {
			if gotSubmitterUID == fileOwnerUID {
				return &submissiondomain.Submission{UID: file.SubmissionUID, TopicUID: topicUID, SubmittedBy: fileOwnerUID}, nil
			}
			return nil, submissiondomain.ErrSubmissionNotFound
		}},
		&fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			return file, nil
		}},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: file.FieldUID, TopicUID: topicUID}, nil
		}},
		&fakeSubmissionFileStorage{},
		DefaultMaxUploadSize,
	)

	_, err := service.FindByIDForSubmitter(context.Background(), file.UID, otherTeacherUID)
	if !errors.Is(err, submissiondomain.ErrSubmissionNotFound) {
		t.Fatalf("expected ErrSubmissionNotFound for another submitter, got %v", err)
	}
}

func TestSubmissionFileServiceOpenForSubmitterSuccess(t *testing.T) {
	topicUID := uuid.New()
	submitterUID := uuid.New()
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{UID: file.SubmissionUID, TopicUID: topicUID, SubmittedBy: submitterUID}, nil
		}},
		&fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			return file, nil
		}},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: file.FieldUID, TopicUID: topicUID}, nil
		}},
		&fakeSubmissionFileStorage{openFn: func(_ context.Context, path string) (io.ReadCloser, error) {
			if path != file.StoragePath {
				t.Fatalf("storage path = %q, want %q", path, file.StoragePath)
			}
			return io.NopCloser(strings.NewReader("owned-file")), nil
		}},
		DefaultMaxUploadSize,
	)

	gotFile, reader, err := service.OpenForSubmitter(context.Background(), file.UID, submitterUID)
	if err != nil {
		t.Fatalf("OpenForSubmitter() error = %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if gotFile != file || string(body) != "owned-file" {
		t.Fatalf("unexpected file/body: file=%v body=%q", gotFile, string(body))
	}
}

func TestSubmissionFileServiceOpenForSubmitterReturnsOwnershipError(t *testing.T) {
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	storageOpened := false
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return nil, submissiondomain.ErrSubmissionNotFound
		}},
		&fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			return file, nil
		}},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: file.FieldUID, TopicUID: uuid.New()}, nil
		}},
		&fakeSubmissionFileStorage{openFn: func(context.Context, string) (io.ReadCloser, error) {
			storageOpened = true
			return nil, nil
		}},
		DefaultMaxUploadSize,
	)

	_, _, err := service.OpenForSubmitter(context.Background(), file.UID, uuid.New())
	if !errors.Is(err, submissiondomain.ErrSubmissionNotFound) {
		t.Fatalf("expected ErrSubmissionNotFound, got %v", err)
	}
	if storageOpened {
		t.Fatal("storage must not be opened when ownership validation fails")
	}
}

func TestSubmissionFileServiceOpenForSubmitterReturnsStorageError(t *testing.T) {
	topicUID := uuid.New()
	submitterUID := uuid.New()
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	expectedErr := errors.New("storage open failed")
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{UID: file.SubmissionUID, TopicUID: topicUID, SubmittedBy: submitterUID}, nil
		}},
		&fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			return file, nil
		}},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: file.FieldUID, TopicUID: topicUID}, nil
		}},
		&fakeSubmissionFileStorage{openFn: func(context.Context, string) (io.ReadCloser, error) {
			return nil, expectedErr
		}},
		DefaultMaxUploadSize,
	)

	_, _, err := service.OpenForSubmitter(context.Background(), file.UID, submitterUID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceDeleteForSubmitterSuccess(t *testing.T) {
	topicUID := uuid.New()
	submitterUID := uuid.New()
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	var calls []string

	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{UID: file.SubmissionUID, TopicUID: topicUID, SubmittedBy: submitterUID}, nil
		}},
		&fakeSubmissionFileRepository{
			findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) { return file, nil },
			deleteFn: func(_ context.Context, gotFileUID uuid.UUID) error {
				if gotFileUID != file.UID {
					t.Fatalf("fileUID = %s, want %s", gotFileUID, file.UID)
				}
				calls = append(calls, "repo")
				return nil
			},
		},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: file.FieldUID, TopicUID: topicUID}, nil
		}},
		&fakeSubmissionFileStorage{deleteFn: func(_ context.Context, path string) error {
			if path != file.StoragePath {
				t.Fatalf("storage path = %q, want %q", path, file.StoragePath)
			}
			calls = append(calls, "storage")
			return nil
		}},
		DefaultMaxUploadSize,
	)

	if err := service.DeleteForSubmitter(context.Background(), file.UID, submitterUID); err != nil {
		t.Fatalf("DeleteForSubmitter() error = %v", err)
	}
	if strings.Join(calls, ",") != "repo,storage" {
		t.Fatalf("delete order = %v, want [repo storage]", calls)
	}
}

func TestSubmissionFileServiceDeleteForSubmitterRejectsOtherSubmitterWithoutDeleting(t *testing.T) {
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	repoDeleted := false
	storageDeleted := false
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return nil, submissiondomain.ErrSubmissionNotFound
		}},
		&fakeSubmissionFileRepository{
			findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) { return file, nil },
			deleteFn: func(context.Context, uuid.UUID) error {
				repoDeleted = true
				return nil
			},
		},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: file.FieldUID, TopicUID: uuid.New()}, nil
		}},
		&fakeSubmissionFileStorage{deleteFn: func(context.Context, string) error {
			storageDeleted = true
			return nil
		}},
		DefaultMaxUploadSize,
	)

	err := service.DeleteForSubmitter(context.Background(), file.UID, uuid.New())
	if !errors.Is(err, submissiondomain.ErrSubmissionNotFound) {
		t.Fatalf("expected ErrSubmissionNotFound, got %v", err)
	}
	if repoDeleted || storageDeleted {
		t.Fatalf("delete should not run after ownership failure: repo=%v storage=%v", repoDeleted, storageDeleted)
	}
}

func TestSubmissionFileServiceDeleteForSubmitterReturnsRepositoryDeleteError(t *testing.T) {
	topicUID := uuid.New()
	submitterUID := uuid.New()
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	expectedErr := errors.New("repository delete failed")
	storageDeleted := false
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{UID: file.SubmissionUID, TopicUID: topicUID, SubmittedBy: submitterUID}, nil
		}},
		&fakeSubmissionFileRepository{
			findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) { return file, nil },
			deleteFn:   func(context.Context, uuid.UUID) error { return expectedErr },
		},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: file.FieldUID, TopicUID: topicUID}, nil
		}},
		&fakeSubmissionFileStorage{deleteFn: func(context.Context, string) error {
			storageDeleted = true
			return nil
		}},
		DefaultMaxUploadSize,
	)

	err := service.DeleteForSubmitter(context.Background(), file.UID, submitterUID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if storageDeleted {
		t.Fatal("storage delete should not run after repository delete error")
	}
}

func TestSubmissionFileServiceDeleteForSubmitterReturnsStorageDeleteError(t *testing.T) {
	topicUID := uuid.New()
	submitterUID := uuid.New()
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	expectedErr := errors.New("storage delete failed")
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{findByIDAndTopicIDAndSubmittedByFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*submissiondomain.Submission, error) {
			return &submissiondomain.Submission{UID: file.SubmissionUID, TopicUID: topicUID, SubmittedBy: submitterUID}, nil
		}},
		&fakeSubmissionFileRepository{
			findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) { return file, nil },
			deleteFn:   func(context.Context, uuid.UUID) error { return nil },
		},
		&fakeFieldRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.TopicField, error) {
			return &topicdomain.TopicField{UID: file.FieldUID, TopicUID: topicUID}, nil
		}},
		&fakeSubmissionFileStorage{deleteFn: func(context.Context, string) error { return expectedErr }},
		DefaultMaxUploadSize,
	)

	err := service.DeleteForSubmitter(context.Background(), file.UID, submitterUID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceOpenSuccess(t *testing.T) {
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			return file, nil
		}},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{openFn: func(_ context.Context, path string) (io.ReadCloser, error) {
			if path != file.StoragePath {
				t.Fatalf("storage path = %q, want %q", path, file.StoragePath)
			}
			return io.NopCloser(strings.NewReader("director-file")), nil
		}},
		DefaultMaxUploadSize,
	)

	gotFile, reader, err := service.Open(context.Background(), file.UID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if gotFile != file || string(body) != "director-file" {
		t.Fatalf("unexpected file/body: file=%v body=%q", gotFile, string(body))
	}
}

func TestSubmissionFileServiceOpenReturnsFileLookupError(t *testing.T) {
	storageOpened := false
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{openFn: func(context.Context, string) (io.ReadCloser, error) {
			storageOpened = true
			return nil, nil
		}},
		DefaultMaxUploadSize,
	)

	_, _, err := service.Open(context.Background(), uuid.New())
	if !errors.Is(err, submissiondomain.ErrSubmissionFileNotFound) {
		t.Fatalf("expected ErrSubmissionFileNotFound, got %v", err)
	}
	if storageOpened {
		t.Fatal("storage should not be opened after file lookup error")
	}
}

func TestSubmissionFileServiceOpenReturnsStorageError(t *testing.T) {
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	expectedErr := errors.New("storage open failed")
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) {
			return file, nil
		}},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{openFn: func(context.Context, string) (io.ReadCloser, error) {
			return nil, expectedErr
		}},
		DefaultMaxUploadSize,
	)

	_, _, err := service.Open(context.Background(), file.UID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceDeleteSuccess(t *testing.T) {
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	var calls []string
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{
			findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) { return file, nil },
			deleteFn: func(context.Context, uuid.UUID) error {
				calls = append(calls, "repo")
				return nil
			},
		},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{deleteFn: func(context.Context, string) error {
			calls = append(calls, "storage")
			return nil
		}},
		DefaultMaxUploadSize,
	)

	if err := service.Delete(context.Background(), file.UID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if strings.Join(calls, ",") != "repo,storage" {
		t.Fatalf("delete order = %v, want [repo storage]", calls)
	}
}

func TestSubmissionFileServiceDeleteReturnsFileLookupError(t *testing.T) {
	repoDeleted := false
	storageDeleted := false
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{deleteFn: func(context.Context, uuid.UUID) error {
			repoDeleted = true
			return nil
		}},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{deleteFn: func(context.Context, string) error {
			storageDeleted = true
			return nil
		}},
		DefaultMaxUploadSize,
	)

	err := service.Delete(context.Background(), uuid.New())
	if !errors.Is(err, submissiondomain.ErrSubmissionFileNotFound) {
		t.Fatalf("expected ErrSubmissionFileNotFound, got %v", err)
	}
	if repoDeleted || storageDeleted {
		t.Fatalf("delete should not run after lookup error: repo=%v storage=%v", repoDeleted, storageDeleted)
	}
}

func TestSubmissionFileServiceDeleteReturnsRepositoryDeleteError(t *testing.T) {
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	expectedErr := errors.New("repository delete failed")
	storageDeleted := false
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{
			findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) { return file, nil },
			deleteFn:   func(context.Context, uuid.UUID) error { return expectedErr },
		},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{deleteFn: func(context.Context, string) error {
			storageDeleted = true
			return nil
		}},
		DefaultMaxUploadSize,
	)

	err := service.Delete(context.Background(), file.UID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if storageDeleted {
		t.Fatal("storage delete should not run after repository delete error")
	}
}

func TestSubmissionFileServiceDeleteReturnsStorageDeleteError(t *testing.T) {
	file := newStoredSubmissionFile(uuid.New(), uuid.New())
	expectedErr := errors.New("storage delete failed")
	service := NewSubmissionFileService(
		&fakeSubmissionRepository{},
		&fakeSubmissionFileRepository{
			findByIDFn: func(context.Context, uuid.UUID) (*submissiondomain.SubmissionFile, error) { return file, nil },
			deleteFn:   func(context.Context, uuid.UUID) error { return nil },
		},
		&fakeFieldRepository{},
		&fakeSubmissionFileStorage{deleteFn: func(context.Context, string) error { return expectedErr }},
		DefaultMaxUploadSize,
	)

	err := service.Delete(context.Background(), file.UID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSubmissionFileServiceDeleteAllRemovesStorageAndMetadata(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	fileA := submissiondomain.SubmissionFile{UID: uuid.New(), SubmissionUID: submissionUID, StoragePath: "submissions/a.pdf"}
	fileB := submissiondomain.SubmissionFile{UID: uuid.New(), SubmissionUID: submissionUID, StoragePath: "submissions/b.jpg"}

	submissionRepo := &fakeSubmissionRepository{findByIDAndTopicIDFn: func(_ context.Context, gotSubmissionUID, gotTopicUID uuid.UUID) (*submissiondomain.Submission, error) {
		if gotSubmissionUID != submissionUID || gotTopicUID != topicUID {
			t.Fatalf("submission lookup = %s / %s", gotSubmissionUID, gotTopicUID)
		}
		return &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID}, nil
	}}
	deletedMetadata := make([]uuid.UUID, 0, 2)
	fileRepo := &fakeSubmissionFileRepository{
		findAllFn: func(_ context.Context, got uuid.UUID) ([]submissiondomain.SubmissionFile, error) {
			if got != submissionUID {
				t.Fatalf("FindAllBySubmissionID uid = %s", got)
			}
			return []submissiondomain.SubmissionFile{fileA, fileB}, nil
		},
		deleteFn: func(_ context.Context, got uuid.UUID) error {
			deletedMetadata = append(deletedMetadata, got)
			return nil
		},
	}
	deletedStorage := make([]string, 0, 2)
	storage := &fakeSubmissionFileStorage{deleteFn: func(_ context.Context, path string) error {
		deletedStorage = append(deletedStorage, path)
		return nil
	}}

	service := NewSubmissionFileService(submissionRepo, fileRepo, &fakeFieldRepository{}, storage, DefaultMaxUploadSize)
	if err := service.DeleteAll(context.Background(), topicUID, submissionUID); err != nil {
		t.Fatalf("DeleteAll() error = %v", err)
	}
	if len(deletedStorage) != 2 || deletedStorage[0] != fileA.StoragePath || deletedStorage[1] != fileB.StoragePath {
		t.Fatalf("deleted storage = %+v", deletedStorage)
	}
	if len(deletedMetadata) != 2 || deletedMetadata[0] != fileA.UID || deletedMetadata[1] != fileB.UID {
		t.Fatalf("deleted metadata = %+v", deletedMetadata)
	}
}

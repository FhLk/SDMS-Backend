package upload

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUploadRejectsSizeHintOverLimitBeforeStorage(t *testing.T) {
	service := NewService(nil, nil, 24*time.Hour)
	_, err := service.Upload(context.Background(), UploadInput{
		TopicUUID:      uuid.New(),
		FieldUUID:      uuid.New(),
		UploadedByUUID: uuid.New(),
		OriginalName:   "large.pdf",
		MimeType:       "application/pdf",
		SizeHint:       MaxFileSize + 1,
		Content:        strings.NewReader("not read"),
	})
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("Upload() error = %v, want ErrFileTooLarge", err)
	}
}

func TestUploadSanitizesClientFilename(t *testing.T) {
	if got, want := cleanOriginalName(`C:\fakepath\report.PDF`), "report.PDF"; got != want {
		t.Fatalf("cleanOriginalName() = %q, want %q", got, want)
	}
	if got, want := safeExtension("report.PDF"), ".pdf"; got != want {
		t.Fatalf("safeExtension() = %q, want %q", got, want)
	}
}

func TestSpoolUploadRejectsActualContentOverLimit(t *testing.T) {
	reader := io.LimitReader(zeroReader{}, MaxFileSize+1)
	file, _, err := spoolUpload(reader)
	if file != nil {
		_ = file.Close()
		t.Fatal("spoolUpload() returned a file for oversized content")
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("spoolUpload() error = %v, want ErrFileTooLarge", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 0
	}
	return len(buffer), nil
}

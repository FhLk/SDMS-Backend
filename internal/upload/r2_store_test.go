package upload

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestR2StoreBuildsPrefixUploadsAndDeletes(t *testing.T) {
	client := &fakeR2Client{}
	store := &R2Store{client: client, bucket: "sdms-files", prefix: "sdms"}
	yearPrefix, err := store.EnsureFolder(context.Background(), store.RootID(), "2569")
	if err != nil {
		t.Fatalf("EnsureFolder(year) error = %v", err)
	}
	divisionPrefix, err := store.EnsureFolder(context.Background(), yearPrefix, "aca")
	if err != nil {
		t.Fatalf("EnsureFolder(division) error = %v", err)
	}
	objectKey, err := store.Put(context.Background(), divisionPrefix, "evidence.pdf", "application/pdf", 8, strings.NewReader("document"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if got, want := objectKey, "sdms/2569/aca/evidence.pdf"; got != want {
		t.Fatalf("object key = %q, want %q", got, want)
	}
	if client.put == nil || aws.ToString(client.put.Bucket) != "sdms-files" || aws.ToInt64(client.put.ContentLength) != 8 {
		t.Fatalf("PutObject input = %+v", client.put)
	}
	content, err := io.ReadAll(client.put.Body)
	if err != nil {
		t.Fatalf("read PutObject body: %v", err)
	}
	if string(content) != "document" || aws.ToString(client.put.ContentType) != "application/pdf" {
		t.Fatalf("uploaded content/type = %q / %q", content, aws.ToString(client.put.ContentType))
	}
	download, err := store.Open(context.Background(), objectKey)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	downloaded, err := io.ReadAll(download)
	if err != nil {
		t.Fatalf("read downloaded object: %v", err)
	}
	if err := download.Close(); err != nil {
		t.Fatalf("close downloaded object: %v", err)
	}
	if got, want := string(downloaded), "stored object"; got != want {
		t.Fatalf("downloaded content = %q, want %q", got, want)
	}
	if client.got == nil || aws.ToString(client.got.Key) != objectKey {
		t.Fatalf("GetObject input = %+v", client.got)
	}
	if err := store.Trash(context.Background(), objectKey); err != nil {
		t.Fatalf("Trash() error = %v", err)
	}
	if client.deleted == nil || aws.ToString(client.deleted.Key) != objectKey {
		t.Fatalf("DeleteObject input = %+v", client.deleted)
	}
}

func TestNewR2StoreBuildsOfficialEndpoint(t *testing.T) {
	store, err := NewR2Store(context.Background(), config.R2Config{
		AccountID:       "account-id",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		Bucket:          "sdms-files",
		Prefix:          "school/sdms",
	})
	if err != nil {
		t.Fatalf("NewR2Store() error = %v", err)
	}
	if store.RootID() != "school/sdms" || store.bucket != "sdms-files" {
		t.Fatalf("R2 store = prefix %q bucket %q", store.RootID(), store.bucket)
	}
}

func TestR2PrefixRejectsTraversal(t *testing.T) {
	if _, err := normalizeR2Prefix("school/../outside"); err == nil {
		t.Fatal("normalizeR2Prefix() error = nil, want traversal rejection")
	}
}

type fakeR2Client struct {
	put     *s3.PutObjectInput
	got     *s3.GetObjectInput
	deleted *s3.DeleteObjectInput
}

func (f *fakeR2Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.put = input
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeR2Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.got = input
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("stored object"))}, nil
}

func (f *fakeR2Client) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.deleted = input
	return &s3.DeleteObjectOutput{}, nil
}

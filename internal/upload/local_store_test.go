package upload

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStoreCreatesHierarchyAndTrashesFile(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	ctx := context.Background()
	yearID, err := store.EnsureFolder(ctx, store.RootID(), "2569")
	if err != nil {
		t.Fatalf("EnsureFolder(year) error = %v", err)
	}
	divisionID, err := store.EnsureFolder(ctx, yearID, "aca")
	if err != nil {
		t.Fatalf("EnsureFolder(division) error = %v", err)
	}
	fileID, err := store.Put(ctx, divisionID, "0199-test.pdf", "application/pdf", 8, strings.NewReader("evidence"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	path := filepath.Join(store.root, filepath.FromSlash(fileID))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if got, want := string(content), "evidence"; got != want {
		t.Fatalf("stored content = %q, want %q", got, want)
	}
	stream, err := store.Open(ctx, fileID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	streamed, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read opened stream: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close opened stream: %v", err)
	}
	if got, want := string(streamed), "evidence"; got != want {
		t.Fatalf("opened content = %q, want %q", got, want)
	}
	if err := store.Trash(ctx, fileID); err != nil {
		t.Fatalf("Trash() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("trashed file stat error = %v, want not exist", err)
	}
	// การลบซ้ำต้องสำเร็จ เพื่อรองรับกรณี storage สำเร็จแต่ DB transaction rollback.
	if err := store.Trash(ctx, fileID); err != nil {
		t.Fatalf("second Trash() error = %v", err)
	}
}

func TestLocalStoreRejectsPathTraversal(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	if _, err := store.Put(context.Background(), "../outside", "file.txt", "text/plain", 1, strings.NewReader("x")); err == nil {
		t.Fatal("Put() error = nil, want path traversal rejection")
	}
	if _, err := store.Put(context.Background(), "", "../file.txt", "text/plain", 1, strings.NewReader("x")); err == nil {
		t.Fatal("Put() unsafe name error = nil, want rejection")
	}
	if _, err := store.Open(context.Background(), "../outside.txt"); err == nil {
		t.Fatal("Open() traversal error = nil, want rejection")
	}
}

func TestLocalStoreRemovesPartialFileWhenReaderFails(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	reader := io.MultiReader(strings.NewReader("partial"), errorReader{})
	if _, err := store.Put(context.Background(), "", "partial.txt", "text/plain", 7, reader); err == nil {
		t.Fatal("Put() error = nil, want reader failure")
	}
	if _, err := os.Stat(filepath.Join(store.root, "partial.txt")); !os.IsNotExist(err) {
		t.Fatalf("partial file stat error = %v, want not exist", err)
	}
}

func TestLocalStoreFileIDFitsDatabaseColumn(t *testing.T) {
	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore() error = %v", err)
	}
	ctx := context.Background()
	yearID, err := store.EnsureFolder(ctx, "", "32767")
	if err != nil {
		t.Fatalf("EnsureFolder(year) error = %v", err)
	}
	divisionID, err := store.EnsureFolder(ctx, yearID, "division")
	if err != nil {
		t.Fatalf("EnsureFolder(division) error = %v", err)
	}
	topicName := strings.Repeat("t", 20) + "-0199d6d0-bec9-7aa0-b2d8-96dc6fb51001"
	topicID, err := store.EnsureFolder(ctx, divisionID, topicName)
	if err != nil {
		t.Fatalf("EnsureFolder(topic) error = %v", err)
	}
	fileName := "0199d6d0-bec9-7aa0-b2d8-96dc6fb51002.abcdefghijklmno"
	fileID, err := store.Put(ctx, topicID, fileName, "application/octet-stream", 1, strings.NewReader("x"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if got, maximum := len(fileID), 128; got > maximum {
		t.Fatalf("local file ID length = %d, want at most %d: %q", got, maximum, fileID)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

package portfolio

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type trackingStore struct {
	mu        sync.Mutex
	objects   map[string][]byte
	active    int
	maxActive int
}

func (s *trackingStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, exists := s.objects[key]
	if !exists {
		return nil, io.ErrUnexpectedEOF
	}
	s.active++
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	return &trackingReadCloser{
		Reader: bytes.NewReader(content),
		close: func() {
			s.mu.Lock()
			s.active--
			s.mu.Unlock()
		},
	}, nil
}

type trackingReadCloser struct {
	*bytes.Reader
	close func()
	once  sync.Once
}

func (r *trackingReadCloser) Close() error {
	r.once.Do(r.close)
	return nil
}

func TestArchiveStreamOpensOneObjectAtATimeAndUsesSafeNames(t *testing.T) {
	firstUUID := uuid.MustParse("0199d70a-0000-7000-8000-000000000001")
	secondUUID := uuid.MustParse("0199d70a-0000-7000-8000-000000000002")
	store := &trackingStore{objects: map[string][]byte{
		"first":  []byte("เนื้อหาไฟล์แรก"),
		"second": bytes.Repeat([]byte("second-file"), 1024),
	}}
	files := []archiveFileRow{
		{
			FileUUID: firstUUID, ObjectKey: "first", OriginalName: `../../รายงาน?.txt`,
			AcademicYear: 2569, DivisionCode: `aca/../other`, TopicTitle: `../หัวข้อ`, CreatedAt: time.Now(),
		},
		{
			FileUUID: secondUUID, ObjectKey: "second", OriginalName: `หลักฐาน\สรุป.pdf`,
			AcademicYear: 2569, DivisionCode: "aca", TopicTitle: "หัวข้อ", CreatedAt: time.Now(),
		},
	}

	stream := newArchiveStream(store, files)
	compressed, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read archive stream: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close archive stream: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(compressed), int64(len(compressed)))
	if err != nil {
		t.Fatalf("open generated ZIP: %v", err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("ZIP entries = %d, want 2", len(reader.File))
	}
	for _, entry := range reader.File {
		if strings.Contains(entry.Name, "\\") || strings.Contains(entry.Name, "/../") || strings.HasPrefix(entry.Name, "../") {
			t.Fatalf("unsafe ZIP entry name = %q", entry.Name)
		}
		if !strings.HasPrefix(entry.Name, "2569/") {
			t.Fatalf("ZIP entry name = %q, want academic-year folder", entry.Name)
		}
		content, err := entry.Open()
		if err != nil {
			t.Fatalf("open ZIP entry %q: %v", entry.Name, err)
		}
		if _, err := io.Copy(io.Discard, content); err != nil {
			t.Fatalf("read ZIP entry %q: %v", entry.Name, err)
		}
		_ = content.Close()
	}

	store.mu.Lock()
	maxActive := store.maxActive
	active := store.active
	store.mu.Unlock()
	if maxActive != 1 || active != 0 {
		t.Fatalf("open object streams: max=%d active=%d, want max=1 active=0", maxActive, active)
	}
}

func TestArchiveStreamProducesValidEmptyZIP(t *testing.T) {
	stream := newArchiveStream(&trackingStore{objects: map[string][]byte{}}, nil)
	compressed, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read empty archive: %v", err)
	}
	_ = stream.Close()
	reader, err := zip.NewReader(bytes.NewReader(compressed), int64(len(compressed)))
	if err != nil {
		t.Fatalf("open empty ZIP: %v", err)
	}
	if len(reader.File) != 0 {
		t.Fatalf("empty ZIP entries = %d, want 0", len(reader.File))
	}
}

func TestEscapeLikeTreatsWildcardCharactersAsText(t *testing.T) {
	if got, want := escapeLike(`100%_ครบ\แล้ว`), `100\%\_ครบ\\แล้ว`; got != want {
		t.Fatalf("escapeLike() = %q, want %q", got, want)
	}
}

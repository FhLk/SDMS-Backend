package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStore เก็บไฟล์ใต้ directory เดียว เหมาะกับการพัฒนาโดยไม่ต้องต่อ Cloudflare R2.
type LocalStore struct {
	root string
}

func NewLocalStore(directory string) (*LocalStore, error) {
	root, err := filepath.Abs(strings.TrimSpace(directory))
	if err != nil {
		return nil, fmt.Errorf("resolve local upload directory: %w", err)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create local upload directory: %w", err)
	}
	return &LocalStore{root: root}, nil
}

// RootID ใช้ค่าว่างแทนราก เพราะ folder ID ของ local store เป็น relative path ใต้ root.
func (s *LocalStore) RootID() string { return "" }

func (s *LocalStore) EnsureFolder(_ context.Context, parentID, name string) (string, error) {
	parent, err := s.resolve(parentID)
	if err != nil {
		return "", err
	}
	if err := validatePathSegment(name); err != nil {
		return "", err
	}

	folder := filepath.Join(parent, name)
	if err := os.MkdirAll(folder, 0o750); err != nil {
		return "", fmt.Errorf("create local upload folder: %w", err)
	}
	relative, err := filepath.Rel(s.root, folder)
	if err != nil {
		return "", fmt.Errorf("build local folder id: %w", err)
	}
	return filepath.ToSlash(relative), nil
}

func (s *LocalStore) Put(_ context.Context, folderID, name, _ string, _ int64, content io.Reader) (string, error) {
	folder, err := s.resolve(folderID)
	if err != nil {
		return "", err
	}
	if err := validatePathSegment(name); err != nil {
		return "", err
	}

	path := filepath.Join(folder, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", fmt.Errorf("create local upload: %w", err)
	}
	// หากอ่านไฟล์ต้นทางไม่สำเร็จ ต้องเอาไฟล์บางส่วนออก ไม่ให้ cleanup มองไม่เห็นไฟล์กำพร้า.
	if _, copyErr := io.Copy(file, content); copyErr != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write local upload: %w", copyErr)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close local upload: %w", err)
	}

	relative, err := filepath.Rel(s.root, path)
	if err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("build local file id: %w", err)
	}
	return filepath.ToSlash(relative), nil
}

// Open เปิดไฟล์แบบ stream หลังตรวจว่า object ID ยังอยู่ภายใน storage root.
// ผู้เรียกไม่ต้องอ่านไฟล์ทั้งหมดเข้า memory และต้องส่งต่อ ReadCloser ให้ HTTP layer ปิดหลังส่งเสร็จ.
func (s *LocalStore) Open(_ context.Context, fileID string) (io.ReadCloser, error) {
	path, err := s.resolve(fileID)
	if err != nil {
		return nil, err
	}
	if path == s.root {
		return nil, errors.New("local file id must not point to storage root")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open local upload: %w", err)
	}
	return file, nil
}

func (s *LocalStore) Trash(_ context.Context, fileID string) error {
	path, err := s.resolve(fileID)
	if err != nil {
		return err
	}
	if path == s.root {
		return errors.New("local file id must not point to storage root")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("trash local upload: %w", err)
	}
	return nil
}

// resolve ป้องกัน path traversal ก่อนนำ ID จากฐานข้อมูลไปแตะ filesystem.
func (s *LocalStore) resolve(id string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(id)))
	if clean == "." {
		return s.root, nil
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("local object id is outside storage root")
	}

	resolved := filepath.Join(s.root, clean)
	relative, err := filepath.Rel(s.root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("local object id is outside storage root")
	}
	return resolved, nil
}

func validatePathSegment(value string) error {
	if value == "" || value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsAny(value, `/\\`) {
		return errors.New("object name must be one safe path segment")
	}
	return nil
}

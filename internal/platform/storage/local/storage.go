package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Storage struct {
	root string
}

func New(root string) (*Storage, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "uploads"
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return nil, err
	}

	return &Storage{root: absRoot}, nil
}

func (s *Storage) Save(
	ctx context.Context,
	relativePath string,
	src io.Reader,
) error {
	path, err := s.safePath(relativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}

	_, copyErr := copyWithContext(ctx, dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return copyErr
	}
	return closeErr
}

func (s *Storage) Open(
	ctx context.Context,
	relativePath string,
) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.safePath(relativePath)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *Storage) Delete(
	ctx context.Context,
	relativePath string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.safePath(relativePath)
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Storage) safePath(relativePath string) (string, error) {
	clean := filepath.Clean(relativePath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", os.ErrPermission
	}

	path := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", os.ErrPermission
	}
	return path, nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}

		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}

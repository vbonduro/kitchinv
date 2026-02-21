package local

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalPhotoStore struct {
	basePath string
}

func NewLocalPhotoStore(basePath string) (*LocalPhotoStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create photo directory: %w", err)
	}
	return &LocalPhotoStore{basePath: basePath}, nil
}

func (s *LocalPhotoStore) Save(ctx context.Context, prefix, mimeType string, r io.Reader) (string, error) {
	filename := fmt.Sprintf("%s_%d%s", prefix, time.Now().UnixNano(), mimeTypeToExt(mimeType))
	filePath := filepath.Join(s.basePath, filename)

	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		if cerr := f.Close(); cerr != nil {
			slog.Error("failed to close file after write error", "error", cerr)
		}
		if rerr := os.Remove(filePath); rerr != nil {
			slog.Error("failed to remove file after write error", "error", rerr)
		}
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	if err := f.Close(); err != nil {
		if rerr := os.Remove(filePath); rerr != nil {
			slog.Error("failed to remove file after close error", "error", rerr)
		}
		return "", fmt.Errorf("failed to close file: %w", err)
	}
	return filename, nil
}

func (s *LocalPhotoStore) Get(ctx context.Context, storageKey string) (io.ReadCloser, string, error) {
	filePath, err := s.safeJoin(storageKey)
	if err != nil {
		return nil, "", err
	}

	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("photo not found")
		}
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}
	return f, extToMimeType(filePath), nil
}

func (s *LocalPhotoStore) Delete(ctx context.Context, storageKey string) error {
	filePath, err := s.safeJoin(storageKey)
	if err != nil {
		return err
	}

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("photo not found")
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// safeJoin resolves storageKey relative to basePath and rejects directory traversal.
func (s *LocalPhotoStore) safeJoin(storageKey string) (string, error) {
	absBase, err := filepath.Abs(s.basePath)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}

	absPath, err := filepath.Abs(filepath.Join(s.basePath, storageKey))
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal attempt")
	}
	return absPath, nil
}

func mimeTypeToExt(mimeType string) string {
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func extToMimeType(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

package assets

import (
	"context"
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

type LocalStore struct {
	rootDir string
}

func NewLocalStore(rootDir string) *LocalStore {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		rootDir = defaultLocalRoot
	}
	return &LocalStore{rootDir: rootDir}
}

func (s *LocalStore) Save(ctx context.Context, key string, body io.Reader, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tempPath := path + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return os.WriteFile(path+".content-type", []byte(contentTypeOrDefault(contentType)), 0o644)
}

func (s *LocalStore) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	path, err := s.pathForKey(key)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	contentTypeBytes, err := os.ReadFile(path + ".content-type")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = file.Close()
		return nil, "", err
	}
	return file, localContentType(path, string(contentTypeBytes)), nil
}

func (s *LocalStore) DeletePrefix(ctx context.Context, prefix string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cleanPrefix, err := cleanKey(prefix)
	if err != nil {
		return err
	}
	path := filepath.Join(s.rootDir, filepath.FromSlash(localDiskKey(cleanPrefix)))
	root, err := filepath.Abs(s.rootDir)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if target == root || !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return ErrNotFound
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return nil
}

func (s *LocalStore) pathForKey(key string) (string, error) {
	clean, err := cleanKey(key)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.rootDir)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(s.rootDir, filepath.FromSlash(localDiskKey(clean))))
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", ErrNotFound
	}
	return target, nil
}

func localDiskKey(cleanKey string) string {
	// Historical design-plan rows store keys as uploads/{upload_id}/...
	// while local deployments used UPLOAD_DIR/{upload_id}/... on disk.
	// Supabase keeps the full logical key; LocalStore maps it back to the
	// existing local directory shape so old assets continue to open.
	return strings.TrimPrefix(cleanKey, "uploads/")
}

func localContentType(path string, stored string) string {
	if contentType := strings.TrimSpace(stored); contentType != "" {
		return contentType
	}
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); contentType != "" {
		return contentType
	}
	return contentTypeOrDefault("")
}

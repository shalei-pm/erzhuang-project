package storespace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultSnapshotDir = "uploads/channel-snapshots"
	maxSnapshotBytes   = 6 << 20
)

type SnapshotStore interface {
	SaveRemote(ctx context.Context, imageURL string) (string, error)
	FilePath(name string) (string, error)
}

type LocalSnapshotStore struct {
	rootDir string
	client  *http.Client
}

func NewLocalSnapshotStore(rootDir string) *LocalSnapshotStore {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		rootDir = defaultSnapshotDir
	}
	return &LocalSnapshotStore{
		rootDir: rootDir,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func NewLocalSnapshotStoreFromEnv() *LocalSnapshotStore {
	rootDir := strings.TrimSpace(os.Getenv("CHANNEL_SNAPSHOT_DIR"))
	if rootDir == "" {
		rootDir = defaultSnapshotDir
	}
	return NewLocalSnapshotStore(rootDir)
}

func (s *LocalSnapshotStore) SaveRemote(ctx context.Context, imageURL string) (string, error) {
	if s == nil {
		return "", errors.New("snapshot store is not configured")
	}
	parsed, err := url.Parse(strings.TrimSpace(imageURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid snapshot image url")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download snapshot failed: http %d", response.StatusCode)
	}

	extension := snapshotExtension(response.Header.Get("Content-Type"), parsed.Path)
	name, err := newSnapshotName(extension)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return "", err
	}
	targetPath := filepath.Join(s.rootDir, name)
	tempPath := targetPath + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxSnapshotBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return "", closeErr
	}
	if written > maxSnapshotBytes {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("snapshot image exceeds %d bytes", maxSnapshotBytes)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	return "/api/store-space/channel-snapshots/" + name, nil
}

func (s *LocalSnapshotStore) FilePath(name string) (string, error) {
	if s == nil || !validSnapshotName(name) {
		return "", ErrNotFound
	}
	path := filepath.Join(s.rootDir, name)
	cleanRoot, err := filepath.Abs(s.rootDir)
	if err != nil {
		return "", err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return "", ErrNotFound
	}
	if _, err := os.Stat(cleanPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotFound
		}
		return "", err
	}
	return cleanPath, nil
}

func snapshotExtension(contentType string, path string) string {
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		switch mediaType {
		case "image/jpeg":
			return ".jpg"
		case "image/png":
			return ".png"
		case "image/webp":
			return ".webp"
		}
	}
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".jpg", ".jpeg":
		return ".jpg"
	case ".png", ".webp":
		return extension
	default:
		return ".jpg"
	}
}

func newSnapshotName(extension string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes) + extension, nil
}

func validSnapshotName(name string) bool {
	if len(name) < len("0.jpg") || strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return false
	}
	extension := strings.ToLower(filepath.Ext(name))
	if extension != ".jpg" && extension != ".png" && extension != ".webp" {
		return false
	}
	base := strings.TrimSuffix(name, extension)
	if len(base) != 32 {
		return false
	}
	for _, ch := range base {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			return false
		}
	}
	return true
}

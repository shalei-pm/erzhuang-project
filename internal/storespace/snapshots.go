package storespace

import (
	"bytes"
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

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

const (
	defaultSnapshotDir = "uploads/channel-snapshots"
	maxSnapshotBytes   = 6 << 20
)

type SnapshotStore interface {
	SaveRemote(ctx context.Context, imageURL string) (string, error)
	Open(ctx context.Context, name string) (io.ReadCloser, string, error)
}

type LocalSnapshotStore struct {
	store  assets.Store
	client *http.Client
}

func NewLocalSnapshotStore(rootDir string) *LocalSnapshotStore {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		rootDir = defaultSnapshotDir
	}
	return NewAssetSnapshotStore(assets.NewLocalStore(rootDir))
}

func NewLocalSnapshotStoreFromEnv() *LocalSnapshotStore {
	rootDir := strings.TrimSpace(os.Getenv("CHANNEL_SNAPSHOT_DIR"))
	if rootDir == "" {
		rootDir = defaultSnapshotDir
	}
	return NewLocalSnapshotStore(rootDir)
}

func NewAssetSnapshotStore(store assets.Store) *LocalSnapshotStore {
	if store == nil {
		store = assets.NewLocalStore(defaultSnapshotDir)
	}
	return &LocalSnapshotStore{
		store:  store,
		client: &http.Client{Timeout: 30 * time.Second},
	}
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
	body := io.LimitReader(response.Body, maxSnapshotBytes+1)
	limited, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	if len(limited) > maxSnapshotBytes {
		return "", fmt.Errorf("snapshot image exceeds %d bytes", maxSnapshotBytes)
	}
	if err := s.store.Save(ctx, snapshotKey(name), bytes.NewReader(limited), contentTypeOrDefault(response.Header.Get("Content-Type"), extension)); err != nil {
		return "", err
	}
	return "/api/store-space/channel-snapshots/" + name, nil
}

func (s *LocalSnapshotStore) Open(ctx context.Context, name string) (io.ReadCloser, string, error) {
	if s == nil || !validSnapshotName(name) {
		return nil, "", ErrNotFound
	}
	reader, contentType, err := s.store.Open(ctx, snapshotKey(name))
	if errors.Is(err, assets.ErrNotFound) {
		return nil, "", ErrNotFound
	}
	return reader, contentType, err
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

func snapshotKey(name string) string {
	return filepath.ToSlash(filepath.Join("channel-snapshots", name))
}

func snapshotKeyForDiagnostics(name string) string {
	if !validSnapshotName(name) {
		return ""
	}
	return snapshotKey(name)
}

func contentTypeOrDefault(contentType string, extension string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType != "" {
		return contentType
	}
	switch extension {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

package designplan

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

func TestUploadManagerDeleteStoredFilesRemovesUploadDirectory(t *testing.T) {
	root := t.TempDir()
	manager := &UploadManager{rootDir: root}
	dir := filepath.Join(root, "tmp_123_test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir upload dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "preview.png"), []byte("preview"), 0o644); err != nil {
		t.Fatalf("write preview: %v", err)
	}

	if err := manager.DeleteStoredFiles("uploads/tmp_123_test/preview.png"); err != nil {
		t.Fatalf("delete stored files: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected upload dir removed, stat err=%v", err)
	}
}

func TestUploadManagerDeleteStoredFilesIgnoresInvalidStoredPath(t *testing.T) {
	root := t.TempDir()
	manager := &UploadManager{rootDir: root}
	keep := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write keep file: %v", err)
	}

	if err := manager.DeleteStoredFiles("../keep.txt", "uploads/bad/unknown.txt"); err != nil {
		t.Fatalf("invalid stored paths should be ignored: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("expected keep file to remain: %v", err)
	}
}

func TestUploadManagerSavesAndOpensStoredAssetsThroughAssetStore(t *testing.T) {
	root := t.TempDir()
	store := assets.NewLocalStore(filepath.Join(root, "asset-store"))
	manager := NewUploadManager(filepath.Join(root, "work"), store)
	source := filepath.Join(root, "preview.png")
	if err := os.WriteFile(source, []byte("preview-data"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := manager.saveFile(context.Background(), "uploads/tmp_123_test/preview.png", source, "image/png"); err != nil {
		t.Fatalf("save file: %v", err)
	}
	reader, contentType, err := manager.OpenStored("uploads/tmp_123_test/preview.png")
	if err != nil {
		t.Fatalf("open stored: %v", err)
	}
	body, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		t.Fatalf("read stored: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close stored: %v", closeErr)
	}
	if contentType != "image/png" {
		t.Fatalf("expected image/png, got %q", contentType)
	}
	if string(body) != "preview-data" {
		t.Fatalf("expected stored preview data, got %q", string(body))
	}

	if err := manager.DeleteStoredFiles("uploads/tmp_123_test/preview.png"); err != nil {
		t.Fatalf("delete stored files: %v", err)
	}
	_, _, err = manager.OpenStored("uploads/tmp_123_test/preview.png")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected missing asset after delete, got %v", err)
	}
}

package designplan

import (
	"os"
	"path/filepath"
	"testing"
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

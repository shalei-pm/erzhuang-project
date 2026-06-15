package designplan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// TODO(container): the current upload manager stores generated PDFs/images on
	// the container local filesystem. In Kubernetes this data is lost when a Pod
	// is recreated unless UPLOAD_DIR is backed by a PVC. Prefer moving design-plan
	// assets to object storage, or at minimum require a persistent volume mount,
	// before treating uploaded previews as durable production data.
	defaultUploadDir = "uploads/design-plan"
	maxPDFBytes      = 5 << 20
	maxPDFPages      = 5
)

type UploadAssetKind string

const (
	UploadAssetOriginal  UploadAssetKind = "original"
	UploadAssetPreview   UploadAssetKind = "preview"
	UploadAssetThumbnail UploadAssetKind = "thumbnail"
)

type UploadInput struct {
	File       multipart.File
	FileName   string
	Header     []byte
	Size       int64
	URLPrefix  string
	MaxPDFSize int64
}

type UploadManager struct {
	rootDir string
}

func NewUploadManagerFromEnv() *UploadManager {
	rootDir := strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
	if rootDir == "" {
		rootDir = defaultUploadDir
	}
	return &UploadManager{rootDir: rootDir}
}

func (m *UploadManager) Save(ctx context.Context, input UploadInput) (*UploadResult, error) {
	if input.File == nil {
		return nil, &ValidationError{Fields: map[string]string{"file": "PDF 文件不能为空"}}
	}
	defer input.File.Close()

	limit := input.MaxPDFSize
	if limit <= 0 {
		limit = maxPDFBytes
	}
	if input.Size > limit {
		return nil, &ValidationError{Fields: map[string]string{"file": "文件过大，请上传 5MB 以内的 PDF"}}
	}
	if !isPDF(input.FileName, input.Header) {
		return nil, &ValidationError{Fields: map[string]string{"file": "仅支持 PDF 文件"}}
	}

	uploadID, err := newUploadID()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(m.rootDir, uploadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	originalPath := filepath.Join(dir, "original.pdf")
	if err := writeUploadedFile(originalPath, input.Header, input.File, limit); err != nil {
		return nil, err
	}

	pagePaths, err := renderPDF(ctx, originalPath, filepath.Join(dir, "page"))
	if err != nil {
		return nil, err
	}
	if len(pagePaths) == 0 {
		return nil, errors.New("pdf render produced no pages")
	}
	if len(pagePaths) > maxPDFPages {
		return nil, &ValidationError{Fields: map[string]string{"file": "当前最多支持 5 页 PDF，请拆分后上传"}}
	}

	previewPath := filepath.Join(dir, "preview.png")
	thumbnailPath := filepath.Join(dir, "thumbnail.png")
	if err := stitchPNGs(pagePaths, previewPath); err != nil {
		return nil, err
	}
	if err := createThumbnail(previewPath, thumbnailPath, 360); err != nil {
		return nil, err
	}

	relativeOriginal := storedPath(uploadID, "original.pdf")
	relativePreview := storedPath(uploadID, "preview.png")
	relativeThumbnail := storedPath(uploadID, "thumbnail.png")

	return &UploadResult{
		UploadID:      uploadID,
		FileName:      filepath.Base(input.FileName),
		PageCount:     len(pagePaths),
		OriginalPath:  relativeOriginal,
		PreviewPath:   relativePreview,
		ThumbnailPath: relativeThumbnail,
		PreviewURL:    uploadURL(input.URLPrefix, uploadID, UploadAssetPreview),
		ThumbnailURL:  uploadURL(input.URLPrefix, uploadID, UploadAssetThumbnail),
	}, nil
}

func (m *UploadManager) Find(uploadID string) (*UploadResult, error) {
	if !validUploadID(uploadID) {
		return nil, ErrNotFound
	}
	dir := filepath.Join(m.rootDir, uploadID)
	if _, err := os.Stat(filepath.Join(dir, "preview.png")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &UploadResult{
		UploadID:      uploadID,
		FileName:      "original.pdf",
		PageCount:     1,
		OriginalPath:  storedPath(uploadID, "original.pdf"),
		PreviewPath:   storedPath(uploadID, "preview.png"),
		ThumbnailPath: storedPath(uploadID, "thumbnail.png"),
		PreviewURL:    uploadURL("", uploadID, UploadAssetPreview),
		ThumbnailURL:  uploadURL("", uploadID, UploadAssetThumbnail),
	}, nil
}

func (m *UploadManager) FilePath(uploadID string, kind UploadAssetKind) (string, error) {
	if !validUploadID(uploadID) {
		return "", ErrNotFound
	}
	name := "preview.png"
	switch kind {
	case UploadAssetOriginal:
		name = "original.pdf"
	case UploadAssetPreview:
		name = "preview.png"
	case UploadAssetThumbnail:
		name = "thumbnail.png"
	default:
		return "", ErrNotFound
	}
	path := filepath.Join(m.rootDir, uploadID, name)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotFound
		}
		return "", err
	}
	return path, nil
}

func (m *UploadManager) StoredFilePath(value string) (string, error) {
	uploadID, name, ok := parseStoredPath(value)
	if !ok {
		return "", ErrNotFound
	}
	if !validUploadID(uploadID) {
		return "", ErrNotFound
	}
	path := filepath.Join(m.rootDir, uploadID, name)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotFound
		}
		return "", err
	}
	return path, nil
}

func (m *UploadManager) DeleteStoredFiles(values ...string) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		uploadID, _, ok := parseStoredPath(value)
		if !ok || !validUploadID(uploadID) {
			continue
		}
		if _, exists := seen[uploadID]; exists {
			continue
		}
		seen[uploadID] = struct{}{}
		dir := filepath.Join(m.rootDir, uploadID)
		cleanRoot, err := filepath.Abs(m.rootDir)
		if err != nil {
			return err
		}
		cleanDir, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		if cleanDir == cleanRoot || !strings.HasPrefix(cleanDir, cleanRoot+string(os.PathSeparator)) {
			return fmt.Errorf("refuse to delete path outside upload root: %s", cleanDir)
		}
		if err := os.RemoveAll(cleanDir); err != nil {
			return err
		}
	}
	return nil
}

func writeUploadedFile(path string, header []byte, file multipart.File, limit int64) error {
	target, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer target.Close()

	if len(header) > 0 {
		if _, err := target.Write(header); err != nil {
			return err
		}
	}
	_, err = io.Copy(target, io.LimitReader(file, limit+1-int64(len(header))))
	return err
}

func renderPDF(ctx context.Context, pdfPath string, outputPrefix string) ([]string, error) {
	command := exec.CommandContext(ctx, "pdftoppm", "-png", "-r", "160", "-f", "1", "-l", fmt.Sprintf("%d", maxPDFPages+1), pdfPath, outputPrefix)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pdf render failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	matches, err := filepath.Glob(outputPrefix + "-*.png")
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

func stitchPNGs(pagePaths []string, outputPath string) error {
	type decodedPage struct {
		image  image.Image
		width  int
		height int
	}
	pages := []decodedPage{}
	maxWidth := 0
	totalHeight := 0

	for _, path := range pagePaths {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		img, err := png.Decode(file)
		file.Close()
		if err != nil {
			return err
		}
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		if width > maxWidth {
			maxWidth = width
		}
		totalHeight += height
		pages = append(pages, decodedPage{image: img, width: width, height: height})
	}

	canvas := image.NewRGBA(image.Rect(0, 0, maxWidth, totalHeight))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	y := 0
	for _, page := range pages {
		x := (maxWidth - page.width) / 2
		target := image.Rect(x, y, x+page.width, y+page.height)
		draw.Draw(canvas, target, page.image, page.image.Bounds().Min, draw.Over)
		y += page.height
	}

	return writePNG(outputPath, canvas)
}

func createThumbnail(inputPath string, outputPath string, maxWidth int) error {
	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	img, err := png.Decode(file)
	file.Close()
	if err != nil {
		return err
	}
	bounds := img.Bounds()
	sourceWidth := bounds.Dx()
	sourceHeight := bounds.Dy()
	if sourceWidth <= maxWidth {
		return writePNG(outputPath, img)
	}
	targetWidth := maxWidth
	targetHeight := maxWidth * sourceHeight / sourceWidth
	thumb := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + x*sourceWidth/targetWidth
			sourceY := bounds.Min.Y + y*sourceHeight/targetHeight
			thumb.Set(x, y, img.At(sourceX, sourceY))
		}
	}
	return writePNG(outputPath, thumb)
}

func writePNG(path string, img image.Image) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func isPDF(fileName string, header []byte) bool {
	if !strings.EqualFold(filepath.Ext(fileName), ".pdf") {
		return false
	}
	return len(header) >= 4 && string(header[:4]) == "%PDF"
}

func newUploadID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("tmp_%d_%s", time.Now().UTC().Unix(), hex.EncodeToString(random[:])), nil
}

func validUploadID(uploadID string) bool {
	if !strings.HasPrefix(uploadID, "tmp_") {
		return false
	}
	for _, char := range uploadID {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func storedPath(uploadID string, name string) string {
	return filepath.ToSlash(filepath.Join("uploads", uploadID, name))
}

func parseStoredPath(value string) (string, string, bool) {
	clean := filepath.ToSlash(strings.TrimSpace(value))
	parts := strings.Split(clean, "/")
	if len(parts) != 3 || parts[0] != "uploads" {
		return "", "", false
	}
	switch parts[2] {
	case "original.pdf", "preview.png", "thumbnail.png":
		return parts[1], parts[2], true
	default:
		return "", "", false
	}
}

func uploadURL(prefix string, uploadID string, kind UploadAssetKind) string {
	base := strings.TrimRight(prefix, "/")
	path := fmt.Sprintf("/api/design-plan/uploads/%s/%s", uploadID, kind)
	if base == "" {
		return path
	}
	return base + path
}

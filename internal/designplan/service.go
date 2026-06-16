package designplan

import (
	"context"
	"io"
	"log"
	"strings"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

type Service struct {
	repo       Repository
	uploads    *UploadManager
	recognizer Recognizer
}

func NewService(repo Repository) *Service {
	uploads := NewUploadManagerFromEnv()
	return &Service{
		repo:       repo,
		uploads:    uploads,
		recognizer: NewOpenAIRecognizerFromEnvWithAssetReader(uploads.OpenStored),
	}
}

func NewServiceWithAssetStore(repo Repository, assetStore assets.Store) *Service {
	rootDir := strings.TrimSpace(getenv("UPLOAD_DIR", defaultUploadDir))
	uploads := NewUploadManager(rootDir, assetStore)
	return &Service{
		repo:       repo,
		uploads:    uploads,
		recognizer: NewOpenAIRecognizerFromEnvWithAssetReader(uploads.OpenStored),
	}
}

func newServiceWithDependencies(repo Repository, uploads *UploadManager, recognizer Recognizer) *Service {
	return &Service{
		repo:       repo,
		uploads:    uploads,
		recognizer: recognizer,
	}
}

func (s *Service) ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error) {
	return s.repo.ListStores(ctx, filters)
}

func (s *Service) GetStore(ctx context.Context, id int64) (*Store, error) {
	return s.repo.GetStore(ctx, id)
}

func (s *Service) CreateStore(ctx context.Context, input StoreInput) (*Store, error) {
	if err := ValidateStoreInput(input); err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if err := s.ensureNoExactDuplicate(ctx, input.Name, 0); err != nil {
		return nil, err
	}
	return s.repo.CreateStore(ctx, input)
}

func (s *Service) UpdateStore(ctx context.Context, id int64, input StoreInput) (*Store, error) {
	if err := ValidateStoreInput(input); err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if err := s.ensureNoExactDuplicate(ctx, input.Name, id); err != nil {
		return nil, err
	}
	return s.repo.UpdateStore(ctx, id, input)
}

func (s *Service) DeleteStore(ctx context.Context, id int64) error {
	store, err := s.repo.GetStore(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteStore(ctx, id); err != nil {
		return err
	}
	if err := s.uploads.DeleteStoredFiles(store.OriginalPDFPath, store.PreviewImagePath, store.ThumbnailPath); err != nil {
		log.Printf("designplan: delete upload files for store %d failed: %v", id, err)
	}
	return nil
}

func (s *Service) CheckDuplicate(ctx context.Context, request DuplicateCheckRequest) (DuplicateCheckResult, error) {
	if strings.TrimSpace(request.Name) == "" {
		return DuplicateCheckResult{}, &ValidationError{Fields: map[string]string{
			"name": "门店名必填",
		}}
	}
	return s.repo.CheckDuplicate(ctx, strings.TrimSpace(request.Name), request.ExcludeStoreID)
}

func (s *Service) SaveUpload(ctx context.Context, input UploadInput) (*UploadResult, error) {
	return s.uploads.Save(ctx, input)
}

func (s *Service) RecognizeUpload(ctx context.Context, uploadID string) (*RecognitionResult, error) {
	upload, err := s.uploads.Find(uploadID)
	if err != nil {
		return nil, err
	}
	return s.recognizer.Recognize(ctx, upload)
}

func (s *Service) UploadFilePath(uploadID string, kind UploadAssetKind) (string, error) {
	return s.uploads.FilePath(uploadID, kind)
}

func (s *Service) OpenUploadAsset(uploadID string, kind UploadAssetKind) (io.ReadCloser, string, error) {
	return s.uploads.Open(uploadID, kind)
}

func (s *Service) StoredFilePath(value string) (string, error) {
	return s.uploads.StoredFilePath(value)
}

func (s *Service) OpenStoredAsset(value string) (io.ReadCloser, string, error) {
	return s.uploads.OpenStored(value)
}

func (s *Service) ensureNoExactDuplicate(ctx context.Context, name string, excludeStoreID int64) error {
	result, err := s.repo.CheckDuplicate(ctx, name, excludeStoreID)
	if err != nil {
		return err
	}
	if result.ExactMatch != nil {
		return &ValidationError{Fields: map[string]string{
			"name": "已存在同名门店",
		}}
	}
	return nil
}

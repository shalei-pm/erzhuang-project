package designplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("design plan store not found")

type Repository interface {
	ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error)
	GetStore(ctx context.Context, id int64) (*Store, error)
	CreateStore(ctx context.Context, input StoreInput) (*Store, error)
	UpdateStore(ctx context.Context, id int64, input StoreInput) (*Store, error)
	DeleteStore(ctx context.Context, id int64) error
	CheckDuplicate(ctx context.Context, name string, excludeStoreID int64) (DuplicateCheckResult, error)
}

type MemoryStore struct {
	mu     sync.Mutex
	nextID int64
	stores map[int64]*Store
	logs   []OperationLog
}

type OperationLog struct {
	ID        int64           `json:"id"`
	Action    OperationAction `json:"action"`
	StoreID   int64           `json:"store_id,omitempty"`
	StoreName string          `json:"store_name"`
	Actor     string          `json:"actor"`
	Summary   string          `json:"summary"`
	CreatedAt time.Time       `json:"created_at"`
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextID: 1,
		stores: map[int64]*Store{},
	}
}

func (s *MemoryStore) ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filters = normalizeFilters(filters)
	items := []StoreListItem{}
	for _, store := range s.stores {
		if !MatchesStoreSearch(store.Name, store.NormalizedName, filters.Query) {
			continue
		}
		items = append(items, storeListItem(*store))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	total := len(items)
	start := (filters.Page - 1) * filters.PageSize
	if start > total {
		start = total
	}
	end := start + filters.PageSize
	if end > total {
		end = total
	}

	return StoreListResult{
		Items:    items[start:end],
		Page:     filters.Page,
		PageSize: filters.PageSize,
		Total:    total,
	}, nil
}

func (s *MemoryStore) GetStore(ctx context.Context, id int64) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[id]
	if !ok {
		return nil, ErrNotFound
	}
	copy := cloneStore(*store)
	return &copy, nil
}

func (s *MemoryStore) CreateStore(ctx context.Context, input StoreInput) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	id := s.nextID
	s.nextID++

	store := storeFromInput(id, input, now, now)
	for index := range store.Areas {
		store.Areas[index].ID = int64(index + 1)
		store.Areas[index].StoreID = id
		store.Areas[index].CreatedAt = now
		store.Areas[index].UpdatedAt = now
	}

	s.stores[id] = &store
	s.appendLog(OperationCreate, id, store.Name, fmt.Sprintf("created store with %d areas", len(store.Areas)), now)

	copy := cloneStore(store)
	return &copy, nil
}

func (s *MemoryStore) UpdateStore(ctx context.Context, id int64, input StoreInput) (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.stores[id]
	if !ok {
		return nil, ErrNotFound
	}

	now := time.Now().UTC()
	store := storeFromInput(id, input, existing.CreatedAt, now)
	for index := range store.Areas {
		store.Areas[index].ID = int64(index + 1)
		store.Areas[index].StoreID = id
		store.Areas[index].CreatedAt = now
		store.Areas[index].UpdatedAt = now
	}

	s.stores[id] = &store
	s.appendLog(OperationUpdate, id, store.Name, fmt.Sprintf("updated store with %d areas", len(store.Areas)), now)
	if areasChanged(existing.Areas, store.Areas) {
		s.appendLog(OperationReplace, id, store.Name, fmt.Sprintf("replaced %d areas", len(store.Areas)), now)
	}

	copy := cloneStore(store)
	return &copy, nil
}

func (s *MemoryStore) DeleteStore(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, ok := s.stores[id]
	if !ok {
		return ErrNotFound
	}
	delete(s.stores, id)
	s.appendLog(OperationDelete, id, store.Name, "deleted store", time.Now().UTC())
	return nil
}

func (s *MemoryStore) CheckDuplicate(ctx context.Context, name string, excludeStoreID int64) (DuplicateCheckResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := NormalizeStoreName(name)
	result := DuplicateCheckResult{SimilarMatches: []DuplicateMatch{}}
	for _, store := range s.stores {
		if store.ID == excludeStoreID {
			continue
		}
		summary := storeListItem(*store)
		match := DuplicateMatch{
			ID:                store.ID,
			Name:              store.Name,
			NormalizedName:    store.NormalizedName,
			ThumbnailURL:      thumbnailURL(store.ID),
			TreatmentCount:    summary.TreatmentCount,
			ConsultationCount: summary.ConsultationCount,
			BeautyCount:       summary.BeautyCount,
			AreaCount:         summary.AreaCount,
			Status:            store.Status,
			UpdatedAt:         store.UpdatedAt,
		}
		if store.NormalizedName == normalized {
			match.Reason = "exact"
			if result.ExactMatch == nil {
				result.ExactMatch = &match
			}
			continue
		}
		if IsSimilarStoreName(name, store.Name) {
			match.Reason = "similar"
			result.SimilarMatches = append(result.SimilarMatches, match)
		}
	}
	return result, nil
}

func (s *MemoryStore) appendLog(action OperationAction, storeID int64, storeName string, summary string, at time.Time) {
	s.logs = append(s.logs, OperationLog{
		ID:        int64(len(s.logs) + 1),
		Action:    action,
		StoreID:   storeID,
		StoreName: storeName,
		Actor:     "admin",
		Summary:   summary,
		CreatedAt: at,
	})
}

func storeFromInput(id int64, input StoreInput, createdAt time.Time, updatedAt time.Time) Store {
	status := input.Status
	if status == "" {
		status = inferStoreStatus(input.Areas)
	}

	store := Store{
		ID:                id,
		Name:              strings.TrimSpace(input.Name),
		NormalizedName:    NormalizeStoreName(input.Name),
		PDFFileName:       strings.TrimSpace(input.PDFFileName),
		OriginalPDFPath:   strings.TrimSpace(input.OriginalPDFPath),
		PreviewImagePath:  strings.TrimSpace(input.PreviewImagePath),
		ThumbnailPath:     strings.TrimSpace(input.ThumbnailPath),
		PageCount:         input.PageCount,
		Status:            status,
		RecognitionResult: json.RawMessage(strings.TrimSpace(string(input.RecognitionResult))),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}
	if store.PageCount < 0 {
		store.PageCount = 0
	}
	store.PreviewURL = previewURL(id)
	store.ThumbnailURL = thumbnailURL(id)

	for index, areaInput := range input.Areas {
		confidence := areaInput.Confidence
		if confidence == "" {
			confidence = ConfidenceHigh
		}
		displayOrder := areaInput.DisplayOrder
		if displayOrder <= 0 {
			displayOrder = index + 1
		}
		box := Box{}
		if areaInput.Box != nil {
			box = *areaInput.Box
		}
		store.Areas = append(store.Areas, Area{
			ID:           areaInput.ID,
			StoreID:      id,
			DisplayOrder: displayOrder,
			Name:         strings.TrimSpace(areaInput.Name),
			Type:         areaInput.Type,
			Number:       RoomNumber(strings.TrimSpace(string(areaInput.Number))),
			Confidence:   confidence,
			NeedsReview:  areaInput.NeedsReview || confidence == ConfidenceLow,
			Box:          box,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
		})
	}
	return store
}

func inferStoreStatus(areas []AreaInput) StoreStatus {
	for _, area := range areas {
		if area.Confidence == ConfidenceLow || area.NeedsReview {
			return StoreStatusNeedsReview
		}
	}
	return StoreStatusCompleted
}

func normalizeFilters(filters StoreFilters) StoreFilters {
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PageSize <= 0 {
		filters.PageSize = 20
	}
	if filters.PageSize > 100 {
		filters.PageSize = 100
	}
	return filters
}

func storeListItem(store Store) StoreListItem {
	item := StoreListItem{
		ID:           store.ID,
		Name:         store.Name,
		ThumbnailURL: thumbnailURL(store.ID),
		Status:       store.Status,
		UpdatedAt:    store.UpdatedAt,
	}
	for _, area := range store.Areas {
		item.AreaCount++
		switch area.Type {
		case AreaTypeTreatment, AreaTypeVIPTreatment:
			item.TreatmentCount++
		case AreaTypeConsultation:
			item.ConsultationCount++
		case AreaTypeBeauty:
			item.BeautyCount++
		}
	}
	return item
}

func cloneStore(store Store) Store {
	copy := store
	copy.Areas = append([]Area(nil), store.Areas...)
	if store.RecognitionResult != nil {
		copy.RecognitionResult = append(json.RawMessage(nil), store.RecognitionResult...)
	}
	return copy
}

func areasChanged(left, right []Area) bool {
	if len(left) != len(right) {
		return true
	}
	for index := range left {
		if left[index].Name != right[index].Name ||
			left[index].Type != right[index].Type ||
			left[index].Number != right[index].Number ||
			left[index].Box != right[index].Box {
			return true
		}
	}
	return false
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return []byte(value)
}

func previewURL(storeID int64) string {
	if storeID <= 0 {
		return ""
	}
	return fmt.Sprintf("/api/design-plan/stores/%d/preview", storeID)
}

func thumbnailURL(storeID int64) string {
	if storeID <= 0 {
		return ""
	}
	return fmt.Sprintf("/api/design-plan/stores/%d/thumbnail", storeID)
}

package designplan

import (
	"context"
	"database/sql"
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
	query := NormalizeStoreName(filters.Query)
	items := []StoreListItem{}
	for _, store := range s.stores {
		if query != "" && !strings.Contains(store.NormalizedName, query) {
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
		match := DuplicateMatch{
			ID:             store.ID,
			Name:           store.Name,
			NormalizedName: store.NormalizedName,
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

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) ListStores(ctx context.Context, filters StoreFilters) (StoreListResult, error) {
	filters = normalizeFilters(filters)
	like := "%" + NormalizeStoreName(filters.Query) + "%"
	offset := (filters.Page - 1) * filters.PageSize

	var total int
	if err := s.db.QueryRowContext(ctx, `
		select count(*)
		from design_plan_stores
		where $1 = '%%' or normalized_name like $1
	`, like).Scan(&total); err != nil {
		return StoreListResult{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		select
			s.id,
			s.name,
			s.status,
			s.updated_at,
			count(a.id) filter (where a.area_type = 'treatment') as treatment_count,
			count(a.id) filter (where a.area_type = 'consultation') as consultation_count,
			count(a.id) filter (where a.area_type = 'beauty') as beauty_count,
			count(a.id) as area_count
		from design_plan_stores s
		left join design_plan_store_areas a on a.store_id = s.id
		where $1 = '%%' or s.normalized_name like $1
		group by s.id
		order by s.updated_at desc
		limit $2 offset $3
	`, like, filters.PageSize, offset)
	if err != nil {
		return StoreListResult{}, err
	}
	defer rows.Close()

	items := []StoreListItem{}
	for rows.Next() {
		var item StoreListItem
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Status,
			&item.UpdatedAt,
			&item.TreatmentCount,
			&item.ConsultationCount,
			&item.BeautyCount,
			&item.AreaCount,
		); err != nil {
			return StoreListResult{}, err
		}
		item.ThumbnailURL = thumbnailURL(item.ID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return StoreListResult{}, err
	}

	return StoreListResult{Items: items, Page: filters.Page, PageSize: filters.PageSize, Total: total}, nil
}

func (s *PostgresStore) GetStore(ctx context.Context, id int64) (*Store, error) {
	var store Store
	var recognitionResult []byte
	err := s.db.QueryRowContext(ctx, `
		select id, name, normalized_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, status, recognition_result, created_at, updated_at
		from design_plan_stores
		where id = $1
	`, id).Scan(
		&store.ID,
		&store.Name,
		&store.NormalizedName,
		&store.OriginalPDFPath,
		&store.PreviewImagePath,
		&store.ThumbnailPath,
		&store.PageCount,
		&store.Status,
		&recognitionResult,
		&store.CreatedAt,
		&store.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	store.RecognitionResult = json.RawMessage(recognitionResult)
	store.PreviewURL = previewURL(store.ID)
	store.ThumbnailURL = thumbnailURL(store.ID)

	areas, err := s.listAreas(ctx, store.ID)
	if err != nil {
		return nil, err
	}
	store.Areas = areas
	return &store, nil
}

func (s *PostgresStore) CreateStore(ctx context.Context, input StoreInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	store := storeFromInput(0, input, now, now)
	recognitionResult := nullableJSON(store.RecognitionResult)
	err = tx.QueryRowContext(ctx, `
		insert into design_plan_stores (
			name, normalized_name, original_pdf_path, preview_image_path,
			thumbnail_path, page_count, status, recognition_result
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		returning id, created_at, updated_at
	`, store.Name, store.NormalizedName, store.OriginalPDFPath, store.PreviewImagePath,
		store.ThumbnailPath, store.PageCount, store.Status, recognitionResult,
	).Scan(&store.ID, &store.CreatedAt, &store.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if err := insertAreas(ctx, tx, store.ID, store.Areas); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, OperationCreate, store.ID, store.Name, fmt.Sprintf("created store with %d areas", len(store.Areas))); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetStore(ctx, store.ID)
}

func (s *PostgresStore) UpdateStore(ctx context.Context, id int64, input StoreInput) (*Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var existingName string
	err = tx.QueryRowContext(ctx, `select name from design_plan_stores where id = $1`, id).Scan(&existingName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	store := storeFromInput(id, input, now, now)
	result, err := tx.ExecContext(ctx, `
		update design_plan_stores
		set name = $2,
			normalized_name = $3,
			original_pdf_path = $4,
			preview_image_path = $5,
			thumbnail_path = $6,
			page_count = $7,
			status = $8,
			recognition_result = $9,
			updated_at = now()
		where id = $1
	`, id, store.Name, store.NormalizedName, store.OriginalPDFPath, store.PreviewImagePath,
		store.ThumbnailPath, store.PageCount, store.Status, nullableJSON(store.RecognitionResult))
	if err != nil {
		return nil, err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return nil, err
	} else if affected == 0 {
		return nil, ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `delete from design_plan_store_areas where store_id = $1`, id); err != nil {
		return nil, err
	}
	if err := insertAreas(ctx, tx, id, store.Areas); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, OperationUpdate, id, store.Name, fmt.Sprintf("updated store previously named %s", existingName)); err != nil {
		return nil, err
	}
	if err := insertOperationLog(ctx, tx, OperationReplace, id, store.Name, fmt.Sprintf("replaced areas for store previously named %s", existingName)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetStore(ctx, id)
}

func (s *PostgresStore) DeleteStore(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRowContext(ctx, `select name from design_plan_stores where id = $1`, id).Scan(&name); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `delete from design_plan_stores where id = $1`, id); err != nil {
		return err
	}
	if err := insertOperationLog(ctx, tx, OperationDelete, id, name, "deleted store"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) CheckDuplicate(ctx context.Context, name string, excludeStoreID int64) (DuplicateCheckResult, error) {
	normalized := NormalizeStoreName(name)
	rows, err := s.db.QueryContext(ctx, `
		select id, name, normalized_name
		from design_plan_stores
		where id <> $1
			and (
				normalized_name = $2
				or normalized_name like $3
				or $2 like '%' || normalized_name || '%'
			)
		order by updated_at desc
		limit 20
	`, excludeStoreID, normalized, "%"+normalized+"%")
	if err != nil {
		return DuplicateCheckResult{}, err
	}
	defer rows.Close()

	var result DuplicateCheckResult
	for rows.Next() {
		var match DuplicateMatch
		if err := rows.Scan(&match.ID, &match.Name, &match.NormalizedName); err != nil {
			return DuplicateCheckResult{}, err
		}
		if match.NormalizedName == normalized {
			match.Reason = "exact"
			if result.ExactMatch == nil {
				copy := match
				result.ExactMatch = &copy
			}
		} else if IsSimilarStoreName(name, match.Name) {
			match.Reason = "similar"
			result.SimilarMatches = append(result.SimilarMatches, match)
		}
	}
	if err := rows.Err(); err != nil {
		return DuplicateCheckResult{}, err
	}
	return result, nil
}

func (s *PostgresStore) listAreas(ctx context.Context, storeID int64) ([]Area, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, store_id, display_order, name, area_type, area_number,
			confidence, needs_review, box_x, box_y, box_width, box_height,
			created_at, updated_at
		from design_plan_store_areas
		where store_id = $1
		order by display_order, id
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var areas []Area
	for rows.Next() {
		var area Area
		var number sql.NullInt64
		if err := rows.Scan(
			&area.ID,
			&area.StoreID,
			&area.DisplayOrder,
			&area.Name,
			&area.Type,
			&number,
			&area.Confidence,
			&area.NeedsReview,
			&area.Box.X,
			&area.Box.Y,
			&area.Box.Width,
			&area.Box.Height,
			&area.CreatedAt,
			&area.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if number.Valid {
			area.Number = RoomNumber(fmt.Sprintf("%d", number.Int64))
		}
		areas = append(areas, area)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return areas, nil
}

type txRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func insertAreas(ctx context.Context, tx txRunner, storeID int64, areas []Area) error {
	for index, area := range areas {
		displayOrder := area.DisplayOrder
		if displayOrder <= 0 {
			displayOrder = index + 1
		}
		var number any
		if strings.TrimSpace(string(area.Number)) != "" {
			number = string(area.Number)
		}
		if _, err := tx.ExecContext(ctx, `
			insert into design_plan_store_areas (
				store_id, display_order, name, area_type, area_number,
				confidence, needs_review, box_x, box_y, box_width, box_height
			)
			values ($1, $2, $3, $4, $5::integer, $6, $7, $8, $9, $10, $11)
		`, storeID, displayOrder, area.Name, area.Type, number, area.Confidence,
			area.NeedsReview, area.Box.X, area.Box.Y, area.Box.Width, area.Box.Height); err != nil {
			return err
		}
	}
	return nil
}

func insertOperationLog(ctx context.Context, tx txRunner, action OperationAction, storeID int64, storeName string, summary string) error {
	_, err := tx.ExecContext(ctx, `
		insert into design_plan_operation_logs (action, store_id, store_name, actor, summary)
		values ($1, $2, $3, 'admin', $4)
	`, action, storeID, storeName, summary)
	return err
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
		case AreaTypeTreatment:
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

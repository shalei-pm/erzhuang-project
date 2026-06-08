package designplan

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type AreaType string

const (
	AreaTypeTreatment    AreaType = "treatment"
	AreaTypeConsultation AreaType = "consultation"
	AreaTypeBeauty       AreaType = "beauty"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type StoreStatus string

const (
	StoreStatusCompleted   StoreStatus = "completed"
	StoreStatusNeedsReview StoreStatus = "needs_review"
	StoreStatusIncomplete  StoreStatus = "incomplete"
)

type OperationAction string

const (
	OperationCreate  OperationAction = "create"
	OperationUpdate  OperationAction = "update"
	OperationDelete  OperationAction = "delete"
	OperationReplace OperationAction = "replace"
)

type RoomNumber string

func (n *RoomNumber) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "" || value == "null" {
		*n = ""
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*n = RoomNumber(strings.TrimSpace(text))
		return nil
	}

	var number float64
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("number must be a string or integer")
	}
	if number != float64(int64(number)) {
		return fmt.Errorf("number must be an integer")
	}
	*n = RoomNumber(strconv.FormatInt(int64(number), 10))
	return nil
}

func (n RoomNumber) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(n))
}

type Box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type AreaInput struct {
	ID           int64      `json:"id,omitempty"`
	Name         string     `json:"name"`
	Type         AreaType   `json:"type"`
	Number       RoomNumber `json:"number,omitempty"`
	Confidence   Confidence `json:"confidence,omitempty"`
	NeedsReview  bool       `json:"needs_review,omitempty"`
	Box          *Box       `json:"box"`
	DisplayOrder int        `json:"display_order,omitempty"`
}

type StoreInput struct {
	Name              string          `json:"name"`
	PDFFileName       string          `json:"pdf_file_name,omitempty"`
	OriginalPDFPath   string          `json:"original_pdf_path,omitempty"`
	PreviewImagePath  string          `json:"preview_image_path,omitempty"`
	ThumbnailPath     string          `json:"thumbnail_path,omitempty"`
	PageCount         int             `json:"page_count,omitempty"`
	Status            StoreStatus     `json:"status,omitempty"`
	RecognitionResult json.RawMessage `json:"recognition_result,omitempty"`
	Areas             []AreaInput     `json:"areas"`
}

type Area struct {
	ID           int64      `json:"id"`
	StoreID      int64      `json:"store_id,omitempty"`
	DisplayOrder int        `json:"display_order"`
	Name         string     `json:"name"`
	Type         AreaType   `json:"type"`
	Number       RoomNumber `json:"number,omitempty"`
	Confidence   Confidence `json:"confidence"`
	NeedsReview  bool       `json:"needs_review"`
	Box          Box        `json:"box"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Store struct {
	ID                int64           `json:"id"`
	Name              string          `json:"name"`
	NormalizedName    string          `json:"normalized_name,omitempty"`
	PDFFileName       string          `json:"pdf_file_name,omitempty"`
	OriginalPDFPath   string          `json:"original_pdf_path,omitempty"`
	PreviewImagePath  string          `json:"preview_image_path,omitempty"`
	ThumbnailPath     string          `json:"thumbnail_path,omitempty"`
	PreviewURL        string          `json:"preview_url,omitempty"`
	ThumbnailURL      string          `json:"thumbnail_url,omitempty"`
	PageCount         int             `json:"page_count"`
	Status            StoreStatus     `json:"status"`
	RecognitionResult json.RawMessage `json:"recognition_result,omitempty"`
	Areas             []Area          `json:"areas,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type StoreListItem struct {
	ID                int64       `json:"id"`
	Name              string      `json:"name"`
	ThumbnailURL      string      `json:"thumbnail_url"`
	TreatmentCount    int         `json:"treatment_count"`
	ConsultationCount int         `json:"consultation_count"`
	BeautyCount       int         `json:"beauty_count"`
	AreaCount         int         `json:"area_count"`
	Status            StoreStatus `json:"status"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type StoreListResult struct {
	Items    []StoreListItem `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int             `json:"total"`
}

type UploadResult struct {
	UploadID      string `json:"upload_id"`
	FileName      string `json:"file_name"`
	PageCount     int    `json:"page_count"`
	OriginalPath  string `json:"original_pdf_path"`
	PreviewPath   string `json:"preview_image_path"`
	ThumbnailPath string `json:"thumbnail_path"`
	PreviewURL    string `json:"preview_url"`
	ThumbnailURL  string `json:"thumbnail_url"`
}

type RecognitionResult struct {
	StoreName           string          `json:"store_name"`
	StoreNameConfidence Confidence      `json:"store_name_confidence"`
	Areas               []AreaInput     `json:"areas"`
	RawNotes            string          `json:"raw_notes"`
	RawResult           json.RawMessage `json:"raw_result,omitempty"`
}

type DuplicateCheckRequest struct {
	Name           string `json:"name"`
	ExcludeStoreID int64  `json:"exclude_store_id,omitempty"`
}

type DuplicateMatch struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	NormalizedName string `json:"normalized_name,omitempty"`
	Reason         string `json:"reason"`
}

type DuplicateCheckResult struct {
	ExactMatch     *DuplicateMatch  `json:"exact_match"`
	SimilarMatches []DuplicateMatch `json:"similar_matches"`
}

type StoreFilters struct {
	Query    string
	Page     int
	PageSize int
}

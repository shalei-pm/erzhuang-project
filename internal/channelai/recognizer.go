package channelai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultModel = "gpt-5.5"

type Result struct {
	SceneType      string          `json:"scene_type"`
	AreaType       string          `json:"area_type"`
	AreaNumber     string          `json:"area_number"`
	CardText       string          `json:"card_text"`
	DecisionSource string          `json:"decision_source"`
	Confidence     string          `json:"confidence"`
	NeedsReview    bool            `json:"needs_review"`
	RawNotes       string          `json:"raw_notes"`
	RawResult      json.RawMessage `json:"raw_result,omitempty"`
}

type Recognizer interface {
	Recognize(ctx context.Context, imageURL string) (Result, error)
}

type OpenAIRecognizer struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewOpenAIRecognizerFromEnv() (Recognizer, bool) {
	apiKey := strings.TrimSpace(os.Getenv("VISION_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if apiKey == "" {
		return nil, false
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("VISION_API_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	model := strings.TrimSpace(os.Getenv("VISION_MODEL"))
	if model == "" {
		model = defaultModel
	}
	return &OpenAIRecognizer{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 75 * time.Second,
		},
	}, true
}

func (r *OpenAIRecognizer) Recognize(ctx context.Context, imageURL string) (Result, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return Result{}, errors.New("missing channel snapshot image url")
	}
	payload := responsesRequest{
		Model: r.model,
		Input: []responsesInput{{
			Role: "user",
			Content: []responsesContent{
				{Type: "input_text", Text: prompt()},
				{Type: "input_image", ImageURL: imageURL},
			},
		}},
		Text: responsesText{
			Format: responsesTextFormat{
				Type:   "json_schema",
				Name:   "channel_scene_recognition",
				Strict: true,
				Schema: schema(),
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Authorization", "Bearer "+r.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := r.httpClient.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return Result{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Result{}, fmt.Errorf("vision recognition failed: status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var parsed responsesResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return Result{}, err
	}
	text := parsed.firstOutputText()
	if strings.TrimSpace(text) == "" {
		return Result{}, errors.New("vision recognition returned empty output")
	}
	var output Result
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return Result{}, fmt.Errorf("parse channel recognition json: %w", err)
	}
	output.RawResult = json.RawMessage(responseBody)
	return normalize(output), nil
}

func normalize(result Result) Result {
	result.SceneType = normalizeSceneType(result.SceneType)
	result.AreaType = normalizeAreaType(result.AreaType)
	result.AreaNumber = strings.TrimSpace(result.AreaNumber)
	result.CardText = strings.TrimSpace(result.CardText)
	result.DecisionSource = normalizeDecisionSource(result.DecisionSource)
	result.Confidence = normalizeConfidence(result.Confidence)
	result.RawNotes = strings.TrimSpace(result.RawNotes)
	if result.AreaType != "" {
		result.SceneType = result.AreaType
	}
	if result.Confidence == "low" {
		result.NeedsReview = true
	}
	return result
}

func normalizeAreaType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "treatment", "consultation", "beauty":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeSceneType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "treatment", "consultation", "beauty", "front_desk", "corridor", "passage", "waiting_area", "hall", "entrance", "storage", "pharmacy", "machine_room", "unknown":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func normalizeDecisionSource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "number_card", "scene", "none":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "none"
	}
}

func normalizeConfidence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

type responsesRequest struct {
	Model string           `json:"model"`
	Input []responsesInput `json:"input"`
	Text  responsesText    `json:"text"`
}

type responsesInput struct {
	Role    string             `json:"role"`
	Content []responsesContent `json:"content"`
}

type responsesContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type responsesText struct {
	Format responsesTextFormat `json:"format"`
}

type responsesTextFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type responsesResponse struct {
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func (r responsesResponse) firstOutputText() string {
	for _, output := range r.Output {
		for _, content := range output.Content {
			if strings.TrimSpace(content.Text) != "" {
				return content.Text
			}
		}
	}
	return ""
}

func prompt() string {
	return strings.TrimSpace(`你是医疗门店监控画面识别助手。请判断这张摄像头截图对应的区域类型，并提取画面中编号卡片的信息。

业务区域只允许三类：
- treatment：治疗室，指医美治疗室。
- consultation：面诊室。
- beauty：生美、生美区、生美治疗室、美容区、美容治疗室。

非业务区域可选：front_desk、corridor、passage、waiting_area、hall、entrance、storage、pharmacy、machine_room、unknown。

规则：
1. 如果画面中有显著编号卡片，且卡片写有“治疗室 1”“面诊室 2”“生美 3”等业务类型和数字，则 area_type 和 area_number 必须以卡片文本为准，即使画面环境判断不同。
2. 如果卡片只有数字，没有业务类型，则 area_number 填该数字，area_type 根据画面环境判断。
3. 如果没有卡片，则根据画面判断 scene_type；能明确属于治疗室、面诊室、生美时，也可以填 area_type，但 area_number 为空。
4. 如果不是三类业务区域，area_type 置空，scene_type 填对应非业务区域，area_number 填非业务区域的中文实体名称，例如“机房”“药房”“前台”“走廊”“通道”“候诊区”“大厅”“门口”“库房”；无法判断时 area_number 置空。
5. AI 结果只是预填，用户会人工确认；不确定时 confidence 用 low，needs_review 为 true。
6. raw_notes 用中文简短说明判断依据。`)
}

func schema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"scene_type", "area_type", "area_number", "card_text", "decision_source", "confidence", "needs_review", "raw_notes"},
		"properties": map[string]any{
			"scene_type":      map[string]any{"type": "string", "enum": []string{"treatment", "consultation", "beauty", "front_desk", "corridor", "passage", "waiting_area", "hall", "entrance", "storage", "pharmacy", "machine_room", "unknown"}},
			"area_type":       map[string]any{"type": "string", "enum": []string{"", "treatment", "consultation", "beauty"}},
			"area_number":     map[string]any{"type": "string"},
			"card_text":       map[string]any{"type": "string"},
			"decision_source": map[string]any{"type": "string", "enum": []string{"number_card", "scene", "none"}},
			"confidence":      map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
			"needs_review":    map[string]any{"type": "boolean"},
			"raw_notes":       map[string]any{"type": "string"},
		},
	}
}

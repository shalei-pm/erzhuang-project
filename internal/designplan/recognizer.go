package designplan

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultOpenAIModel = "gpt-4o"

type Recognizer interface {
	Recognize(ctx context.Context, upload *UploadResult) (*RecognitionResult, error)
}

type OpenAIRecognizer struct {
	apiKey     string
	baseURL    string
	apiStyle   string
	model      string
	httpClient *http.Client
}

func NewOpenAIRecognizerFromEnv() Recognizer {
	return &OpenAIRecognizer{
		apiKey: strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		baseURL: strings.TrimRight(
			getenv("OPENAI_BASE_URL", "https://api.openai.com"),
			"/",
		),
		apiStyle: normalizeOpenAIAPIStyle(os.Getenv("OPENAI_API_STYLE")),
		model:    getenv("OPENAI_MODEL", defaultOpenAIModel),
		httpClient: &http.Client{
			Timeout: 75 * time.Second,
		},
	}
}

func (r *OpenAIRecognizer) Recognize(ctx context.Context, upload *UploadResult) (*RecognitionResult, error) {
	if strings.TrimSpace(r.apiKey) == "" {
		return nil, &ValidationError{Fields: map[string]string{"openai_api_key": "服务器未配置 OpenAI API Key"}}
	}
	if upload == nil || strings.TrimSpace(upload.PreviewPath) == "" {
		return nil, &ValidationError{Fields: map[string]string{"upload_id": "上传文件不存在"}}
	}

	imageBytes, err := os.ReadFile(filepathFromStoredUpload(upload.PreviewPath))
	if err != nil {
		return nil, err
	}

	var output recognizerOutput
	var raw json.RawMessage
	switch r.apiStyle {
	case "chat_completions":
		output, raw, err = r.callChatCompletionsAPI(ctx, imageBytes)
	default:
		output, raw, err = r.callResponsesAPI(ctx, imageBytes)
	}
	if err != nil {
		return nil, err
	}
	result := output.toRecognitionResult()
	result.RawResult = raw
	return result, nil
}

func (r *OpenAIRecognizer) callResponsesAPI(ctx context.Context, imageBytes []byte) (recognizerOutput, json.RawMessage, error) {
	payload := responsesRequest{
		Model: r.model,
		Input: []responsesInput{{
			Role: "user",
			Content: []responsesContent{
				{Type: "input_text", Text: recognitionPrompt()},
				{Type: "input_image", ImageURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes)},
			},
		}},
		Text: responsesText{
			Format: responsesTextFormat{
				Type:   "json_schema",
				Name:   "design_plan_recognition",
				Strict: true,
				Schema: recognitionJSONSchema(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return recognizerOutput{}, nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint("/v1/responses"), bytes.NewReader(body))
	if err != nil {
		return recognizerOutput{}, nil, err
	}
	request.Header.Set("Authorization", "Bearer "+r.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := r.httpClient.Do(request)
	if err != nil {
		return recognizerOutput{}, nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return recognizerOutput{}, nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return recognizerOutput{}, nil, fmt.Errorf("openai recognition failed: status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var parsed responsesResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return recognizerOutput{}, nil, err
	}
	text := parsed.firstOutputText()
	if strings.TrimSpace(text) == "" {
		return recognizerOutput{}, nil, errors.New("openai recognition returned empty output")
	}
	var output recognizerOutput
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return recognizerOutput{}, nil, fmt.Errorf("parse recognition json: %w", err)
	}
	return output, json.RawMessage(responseBody), nil
}

func (r *OpenAIRecognizer) callChatCompletionsAPI(ctx context.Context, imageBytes []byte) (recognizerOutput, json.RawMessage, error) {
	payload := chatCompletionsRequest{
		Model: r.model,
		Messages: []chatMessage{{
			Role: "user",
			Content: []chatContent{
				{Type: "text", Text: recognitionPrompt()},
				{Type: "image_url", ImageURL: &chatImageURL{URL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes)}},
			},
		}},
		ResponseFormat: chatResponseFormat{
			Type: "json_schema",
			JSONSchema: chatJSONSchema{
				Name:   "design_plan_recognition",
				Strict: true,
				Schema: recognitionJSONSchema(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return recognizerOutput{}, nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint("/v1/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return recognizerOutput{}, nil, err
	}
	request.Header.Set("Authorization", "Bearer "+r.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := r.httpClient.Do(request)
	if err != nil {
		return recognizerOutput{}, nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return recognizerOutput{}, nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return recognizerOutput{}, nil, fmt.Errorf("openai chat recognition failed: status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return recognizerOutput{}, nil, err
	}
	text := parsed.firstChoiceText()
	if strings.TrimSpace(text) == "" {
		return recognizerOutput{}, nil, errors.New("openai chat recognition returned empty output")
	}
	var output recognizerOutput
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return recognizerOutput{}, nil, fmt.Errorf("parse chat recognition json: %w", err)
	}
	return output, json.RawMessage(responseBody), nil
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

type chatCompletionsRequest struct {
	Model          string             `json:"model"`
	Messages       []chatMessage      `json:"messages"`
	ResponseFormat chatResponseFormat `json:"response_format"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []chatContent `json:"content"`
}

type chatContent struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL string `json:"url"`
}

type chatResponseFormat struct {
	Type       string         `json:"type"`
	JSONSchema chatJSONSchema `json:"json_schema"`
}

type chatJSONSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (r chatCompletionsResponse) firstChoiceText() string {
	for _, choice := range r.Choices {
		if strings.TrimSpace(choice.Message.Content) != "" {
			return choice.Message.Content
		}
	}
	return ""
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

type recognizerOutput struct {
	StoreName           string                 `json:"store_name"`
	StoreNameConfidence Confidence             `json:"store_name_confidence"`
	Areas               []recognizerOutputArea `json:"areas"`
	RawNotes            string                 `json:"raw_notes"`
}

type recognizerOutputArea struct {
	Name        string     `json:"name"`
	Type        AreaType   `json:"type"`
	Number      string     `json:"number"`
	Confidence  Confidence `json:"confidence"`
	NeedsReview bool       `json:"needs_review"`
	Box         Box        `json:"box"`
}

func (o recognizerOutput) toRecognitionResult() *RecognitionResult {
	confidence := o.StoreNameConfidence
	if confidence == "" {
		confidence = ConfidenceMedium
	}
	result := &RecognitionResult{
		StoreName:           strings.TrimSpace(o.StoreName),
		StoreNameConfidence: confidence,
		Areas:               []AreaInput{},
		RawNotes:            o.RawNotes,
	}
	for index, area := range o.Areas {
		areaConfidence := area.Confidence
		if areaConfidence == "" {
			areaConfidence = ConfidenceMedium
		}
		name := strings.TrimSpace(area.Name)
		number := strings.TrimSpace(area.Number)
		if name == "" {
			name = generatedAreaName(area.Type, number)
		}
		box := area.Box
		result.Areas = append(result.Areas, AreaInput{
			Name:         name,
			Type:         area.Type,
			Number:       RoomNumber(number),
			Confidence:   areaConfidence,
			NeedsReview:  area.NeedsReview || areaConfidence == ConfidenceLow,
			Box:          &box,
			DisplayOrder: index + 1,
		})
	}
	return result
}

func recognitionPrompt() string {
	return strings.TrimSpace(`你是医疗门店装修图纸识别助手。请从这张门店设计图中识别门店名称和目标空间区域。

只识别三类区域：
1. treatment：治疗室，指医美治疗室。
2. consultation：面诊室。
3. beauty：生美、生美区、生美治疗室、美容区、美容治疗室。

要求：
- 门店名称优先来自图纸标题或图签。
- 区域名称尽量保留图纸原文。
- 如果区域文字里有明显数字编号，可以填入 number；没有编号则填空字符串。
- treatment 和 consultation 的 number 很重要；beauty 可为空。
- box 使用相对坐标，基于整张拼接图片，x/y/width/height 都是 0 到 1 的小数。
- 只输出目标三类区域，忽略前台、走廊、仓库、卫生间等非目标区域。
- 按图纸位置从上到下、从左到右排序。
- 如果不确定，confidence 用 low，并设置 needs_review 为 true。
- raw_notes 用中文简短说明识别依据或问题。`)
}

func recognitionJSONSchema() map[string]any {
	area := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"name", "type", "number", "confidence", "needs_review", "box"},
		"properties": map[string]any{
			"name":         map[string]any{"type": "string"},
			"type":         map[string]any{"type": "string", "enum": []string{"treatment", "consultation", "beauty"}},
			"number":       map[string]any{"type": "string"},
			"confidence":   map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
			"needs_review": map[string]any{"type": "boolean"},
			"box": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"x", "y", "width", "height"},
				"properties": map[string]any{
					"x":      map[string]any{"type": "number", "minimum": 0, "maximum": 1},
					"y":      map[string]any{"type": "number", "minimum": 0, "maximum": 1},
					"width":  map[string]any{"type": "number", "minimum": 0, "maximum": 1},
					"height": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
				},
			},
		},
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"store_name", "store_name_confidence", "areas", "raw_notes"},
		"properties": map[string]any{
			"store_name":            map[string]any{"type": "string"},
			"store_name_confidence": map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
			"areas":                 map[string]any{"type": "array", "items": area},
			"raw_notes":             map[string]any{"type": "string"},
		},
	}
}

func generatedAreaName(areaType AreaType, number string) string {
	switch areaType {
	case AreaTypeTreatment:
		if number != "" {
			return "治疗室 " + number
		}
		return "治疗室"
	case AreaTypeConsultation:
		if number != "" {
			return "面诊室 " + number
		}
		return "面诊室"
	case AreaTypeBeauty:
		if number != "" {
			return "生美 " + number
		}
		return "生美"
	default:
		return ""
	}
}

func filepathFromStoredUpload(value string) string {
	uploadID, name, ok := parseStoredPath(value)
	if !ok {
		return value
	}
	rootDir := strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
	if rootDir == "" {
		rootDir = defaultUploadDir
	}
	return filepath.Join(rootDir, uploadID, name)
}

func (r *OpenAIRecognizer) endpoint(path string) string {
	return strings.TrimRight(r.baseURL, "/") + path
}

func normalizeOpenAIAPIStyle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "chat", "chat_completions", "chat-completions", "openai-completions", "completions":
		return "chat_completions"
	default:
		return "responses"
	}
}

func getenv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

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
	"strings"
	"time"
)

const minimaxDefaultBaseURL = "https://api.minimaxi.com"
const minimaxDefaultModel = "MiniMax-M3"

type MiniMaxRecognizer struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	readAsset  AssetReader
}

func NewMiniMaxRecognizerFromEnvWithAssetReader(readAsset AssetReader) Recognizer {
	apiKey := strings.TrimSpace(os.Getenv("MINIMAX_API_KEY"))
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("MINIMAX_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = minimaxDefaultBaseURL
	}
	model := strings.TrimSpace(os.Getenv("MINIMAX_MODEL"))
	if model == "" {
		model = minimaxDefaultModel
	}
	return &MiniMaxRecognizer{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 75 * time.Second},
		readAsset:  readAsset,
	}
}

func (r *MiniMaxRecognizer) Recognize(ctx context.Context, upload *UploadResult) (*RecognitionResult, error) {
	if strings.TrimSpace(r.apiKey) == "" {
		return nil, &ValidationError{Fields: map[string]string{"minimax_api_key": "服务器未配置 MiniMax API Key"}}
	}
	if upload == nil || strings.TrimSpace(upload.PreviewPath) == "" {
		return nil, &ValidationError{Fields: map[string]string{"upload_id": "上传文件不存在"}}
	}
	imageBytes, err := r.readImageBytes(upload.PreviewPath)
	if err != nil {
		return nil, err
	}
	imageB64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes)
	payload := minimaxChatRequest{
		Model: r.model,
		Messages: []minimaxChatMessage{{
			Role: "user",
			Content: []minimaxChatContent{
				{Type: "text", Text: minimaxRecognitionPrompt()},
				{Type: "image_url", ImageURL: &minimaxImageURL{URL: imageB64}},
			},
		}},
		ResponseFormat: minimaxResponseFormat{
			Type: "json_schema",
			JSONSchema: minimaxJSONSchema{
				Name:   "design_plan_recognition",
				Strict: true,
				Schema: recognitionJSONSchema(),
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint("/v1/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+r.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := r.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("minimax recognition failed: status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var parsed minimaxChatResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return nil, err
	}
	text := parsed.firstChoiceText()
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("minimax recognition returned empty output")
	}
	var output recognizerOutput
	if err := json.Unmarshal([]byte(extractRecognitionJSONTextFromModel(text)), &output); err != nil {
		return nil, fmt.Errorf("parse minimax recognition json: %w: %s", err, compactMiniMaxModelText(text))
	}
	result := output.toRecognitionResult()
	result.RawResult = json.RawMessage(responseBody)
	return result, nil
}

func (r *MiniMaxRecognizer) readImageBytes(value string) ([]byte, error) {
	if r.readAsset != nil {
		reader, _, err := r.readAsset(value)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	}
	return os.ReadFile(filepathFromStoredUpload(value))
}

func (r *MiniMaxRecognizer) endpoint(path string) string {
	return apiEndpoint(r.baseURL, path)
}

type minimaxChatRequest struct {
	Model          string                `json:"model"`
	Messages       []minimaxChatMessage  `json:"messages"`
	ResponseFormat minimaxResponseFormat `json:"response_format"`
}

type minimaxChatMessage struct {
	Role    string               `json:"role"`
	Content []minimaxChatContent `json:"content"`
}

type minimaxChatContent struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	ImageURL *minimaxImageURL `json:"image_url,omitempty"`
}

type minimaxImageURL struct {
	URL string `json:"url"`
}

type minimaxResponseFormat struct {
	Type       string            `json:"type"`
	JSONSchema minimaxJSONSchema `json:"json_schema"`
}

type minimaxJSONSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type minimaxChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (r minimaxChatResponse) firstChoiceText() string {
	for _, choice := range r.Choices {
		if strings.TrimSpace(choice.Message.Content) != "" {
			return choice.Message.Content
		}
	}
	return ""
}

func compactMiniMaxModelText(value string) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	const maxLength = 500
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "...[truncated]"
}

func extractRecognitionJSONTextFromModel(value string) string {
	text := extractRecognitionJSONText(value)
	if json.Valid([]byte(text)) {
		return text
	}
	if extracted, ok := firstRecognitionJSONObject(text); ok {
		return extracted
	}
	return text
}

func firstRecognitionJSONObject(value string) (string, bool) {
	text := strings.TrimSpace(value)
	start := strings.Index(text, "{")
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(text); index++ {
		character := text[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
				continue
			}
			if character == '"' {
				inString = false
			}
			continue
		}
		switch character {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				candidate := text[start : index+1]
				if json.Valid([]byte(candidate)) {
					return candidate, true
				}
				return "", false
			}
		}
	}
	return "", false
}

func minimaxRecognitionPrompt() string {
	return strings.TrimSpace(`重要：禁止输出 <think>、分析过程、解释、Markdown 或代码块。你必须只返回一个合法 JSON 对象，且 JSON 必须符合下面要求。

` + recognitionPrompt())
}

func NewRecognizerFromEnvWithAssetReader(readAsset AssetReader) Recognizer {
	return NewRecognizerForProviderWithAssetReader("", readAsset)
}

func NewRecognizerForProviderWithAssetReader(provider string, readAsset AssetReader) Recognizer {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(os.Getenv("DESIGN_PLAN_AI_PROVIDER")))
	}
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(os.Getenv("CHANNEL_AI_PROVIDER")))
	}
	switch provider {
	case "minimax":
		return NewMiniMaxRecognizerFromEnvWithAssetReader(readAsset)
	default:
		return NewOpenAIRecognizerFromEnvWithAssetReader(readAsset)
	}
}

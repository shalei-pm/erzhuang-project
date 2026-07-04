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

const minimaxDefaultBaseURL = "https://api.minimaxi.com"
const minimaxDefaultModel = "MiniMax-M3"

type MiniMaxRecognizer struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewMiniMaxRecognizerFromEnv() (Recognizer, bool, error) {
	apiKey := strings.TrimSpace(os.Getenv("MINIMAX_API_KEY"))
	if apiKey == "" {
		return nil, false, errors.New("MINIMAX_API_KEY is required when CHANNEL_AI_PROVIDER=minimax")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("MINIMAX_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = minimaxDefaultBaseURL
	}
	model := strings.TrimSpace(os.Getenv("MINIMAX_MODEL"))
	if model == "" {
		model = minimaxDefaultModel
	}
	return &MiniMaxRecognizer{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 75 * time.Second,
		},
	}, true, nil
}

func (r *MiniMaxRecognizer) Recognize(ctx context.Context, imageURL string) (Result, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return Result{}, errors.New("missing channel snapshot image url")
	}
	payload := chatCompletionsRequest{
		Model: r.model,
		Messages: []chatCompletionsMessage{{
			Role: "user",
			Content: []chatCompletionsContent{
				{Type: "text", Text: minimaxPrompt(prompt())},
				{Type: "image_url", ImageURL: &chatCompletionsImageURL{URL: imageURL}},
			},
		}},
		ResponseFormat: chatCompletionsResponseFormat{
			Type: "json_schema",
			JSONSchema: chatCompletionsJSONSchema{
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
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint("/v1/chat/completions"), bytes.NewReader(body))
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
		return Result{}, fmt.Errorf("minimax recognition failed: status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var parsed chatCompletionsResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return Result{}, err
	}
	text := parsed.firstChoiceText()
	if strings.TrimSpace(text) == "" {
		return Result{}, errors.New("minimax recognition returned empty output")
	}
	var output Result
	if err := json.Unmarshal([]byte(extractModelJSONText(text)), &output); err != nil {
		fallback, ok := fallbackMiniMaxTextResult(text)
		if !ok {
			return Result{}, fmt.Errorf("parse minimax recognition json: %w: %s", err, compactModelText(text))
		}
		output = fallback
	}
	output.RawResult = json.RawMessage(responseBody)
	output.Provider = "minimax"
	return normalize(output), nil
}

func (r *MiniMaxRecognizer) endpoint(path string) string {
	return endpoint(r.baseURL, path)
}

type chatCompletionsRequest struct {
	Model          string                        `json:"model"`
	Messages       []chatCompletionsMessage      `json:"messages"`
	ResponseFormat chatCompletionsResponseFormat `json:"response_format"`
}

type chatCompletionsMessage struct {
	Role    string                   `json:"role"`
	Content []chatCompletionsContent `json:"content"`
}

type chatCompletionsContent struct {
	Type     string                   `json:"type"`
	Text     string                   `json:"text,omitempty"`
	ImageURL *chatCompletionsImageURL `json:"image_url,omitempty"`
}

type chatCompletionsImageURL struct {
	URL string `json:"url"`
}

type chatCompletionsResponseFormat struct {
	Type       string                    `json:"type"`
	JSONSchema chatCompletionsJSONSchema `json:"json_schema"`
}

type chatCompletionsJSONSchema struct {
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

func compactModelText(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	var builder strings.Builder
	for index, field := range fields {
		if index > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(field)
	}
	text := builder.String()
	const maxLength = 500
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "...[truncated]"
}

func extractModelJSONText(value string) string {
	text := extractJSONText(value)
	if json.Valid([]byte(text)) {
		return text
	}
	if extracted, ok := firstJSONObject(text); ok {
		return extracted
	}
	return text
}

func firstJSONObject(value string) (string, bool) {
	text := strings.TrimSpace(value)
	offset := 0
	for offset < len(text) {
		relativeStart := strings.Index(text[offset:], "{")
		if relativeStart < 0 {
			return "", false
		}
		start := offset + relativeStart
		depth := 0
		inString := false
		escaped := false
	scanCandidate:
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
					break scanCandidate
				}
			}
		}
		offset = start + 1
	}
	return "", false
}

func fallbackMiniMaxTextResult(value string) (Result, bool) {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return Result{}, false
	}
	for _, fallback := range []struct {
		markers    []string
		sceneType  string
		areaNumber string
	}{
		{markers: []string{"弱电室"}, sceneType: "machine_room", areaNumber: "弱电室"},
		{markers: []string{"弱电间"}, sceneType: "machine_room", areaNumber: "弱电间"},
		{markers: []string{"机房", "machine room", "weak current room"}, sceneType: "machine_room", areaNumber: "机房"},
		{markers: []string{"医生办公室", "doctor's office", "doctor office"}, sceneType: "unknown", areaNumber: "医生办公室"},
		{markers: []string{"北侧门", "north side door"}, sceneType: "entrance", areaNumber: "北侧门"},
		{markers: []string{"南侧门", "south side door"}, sceneType: "entrance", areaNumber: "南侧门"},
		{markers: []string{"东侧门", "east side door"}, sceneType: "entrance", areaNumber: "东侧门"},
		{markers: []string{"西侧门", "west side door"}, sceneType: "entrance", areaNumber: "西侧门"},
		{markers: []string{"入口", "entrance", "door area"}, sceneType: "entrance", areaNumber: "入口"},
		{markers: []string{"走廊", "corridor", "hallway"}, sceneType: "corridor", areaNumber: "走廊"},
		{markers: []string{"通道", "passage"}, sceneType: "passage", areaNumber: "通道"},
	} {
		if !containsAny(text, fallback.markers) {
			continue
		}
		return Result{
			SceneType:      fallback.sceneType,
			AreaNumber:     fallback.areaNumber,
			DecisionSource: "scene",
			Confidence:     "low",
			NeedsReview:    true,
			RawNotes:       "模型未返回合法 JSON，已根据文本描述识别为非业务区域，需人工复核。",
		}, true
	}
	return Result{}, false
}

func containsAny(value string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(value, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func minimaxPrompt(base string) string {
	return strings.TrimSpace(`重要：禁止输出 <think>、分析过程、解释、Markdown 或代码块。你必须只返回一个合法 JSON 对象，且 JSON 必须符合下面要求。

` + base)
}

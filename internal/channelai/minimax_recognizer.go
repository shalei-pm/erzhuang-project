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
		return Result{}, fmt.Errorf("parse minimax recognition json: %w: %s", err, compactModelText(text))
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
	text := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
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

func minimaxPrompt(base string) string {
	return strings.TrimSpace(`重要：禁止输出 <think>、分析过程、解释、Markdown 或代码块。你必须只返回一个合法 JSON 对象，且 JSON 必须符合下面要求。

` + base)
}

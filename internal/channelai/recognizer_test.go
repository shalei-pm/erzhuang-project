package channelai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewRecognizerFromEnvUsesOpenAIByDefault(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("VISION_API_KEY", "vision-key")
	t.Setenv("CHANNEL_AI_PROVIDER", "")

	recognizer, enabled, err := NewRecognizerFromEnv()
	if err != nil {
		t.Fatalf("new recognizer: %v", err)
	}
	if !enabled {
		t.Fatalf("expected recognizer to be enabled")
	}
	if _, ok := recognizer.(*OpenAIRecognizer); !ok {
		t.Fatalf("expected OpenAI recognizer, got %T", recognizer)
	}
}

func TestNewRecognizerFromEnvUsesExternalCommandProvider(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "external-command")
	t.Setenv("CHANNEL_AI_COMMAND", "/opt/tools/understand_image.py")

	recognizer, enabled, err := NewRecognizerFromEnv()
	if err != nil {
		t.Fatalf("new recognizer: %v", err)
	}
	if !enabled {
		t.Fatalf("expected recognizer to be enabled")
	}
	commandRecognizer, ok := recognizer.(*CommandRecognizer)
	if !ok {
		t.Fatalf("expected command recognizer, got %T", recognizer)
	}
	if commandRecognizer.command != "/opt/tools/understand_image.py" {
		t.Fatalf("unexpected command path %q", commandRecognizer.command)
	}
}

func TestNewRecognizerFromEnvRejectsExternalCommandWithoutPath(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "external-command")
	t.Setenv("CHANNEL_AI_COMMAND", "")

	_, _, err := NewRecognizerFromEnv()
	if err == nil {
		t.Fatalf("expected missing command error")
	}
}

func TestNewRecognizerFromEnvUsesMiniMaxScriptDefaultPath(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax-script")
	t.Setenv("MINIMAX_UNDERSTAND_IMAGE_SCRIPT", "")
	t.Setenv("CHANNEL_AI_COMMAND", "")

	recognizer, enabled, err := NewRecognizerFromEnv()
	if err != nil {
		t.Fatalf("new recognizer: %v", err)
	}
	if !enabled {
		t.Fatalf("expected recognizer to be enabled")
	}
	commandRecognizer, ok := recognizer.(*CommandRecognizer)
	if !ok {
		t.Fatalf("expected command recognizer, got %T", recognizer)
	}
	if commandRecognizer.command != "/root/.openclaw/workspace/skills/minimax-understand-image/scripts/understand_image.py" {
		t.Fatalf("unexpected command path %q", commandRecognizer.command)
	}
}

func TestNewRecognizerFromEnvUsesMiniMaxHTTPProvider(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")
	t.Setenv("MINIMAX_BASE_URL", "https://api.minimaxi.com")
	t.Setenv("MINIMAX_MODEL", "MiniMax-M1")

	recognizer, enabled, err := NewRecognizerFromEnv()
	if err != nil {
		t.Fatalf("new recognizer: %v", err)
	}
	if !enabled {
		t.Fatalf("expected recognizer to be enabled")
	}
	minimaxRecognizer, ok := recognizer.(*MiniMaxRecognizer)
	if !ok {
		t.Fatalf("expected minimax recognizer, got %T", recognizer)
	}
	if minimaxRecognizer.apiKey != "minimax-key" {
		t.Fatalf("unexpected api key")
	}
	if minimaxRecognizer.model != "MiniMax-M1" {
		t.Fatalf("unexpected model %q", minimaxRecognizer.model)
	}
}

func TestNewRecognizerFromEnvUsesSmokeTestedMiniMaxDefaultModel(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")
	t.Setenv("MINIMAX_MODEL", "")

	recognizer, enabled, err := NewRecognizerFromEnv()
	if err != nil {
		t.Fatalf("new recognizer: %v", err)
	}
	if !enabled {
		t.Fatalf("expected recognizer to be enabled")
	}
	minimaxRecognizer, ok := recognizer.(*MiniMaxRecognizer)
	if !ok {
		t.Fatalf("expected minimax recognizer, got %T", recognizer)
	}
	if minimaxRecognizer.model != "MiniMax-M3" {
		t.Fatalf("unexpected default model %q", minimaxRecognizer.model)
	}
}

func TestNewRecognizerFromEnvRejectsMiniMaxHTTPWithoutKey(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax")
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("VISION_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, _, err := NewRecognizerFromEnv()
	if err == nil {
		t.Fatalf("expected missing minimax key error")
	}
}

func TestNewRecognizerFromEnvDoesNotUseOpenAIKeyForMiniMax(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax")
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("VISION_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "openai-key")

	_, _, err := NewRecognizerFromEnv()
	if err == nil {
		t.Fatalf("expected missing minimax key error")
	}
}

func TestMiniMaxRecognizerDoesNotDuplicateV1Path(t *testing.T) {
	var requestedPath string
	recognizer := &MiniMaxRecognizer{
		apiKey:  "test-key",
		baseURL: "https://api.minimaxi.com/v1",
		model:   "MiniMax-M1",
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requestedPath = r.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"choices": [{
						"message": {
							"content": "{\"scene_type\":\"treatment\",\"area_type\":\"treatment\",\"area_number\":\"1\",\"card_text\":\"治疗室 1\",\"decision_source\":\"number_card\",\"confidence\":\"high\",\"needs_review\":false,\"raw_notes\":\"测试\"}"
						}
					}]
				}`)),
			}, nil
		})},
	}
	if _, err := recognizer.Recognize(context.Background(), "https://example.test/channel.jpg"); err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if strings.Count(requestedPath, "/v1") != 1 || requestedPath != "/v1/chat/completions" {
		t.Fatalf("expected /v1/chat/completions without duplicate v1, got %q", requestedPath)
	}
}

func TestExtractModelJSONTextSkipsThinkPrefix(t *testing.T) {
	text := `<think>我先分析图片。</think>
这里是说明文字
{"scene_type":"machine_room","area_type":"","area_number":"机房","card_text":"","decision_source":"scene","confidence":"high","needs_review":false,"raw_notes":"画面为机房"}`

	var result Result
	if err := json.Unmarshal([]byte(extractModelJSONText(text)), &result); err != nil {
		t.Fatalf("extract model json: %v", err)
	}
	if result.SceneType != "machine_room" || result.AreaNumber != "机房" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestExtractModelJSONTextSkipsInvalidBraceBeforeResult(t *testing.T) {
	text := `<think>The image has text {weak current room}, but this is analysis, not JSON.
The final answer is below.</think>
{"scene_type":"machine_room","area_type":"","area_number":"弱电室","card_text":"","decision_source":"scene","confidence":"high","needs_review":false,"raw_notes":"画面右下角文字为弱电室"}`

	var result Result
	if err := json.Unmarshal([]byte(extractModelJSONText(text)), &result); err != nil {
		t.Fatalf("extract model json: %v", err)
	}
	if result.SceneType != "machine_room" || result.AreaNumber != "弱电室" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestMiniMaxRecognizerFallsBackFromWeakCurrentRoomExplanation(t *testing.T) {
	recognizer := &MiniMaxRecognizer{
		apiKey:  "test-key",
		baseURL: "https://api.minimaxi.com/v1",
		model:   "MiniMax-M3",
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"choices": [{
						"message": {
							"content": "<think>The image shows a surveillance screenshot. In the bottom right, there is text that reads \"弱电室\" which means weak current room or machine room. This is not a business area like treatment, consultation, or beauty.</think>"
						}
					}]
				}`)),
			}, nil
		})},
	}

	result, err := recognizer.Recognize(context.Background(), "https://example.test/channel.jpg")
	if err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if result.SceneType != "machine_room" || result.AreaNumber != "弱电室" {
		t.Fatalf("unexpected result %#v", result)
	}
	if !result.NeedsReview || result.Confidence != "low" {
		t.Fatalf("expected fallback result to require review, got %#v", result)
	}
}

func TestParseCommandRecognitionOutputAcceptsWrappedResult(t *testing.T) {
	result, err := parseCommandRecognitionOutput([]byte(`{
		"result": {
			"scene_type": "machine_room",
			"area_type": "",
			"area_number": "机房",
			"card_text": "",
			"decision_source": "scene",
			"confidence": "high",
			"needs_review": false,
			"raw_notes": "画面是机房"
		}
	}`))
	if err != nil {
		t.Fatalf("parse command output: %v", err)
	}
	if result.SceneType != "machine_room" || result.AreaNumber != "机房" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestOpenAIRecognizerDoesNotDuplicateV1Path(t *testing.T) {
	var requestedPath string
	recognizer := &OpenAIRecognizer{
		apiKey:  "test-key",
		baseURL: "https://example.test/v1",
		model:   "MiniMax-M3",
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requestedPath = r.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
			"output": [{
				"content": [{
					"type": "output_text",
					"text": "{\"scene_type\":\"treatment\",\"area_type\":\"treatment\",\"area_number\":\"1\",\"card_text\":\"治疗室 1\",\"decision_source\":\"number_card\",\"confidence\":\"high\",\"needs_review\":false,\"raw_notes\":\"测试\"}"
				}]
			}]
		}`)),
			}, nil
		})},
	}
	if _, err := recognizer.Recognize(context.Background(), "https://example.test/channel.jpg"); err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if strings.Count(requestedPath, "/v1") != 1 || requestedPath != "/v1/responses" {
		t.Fatalf("expected /v1/responses without duplicate v1, got %q", requestedPath)
	}
}

func TestOpenAIRecognizerParsesMarkdownWrappedJSON(t *testing.T) {
	recognizer := &OpenAIRecognizer{
		apiKey:  "test-key",
		baseURL: "https://example.test/v1",
		model:   "MiniMax-M3",
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader("{\"output\":[{\"content\":[{\"type\":\"output_text\",\"text\":\"```json\\n{\\\"scene_type\\\":\\\"machine_room\\\",\\\"area_type\\\":\\\"\\\",\\\"area_number\\\":\\\"机房\\\",\\\"card_text\\\":\\\"\\\",\\\"decision_source\\\":\\\"scene\\\",\\\"confidence\\\":\\\"high\\\",\\\"needs_review\\\":false,\\\"raw_notes\\\":\\\"画面是机房\\\"}\\n```\"}]}]}")),
			}, nil
		})},
	}

	result, err := recognizer.Recognize(context.Background(), "https://example.test/channel.jpg")
	if err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if result.SceneType != "machine_room" || result.AreaNumber != "机房" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestOpenAIRecognizerParsesThinkWrappedJSON(t *testing.T) {
	recognizer := &OpenAIRecognizer{
		apiKey:  "test-key",
		baseURL: "https://example.test/v1",
		model:   "gpt-5.5",
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader("{\"output\":[{\"content\":[{\"type\":\"output_text\",\"text\":\"<think>analysis {not json}</think>\\n{\\\"scene_type\\\":\\\"machine_room\\\",\\\"area_type\\\":\\\"\\\",\\\"area_number\\\":\\\"弱电室\\\",\\\"card_text\\\":\\\"\\\",\\\"decision_source\\\":\\\"scene\\\",\\\"confidence\\\":\\\"high\\\",\\\"needs_review\\\":false,\\\"raw_notes\\\":\\\"画面为弱电室\\\"}\"}]}]}")),
			}, nil
		})},
	}

	result, err := recognizer.Recognize(context.Background(), "https://example.test/channel.jpg")
	if err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if result.SceneType != "machine_room" || result.AreaNumber != "弱电室" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestOpenAIProviderNameDetectsMiniMax(t *testing.T) {
	if got := openAIProviderName("https://api.minimaxi.com/v1", "MiniMax-M3"); got != "minimax" {
		t.Fatalf("expected minimax provider, got %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

package channelai

import (
	"context"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

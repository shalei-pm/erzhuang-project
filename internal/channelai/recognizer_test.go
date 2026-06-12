package channelai

import (
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

package designplan

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewRecognizerFromEnvWithAssetReaderUsesOpenAIByDefault(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "")
	t.Setenv("DESIGN_PLAN_AI_PROVIDER", "")
	t.Setenv("OPENAI_API_KEY", "openai-key")

	recognizer := NewRecognizerFromEnvWithAssetReader(nil)
	openAIRecognizer, ok := recognizer.(*OpenAIRecognizer)
	if !ok {
		t.Fatalf("expected openai recognizer, got %T", recognizer)
	}
	if openAIRecognizer.apiKey != "openai-key" {
		t.Fatalf("unexpected openai key")
	}
}

func TestNewRecognizerFromEnvWithAssetReaderUsesMiniMaxProvider(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax")
	t.Setenv("DESIGN_PLAN_AI_PROVIDER", "")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")
	t.Setenv("MINIMAX_MODEL", "MiniMax-M1")

	recognizer := NewRecognizerFromEnvWithAssetReader(nil)
	minimaxRecognizer, ok := recognizer.(*MiniMaxRecognizer)
	if !ok {
		t.Fatalf("expected minimax recognizer, got %T", recognizer)
	}
	if minimaxRecognizer.apiKey != "minimax-key" {
		t.Fatalf("unexpected minimax key")
	}
	if minimaxRecognizer.model != "MiniMax-M1" {
		t.Fatalf("unexpected model %q", minimaxRecognizer.model)
	}
}

func TestNewRecognizerFromEnvWithAssetReaderUsesSmokeTestedMiniMaxDefaultModel(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax")
	t.Setenv("DESIGN_PLAN_AI_PROVIDER", "")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")
	t.Setenv("MINIMAX_MODEL", "")

	recognizer := NewRecognizerFromEnvWithAssetReader(nil)
	minimaxRecognizer, ok := recognizer.(*MiniMaxRecognizer)
	if !ok {
		t.Fatalf("expected minimax recognizer, got %T", recognizer)
	}
	if minimaxRecognizer.model != "MiniMax-M3" {
		t.Fatalf("unexpected default model %q", minimaxRecognizer.model)
	}
}

func TestNewRecognizerFromEnvWithAssetReaderDoesNotUseOpenAIKeyForMiniMax(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax")
	t.Setenv("DESIGN_PLAN_AI_PROVIDER", "")
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "openai-key")

	recognizer := NewRecognizerFromEnvWithAssetReader(nil)
	minimaxRecognizer, ok := recognizer.(*MiniMaxRecognizer)
	if !ok {
		t.Fatalf("expected minimax recognizer, got %T", recognizer)
	}
	if minimaxRecognizer.apiKey != "" {
		t.Fatalf("expected empty minimax api key when MINIMAX_API_KEY is not configured")
	}
}

func TestNewRecognizerFromEnvWithAssetReaderAllowsDesignPlanOverride(t *testing.T) {
	t.Setenv("CHANNEL_AI_PROVIDER", "minimax")
	t.Setenv("DESIGN_PLAN_AI_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("MINIMAX_API_KEY", "minimax-key")

	recognizer := NewRecognizerFromEnvWithAssetReader(nil)
	if _, ok := recognizer.(*OpenAIRecognizer); !ok {
		t.Fatalf("expected design plan override to use openai, got %T", recognizer)
	}
}

func TestMiniMaxRecognizerParsesMarkdownWrappedJSON(t *testing.T) {
	recognizer := &MiniMaxRecognizer{
		apiKey: "test-key",
		model:  "MiniMax-M1",
		readAsset: func(value string) (io.ReadCloser, string, error) {
			return io.NopCloser(strings.NewReader("fake image bytes")), "image/png", nil
		},
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader("{\"choices\":[{\"message\":{\"content\":\"```json\\n{\\\"store_name\\\":\\\"新氧青春测试店\\\",\\\"store_name_confidence\\\":\\\"high\\\",\\\"areas\\\":[{\\\"name\\\":\\\"治疗室 1\\\",\\\"type\\\":\\\"treatment\\\",\\\"number\\\":\\\"1\\\",\\\"confidence\\\":\\\"high\\\",\\\"needs_review\\\":false,\\\"box\\\":{\\\"x\\\":0.1,\\\"y\\\":0.2,\\\"width\\\":0.3,\\\"height\\\":0.4}}],\\\"raw_notes\\\":\\\"测试\\\"}\\n```\"}}]}")),
			}, nil
		})},
	}

	result, err := recognizer.Recognize(context.Background(), &UploadResult{PreviewPath: "preview.png"})
	if err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if result.StoreName != "新氧青春测试店" {
		t.Fatalf("unexpected store name %q", result.StoreName)
	}
	if len(result.Areas) != 1 || result.Areas[0].Type != AreaTypeTreatment || result.Areas[0].Number != "1" {
		t.Fatalf("unexpected areas %#v", result.Areas)
	}
}

func TestMiniMaxRecognizerDoesNotDuplicateV1Path(t *testing.T) {
	var requestedPath string
	recognizer := &MiniMaxRecognizer{
		apiKey:  "test-key",
		baseURL: "https://api.minimaxi.com/v1",
		model:   "MiniMax-M1",
		readAsset: func(value string) (io.ReadCloser, string, error) {
			return io.NopCloser(strings.NewReader("fake image bytes")), "image/png", nil
		},
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requestedPath = r.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader("{\"choices\":[{\"message\":{\"content\":\"{\\\"store_name\\\":\\\"新氧青春测试店\\\",\\\"store_name_confidence\\\":\\\"high\\\",\\\"areas\\\":[],\\\"raw_notes\\\":\\\"测试\\\"}\"}}]}")),
			}, nil
		})},
	}

	if _, err := recognizer.Recognize(context.Background(), &UploadResult{PreviewPath: "preview.png"}); err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if strings.Count(requestedPath, "/v1") != 1 || requestedPath != "/v1/chat/completions" {
		t.Fatalf("expected /v1/chat/completions without duplicate v1, got %q", requestedPath)
	}
}

func TestExtractRecognitionJSONTextFromModelSkipsThinkPrefix(t *testing.T) {
	text := `<think>先判断图纸。</think>
无法识别门店名称，但返回结构如下：
{"store_name":"","store_name_confidence":"low","areas":[],"raw_notes":"图片内容为空"}`

	var output recognizerOutput
	if err := json.Unmarshal([]byte(extractRecognitionJSONTextFromModel(text)), &output); err != nil {
		t.Fatalf("extract model json: %v", err)
	}
	if output.StoreNameConfidence != ConfidenceLow || output.RawNotes != "图片内容为空" {
		t.Fatalf("unexpected output %#v", output)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

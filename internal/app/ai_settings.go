package app

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type AIProvider string

const (
	AIProviderOpenAI  AIProvider = "openai"
	AIProviderMiniMax AIProvider = "minimax"
)

type AISettings struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Label    string `json:"label"`
}

type AISettingsStore interface {
	GetAIProvider(ctx context.Context) (string, error)
	SetAIProvider(ctx context.Context, provider string) error
}

const monitorScreenshotWatermarkSettingKey = "monitor_screenshot_watermark_enabled"

type MonitorScreenshotSettingsStore interface {
	GetMonitorScreenshotWatermarkEnabled(ctx context.Context) (bool, error)
	SetMonitorScreenshotWatermarkEnabled(ctx context.Context, enabled bool) error
}

func NormalizeMonitorScreenshotWatermarkEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "no":
		return false
	default:
		return true
	}
}

func NormalizeAIProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "openai", "responses", "openai-responses":
		return string(AIProviderOpenAI)
	case "minimax":
		return string(AIProviderMiniMax)
	default:
		return ""
	}
}

func NextAIProvider(current string) string {
	if NormalizeAIProvider(current) == string(AIProviderMiniMax) {
		return string(AIProviderOpenAI)
	}
	return string(AIProviderMiniMax)
}

func AISettingsFromProvider(provider string) AISettings {
	normalized := NormalizeAIProvider(provider)
	if normalized == "" {
		normalized = string(AIProviderOpenAI)
	}
	switch normalized {
	case string(AIProviderMiniMax):
		model := settingEnv("MINIMAX_MODEL", "MiniMax-M3")
		return AISettings{Provider: normalized, Model: model, Label: fmt.Sprintf("MiniMax / %s", model)}
	default:
		model := settingEnv("VISION_MODEL", settingEnv("OPENAI_MODEL", "gpt-5.5"))
		return AISettings{Provider: string(AIProviderOpenAI), Model: model, Label: fmt.Sprintf("OpenAI / %s", model)}
	}
}

func settingEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

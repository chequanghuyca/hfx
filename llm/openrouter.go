package llm

import (
	"os"
	"strconv"
	"strings"
	"time"

	"nofx/mcp"
)

const (
	OpenRouterModelID      = "openrouter_env"
	OpenRouterProvider     = "custom"
	DefaultOpenRouterURL   = "https://openrouter.ai/api/v1"
	DefaultOpenRouterModel = "openai/gpt-oss-20b:free"

	DefaultOpenRouterMaxTokens      = 1000
	DefaultOpenRouterTimeoutSeconds = 240
	DefaultOpenRouterSingleModel    = true
)

var DefaultOpenRouterFallbackModels = []string{
	"poolside/laguna-xs.2:free",
	"openai/gpt-oss-20b:free",
	"nvidia/nemotron-3-nano-30b-a3b:free",
	"openai/gpt-oss-20b:free",
}

// OpenRouterConfig is a process-wide AI override loaded from environment.
// The API key remains outside the database and source control.
type OpenRouterConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	FallbackModels []string
	SingleModel    bool
	MaxTokens      int
	TimeoutSeconds int
}

// ForcedOpenRouterConfig returns the fixed OpenRouter configuration when enabled.
// OPENROUTER_* aliases are accepted for compatibility, while OPEN_ROUTER_* is preferred.
func ForcedOpenRouterConfig() (OpenRouterConfig, bool) {
	apiKey := firstEnv("OPEN_ROUTER_API_KEY", "OPENROUTER_API_KEY")
	if apiKey == "" {
		return OpenRouterConfig{}, false
	}

	baseURL := firstEnv("OPEN_ROUTER_BASE_URL", "OPENROUTER_BASE_URL")
	if baseURL == "" {
		baseURL = DefaultOpenRouterURL
	}

	model := firstEnv("OPEN_ROUTER_MODEL", "OPENROUTER_MODEL")
	if model == "" {
		model = DefaultOpenRouterModel
	}

	singleModel := envBool(DefaultOpenRouterSingleModel, "OPEN_ROUTER_SINGLE_MODEL", "OPENROUTER_SINGLE_MODEL")
	fallbackModels := envStringList("OPEN_ROUTER_FALLBACK_MODELS", "OPENROUTER_FALLBACK_MODELS")
	if singleModel {
		fallbackModels = nil
	} else if len(fallbackModels) == 0 {
		fallbackModels = append([]string(nil), DefaultOpenRouterFallbackModels...)
	}

	return OpenRouterConfig{
		APIKey:         apiKey,
		BaseURL:        strings.TrimRight(baseURL, "/"),
		Model:          model,
		FallbackModels: cleanModelList(fallbackModels, model),
		SingleModel:    singleModel,
		MaxTokens:      envInt(DefaultOpenRouterMaxTokens, "OPEN_ROUTER_MAX_TOKENS", "OPENROUTER_MAX_TOKENS"),
		TimeoutSeconds: envInt(DefaultOpenRouterTimeoutSeconds, "OPEN_ROUTER_TIMEOUT_SECONDS", "OPENROUTER_TIMEOUT_SECONDS"),
	}, true
}

// NewClient creates an AI client. When OpenRouter is configured, it always wins
// over per-user/provider settings so every trading call uses one fixed model.
func NewClient(provider, apiKey, customURL, customModel string) (mcp.AIClient, string) {
	if forced, ok := ForcedOpenRouterConfig(); ok {
		maxRetries := 4
		if !forced.SingleModel {
			maxRetries = len(forced.FallbackModels) + 1
			if maxRetries < 3 {
				maxRetries = 3
			}
		}
		client := mcp.NewClient(
			mcp.WithProvider(OpenRouterProvider),
			mcp.WithTemperature(0.2),
			mcp.WithMaxTokens(forced.MaxTokens),
			mcp.WithMaxContext(180000),
			mcp.WithTimeout(time.Duration(forced.TimeoutSeconds)*time.Second),
			mcp.WithFallbackModels(forced.FallbackModels),
			mcp.WithMaxRetries(maxRetries),
		)
		client.SetAPIKey(forced.APIKey, forced.BaseURL, forced.Model)
		return client, forced.Model
	}

	client := mcp.NewAIClientByProvider(provider)
	if client == nil {
		client = mcp.NewClient()
	}

	switch provider {
	case "blockrun-base", "blockrun-sol", "claw402":
		client.SetAPIKey(apiKey, "", customModel)
	default:
		client.SetAPIKey(apiKey, customURL, customModel)
	}

	return client, customModel
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func envInt(defaultValue int, keys ...string) int {
	value := firstEnv(keys...)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}

func envBool(defaultValue bool, keys ...string) bool {
	value := strings.ToLower(firstEnv(keys...))
	if value == "" {
		return defaultValue
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func envStringList(keys ...string) []string {
	value := firstEnv(keys...)
	if value == "" {
		return nil
	}
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
}

func cleanModelList(models []string, primary string) []string {
	seen := map[string]bool{strings.TrimSpace(primary): true}
	cleaned := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		cleaned = append(cleaned, model)
	}
	return cleaned
}

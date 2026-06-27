package llm

import (
	"testing"

	"nofx/mcp"
)

func TestForcedOpenRouterConfig(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "test-key")
	t.Setenv("OPEN_ROUTER_MODEL", "")
	t.Setenv("OPEN_ROUTER_BASE_URL", "")

	cfg, ok := ForcedOpenRouterConfig()
	if !ok {
		t.Fatal("expected OpenRouter override to be enabled")
	}
	if cfg.Model != DefaultOpenRouterModel {
		t.Fatalf("unexpected default model: %s", cfg.Model)
	}
	if cfg.BaseURL != DefaultOpenRouterURL {
		t.Fatalf("unexpected default base URL: %s", cfg.BaseURL)
	}
	if cfg.MaxTokens != DefaultOpenRouterMaxTokens {
		t.Fatalf("unexpected default max tokens: %d", cfg.MaxTokens)
	}
	if cfg.TimeoutSeconds != DefaultOpenRouterTimeoutSeconds {
		t.Fatalf("unexpected default timeout: %d", cfg.TimeoutSeconds)
	}
	if !cfg.SingleModel {
		t.Fatal("expected single-model mode by default")
	}
	if len(cfg.FallbackModels) != 0 {
		t.Fatalf("single-model mode should disable fallbacks, got %v", cfg.FallbackModels)
	}
}

func TestForcedOpenRouterConfigDisabledWithoutKey(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	if _, ok := ForcedOpenRouterConfig(); ok {
		t.Fatal("expected OpenRouter override to be disabled")
	}
}

func TestForcedOpenRouterConfigReadsTimeoutAndMaxTokens(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "test-key")
	t.Setenv("OPEN_ROUTER_MAX_TOKENS", "900")
	t.Setenv("OPEN_ROUTER_TIMEOUT_SECONDS", "480")

	cfg, ok := ForcedOpenRouterConfig()
	if !ok {
		t.Fatal("expected OpenRouter override to be enabled")
	}
	if cfg.MaxTokens != 900 {
		t.Fatalf("unexpected max tokens: %d", cfg.MaxTokens)
	}
	if cfg.TimeoutSeconds != 480 {
		t.Fatalf("unexpected timeout: %d", cfg.TimeoutSeconds)
	}
}

func TestForcedOpenRouterConfigReadsFallbackModels(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "test-key")
	t.Setenv("OPEN_ROUTER_MODEL", "primary-model")
	t.Setenv("OPEN_ROUTER_SINGLE_MODEL", "false")
	t.Setenv("OPEN_ROUTER_FALLBACK_MODELS", "fallback-a, primary-model; fallback-b\nfallback-c")

	cfg, ok := ForcedOpenRouterConfig()
	if !ok {
		t.Fatal("expected OpenRouter override to be enabled")
	}
	expected := []string{"fallback-a", "fallback-b", "fallback-c"}
	if len(cfg.FallbackModels) != len(expected) {
		t.Fatalf("unexpected fallback count: got %d want %d (%v)", len(cfg.FallbackModels), len(expected), cfg.FallbackModels)
	}
	for i, model := range expected {
		if cfg.FallbackModels[i] != model {
			t.Fatalf("unexpected fallback model at %d: got %s want %s", i, cfg.FallbackModels[i], model)
		}
	}
}

func TestForcedOpenRouterConfigSingleModelIgnoresFallbackList(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "test-key")
	t.Setenv("OPEN_ROUTER_MODEL", "primary-model")
	t.Setenv("OPEN_ROUTER_SINGLE_MODEL", "true")
	t.Setenv("OPEN_ROUTER_FALLBACK_MODELS", "fallback-a,fallback-b")

	cfg, ok := ForcedOpenRouterConfig()
	if !ok {
		t.Fatal("expected OpenRouter override to be enabled")
	}
	if !cfg.SingleModel {
		t.Fatal("expected single-model mode")
	}
	if len(cfg.FallbackModels) != 0 {
		t.Fatalf("expected no fallback models, got %v", cfg.FallbackModels)
	}
}

func TestNewClientSingleModelUsesFourRetries(t *testing.T) {
	t.Setenv("OPEN_ROUTER_API_KEY", "test-key")
	t.Setenv("OPEN_ROUTER_MODEL", "openai/gpt-oss-20b:free")
	t.Setenv("OPEN_ROUTER_SINGLE_MODEL", "true")

	client, model := NewClient("", "", "", "")
	if model != "openai/gpt-oss-20b:free" {
		t.Fatalf("unexpected model: %s", model)
	}
	base, ok := client.(*mcp.Client)
	if !ok {
		t.Fatalf("expected base MCP client, got %T", client)
	}
	if base.Cfg.MaxRetries != 4 {
		t.Fatalf("single-model mode should use four retries, got %d", base.Cfg.MaxRetries)
	}
	if len(base.FallbackModels) != 0 {
		t.Fatalf("single-model mode should have no fallback models, got %v", base.FallbackModels)
	}
}

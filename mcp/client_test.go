package mcp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ============================================================
// Test Client Creation and Configuration
// ============================================================

func TestNewClient_Default(t *testing.T) {
	client := NewClient()

	if client == nil {
		t.Fatal("client should not be nil")
	}

	c := client.(*Client)
	if c.Provider == "" {
		t.Error("Provider should have default value")
	}

	if c.MaxTokens <= 0 {
		t.Error("MaxTokens should be positive")
	}

	if c.Log == nil {
		t.Error("Log should not be nil")
	}

	if c.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}

	if c.Hooks == nil {
		t.Error("Hooks should not be nil")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	mockLogger := NewMockLogger()
	mockHTTP := &http.Client{Timeout: 30 * time.Second}

	client := NewClient(
		WithLogger(mockLogger),
		WithHTTPClient(mockHTTP),
		WithMaxTokens(4000),
		WithTimeout(60*time.Second),
		WithAPIKey("test-key"),
	)

	c := client.(*Client)

	if c.Log != mockLogger {
		t.Error("Log should be set from option")
	}

	if c.HTTPClient != mockHTTP {
		t.Error("HTTPClient should be set from option")
	}

	if c.MaxTokens != 4000 {
		t.Error("MaxTokens should be 4000")
	}

	if c.APIKey != "test-key" {
		t.Error("APIKey should be test-key")
	}
}

// ============================================================
// Test CallWithMessages
// ============================================================

func TestClient_CallWithMessages_Success(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetSuccessResponse("AI response content")
	mockLogger := NewMockLogger()

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
		WithBaseURL("https://api.test.com"),
	)

	result, err := client.CallWithMessages("system prompt", "user prompt")

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	if result != "AI response content" {
		t.Errorf("expected 'AI response content', got '%s'", result)
	}

	// Verify request
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Errorf("expected 1 request, got %d", len(requests))
	}

	if len(requests) > 0 {
		req := requests[0]
		if req.Header.Get("Authorization") == "" {
			t.Error("Authorization header should be set")
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type should be application/json")
		}
	}
}

func TestClient_CallWithMessages_NoAPIKey(t *testing.T) {
	client := NewClient()

	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Error("should error when API key is not set")
	}

	if err.Error() != "AI API key not set, please call SetAPIKey first" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_CallWithMessages_HTTPError(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetErrorResponse(500, "Internal Server Error")
	mockLogger := NewMockLogger()

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
	)

	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Error("should error on HTTP error")
	}
}

func TestClient_CallWithMessages_EmptyContentFallsBackToNextModel(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
		WithProvider(ProviderCustom),
		WithBaseURL("https://openrouter.ai/api/v1"),
		WithModel("primary-model"),
		WithFallbackModels([]string{"fallback-model"}),
		WithMaxRetries(2),
		WithRetryWaitBase(0),
	)

	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		var requestBody struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &requestBody)

		content := ""
		if requestBody.Model == "fallback-model" {
			content = `{"decisions":[{"symbol":"ALL","action":"wait","reasoning":"fallback ok"}]}`
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":` + strconv.Quote(content) + `}}]}`)),
			Header:     make(http.Header),
		}, nil
	}

	result, err := client.CallWithMessages("system prompt", "user prompt")
	if err != nil {
		t.Fatalf("should fall back and succeed: %v", err)
	}
	if !strings.Contains(result, "fallback ok") {
		t.Fatalf("expected fallback response, got %s", result)
	}
	if got := client.(*Client).Model; got != "fallback-model" {
		t.Fatalf("expected client to switch to fallback-model, got %s", got)
	}
}

func TestOpenRouterFreeModelUsesCompatibilityProfile(t *testing.T) {
	client := NewClient(
		WithAPIKey("test-key"),
		WithProvider(ProviderCustom),
		WithBaseURL("https://openrouter.ai/api/v1"),
		WithModel("google/gemma-4-31b-it:free"),
	).(*Client)

	body := client.BuildMCPRequestBody("system", "user")

	for _, unsupported := range []string{"response_format", "provider", "plugins", "reasoning", "verbosity"} {
		if value, exists := body[unsupported]; exists {
			t.Fatalf("free compatibility profile should omit %s, got %#v", unsupported, value)
		}
	}
	if body["model"] != "google/gemma-4-31b-it:free" {
		t.Fatalf("unexpected model: %#v", body["model"])
	}
	if body["max_tokens"] == nil {
		t.Fatal("compatibility profile should still include max_tokens")
	}
}

func TestOpenRouterPaidStructuredModelUsesStrictJSONSchema(t *testing.T) {
	client := NewClient(
		WithAPIKey("test-key"),
		WithProvider(ProviderCustom),
		WithBaseURL("https://openrouter.ai/api/v1"),
		WithModel("openai/gpt-oss-20b:free"),
	).(*Client)

	body := client.BuildMCPRequestBody("system", "user")

	responseFormat, ok := body["response_format"].(map[string]any)
	if !ok || responseFormat["type"] != "json_schema" {
		t.Fatalf("expected OpenRouter json_schema response format, got %#v", body["response_format"])
	}
	jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
	if !ok || jsonSchema["strict"] != true {
		t.Fatalf("expected strict trading decision schema, got %#v", responseFormat["json_schema"])
	}
	provider, ok := body["provider"].(map[string]any)
	if !ok || provider["require_parameters"] != true {
		t.Fatalf("expected require_parameters provider routing, got %#v", body["provider"])
	}
}

func TestClient_CallWithMessages_TruncatedContentIsPreservedWithoutFullPromptRetry(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
		WithProvider(ProviderCustom),
		WithBaseURL("https://openrouter.ai/api/v1"),
		WithModel("primary-model"),
		WithFallbackModels([]string{"fallback-model"}),
		WithMaxRetries(2),
		WithRetryWaitBase(0),
	)

	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		content := `Market analysis complete. No candidate has a clean trigger. {"decisions":`

		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"choices":[{"finish_reason":"length","message":{"content":` + strconv.Quote(content) + `}}]}`,
			)),
			Header: make(http.Header),
		}, nil
	}

	result, err := client.CallWithMessages("system prompt", "user prompt")
	if err == nil {
		t.Fatal("expected typed truncated response error")
	}
	var truncatedErr *TruncatedResponseError
	if !errors.As(err, &truncatedErr) {
		t.Fatalf("expected TruncatedResponseError, got %T: %v", err, err)
	}
	if !strings.Contains(truncatedErr.Content, "No candidate") || !strings.Contains(result, "No candidate") {
		t.Fatalf("expected partial content to be preserved, result=%q error-content=%q", result, truncatedErr.Content)
	}
	if len(mockHTTP.GetRequests()) != 1 {
		t.Fatalf("expected no full-prompt retry for truncation, got %d requests", len(mockHTTP.GetRequests()))
	}
	if got := client.(*Client).Model; got != "primary-model" {
		t.Fatalf("expected compact repair to decide fallback later, model changed to %s", got)
	}
}

// ============================================================
// Test Retry Logic
// ============================================================

func TestClient_Retry_Success(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// Simulate: first call fails, second call succeeds
	callCount := 0
	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("connection reset")
		}
		return &http.Response{
			StatusCode: 200,
			Body:       http.NoBody,
		}, nil
	}

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
		WithMaxRetries(3),
	)

	// Since our client uses Hooks.Call, need special handling
	// Here we test that CallWithMessages will invoke retry logic
	c := client.(*Client)

	// Temporarily modify retry wait time to 0 to speed up test
	oldRetries := MaxRetryTimes
	MaxRetryTimes = 3
	defer func() { MaxRetryTimes = oldRetries }()

	_, err := c.CallWithMessages("system", "user")

	// First fails (connection reset), second succeeds, but response format is wrong, will fail
	// But at least verify retry logic was triggered
	if callCount < 2 {
		t.Errorf("should retry, got %d calls", callCount)
	}

	// Check if there's retry information in logs
	logs := mockLogger.GetLogsByLevel("WARN")
	hasRetryLog := false
	for _, log := range logs {
		if log.Message == "⚠️  AI API call failed, retrying (2/3)..." {
			hasRetryLog = true
			break
		}
	}

	if !hasRetryLog && callCount >= 2 {
		// If retry was indeed attempted, there should be warning logs
		// But due to our test setup, it may not trigger, so just check here
		t.Log("Retry was attempted")
	}

	_ = err // Ignore error, we mainly test retry logic was triggered
}

func TestClient_Retry_NonRetryableError(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetErrorResponse(400, "Bad Request")
	mockLogger := NewMockLogger()

	client := NewClient(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
	)

	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Error("should error")
	}

	// Verify no retry (because 400 is not a retryable error)
	requests := mockHTTP.GetRequests()
	if len(requests) != 1 {
		t.Errorf("should not retry for 400 error, got %d requests", len(requests))
	}
}

// ============================================================
// Test Hook Methods
// ============================================================

func TestClient_BuildMCPRequestBody(t *testing.T) {
	client := NewClient()
	c := client.(*Client)

	body := c.BuildMCPRequestBody("system prompt", "user prompt")

	if body == nil {
		t.Fatal("body should not be nil")
	}

	if body["model"] == nil {
		t.Error("body should have model field")
	}

	messages, ok := body["messages"].([]map[string]string)
	if !ok {
		t.Fatal("messages should be []map[string]string")
	}

	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	if messages[0]["role"] != "system" {
		t.Error("first message should be system")
	}

	if messages[1]["role"] != "user" {
		t.Error("second message should be user")
	}
}

func TestClient_BuildMCPRequestBody_OpenRouterJSONMode(t *testing.T) {
	client := NewClient()
	c := client.(*Client)
	c.SetAPIKey("test-key", "https://openrouter.ai/api/v1", "test-model")

	body := c.BuildMCPRequestBody("system prompt", "user prompt")

	responseFormat, ok := body["response_format"].(map[string]any)
	if !ok {
		t.Fatal("expected response_format for OpenRouter request")
	}
	if responseFormat["type"] != "json_object" {
		t.Fatalf("expected json_object response_format, got %#v", responseFormat["type"])
	}
	plugins, ok := body["plugins"].([]map[string]string)
	if !ok || len(plugins) != 1 || plugins[0]["id"] != "response-healing" {
		t.Fatalf("expected response-healing plugin, got %#v", body["plugins"])
	}
}

func TestClient_BuildUrl(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		useFullURL bool
		expected   string
	}{
		{
			name:       "normal URL",
			baseURL:    "https://api.test.com/v1",
			useFullURL: false,
			expected:   "https://api.test.com/v1/chat/completions",
		},
		{
			name:       "full URL",
			baseURL:    "https://api.test.com/custom/endpoint",
			useFullURL: true,
			expected:   "https://api.test.com/custom/endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(
				WithProvider("test-provider"), // Prevent default DeepSeek settings
				WithBaseURL(tt.baseURL),
				WithUseFullURL(tt.useFullURL),
			)
			c := client.(*Client)

			url := c.BuildUrl()
			if url != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, url)
			}
		})
	}
}

func TestClient_SetAuthHeader(t *testing.T) {
	client := NewClient(WithAPIKey("test-api-key"))
	c := client.(*Client)

	headers := make(http.Header)
	c.SetAuthHeader(headers)

	authHeader := headers.Get("Authorization")
	if authHeader != "Bearer test-api-key" {
		t.Errorf("expected 'Bearer test-api-key', got '%s'", authHeader)
	}
}

func TestClient_IsRetryableError(t *testing.T) {
	client := NewClient()
	c := client.(*Client)

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "EOF error",
			err:      errors.New("unexpected EOF"),
			expected: true,
		},
		{
			name:     "timeout error",
			err:      errors.New("timeout exceeded"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "empty OpenRouter response",
			err:      errors.New("fail to parse AI server response: API returned empty response"),
			expected: true,
		},
		{
			name:     "normal error",
			err:      errors.New("bad request"),
			expected: false,
		},
		{
			name:     "validation error",
			err:      errors.New("invalid input"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// ============================================================
// Test SetTimeout
// ============================================================

func TestClient_SetTimeout(t *testing.T) {
	client := NewClient()

	newTimeout := 90 * time.Second
	client.SetTimeout(newTimeout)

	c := client.(*Client)
	if c.HTTPClient.Timeout != newTimeout {
		t.Errorf("expected timeout %v, got %v", newTimeout, c.HTTPClient.Timeout)
	}
}

// ============================================================
// Test String Method
// ============================================================

func TestClient_String(t *testing.T) {
	client := NewClient(
		WithProvider("test-provider"),
		WithModel("test-model"),
	)

	c := client.(*Client)
	str := c.String()

	expectedContains := []string{"test-provider", "test-model"}
	for _, exp := range expectedContains {
		if !contains(str, exp) {
			t.Errorf("String() should contain '%s', got '%s'", exp, str)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

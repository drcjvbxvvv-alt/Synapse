package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/shaia/Synapse/internal/models"
)

// ─── SanitizeK8sContext tests ────────────────────────────────────────────────

func TestSanitizeK8sContext_PEMCertificate(t *testing.T) {
	// PEM blocks outside of data: section — processed by pemPattern regex
	input := "spec:\n  cert: |\n    -----BEGIN CERTIFICATE-----\n    MIIBxxx\n    -----END CERTIFICATE-----\n  key: |\n    -----BEGIN RSA PRIVATE KEY-----\n    MIIEyyy\n    -----END RSA PRIVATE KEY-----\n"
	result := SanitizeK8sContext(input)
	assert.Contains(t, result, "[REDACTED: certificate]")
	assert.NotContains(t, result, "MIIBxxx")
	assert.NotContains(t, result, "MIIEyyy")
}

func TestSanitizeK8sContext_EnvVarRedaction(t *testing.T) {
	// The env var pattern: "- name: DB_PASSWORD\n  value: supersecret" → redacts the value line
	input := "env:\n- name: DB_PASSWORD\n  value: supersecret\n- name: NORMAL_VAR\n  value: normalvalue\n"
	result := SanitizeK8sContext(input)
	assert.Contains(t, result, "[REDACTED]")
	assert.NotContains(t, result, "supersecret")
	assert.Contains(t, result, "normalvalue")
}

func TestSanitizeK8sContext_SecretDataBlock(t *testing.T) {
	input := `
apiVersion: v1
kind: Secret
metadata:
  name: mysecret
data:
  username: YWRtaW4=
  password: c2VjcmV0
type: Opaque
`
	result := SanitizeK8sContext(input)
	assert.Contains(t, result, "[REDACTED]")
	assert.NotContains(t, result, "YWRtaW4=")
	assert.NotContains(t, result, "c2VjcmV0")
}

func TestSanitizeK8sContext_StringDataBlock(t *testing.T) {
	input := `
apiVersion: v1
kind: Secret
stringData:
  token: mytoken123
  config: myconfig
`
	result := SanitizeK8sContext(input)
	assert.Contains(t, result, "[REDACTED]")
	assert.NotContains(t, result, "mytoken123")
}

func TestSanitizeK8sContext_NoSensitiveData(t *testing.T) {
	input := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  namespace: default
spec:
  replicas: 3
`
	result := SanitizeK8sContext(input)
	// No sensitive data — result should be close to the input
	assert.Contains(t, result, "myapp")
	assert.Contains(t, result, "replicas: 3")
}

func TestSanitizeK8sContext_EmptyInput(t *testing.T) {
	result := SanitizeK8sContext("")
	assert.Equal(t, "", result)
}

func TestRedactSecretDataBlocks_NormalYAML(t *testing.T) {
	input := "apiVersion: v1\nkind: ConfigMap\ndata:\n  key1: value1\n  key2: value2\nfoo: bar"
	result := redactSecretDataBlocks(input)
	// data block values should be redacted
	assert.Contains(t, result, "[REDACTED]")
	assert.Contains(t, result, "foo: bar")
}

func TestRedactSecretDataBlocks_MultipleBlocks(t *testing.T) {
	input := "data:\n  a: secret1\nstringData:\n  b: secret2\nother: value"
	result := redactSecretDataBlocks(input)
	assert.NotContains(t, result, "secret1")
	assert.NotContains(t, result, "secret2")
	assert.Contains(t, result, "other: value")
}

// ─── AIProvider pure method tests ───────────────────────────────────────────

func makeTestProvider(provider, endpoint, model, apiVersion, apiKey string) *AIProvider {
	cfg := &models.AIConfig{
		Provider:   provider,
		Endpoint:   endpoint,
		Model:      model,
		APIVersion: apiVersion,
		APIKey:     apiKey,
	}
	return NewAIProvider(cfg)
}

func TestAIProvider_IsAnthropic(t *testing.T) {
	p := makeTestProvider("anthropic", "https://api.anthropic.com", "claude-3", "", "key")
	assert.True(t, p.isAnthropic())
	assert.False(t, p.isAzure())
}

func TestAIProvider_IsAzure(t *testing.T) {
	p := makeTestProvider("azure", "https://myresource.openai.azure.com", "gpt-4o", "2024-05-01-preview", "key")
	assert.True(t, p.isAzure())
	assert.False(t, p.isAnthropic())
}

func TestAIProvider_IsOpenAI(t *testing.T) {
	p := makeTestProvider("openai", "https://api.openai.com/v1", "gpt-4o", "", "key")
	assert.False(t, p.isAnthropic())
	assert.False(t, p.isAzure())
}

func TestAIProvider_ChatURL_Anthropic(t *testing.T) {
	p := makeTestProvider("anthropic", "https://api.anthropic.com", "claude-3", "", "key")
	url := p.chatURL(false)
	assert.Equal(t, "https://api.anthropic.com/v1/messages", url)
}

func TestAIProvider_ChatURL_Azure(t *testing.T) {
	p := makeTestProvider("azure", "https://myresource.openai.azure.com", "gpt-4o", "2024-05-01-preview", "key")
	url := p.chatURL(false)
	assert.Contains(t, url, "openai/deployments/gpt-4o/chat/completions")
	assert.Contains(t, url, "api-version=2024-05-01-preview")
}

func TestAIProvider_ChatURL_AzureDefaultVersion(t *testing.T) {
	p := makeTestProvider("azure", "https://myresource.openai.azure.com", "gpt-4o", "", "key")
	url := p.chatURL(false)
	assert.Contains(t, url, "api-version=2024-05-01-preview") // default version
}

func TestAIProvider_ChatURL_OpenAI(t *testing.T) {
	p := makeTestProvider("openai", "https://api.openai.com/v1", "gpt-4o", "", "key")
	url := p.chatURL(false)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", url)
}

func TestAIProvider_ChatURL_TrailingSlash(t *testing.T) {
	p := makeTestProvider("openai", "https://api.openai.com/v1/", "gpt-4o", "", "key")
	url := p.chatURL(false)
	// Trailing slash should be stripped
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", url)
}

func TestAIProvider_BuildOpenAIBody_Basic(t *testing.T) {
	p := makeTestProvider("openai", "https://api.openai.com/v1", "gpt-4o", "", "key")
	req := ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
	}
	body := p.buildOpenAIBody(req, false)
	assert.Equal(t, "gpt-4o", body["model"])
	assert.Equal(t, false, body["stream"])
}

func TestAIProvider_BuildOpenAIBody_WithTools(t *testing.T) {
	p := makeTestProvider("openai", "https://api.openai.com/v1", "gpt-4o", "", "key")
	req := ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "List pods"}},
		Tools:    []ToolDefinition{{Type: "function", Function: FunctionDefinition{Name: "list_pods"}}},
	}
	body := p.buildOpenAIBody(req, false)
	tools, ok := body["tools"]
	assert.True(t, ok)
	assert.NotNil(t, tools)
}

func TestAIProvider_BuildOpenAIBody_Azure_NoModel(t *testing.T) {
	p := makeTestProvider("azure", "https://x.openai.azure.com", "gpt-4o", "", "key")
	req := ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "Hi"}}}
	body := p.buildOpenAIBody(req, false)
	// Azure should not include model in body
	_, hasModel := body["model"]
	assert.False(t, hasModel)
}

func TestAIProvider_BuildAnthropicBody_Basic(t *testing.T) {
	p := makeTestProvider("anthropic", "https://api.anthropic.com", "claude-3-opus-20240229", "", "key")
	req := ChatRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
		},
	}
	body := p.buildAnthropicBody(req, false)
	assert.Equal(t, "claude-3-opus-20240229", body["model"])
	assert.Equal(t, "You are a helpful assistant", body["system"])
	msgs := body["messages"].([]anthropicMessage)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0].Role)
}

func TestAIProvider_BuildAnthropicBody_NoSystem(t *testing.T) {
	p := makeTestProvider("anthropic", "https://api.anthropic.com", "claude-3", "", "key")
	req := ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	}
	body := p.buildAnthropicBody(req, false)
	_, hasSystem := body["system"]
	assert.False(t, hasSystem)
}

func TestParseAnthropicResponse_Success(t *testing.T) {
	data := []byte(`{
		"id": "msg_01abc",
		"content": [
			{"type": "text", "text": "Hello"},
			{"type": "text", "text": " world"}
		],
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`)

	resp, err := parseAnthropicResponse(data)
	require.NoError(t, err)
	assert.Equal(t, "msg_01abc", resp.ID)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "Hello world", resp.Choices[0].Message.Content)
	assert.Equal(t, 10, resp.Usage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.CompletionTokens)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestParseAnthropicResponse_NonTextContent(t *testing.T) {
	data := []byte(`{
		"id": "msg_02",
		"content": [
			{"type": "tool_use", "text": "should be ignored"},
			{"type": "text", "text": "actual text"}
		],
		"usage": {"input_tokens": 5, "output_tokens": 3}
	}`)

	resp, err := parseAnthropicResponse(data)
	require.NoError(t, err)
	assert.Equal(t, "actual text", resp.Choices[0].Message.Content)
}

func TestParseAnthropicResponse_InvalidJSON(t *testing.T) {
	resp, err := parseAnthropicResponse([]byte("{invalid}"))
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "解析 Anthropic 響應失敗")
}

func TestAIProvider_SetAuthHeaders_OpenAI(t *testing.T) {
	p := makeTestProvider("openai", "https://api.openai.com/v1", "gpt-4o", "", "sk-testkey")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	p.setAuthHeaders(req)
	assert.Equal(t, "Bearer sk-testkey", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestAIProvider_SetAuthHeaders_Anthropic(t *testing.T) {
	p := makeTestProvider("anthropic", "https://api.anthropic.com", "claude-3", "", "sk-ant-key")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	p.setAuthHeaders(req)
	assert.Equal(t, "sk-ant-key", req.Header.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", req.Header.Get("anthropic-version"))
}

func TestAIProvider_SetAuthHeaders_Azure(t *testing.T) {
	p := makeTestProvider("azure", "https://x.openai.azure.com", "gpt-4o", "", "azure-key")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	p.setAuthHeaders(req)
	assert.Equal(t, "azure-key", req.Header.Get("api-key"))
}

// ─── GetToolDefinitions (pure function) ─────────────────────────────────────

func TestGetToolDefinitions_ReturnsNonEmpty(t *testing.T) {
	tools := GetToolDefinitions()
	assert.NotEmpty(t, tools)
	for _, tool := range tools {
		assert.Equal(t, "function", tool.Type)
		assert.NotEmpty(t, tool.Function.Name)
		assert.NotEmpty(t, tool.Function.Description)
	}
}

func TestGetToolDefinitions_SerializableToJSON(t *testing.T) {
	tools := GetToolDefinitions()
	data, err := json.Marshal(tools)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
}

// ─── formatAge tests ─────────────────────────────────────────────────────────

func TestFormatAge_Seconds(t *testing.T) {
	result := formatAge(time.Now().Add(-45 * time.Second))
	assert.True(t, strings.HasSuffix(result, "s"), "expected seconds suffix, got: "+result)
}

func TestFormatAge_Minutes(t *testing.T) {
	result := formatAge(time.Now().Add(-30 * time.Minute))
	assert.True(t, strings.HasSuffix(result, "m"), "expected minutes suffix, got: "+result)
}

func TestFormatAge_Hours(t *testing.T) {
	result := formatAge(time.Now().Add(-5 * time.Hour))
	assert.True(t, strings.HasSuffix(result, "h"), "expected hours suffix, got: "+result)
}

func TestFormatAge_Days(t *testing.T) {
	result := formatAge(time.Now().Add(-3 * 24 * time.Hour))
	assert.True(t, strings.HasSuffix(result, "d"), "expected days suffix, got: "+result)
}

func TestFormatAge_Years(t *testing.T) {
	result := formatAge(time.Now().Add(-400 * 24 * time.Hour))
	assert.True(t, strings.HasSuffix(result, "y"), "expected years suffix, got: "+result)
}

// ─── getContainerImages tests ─────────────────────────────────────────────────

func TestGetContainerImages_Single(t *testing.T) {
	containers := []corev1.Container{
		{Name: "web", Image: "nginx:1.21"},
	}
	result := getContainerImages(containers)
	assert.Equal(t, "nginx:1.21", result)
}

func TestGetContainerImages_Multiple(t *testing.T) {
	containers := []corev1.Container{
		{Name: "web", Image: "nginx:1.21"},
		{Name: "sidecar", Image: "istio/proxy:1.16"},
	}
	result := getContainerImages(containers)
	assert.Equal(t, "nginx:1.21, istio/proxy:1.16", result)
}

func TestGetContainerImages_Empty(t *testing.T) {
	result := getContainerImages([]corev1.Container{})
	assert.Equal(t, "", result)
}

// ─── formatServicePorts tests ─────────────────────────────────────────────────

func TestFormatServicePorts_ClusterIP(t *testing.T) {
	ports := []corev1.ServicePort{
		{Port: 80, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(8080)},
	}
	result := formatServicePorts(ports)
	assert.Equal(t, "80/TCP", result)
}

func TestFormatServicePorts_NodePort(t *testing.T) {
	ports := []corev1.ServicePort{
		{Port: 80, NodePort: 30080, Protocol: corev1.ProtocolTCP},
	}
	result := formatServicePorts(ports)
	assert.Equal(t, "80:30080/TCP", result)
}

func TestFormatServicePorts_Multiple(t *testing.T) {
	ports := []corev1.ServicePort{
		{Port: 80, Protocol: corev1.ProtocolTCP},
		{Port: 443, Protocol: corev1.ProtocolTCP},
	}
	result := formatServicePorts(ports)
	assert.Equal(t, "80/TCP, 443/TCP", result)
}

func TestFormatServicePorts_Empty(t *testing.T) {
	result := formatServicePorts([]corev1.ServicePort{})
	assert.Equal(t, "", result)
}

// ─── AIProvider.Chat tests (mock HTTP server) ────────────────────────────────

func makeProviderWithServer(t *testing.T, provider string, handler http.HandlerFunc) (*AIProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	cfg := &models.AIConfig{
		Provider: provider,
		Endpoint: srv.URL,
		Model:    "gpt-4o",
		APIKey:   "test-key",
		Enabled:  true,
	}
	p := NewAIProvider(cfg)
	return p, srv
}

func TestAIProvider_Chat_OpenAI_Success(t *testing.T) {
	respBody := `{"id":"chatcmpl-1","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}]}`
	p, srv := makeProviderWithServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})
	defer srv.Close()

	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-1", resp.ID)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "Hello!", resp.Choices[0].Message.Content)
}

func TestAIProvider_Chat_OpenAI_HTTPError(t *testing.T) {
	p, srv := makeProviderWithServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_api_key"}`))
	})
	defer srv.Close()

	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "401")
}

func TestAIProvider_Chat_Anthropic_Success(t *testing.T) {
	respBody := `{"id":"msg_01","content":[{"type":"text","text":"Hi there"}],"usage":{"input_tokens":5,"output_tokens":3}}`
	p, srv := makeProviderWithServer(t, "anthropic", func(w http.ResponseWriter, r *http.Request) {
		// Anthropic uses /v1/messages
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})
	defer srv.Close()

	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "msg_01", resp.ID)
	assert.Equal(t, "Hi there", resp.Choices[0].Message.Content)
}

func TestAIProvider_Chat_InvalidJSONResponse(t *testing.T) {
	p, srv := makeProviderWithServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{not json}"))
	})
	defer srv.Close()

	resp, err := p.Chat(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestAIProvider_TestConnection_Success(t *testing.T) {
	respBody := `{"id":"chatcmpl-ok","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`
	p, srv := makeProviderWithServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})
	defer srv.Close()

	err := p.TestConnection(context.Background())
	assert.NoError(t, err)
}

func TestAIProvider_TestConnection_EmptyChoices(t *testing.T) {
	respBody := `{"id":"chatcmpl-empty","choices":[]}`
	p, srv := makeProviderWithServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})
	defer srv.Close()

	err := p.TestConnection(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "空響應")
}

func TestAIProvider_TestConnection_HTTPError(t *testing.T) {
	p, srv := makeProviderWithServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("service unavailable"))
	})
	defer srv.Close()

	err := p.TestConnection(context.Background())
	assert.Error(t, err)
}

func TestAIProvider_ChatStream_Success(t *testing.T) {
	// SSE stream with a couple of chunks and a DONE marker
	sseData := "data: {\"id\":\"s1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"s2\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	p, srv := makeProviderWithServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseData))
	})
	defer srv.Close()

	ch, err := p.ChatStream(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	require.NoError(t, err)

	var content strings.Builder
	for ev := range ch {
		if ev.Error != nil {
			break
		}
		if ev.Done {
			break
		}
		content.WriteString(ev.Content)
	}
	assert.Contains(t, content.String(), "Hello")
}

func TestAIProvider_ChatStream_HTTPError(t *testing.T) {
	p, srv := makeProviderWithServer(t, "openai", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	})
	defer srv.Close()

	ch, err := p.ChatStream(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	})
	assert.Error(t, err)
	assert.Nil(t, ch)
}

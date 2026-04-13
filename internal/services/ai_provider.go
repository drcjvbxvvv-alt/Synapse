package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ChatMessage OpenAI Chat 訊息格式
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall 工具呼叫
type ToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函式呼叫
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition OpenAI Function Calling 的工具定義
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition 函式定義
type FunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ChatRequest 聊天請求
type ChatRequest struct {
	Messages []ChatMessage  `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

// ChatResponse 普通聊天響應
type ChatResponse struct {
	ID      string         `json:"id"`
	Choices []ChatChoice   `json:"choices"`
	Usage   *ChatUsage     `json:"usage,omitempty"`
}

// ChatChoice 響應選項
type ChatChoice struct {
	Index        int          `json:"index"`
	Message      ChatMessage  `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

// ChatUsage Token 使用量
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk SSE 流式響應 chunk
type StreamChunk struct {
	ID      string              `json:"id"`
	Choices []StreamChunkChoice `json:"choices"`
}

// StreamChunkChoice 流式響應選項
type StreamChunkChoice struct {
	Index        int              `json:"index"`
	Delta        StreamChunkDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

// StreamChunkDelta 流式增量內容
type StreamChunkDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// AIProvider OpenAI 相容 API 呼叫封裝
type AIProvider struct {
	config *models.AIConfig
	client *http.Client
}

// NewAIProvider 建立 AI Provider
func NewAIProvider(config *models.AIConfig) *AIProvider {
	return &AIProvider{
		config: config,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// isAnthropic 判斷是否為 Anthropic 提供者
func (p *AIProvider) isAnthropic() bool {
	return p.config.Provider == "anthropic"
}

// isAzure 判斷是否為 Azure OpenAI 提供者
func (p *AIProvider) isAzure() bool {
	return p.config.Provider == "azure"
}

// isOllama 判斷是否為 Ollama 本地部署
func (p *AIProvider) isOllama() bool {
	return p.config.Provider == "ollama"
}

// chatURL 返回聊天請求 URL
func (p *AIProvider) chatURL(stream bool) string {
	base := strings.TrimRight(p.config.Endpoint, "/")
	switch {
	case p.isAnthropic():
		return base + "/v1/messages"
	case p.isAzure():
		apiVersion := p.config.APIVersion
		if apiVersion == "" {
			apiVersion = "2024-05-01-preview"
		}
		return fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
			base, p.config.Model, apiVersion)
	case p.isOllama():
		// Ollama 預設只暴露 host:port，OpenAI 相容端點在 /v1/chat/completions
		// 若使用者只填了 host:port（沒有 /v1 或 /api 路徑），自動補 /v1
		if !strings.Contains(base, "/v1") && !strings.Contains(base, "/api") {
			base = base + "/v1"
		}
		return base + "/chat/completions"
	default: // openai / compatible
		return base + "/chat/completions"
	}
}

// setAuthHeaders 設定鑑權請求頭
func (p *AIProvider) setAuthHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	switch {
	case p.isAnthropic():
		req.Header.Set("x-api-key", p.config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case p.isAzure():
		req.Header.Set("api-key", p.config.APIKey)
	case p.isOllama():
		// Ollama 本地部署不需要鑑權頭；若使用者仍設定了 API Key 則帶上
		if p.config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
		}
	default:
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}
}

// buildOpenAIBody 構造 OpenAI/Azure 請求體
func (p *AIProvider) buildOpenAIBody(req ChatRequest, stream bool) map[string]interface{} {
	body := map[string]interface{}{
		"messages": req.Messages,
		"stream":   stream,
	}
	// Azure 的 model 已經在 URL 中，但傳 model 欄位不影響
	if !p.isAzure() {
		body["model"] = p.config.Model
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	return body
}

// anthropicMessage Anthropic Messages API 請求/響應結構
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// buildAnthropicBody 構造 Anthropic Messages API 請求體
func (p *AIProvider) buildAnthropicBody(req ChatRequest, stream bool) map[string]interface{} {
	// 將 OpenAI messages 格式轉換為 Anthropic 格式
	msgs := make([]anthropicMessage, 0, len(req.Messages))
	system := ""
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
	}

	body := map[string]interface{}{
		"model":      p.config.Model,
		"max_tokens": 4096,
		"messages":   msgs,
		"stream":     stream,
	}
	if system != "" {
		body["system"] = system
	}
	return body
}

// parseAnthropicResponse 將 Anthropic 響應轉換為 OpenAI 格式
func parseAnthropicResponse(data []byte) (*ChatResponse, error) {
	var result struct {
		ID      string `json:"id"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析 Anthropic 響應失敗: %w", err)
	}

	text := ""
	for _, c := range result.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	return &ChatResponse{
		ID: result.ID,
		Choices: []ChatChoice{
			{
				Index:        0,
				Message:      ChatMessage{Role: "assistant", Content: text},
				FinishReason: "stop",
			},
		},
		Usage: &ChatUsage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
	}, nil
}

// Chat 普通（非流式）聊天
func (p *AIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	var body map[string]interface{}
	if p.isAnthropic() {
		body = p.buildAnthropicBody(req, false)
	} else {
		body = p.buildOpenAIBody(req, false)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化請求失敗: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.chatURL(false), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}
	p.setAuthHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("請求 LLM API 失敗: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API 返回錯誤 (status=%d): %s", resp.StatusCode, string(respBody))
	}

	if p.isAnthropic() {
		return parseAnthropicResponse(respBody)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(bytes.NewReader(respBody)).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("解析 LLM 響應失敗: %w", err)
	}

	return &chatResp, nil
}

// ChatStreamEvent SSE 事件
type ChatStreamEvent struct {
	Content      string     // 文字增量
	ToolCalls    []ToolCall // 工具呼叫增量
	FinishReason string     // 結束原因
	Done         bool       // 流結束
	Error        error      // 錯誤
}

// ChatStream 流式聊天，返回事件 channel
func (p *AIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamEvent, error) {
	var body map[string]interface{}
	if p.isAnthropic() {
		body = p.buildAnthropicBody(req, true)
	} else {
		body = p.buildOpenAIBody(req, true)
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化請求失敗: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.chatURL(true), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}
	p.setAuthHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("請求 LLM API 失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("LLM API 返回錯誤 (status=%d): %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan ChatStreamEvent, 64)

	isAnthropic := p.isAnthropic()

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		// SSE 行可能很長，增大緩衝
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		currentEvent := ""

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				currentEvent = ""
				continue
			}

			// 記錄 Anthropic event: 行
			if strings.HasPrefix(line, "event: ") {
				currentEvent = strings.TrimPrefix(line, "event: ")
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				ch <- ChatStreamEvent{Done: true}
				return
			}

			if isAnthropic {
				// Anthropic stream 事件解析
				switch currentEvent {
				case "content_block_delta":
					var delta struct {
						Delta struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"delta"`
					}
					if err := json.Unmarshal([]byte(payload), &delta); err != nil {
						continue
					}
					if delta.Delta.Type == "text_delta" && delta.Delta.Text != "" {
						select {
						case ch <- ChatStreamEvent{Content: delta.Delta.Text}:
						case <-ctx.Done():
							return
						}
					}
				case "message_stop":
					ch <- ChatStreamEvent{Done: true}
					return
				}
				continue
			}

			// OpenAI / Azure / Ollama stream 解析
			var chunk StreamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				logger.Error("解析 SSE chunk 失敗", "error", err, "payload", payload)
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]
			evt := ChatStreamEvent{
				Content:   choice.Delta.Content,
				ToolCalls: choice.Delta.ToolCalls,
			}
			if choice.FinishReason != nil {
				evt.FinishReason = *choice.FinishReason
			}

			select {
			case ch <- evt:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case ch <- ChatStreamEvent{Error: fmt.Errorf("讀取 SSE 流失敗: %w", err)}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// TestConnection 測試 AI 配置連線
func (p *AIProvider) TestConnection(ctx context.Context) error {
	req := ChatRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "Hi, reply with just 'ok'."},
		},
	}

	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("LLM 返回空響應")
	}

	return nil
}

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

	"github.com/clay-wangzhi/KubePolaris/internal/models"
	"github.com/clay-wangzhi/KubePolaris/pkg/logger"
)

// ChatMessage OpenAI Chat 消息格式
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition OpenAI Function Calling 的工具定义
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition 函数定义
type FunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Messages []ChatMessage  `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

// ChatResponse 普通聊天响应
type ChatResponse struct {
	ID      string         `json:"id"`
	Choices []ChatChoice   `json:"choices"`
	Usage   *ChatUsage     `json:"usage,omitempty"`
}

// ChatChoice 响应选项
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

// StreamChunk SSE 流式响应 chunk
type StreamChunk struct {
	ID      string              `json:"id"`
	Choices []StreamChunkChoice `json:"choices"`
}

// StreamChunkChoice 流式响应选项
type StreamChunkChoice struct {
	Index        int              `json:"index"`
	Delta        StreamChunkDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

// StreamChunkDelta 流式增量内容
type StreamChunkDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// AIProvider OpenAI 兼容 API 调用封装
type AIProvider struct {
	config *models.AIConfig
	client *http.Client
}

// NewAIProvider 创建 AI Provider
func NewAIProvider(config *models.AIConfig) *AIProvider {
	return &AIProvider{
		config: config,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Chat 普通（非流式）聊天
func (p *AIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body := map[string]interface{}{
		"model":    p.config.Model,
		"messages": req.Messages,
		"stream":   false,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	endpoint := strings.TrimRight(p.config.Endpoint, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 LLM API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LLM API 返回错误 (status=%d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}

	return &chatResp, nil
}

// ChatStreamEvent SSE 事件
type ChatStreamEvent struct {
	Content      string     // 文本增量
	ToolCalls    []ToolCall // 工具调用增量
	FinishReason string     // 结束原因
	Done         bool       // 流结束
	Error        error      // 错误
}

// ChatStream 流式聊天，返回事件 channel
func (p *AIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamEvent, error) {
	body := map[string]interface{}{
		"model":    p.config.Model,
		"messages": req.Messages,
		"stream":   true,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	endpoint := strings.TrimRight(p.config.Endpoint, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 LLM API 失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("LLM API 返回错误 (status=%d): %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan ChatStreamEvent, 64)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		// SSE 行可能很长，增大缓冲
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
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

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				logger.Error("解析 SSE chunk 失败", "error", err, "payload", payload)
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
			case ch <- ChatStreamEvent{Error: fmt.Errorf("读取 SSE 流失败: %w", err)}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// TestConnection 测试 AI 配置连接
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
		return fmt.Errorf("LLM 返回空响应")
	}

	return nil
}

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
)

// AIChatHandler AI 對話處理器
type AIChatHandler struct {
	clusterService  *services.ClusterService
	aiConfigService *services.AIConfigService
	toolExecutor    *services.ToolExecutor
}

// NewAIChatHandler 建立 AI 對話處理器
func NewAIChatHandler(db *gorm.DB, clusterSvc *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *AIChatHandler {
	return &AIChatHandler{
		clusterService:  clusterSvc,
		aiConfigService: services.NewAIConfigService(db),
		toolExecutor:    services.NewToolExecutor(k8sMgr, clusterSvc),
	}
}

// chatRequest 對話請求
type chatRequest struct {
	Messages []services.ChatMessage `json:"messages"`
}

// Chat 處理 AI 對話請求（SSE 流式響應）
func (h *AIChatHandler) Chat(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤："+err.Error())
		return
	}

	if len(req.Messages) == 0 {
		response.BadRequest(c, "訊息不能為空")
		return
	}

	aiConfig, err := h.aiConfigService.GetConfigWithAPIKey()
	if err != nil || aiConfig == nil {
		response.BadRequest(c, "AI 功能未設定，請至系統設定配置 AI")
		return
	}
	if !aiConfig.Enabled || aiConfig.APIKey == "" {
		response.BadRequest(c, "AI 功能未啟用")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(clusterID))
	if err != nil {
		response.BadRequest(c, "叢集不存在")
		return
	}

	systemPrompt := buildSystemPrompt(cluster)

	messages := make([]services.ChatMessage, 0, len(req.Messages)+1)
	messages = append(messages, services.ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})
	messages = append(messages, req.Messages...)

	provider := services.NewAIProvider(aiConfig)
	tools := services.GetToolDefinitions()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.InternalError(c, "不支援流式響應")
		return
	}

	sendSSE := func(eventType, data string) {
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, data)
		flusher.Flush()
	}

	// Function Calling 迴圈：最多 10 輪工具呼叫
	for round := 0; round < 10; round++ {
		chatReq := services.ChatRequest{
			Messages: messages,
			Tools:    tools,
		}

		eventCh, err := provider.ChatStream(ctx, chatReq)
		if err != nil {
			sendSSE("error", fmt.Sprintf(`{"error":"%s"}`, escapeJSON(err.Error())))
			sendSSE("done", "{}")
			return
		}

		var contentBuilder strings.Builder
		toolCallsMap := make(map[int]*services.ToolCall)
		finishReason := ""

		for evt := range eventCh {
			if evt.Error != nil {
				sendSSE("error", fmt.Sprintf(`{"error":"%s"}`, escapeJSON(evt.Error.Error())))
				sendSSE("done", "{}")
				return
			}

			if evt.Content != "" {
				contentBuilder.WriteString(evt.Content)
				sendSSE("content", fmt.Sprintf(`{"content":"%s"}`, escapeJSON(evt.Content)))
			}

			for _, tc := range evt.ToolCalls {
				idx := tc.Index
				existing, ok := toolCallsMap[idx]
				if !ok {
					toolCallsMap[idx] = &services.ToolCall{
						Index:    idx,
						ID:       tc.ID,
						Type:     "function",
						Function: services.FunctionCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments},
					}
				} else {
					if tc.ID != "" {
						existing.ID = tc.ID
					}
					if tc.Function.Name != "" {
						existing.Function.Name += tc.Function.Name
					}
					existing.Function.Arguments += tc.Function.Arguments
				}
			}

			if evt.FinishReason != "" {
				finishReason = evt.FinishReason
			}

			if evt.Done {
				break
			}
		}

		if finishReason == "tool_calls" && len(toolCallsMap) > 0 {
			toolCalls := make([]services.ToolCall, 0, len(toolCallsMap))
			for i := 0; i < len(toolCallsMap); i++ {
				if tc, ok := toolCallsMap[i]; ok {
					toolCalls = append(toolCalls, *tc)
				}
			}

			// 將 assistant 訊息（包含 tool_calls）加入歷史
			messages = append(messages, services.ChatMessage{
				Role:      "assistant",
				Content:   contentBuilder.String(),
				ToolCalls: toolCalls,
			})

			// 執行每個工具呼叫
			for _, tc := range toolCalls {
				sendSSE("tool_call", fmt.Sprintf(`{"id":"%s","name":"%s","arguments":%s}`,
					escapeJSON(tc.ID), escapeJSON(tc.Function.Name), tc.Function.Arguments))

				result, execErr := h.toolExecutor.ExecuteTool(ctx, uint(clusterID), tc.Function.Name, tc.Function.Arguments)
				if execErr != nil {
					result = fmt.Sprintf(`{"error":"%s"}`, escapeJSON(execErr.Error()))
					logger.Error("工具執行失敗", "tool", tc.Function.Name, "error", execErr)
				}

				// Sanitize tool results before sending to AI to prevent leaking secrets
				sanitizedResult := services.SanitizeK8sContext(result)

				sendSSE("tool_result", fmt.Sprintf(`{"id":"%s","name":"%s","result":%s}`,
					escapeJSON(tc.ID), escapeJSON(tc.Function.Name), ensureJSON(result)))

				messages = append(messages, services.ChatMessage{
					Role:       "tool",
					Content:    sanitizedResult,
					ToolCallID: tc.ID,
				})
			}

			continue
		}

		// 文字回覆完成
		sendSSE("done", "{}")
		return
	}

	sendSSE("error", `{"error":"工具呼叫輪次超出限制"}`)
	sendSSE("done", "{}")
}

func buildSystemPrompt(cluster *models.Cluster) string {
	name := cluster.Name
	version := cluster.Version

	return fmt.Sprintf(`You are Synapse AI Assistant, helping users manage and troubleshoot Kubernetes clusters.
Current cluster: %s, K8s version: %s

You have access to the following tools to query cluster resources and analyze issues:
- list_pods / get_pod_detail / get_pod_logs: inspect Pod status and logs
- list_deployments / get_deployment_detail: inspect Deployment state
- list_nodes / get_node_detail: inspect node health and resources
- list_events: retrieve K8s events
- list_services / list_ingresses: inspect network resources
- scale_deployment / restart_deployment: scale or restart workloads (always ask for confirmation first)

YAML generation mode:
- When the user's message starts with /yaml or explicitly asks to generate YAML, respond as a senior K8s engineer producing complete, production-ready YAML
- Always include correct apiVersion, kind, metadata, and spec fields
- Include reasonable resources.requests/limits defaults
- Output the YAML inside a ` + "```" + `yaml code block

Rules:
1. Proactively call tools to gather data before answering; never guess resource state
2. For write operations (scale, restart, delete), clearly describe the action and ask for explicit confirmation before proceeding
3. Format responses in Markdown; use tables for lists of resources
4. If a tool returns an error, explain the cause clearly to the user
5. Always reply in the same language the user writes in (Traditional Chinese if they write in Chinese, English if they write in English)`, name, version)
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func ensureJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return `""`
	}
	if (s[0] == '{' || s[0] == '[') && json.Valid([]byte(s)) {
		return s
	}
	b, _ := json.Marshal(s)
	return string(b)
}

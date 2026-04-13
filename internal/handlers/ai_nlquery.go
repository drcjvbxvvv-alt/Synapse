package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
)

// AINLQueryHandler handles natural language K8s queries.
type AINLQueryHandler struct {
	clusterService  *services.ClusterService
	aiConfigService *services.AIConfigService
	toolExecutor    *services.ToolExecutor
}

func NewAINLQueryHandler(clusterSvc *services.ClusterService, aiConfigSvc *services.AIConfigService, k8sMgr *k8s.ClusterInformerManager) *AINLQueryHandler {
	return &AINLQueryHandler{
		clusterService:  clusterSvc,
		aiConfigService: aiConfigSvc,
		toolExecutor:    services.NewToolExecutor(k8sMgr, clusterSvc),
	}
}

type nlQueryRequest struct {
	Question  string `json:"question" binding:"required"`
	Namespace string `json:"namespace"`
}

type nlQueryResponse struct {
	Question string      `json:"question"`
	ToolUsed string      `json:"tool_used,omitempty"`
	Result   interface{} `json:"result"`
	Summary  string      `json:"summary"`
}

const nlQuerySystemPrompt = `You are a Kubernetes query assistant for Synapse. Given the user's natural-language question, select and call the single most appropriate query tool.

Rules:
1. Call exactly ONE tool — choose the one that best answers the question
2. Respond using function calling format only; do not output any text
3. Default to querying all namespaces (set namespace to empty string) unless the question specifies one
4. If the question mentions a specific namespace, use it

Available tools: list_pods, get_pod_detail, get_pod_logs, list_deployments, get_deployment_detail, list_nodes, get_node_detail, list_events, list_services, list_ingresses

Current cluster: %s`

const nlSummarySystemPrompt = `You are Synapse AI Assistant. Given a Kubernetes query result, write a concise 1-3 sentence summary of the key findings.
Focus on: how many resources were found, which ones are unhealthy or abnormal, and what the operator should pay attention to.
Do not repeat every item — give a direct, actionable conclusion.
Reply in the same language as the user's original question.`

// NLQuery handles POST /clusters/:clusterID/ai/nl-query
func (h *AINLQueryHandler) NLQuery(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var req nlQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤："+err.Error())
		return
	}

	aiConfig, err := h.aiConfigService.GetConfigWithAPIKey()
	if err != nil || aiConfig == nil {
		response.BadRequest(c, "AI 功能未設定，請至系統設定配置 AI")
		return
	}
	// Ollama 本地部署不需要 API Key
	needsAPIKey := aiConfig.Provider != "ollama"
	if !aiConfig.Enabled || (needsAPIKey && aiConfig.APIKey == "") {
		response.BadRequest(c, "AI 功能未啟用，請至系統設定配置 AI")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(clusterID))
	if err != nil {
		response.BadRequest(c, "叢集不存在")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	provider := services.NewAIProvider(aiConfig)
	tools := services.GetToolDefinitions()

	// Step 1: Ask AI to select and call a tool
	systemPrompt := fmt.Sprintf(nlQuerySystemPrompt, cluster.Name)
	messages := []services.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: req.Question},
	}

	chatReq := services.ChatRequest{
		Messages: messages,
		Tools:    tools,
	}

	chatResp, err := provider.Chat(ctx, chatReq)
	if err != nil {
		response.InternalError(c, "AI 呼叫失敗："+err.Error())
		return
	}

	if len(chatResp.Choices) == 0 {
		response.InternalError(c, "AI 回傳空回應")
		return
	}

	choice := chatResp.Choices[0]

	// Step 2: Execute the tool if AI made a tool call
	var toolUsed string
	var toolResult string

	if choice.FinishReason == "tool_calls" && len(choice.Message.ToolCalls) > 0 {
		tc := choice.Message.ToolCalls[0]
		toolUsed = tc.Function.Name

		result, execErr := h.toolExecutor.ExecuteTool(ctx, uint(clusterID), tc.Function.Name, tc.Function.Arguments)
		if execErr != nil {
			toolResult = fmt.Sprintf(`{"error":"%s"}`, escapeJSON(execErr.Error()))
		} else {
			toolResult = result
		}
	} else {
		// AI answered directly (no tool needed or tool calling not triggered)
		result := &nlQueryResponse{
			Question: req.Question,
			Result:   nil,
			Summary:  choice.Message.Content,
		}
		response.OK(c, result)
		return
	}

	// Step 3: Parse tool result
	var parsedResult interface{}
	if err := json.Unmarshal([]byte(toolResult), &parsedResult); err != nil {
		parsedResult = toolResult
	}

	// Step 4: Ask AI to summarize the result
	summaryMessages := []services.ChatMessage{
		{Role: "system", Content: nlSummarySystemPrompt},
		{Role: "user", Content: fmt.Sprintf("問題：%s\n\n查詢結果：%s", req.Question, truncateString(toolResult, 3000))},
	}
	summaryResp, err := provider.Chat(ctx, services.ChatRequest{Messages: summaryMessages})

	summary := ""
	if err == nil && len(summaryResp.Choices) > 0 {
		summary = summaryResp.Choices[0].Message.Content
	} else {
		// Fallback: generate basic summary from result
		summary = generateBasicSummary(toolUsed, toolResult)
	}

	result := &nlQueryResponse{
		Question: req.Question,
		ToolUsed: toolUsed,
		Result:   parsedResult,
		Summary:  summary,
	}
	response.OK(c, result)
}

// truncateString truncates a string to the specified max length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

// generateBasicSummary provides a fallback summary when AI summarization fails.
func generateBasicSummary(toolUsed, result string) string {
	switch {
	case strings.HasPrefix(toolUsed, "list_"):
		// Count items in the JSON array
		var items []interface{}
		if err := json.Unmarshal([]byte(result), &items); err == nil {
			return fmt.Sprintf("查詢完成，共找到 %d 筆資源。", len(items))
		}
	}
	return "查詢完成。"
}

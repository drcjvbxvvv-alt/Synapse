package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/runbooks"
)

// AIRunbookHandler serves the built-in runbook knowledge base.
type AIRunbookHandler struct{}

func NewAIRunbookHandler() *AIRunbookHandler {
	return &AIRunbookHandler{}
}

// GetRunbooks godoc
// @Summary     Search runbooks by reason or keyword
// @Description Returns runbooks matching the given reason (e.g. OOMKilled) or keyword.
//
//	Pass no query param to return all runbooks.
//
// @Tags        AI
// @Produce     json
// @Param       reason  query  string  false  "K8s event reason or keyword"
// @Success     200     {array}  runbooks.Runbook
// @Router      /ai/runbooks [get]
func (h *AIRunbookHandler) GetRunbooks(c *gin.Context) {
	reason := c.Query("reason")
	result := runbooks.Search(reason)
	if result == nil {
		result = []runbooks.Runbook{}
	}
	response.OK(c, result)
}

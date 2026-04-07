package handlers

import (
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LogSourceHandler manages external log source configs and external log search.
type LogSourceHandler struct {
	db *gorm.DB
}

// NewLogSourceHandler creates a LogSourceHandler.
func NewLogSourceHandler(db *gorm.DB) *LogSourceHandler {
	return &LogSourceHandler{db: db}
}

// ListLogSources GET /clusters/:id/log-sources
func (h *LogSourceHandler) ListLogSources(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var sources []models.LogSourceConfig
	if err := h.db.Where("cluster_id = ?", clusterID).Find(&sources).Error; err != nil {
		response.InternalError(c, "查詢日誌源失敗")
		return
	}
	// Mask credentials before returning
	for i := range sources {
		sources[i].Password = ""
		sources[i].APIKey = ""
	}
	response.OK(c, sources)
}

// CreateLogSource POST /clusters/:id/log-sources
func (h *LogSourceHandler) CreateLogSource(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var req struct {
		Type     string `json:"type" binding:"required"`
		Name     string `json:"name" binding:"required"`
		URL      string `json:"url" binding:"required"`
		Username string `json:"username"`
		Password string `json:"password"`
		APIKey   string `json:"apiKey"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Type != "loki" && req.Type != "elasticsearch" {
		response.BadRequest(c, "type 必須為 loki 或 elasticsearch")
		return
	}

	src := &models.LogSourceConfig{
		ClusterID: uint(clusterID),
		Type:      req.Type,
		Name:      req.Name,
		URL:       req.URL,
		Username:  req.Username,
		Password:  req.Password,
		APIKey:    req.APIKey,
		Enabled:   req.Enabled,
	}
	if err := h.db.Create(src).Error; err != nil {
		response.InternalError(c, "建立日誌源失敗")
		return
	}
	src.Password = ""
	src.APIKey = ""
	response.OK(c, src)
}

// UpdateLogSource PUT /clusters/:id/log-sources/:sourceId
func (h *LogSourceHandler) UpdateLogSource(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	srcID, err := strconv.Atoi(c.Param("sourceId"))
	if err != nil {
		response.BadRequest(c, "無效的日誌源ID")
		return
	}

	var src models.LogSourceConfig
	if err := h.db.Where("id = ? AND cluster_id = ?", srcID, clusterID).First(&src).Error; err != nil {
		response.NotFound(c, "日誌源不存在")
		return
	}

	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
		APIKey   string `json:"apiKey"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.URL != "" {
		updates["url"] = req.URL
	}
	if req.Username != "" {
		updates["username"] = req.Username
	}
	if req.Password != "" {
		updates["password"] = req.Password
	}
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := h.db.Model(&src).Updates(updates).Error; err != nil {
		response.InternalError(c, "更新日誌源失敗")
		return
	}
	response.OK(c, gin.H{"message": "更新成功"})
}

// DeleteLogSource DELETE /clusters/:id/log-sources/:sourceId
func (h *LogSourceHandler) DeleteLogSource(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	srcID, err := strconv.Atoi(c.Param("sourceId"))
	if err != nil {
		response.BadRequest(c, "無效的日誌源ID")
		return
	}

	if err := h.db.Where("id = ? AND cluster_id = ?", srcID, clusterID).Delete(&models.LogSourceConfig{}).Error; err != nil {
		response.InternalError(c, "刪除日誌源失敗")
		return
	}
	response.OK(c, gin.H{"message": "刪除成功"})
}

// SearchExternalLogs POST /clusters/:id/log-sources/:sourceId/search
func (h *LogSourceHandler) SearchExternalLogs(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	srcID, err := strconv.Atoi(c.Param("sourceId"))
	if err != nil {
		response.BadRequest(c, "無效的日誌源ID")
		return
	}

	var src models.LogSourceConfig
	if err := h.db.Where("id = ? AND cluster_id = ? AND enabled = ?", srcID, clusterID, true).First(&src).Error; err != nil {
		response.NotFound(c, "日誌源不存在或已禁用")
		return
	}

	var req struct {
		Query     string `json:"query"`     // LogQL (Loki) or Lucene (ES)
		Index     string `json:"index"`     // ES only
		StartTime string `json:"startTime"` // RFC3339
		EndTime   string `json:"endTime"`   // RFC3339
		Limit     int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	now := time.Now()
	startTime := now.Add(-1 * time.Hour)
	endTime := now
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			startTime = t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			endTime = t
		}
	}

	var entries []models.LogEntry
	switch src.Type {
	case "loki":
		svc := services.NewLokiService(&src)
		entries, err = svc.QueryRange(req.Query, startTime, endTime, req.Limit)
	case "elasticsearch":
		svc := services.NewElasticsearchService(&src)
		entries, err = svc.Search(req.Index, req.Query, startTime, endTime, req.Limit)
	default:
		response.BadRequest(c, "不支援的日誌源型別")
		return
	}

	if err != nil {
		response.InternalError(c, "查詢外部日誌失敗: "+err.Error())
		return
	}

	if entries == nil {
		entries = []models.LogEntry{}
	}
	response.OK(c, gin.H{
		"items": entries,
		"total": len(entries),
	})
}

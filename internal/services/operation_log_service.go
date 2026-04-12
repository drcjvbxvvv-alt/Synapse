package services

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"gorm.io/gorm"
)

// OperationLogService manages operation audit logging
type OperationLogService struct {
	db *gorm.DB
}

// NewOperationLogService creates a new operation log service
func NewOperationLogService(db *gorm.DB) *OperationLogService {
	return &OperationLogService{db: db}
}

// LogEntry is a log entry structure for recording operations
type LogEntry struct {
	UserID       *uint
	Username     string
	Method       string
	Path         string
	Query        string
	Module       string
	Action       string
	ClusterID    *uint
	ClusterName  string
	Namespace    string
	ResourceType string
	ResourceName string
	RequestBody  interface{}
	StatusCode   int
	Success      bool
	ErrorMessage string
	ClientIP     string
	UserAgent    string
	Duration     int64
}

// Record records an operation log entry
func (s *OperationLogService) Record(entry *LogEntry) error {
	log := &models.OperationLog{
		UserID:       entry.UserID,
		Username:     entry.Username,
		Method:       entry.Method,
		Path:         entry.Path,
		Query:        entry.Query,
		Module:       entry.Module,
		Action:       entry.Action,
		ClusterID:    entry.ClusterID,
		ClusterName:  entry.ClusterName,
		Namespace:    entry.Namespace,
		ResourceType: entry.ResourceType,
		ResourceName: entry.ResourceName,
		RequestBody:  sanitizeAndMarshal(entry.RequestBody),
		StatusCode:   entry.StatusCode,
		Success:      entry.Success,
		ErrorMessage: entry.ErrorMessage,
		ClientIP:     entry.ClientIP,
		UserAgent:    entry.UserAgent,
		Duration:     entry.Duration,
	}

	if err := s.db.Create(log).Error; err != nil {
		logger.Error("failed to record operation log", "error", err)
		return err
	}

	return nil
}

// RecordAsync asynchronously records an operation log without blocking the request
func (s *OperationLogService) RecordAsync(entry *LogEntry) {
	go func() {
		if err := s.Record(entry); err != nil {
			logger.Error("failed to record operation log asynchronously", "error", err, "path", entry.Path, "action", entry.Action)
		}
	}()
}

// sensitiveKeys is a list of sensitive field names that should be redacted
var sensitiveKeys = map[string]bool{
	"password":      true,
	"token":         true,
	"secret":        true,
	"kubeconfig":    true,
	"credential":    true,
	"api_key":       true,
	"apikey":        true,
	"authorization": true,
	"auth":          true,
	"key":           true,
	"private":       true,
	"kubeconfigenc": true,
	"passwordhash":  true,
	"salt":          true,
}

// sanitizeAndMarshal sanitizes sensitive fields and marshals the request body
func sanitizeAndMarshal(body interface{}) string {
	if body == nil {
		return ""
	}

	// If string, try to parse as JSON and then sanitize
	if str, ok := body.(string); ok {
		if str == "" {
			return ""
		}
		var data interface{}
		if err := json.Unmarshal([]byte(str), &data); err == nil {
			body = data
		} else {
			// Not valid JSON, return as-is
			return str
		}
	}

	// Deep sanitization of sensitive values
	sanitized := sanitizeValue(body)

	result, err := json.Marshal(sanitized)
	if err != nil {
		return ""
	}

	// Limit length to avoid storing oversized values
	if len(result) > 4000 {
		return string(result[:4000]) + "...(truncated)"
	}

	return string(result)
}

// sanitizeValue recursively sanitizes sensitive values
func sanitizeValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			if isSensitiveKey(k) {
				result[k] = "***REDACTED***"
			} else {
				result[k] = sanitizeValue(v)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = sanitizeValue(item)
		}
		return result
	default:
		// Use reflection to handle structs and maps
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return nil
			}
			rv = rv.Elem()
		}

		switch rv.Kind() {
		case reflect.Struct:
			result := make(map[string]interface{})
			rt := rv.Type()
			for i := 0; i < rv.NumField(); i++ {
				field := rt.Field(i)
				if !field.IsExported() {
					continue
				}
				fieldName := field.Tag.Get("json")
				if fieldName == "" || fieldName == "-" {
					fieldName = field.Name
				}
				// Strip tags like omitempty
				if idx := strings.Index(fieldName, ","); idx != -1 {
					fieldName = fieldName[:idx]
				}
				if isSensitiveKey(fieldName) || isSensitiveKey(field.Name) {
					result[fieldName] = "***REDACTED***"
				} else {
					result[fieldName] = sanitizeValue(rv.Field(i).Interface())
				}
			}
			return result
		case reflect.Map:
			result := make(map[string]interface{})
			for _, key := range rv.MapKeys() {
				keyStr := key.String()
				if isSensitiveKey(keyStr) {
					result[keyStr] = "***REDACTED***"
				} else {
					result[keyStr] = sanitizeValue(rv.MapIndex(key).Interface())
				}
			}
			return result
		}
		return v
	}
}

// isSensitiveKey checks if a field name is sensitive and should be redacted
func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	if sensitiveKeys[lowerKey] {
		return true
	}
	// Check if the key contains any sensitive words
	for sensitiveWord := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitiveWord) {
			return true
		}
	}
	return false
}

// OperationLogListRequest is the request for listing operation logs
type OperationLogListRequest struct {
	Page         int
	PageSize     int
	UserID       *uint
	Username     string
	Module       string
	Action       string
	ResourceType string
	ClusterID    *uint
	Success      *bool
	StartTime    *time.Time
	EndTime      *time.Time
	Keyword      string
}

// OperationLogListResponse is the response for listing operation logs
type OperationLogListResponse struct {
	Items    []OperationLogItem `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

// OperationLogItem is a single operation log item in the list
type OperationLogItem struct {
	ID           uint      `json:"id"`
	UserID       *uint     `json:"user_id"`
	Username     string    `json:"username"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Module       string    `json:"module"`
	ModuleName   string    `json:"module_name"`
	Action       string    `json:"action"`
	ActionName   string    `json:"action_name"`
	ClusterID    *uint     `json:"cluster_id"`
	ClusterName  string    `json:"cluster_name"`
	Namespace    string    `json:"namespace"`
	ResourceType string    `json:"resource_type"`
	ResourceName string    `json:"resource_name"`
	StatusCode   int       `json:"status_code"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message"`
	ClientIP     string    `json:"client_ip"`
	Duration     int64     `json:"duration"`
	CreatedAt    time.Time `json:"created_at"`
}

// List retrieves a list of operation logs
func (s *OperationLogService) List(req *OperationLogListRequest) (*OperationLogListResponse, error) {
	query := s.db.Model(&models.OperationLog{})

	// Apply filters
	if req.UserID != nil {
		query = query.Where("user_id = ?", *req.UserID)
	}
	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Module != "" {
		query = query.Where("module = ?", req.Module)
	}
	if req.Action != "" {
		query = query.Where("action = ?", req.Action)
	}
	if req.ResourceType != "" {
		query = query.Where("resource_type = ?", req.ResourceType)
	}
	if req.ClusterID != nil {
		query = query.Where("cluster_id = ?", *req.ClusterID)
	}
	if req.Success != nil {
		query = query.Where("success = ?", *req.Success)
	}
	if req.StartTime != nil {
		query = query.Where("created_at >= ?", req.StartTime)
	}
	if req.EndTime != nil {
		query = query.Where("created_at <= ?", req.EndTime)
	}
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		query = query.Where("(username LIKE ? OR resource_name LIKE ? OR cluster_name LIKE ? OR path LIKE ?)",
			keyword, keyword, keyword, keyword)
	}

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// Pagination
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	offset := (req.Page - 1) * req.PageSize

	// Query data
	var logs []models.OperationLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&logs).Error; err != nil {
		return nil, err
	}

	// Convert to response format
	items := make([]OperationLogItem, len(logs))
	for i, log := range logs {
		items[i] = OperationLogItem{
			ID:           log.ID,
			UserID:       log.UserID,
			Username:     log.Username,
			Method:       log.Method,
			Path:         log.Path,
			Module:       log.Module,
			ModuleName:   log.Module, // Backend returns code, frontend translates via i18n
			Action:       log.Action,
			ActionName:   log.Action, // Backend returns code, frontend translates via i18n
			ClusterID:    log.ClusterID,
			ClusterName:  log.ClusterName,
			Namespace:    log.Namespace,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			StatusCode:   log.StatusCode,
			Success:      log.Success,
			ErrorMessage: log.ErrorMessage,
			ClientIP:     log.ClientIP,
			Duration:     log.Duration,
			CreatedAt:    log.CreatedAt,
		}
	}

	return &OperationLogListResponse{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetDetail retrieves the details of an operation log by ID
func (s *OperationLogService) GetDetail(id uint) (*models.OperationLog, error) {
	var log models.OperationLog
	if err := s.db.First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// OperationLogStats contains statistics for operation logs
type OperationLogStats struct {
	TotalCount     int64               `json:"total_count"`
	TodayCount     int64               `json:"today_count"`
	SuccessCount   int64               `json:"success_count"`
	FailedCount    int64               `json:"failed_count"`
	ModuleStats    []ModuleStat        `json:"module_stats"`
	ActionStats    []ActionStat        `json:"action_stats"`
	RecentFailures []OperationLogItem  `json:"recent_failures"`
	UserStats      []UserOperationStat `json:"user_stats"`
}

// ModuleStat contains statistics for a module
type ModuleStat struct {
	Module     string `json:"module"`
	ModuleName string `json:"module_name"`
	Count      int64  `json:"count"`
}

// ActionStat contains statistics for an action
type ActionStat struct {
	Action     string `json:"action"`
	ActionName string `json:"action_name"`
	Count      int64  `json:"count"`
}

// UserOperationStat contains operation statistics for a user
type UserOperationStat struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Count    int64  `json:"count"`
}

// GetStats retrieves statistics for operation logs within a time range
func (s *OperationLogService) GetStats(startTime, endTime *time.Time) (*OperationLogStats, error) {
	stats := &OperationLogStats{}

	baseQuery := s.db.Model(&models.OperationLog{})
	if startTime != nil {
		baseQuery = baseQuery.Where("created_at >= ?", startTime)
	}
	if endTime != nil {
		baseQuery = baseQuery.Where("created_at <= ?", endTime)
	}

	// Total count
	baseQuery.Count(&stats.TotalCount)

	// Count for today
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&models.OperationLog{}).Where("created_at >= ?", today).Count(&stats.TodayCount)

	// Success/failed counts
	baseQuery.Where("success = ?", true).Count(&stats.SuccessCount)
	s.db.Model(&models.OperationLog{}).Where("success = ?", false).Count(&stats.FailedCount)

	// Module statistics
	var moduleStats []struct {
		Module string
		Count  int64
	}
	s.db.Model(&models.OperationLog{}).
		Select("module, COUNT(*) as count").
		Group("module").
		Order("count DESC").
		Limit(10).
		Scan(&moduleStats)

	stats.ModuleStats = make([]ModuleStat, len(moduleStats))
	for i, ms := range moduleStats {
		stats.ModuleStats[i] = ModuleStat{
			Module:     ms.Module,
			ModuleName: ms.Module, // Backend returns code, frontend translates via i18n
			Count:      ms.Count,
		}
	}

	// Action statistics
	var actionStats []struct {
		Action string
		Count  int64
	}
	s.db.Model(&models.OperationLog{}).
		Select("action, COUNT(*) as count").
		Group("action").
		Order("count DESC").
		Limit(10).
		Scan(&actionStats)

	stats.ActionStats = make([]ActionStat, len(actionStats))
	for i, as := range actionStats {
		stats.ActionStats[i] = ActionStat{
			Action:     as.Action,
			ActionName: as.Action, // Backend returns code, frontend translates via i18n
			Count:      as.Count,
		}
	}

	// Recent failed operations
	var recentFailures []models.OperationLog
	s.db.Model(&models.OperationLog{}).
		Where("success = ?", false).
		Order("created_at DESC").
		Limit(10).
		Find(&recentFailures)

	stats.RecentFailures = make([]OperationLogItem, len(recentFailures))
	for i, log := range recentFailures {
		stats.RecentFailures[i] = OperationLogItem{
			ID:           log.ID,
			UserID:       log.UserID,
			Username:     log.Username,
			Method:       log.Method,
			Path:         log.Path,
			Module:       log.Module,
			ModuleName:   log.Module, // Backend returns code, frontend translates via i18n
			Action:       log.Action,
			ActionName:   log.Action, // Backend returns code, frontend translates via i18n
			ClusterName:  log.ClusterName,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			Success:      log.Success,
			ErrorMessage: log.ErrorMessage,
			ClientIP:     log.ClientIP,
			CreatedAt:    log.CreatedAt,
		}
	}

	// User operation statistics
	var userStats []struct {
		UserID   uint
		Username string
		Count    int64
	}
	s.db.Model(&models.OperationLog{}).
		Select("user_id, username, COUNT(*) as count").
		Where("user_id IS NOT NULL").
		Group("user_id, username").
		Order("count DESC").
		Limit(10).
		Scan(&userStats)

	stats.UserStats = make([]UserOperationStat, len(userStats))
	for i, us := range userStats {
		stats.UserStats[i] = UserOperationStat{
			UserID:   us.UserID,
			Username: us.Username,
			Count:    us.Count,
		}
	}

	return stats, nil
}

// NOTE: getModuleName and getActionName have been removed.
// Backend now returns module and action codes; frontend handles all translations via i18n.
// This ensures a single source of truth for translations and prevents hardcoded strings.

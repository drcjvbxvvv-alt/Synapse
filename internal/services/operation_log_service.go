package services

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"kubepolaris/internal/models"
	"kubepolaris/pkg/logger"

	"gorm.io/gorm"
)

// OperationLogService 操作审计日志服务
type OperationLogService struct {
	db *gorm.DB
}

// NewOperationLogService 创建操作审计日志服务
func NewOperationLogService(db *gorm.DB) *OperationLogService {
	return &OperationLogService{db: db}
}

// LogEntry 日志条目（用于记录）
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

// Record 记录操作日志
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
		logger.Error("记录操作日志失败", "error", err)
		return err
	}

	return nil
}

// RecordAsync 异步记录操作日志（不阻塞请求）
func (s *OperationLogService) RecordAsync(entry *LogEntry) {
	go func() {
		if err := s.Record(entry); err != nil {
			logger.Error("异步记录操作日志失败", "error", err, "path", entry.Path, "action", entry.Action)
		}
	}()
}

// sensitiveKeys 敏感字段列表
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

// sanitizeAndMarshal 脱敏并序列化请求体
func sanitizeAndMarshal(body interface{}) string {
	if body == nil {
		return ""
	}

	// 如果是字符串，尝试解析为JSON再脱敏
	if str, ok := body.(string); ok {
		if str == "" {
			return ""
		}
		var data interface{}
		if err := json.Unmarshal([]byte(str), &data); err == nil {
			body = data
		} else {
			// 不是有效JSON，直接返回
			return str
		}
	}

	// 深度脱敏
	sanitized := sanitizeValue(body)

	result, err := json.Marshal(sanitized)
	if err != nil {
		return ""
	}

	// 限制长度，避免存储过大
	if len(result) > 4000 {
		return string(result[:4000]) + "...(truncated)"
	}

	return string(result)
}

// sanitizeValue 递归脱敏值
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
		// 使用反射处理struct和map
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
				// 移除omitempty等标签
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

// isSensitiveKey 判断是否是敏感字段
func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	if sensitiveKeys[lowerKey] {
		return true
	}
	// 检查是否包含敏感词
	for sensitiveWord := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitiveWord) {
			return true
		}
	}
	return false
}

// OperationLogListRequest 操作日志列表请求
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

// OperationLogListResponse 操作日志列表响应
type OperationLogListResponse struct {
	Items    []OperationLogItem `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

// OperationLogItem 操作日志列表项
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

// List 获取操作日志列表
func (s *OperationLogService) List(req *OperationLogListRequest) (*OperationLogListResponse, error) {
	query := s.db.Model(&models.OperationLog{})

	// 应用过滤条件
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

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	offset := (req.Page - 1) * req.PageSize

	// 查询数据
	var logs []models.OperationLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&logs).Error; err != nil {
		return nil, err
	}

	// 转换为响应格式
	items := make([]OperationLogItem, len(logs))
	for i, log := range logs {
		items[i] = OperationLogItem{
			ID:           log.ID,
			UserID:       log.UserID,
			Username:     log.Username,
			Method:       log.Method,
			Path:         log.Path,
			Module:       log.Module,
			ModuleName:   getModuleName(log.Module),
			Action:       log.Action,
			ActionName:   getActionName(log.Action),
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

// GetDetail 获取操作日志详情
func (s *OperationLogService) GetDetail(id uint) (*models.OperationLog, error) {
	var log models.OperationLog
	if err := s.db.First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// OperationLogStats 操作日志统计
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

// ModuleStat 模块统计
type ModuleStat struct {
	Module     string `json:"module"`
	ModuleName string `json:"module_name"`
	Count      int64  `json:"count"`
}

// ActionStat 操作统计
type ActionStat struct {
	Action     string `json:"action"`
	ActionName string `json:"action_name"`
	Count      int64  `json:"count"`
}

// UserOperationStat 用户操作统计
type UserOperationStat struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Count    int64  `json:"count"`
}

// GetStats 获取操作日志统计
func (s *OperationLogService) GetStats(startTime, endTime *time.Time) (*OperationLogStats, error) {
	stats := &OperationLogStats{}

	baseQuery := s.db.Model(&models.OperationLog{})
	if startTime != nil {
		baseQuery = baseQuery.Where("created_at >= ?", startTime)
	}
	if endTime != nil {
		baseQuery = baseQuery.Where("created_at <= ?", endTime)
	}

	// 总数
	baseQuery.Count(&stats.TotalCount)

	// 今日数量
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&models.OperationLog{}).Where("created_at >= ?", today).Count(&stats.TodayCount)

	// 成功/失败数量
	baseQuery.Where("success = ?", true).Count(&stats.SuccessCount)
	s.db.Model(&models.OperationLog{}).Where("success = ?", false).Count(&stats.FailedCount)

	// 模块统计
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
			ModuleName: getModuleName(ms.Module),
			Count:      ms.Count,
		}
	}

	// 操作统计
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
			ActionName: getActionName(as.Action),
			Count:      as.Count,
		}
	}

	// 最近失败的操作
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
			ModuleName:   getModuleName(log.Module),
			Action:       log.Action,
			ActionName:   getActionName(log.Action),
			ClusterName:  log.ClusterName,
			ResourceType: log.ResourceType,
			ResourceName: log.ResourceName,
			Success:      log.Success,
			ErrorMessage: log.ErrorMessage,
			ClientIP:     log.ClientIP,
			CreatedAt:    log.CreatedAt,
		}
	}

	// 用户操作统计
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

// getModuleName 获取模块中文名称
func getModuleName(module string) string {
	names := map[string]string{
		"auth":       "认证管理",
		"cluster":    "集群管理",
		"node":       "节点管理",
		"pod":        "Pod管理",
		"workload":   "工作负载",
		"config":     "配置管理",
		"network":    "网络管理",
		"storage":    "存储管理",
		"namespace":  "命名空间",
		"permission": "权限管理",
		"system":     "系统设置",
		"monitoring": "监控配置",
		"alert":      "告警管理",
		"argocd":     "GitOps",
		"unknown":    "未知",
	}
	if name, ok := names[module]; ok {
		return name
	}
	return module
}

// getActionName 获取操作中文名称
func getActionName(action string) string {
	names := map[string]string{
		"login":           "登录",
		"logout":          "登出",
		"login_failed":    "登录失败",
		"change_password": "修改密码",
		"create":          "创建",
		"update":          "更新",
		"delete":          "删除",
		"apply":           "应用YAML",
		"scale":           "扩缩容",
		"rollback":        "回滚",
		"restart":         "重启",
		"cordon":          "禁止调度",
		"uncordon":        "允许调度",
		"drain":           "驱逐节点",
		"sync":            "同步",
		"test":            "测试",
		"import":          "导入",
	}
	if name, ok := names[action]; ok {
		return name
	}
	return action
}

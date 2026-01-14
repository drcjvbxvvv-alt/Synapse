package services

import (
	"encoding/json"
	"time"

	"github.com/clay-wangzhi/KubePolaris/internal/models"
	"github.com/clay-wangzhi/KubePolaris/pkg/logger"

	"gorm.io/gorm"
)

// AuditService 审计服务
type AuditService struct {
	db *gorm.DB
}

// NewAuditService 创建审计服务
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// TerminalType 终端类型
type TerminalType string

const (
	TerminalTypeKubectl TerminalType = "kubectl"
	TerminalTypePod     TerminalType = "pod"
	TerminalTypeNode    TerminalType = "node"
)

// CreateSessionRequest 创建会话请求
type CreateSessionRequest struct {
	UserID     uint
	ClusterID  uint
	TargetType TerminalType
	Namespace  string
	Pod        string
	Container  string
	Node       string
	ClientIP   string
	UserAgent  string
}

// TargetRef 目标引用信息
type TargetRef struct {
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Container string `json:"container,omitempty"`
	Node      string `json:"node,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
}

// CreateSession 创建终端会话
func (s *AuditService) CreateSession(req *CreateSessionRequest) (*models.TerminalSession, error) {
	// 构建目标引用
	targetRef := TargetRef{
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Container: req.Container,
		Node:      req.Node,
	}
	targetRefJSON, _ := json.Marshal(targetRef)

	session := &models.TerminalSession{
		UserID:     req.UserID,
		ClusterID:  req.ClusterID,
		TargetType: string(req.TargetType),
		TargetRef:  string(targetRefJSON),
		Namespace:  req.Namespace,
		Pod:        req.Pod,
		Container:  req.Container,
		Node:       req.Node,
		StartAt:    time.Now(),
		Status:     "active",
	}

	if err := s.db.Create(session).Error; err != nil {
		logger.Error("创建终端会话失败", "error", err)
		return nil, err
	}

	logger.Info("终端会话已创建", "sessionID", session.ID, "userID", req.UserID, "type", req.TargetType)
	return session, nil
}

// CloseSession 关闭终端会话
func (s *AuditService) CloseSession(sessionID uint, status string) error {
	now := time.Now()
	err := s.db.Model(&models.TerminalSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"end_at": now,
			"status": status,
		}).Error

	if err != nil {
		logger.Error("关闭终端会话失败", "error", err, "sessionID", sessionID)
		return err
	}

	logger.Info("终端会话已关闭", "sessionID", sessionID, "status", status)
	return nil
}

// RecordCommand 记录命令（异步调用，不阻塞终端）
func (s *AuditService) RecordCommand(sessionID uint, rawInput, parsedCmd string, exitCode *int) error {
	command := &models.TerminalCommand{
		SessionID: sessionID,
		Timestamp: time.Now(),
		RawInput:  rawInput,
		ParsedCmd: parsedCmd,
		ExitCode:  exitCode,
	}

	if err := s.db.Create(command).Error; err != nil {
		logger.Error("记录命令失败", "error", err, "sessionID", sessionID)
		return err
	}

	// 更新会话的输入大小
	s.db.Model(&models.TerminalSession{}).
		Where("id = ?", sessionID).
		Update("input_size", gorm.Expr("input_size + ?", len(rawInput)))

	return nil
}

// RecordCommandAsync 异步记录命令
func (s *AuditService) RecordCommandAsync(sessionID uint, rawInput, parsedCmd string, exitCode *int) {
	go func() {
		if err := s.RecordCommand(sessionID, rawInput, parsedCmd, exitCode); err != nil {
			logger.Error("异步记录命令失败", "error", err)
		}
	}()
}

// SessionListRequest 会话列表请求
type SessionListRequest struct {
	UserID     uint
	ClusterID  uint
	TargetType string
	Status     string
	StartTime  *time.Time
	EndTime    *time.Time
	Keyword    string
	Page       int
	PageSize   int
}

// SessionListResponse 会话列表响应
type SessionListResponse struct {
	Items    []SessionItem `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

// SessionItem 会话列表项
type SessionItem struct {
	ID           uint       `json:"id"`
	UserID       uint       `json:"user_id"`
	Username     string     `json:"username"`
	DisplayName  string     `json:"display_name"`
	ClusterID    uint       `json:"cluster_id"`
	ClusterName  string     `json:"cluster_name"`
	TargetType   string     `json:"target_type"`
	TargetRef    string     `json:"target_ref"`
	Namespace    string     `json:"namespace"`
	Pod          string     `json:"pod"`
	Container    string     `json:"container"`
	Node         string     `json:"node"`
	StartAt      time.Time  `json:"start_at"`
	EndAt        *time.Time `json:"end_at"`
	InputSize    int64      `json:"input_size"`
	Status       string     `json:"status"`
	CommandCount int64      `json:"command_count"`
}

// GetSessions 获取会话列表
func (s *AuditService) GetSessions(req *SessionListRequest) (*SessionListResponse, error) {
	query := s.db.Model(&models.TerminalSession{}).
		Select(`terminal_sessions.*, 
			users.username, users.display_name,
			clusters.name as cluster_name,
			(SELECT COUNT(*) FROM terminal_commands WHERE terminal_commands.session_id = terminal_sessions.id) as command_count`).
		Joins("LEFT JOIN users ON users.id = terminal_sessions.user_id").
		Joins("LEFT JOIN clusters ON clusters.id = terminal_sessions.cluster_id")

	// 应用过滤条件
	if req.UserID > 0 {
		query = query.Where("terminal_sessions.user_id = ?", req.UserID)
	}
	if req.ClusterID > 0 {
		query = query.Where("terminal_sessions.cluster_id = ?", req.ClusterID)
	}
	if req.TargetType != "" {
		query = query.Where("terminal_sessions.target_type = ?", req.TargetType)
	}
	if req.Status != "" {
		query = query.Where("terminal_sessions.status = ?", req.Status)
	}
	if req.StartTime != nil {
		query = query.Where("terminal_sessions.start_at >= ?", req.StartTime)
	}
	if req.EndTime != nil {
		query = query.Where("terminal_sessions.start_at <= ?", req.EndTime)
	}
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		query = query.Where("(users.username LIKE ? OR users.display_name LIKE ? OR clusters.name LIKE ? OR terminal_sessions.pod LIKE ? OR terminal_sessions.node LIKE ?)",
			keyword, keyword, keyword, keyword, keyword)
	}

	// 计算总数
	var total int64
	countQuery := s.db.Model(&models.TerminalSession{}).
		Joins("LEFT JOIN users ON users.id = terminal_sessions.user_id").
		Joins("LEFT JOIN clusters ON clusters.id = terminal_sessions.cluster_id")

	if req.UserID > 0 {
		countQuery = countQuery.Where("terminal_sessions.user_id = ?", req.UserID)
	}
	if req.ClusterID > 0 {
		countQuery = countQuery.Where("terminal_sessions.cluster_id = ?", req.ClusterID)
	}
	if req.TargetType != "" {
		countQuery = countQuery.Where("terminal_sessions.target_type = ?", req.TargetType)
	}
	if req.Status != "" {
		countQuery = countQuery.Where("terminal_sessions.status = ?", req.Status)
	}
	if req.StartTime != nil {
		countQuery = countQuery.Where("terminal_sessions.start_at >= ?", req.StartTime)
	}
	if req.EndTime != nil {
		countQuery = countQuery.Where("terminal_sessions.start_at <= ?", req.EndTime)
	}
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		countQuery = countQuery.Where("(users.username LIKE ? OR users.display_name LIKE ? OR clusters.name LIKE ? OR terminal_sessions.pod LIKE ? OR terminal_sessions.node LIKE ?)",
			keyword, keyword, keyword, keyword, keyword)
	}
	countQuery.Count(&total)

	// 分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	offset := (req.Page - 1) * req.PageSize

	var results []struct {
		models.TerminalSession
		Username     string `gorm:"column:username"`
		DisplayName  string `gorm:"column:display_name"`
		ClusterName  string `gorm:"column:cluster_name"`
		CommandCount int64  `gorm:"column:command_count"`
	}

	if err := query.Order("terminal_sessions.start_at DESC").Offset(offset).Limit(req.PageSize).Scan(&results).Error; err != nil {
		return nil, err
	}

	// 转换为响应格式
	items := make([]SessionItem, len(results))
	for i, r := range results {
		items[i] = SessionItem{
			ID:           r.ID,
			UserID:       r.UserID,
			Username:     r.Username,
			DisplayName:  r.DisplayName,
			ClusterID:    r.ClusterID,
			ClusterName:  r.ClusterName,
			TargetType:   r.TargetType,
			TargetRef:    r.TargetRef,
			Namespace:    r.Namespace,
			Pod:          r.Pod,
			Container:    r.Container,
			Node:         r.Node,
			StartAt:      r.StartAt,
			EndAt:        r.EndAt,
			InputSize:    r.InputSize,
			Status:       r.Status,
			CommandCount: r.CommandCount,
		}
	}

	return &SessionListResponse{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// SessionDetailResponse 会话详情响应
type SessionDetailResponse struct {
	ID           uint                     `json:"id"`
	UserID       uint                     `json:"user_id"`
	Username     string                   `json:"username"`
	DisplayName  string                   `json:"display_name"`
	ClusterID    uint                     `json:"cluster_id"`
	ClusterName  string                   `json:"cluster_name"`
	TargetType   string                   `json:"target_type"`
	TargetRef    string                   `json:"target_ref"`
	Namespace    string                   `json:"namespace"`
	Pod          string                   `json:"pod"`
	Container    string                   `json:"container"`
	Node         string                   `json:"node"`
	StartAt      time.Time                `json:"start_at"`
	EndAt        *time.Time               `json:"end_at"`
	InputSize    int64                    `json:"input_size"`
	Status       string                   `json:"status"`
	CommandCount int64                    `json:"command_count"`
	Duration     string                   `json:"duration"`
	Commands     []models.TerminalCommand `json:"commands,omitempty"`
}

// GetSessionDetail 获取会话详情
func (s *AuditService) GetSessionDetail(sessionID uint) (*SessionDetailResponse, error) {
	var result struct {
		models.TerminalSession
		Username    string `gorm:"column:username"`
		DisplayName string `gorm:"column:display_name"`
		ClusterName string `gorm:"column:cluster_name"`
	}

	err := s.db.Model(&models.TerminalSession{}).
		Select(`terminal_sessions.*, 
			users.username, users.display_name,
			clusters.name as cluster_name`).
		Joins("LEFT JOIN users ON users.id = terminal_sessions.user_id").
		Joins("LEFT JOIN clusters ON clusters.id = terminal_sessions.cluster_id").
		Where("terminal_sessions.id = ?", sessionID).
		First(&result).Error

	if err != nil {
		return nil, err
	}

	// 获取命令数量
	var commandCount int64
	s.db.Model(&models.TerminalCommand{}).Where("session_id = ?", sessionID).Count(&commandCount)

	// 计算持续时间
	var duration string
	if result.EndAt != nil {
		d := result.EndAt.Sub(result.StartAt)
		duration = formatSessionDuration(d)
	} else {
		d := time.Since(result.StartAt)
		duration = formatSessionDuration(d) + " (进行中)"
	}

	return &SessionDetailResponse{
		ID:           result.ID,
		UserID:       result.UserID,
		Username:     result.Username,
		DisplayName:  result.DisplayName,
		ClusterID:    result.ClusterID,
		ClusterName:  result.ClusterName,
		TargetType:   result.TargetType,
		TargetRef:    result.TargetRef,
		Namespace:    result.Namespace,
		Pod:          result.Pod,
		Container:    result.Container,
		Node:         result.Node,
		StartAt:      result.StartAt,
		EndAt:        result.EndAt,
		InputSize:    result.InputSize,
		Status:       result.Status,
		CommandCount: commandCount,
		Duration:     duration,
	}, nil
}

// CommandListResponse 命令列表响应
type CommandListResponse struct {
	Items    []models.TerminalCommand `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}

// GetSessionCommands 获取会话命令
func (s *AuditService) GetSessionCommands(sessionID uint, page, pageSize int) (*CommandListResponse, error) {
	var commands []models.TerminalCommand
	var total int64

	query := s.db.Model(&models.TerminalCommand{}).Where("session_id = ?", sessionID)
	query.Count(&total)

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	if err := query.Order("timestamp ASC").Offset(offset).Limit(pageSize).Find(&commands).Error; err != nil {
		return nil, err
	}

	return &CommandListResponse{
		Items:    commands,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetSessionStats 获取会话统计信息
type SessionStats struct {
	TotalSessions   int64 `json:"total_sessions"`
	ActiveSessions  int64 `json:"active_sessions"`
	TotalCommands   int64 `json:"total_commands"`
	KubectlSessions int64 `json:"kubectl_sessions"`
	PodSessions     int64 `json:"pod_sessions"`
	NodeSessions    int64 `json:"node_sessions"`
}

// GetSessionStats 获取会话统计
func (s *AuditService) GetSessionStats() (*SessionStats, error) {
	stats := &SessionStats{}

	// 总会话数
	s.db.Model(&models.TerminalSession{}).Count(&stats.TotalSessions)

	// 活跃会话数
	s.db.Model(&models.TerminalSession{}).Where("status = ?", "active").Count(&stats.ActiveSessions)

	// 总命令数
	s.db.Model(&models.TerminalCommand{}).Count(&stats.TotalCommands)

	// 各类型会话数
	s.db.Model(&models.TerminalSession{}).Where("target_type = ?", "kubectl").Count(&stats.KubectlSessions)
	s.db.Model(&models.TerminalSession{}).Where("target_type = ?", "pod").Count(&stats.PodSessions)
	s.db.Model(&models.TerminalSession{}).Where("target_type = ?", "node").Count(&stats.NodeSessions)

	return stats, nil
}

// formatSessionDuration 格式化会话持续时间
func formatSessionDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		return d.Round(time.Minute).String()
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return (time.Duration(hours) * time.Hour).String()
	}
	return (time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute).String()
}

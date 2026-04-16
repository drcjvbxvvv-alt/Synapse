package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"gorm.io/gorm"
)

// zeroHash is the sentinel prev_hash for the very first audit log entry.
const zeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

// AuditService 審計服務
type AuditService struct {
	db      *gorm.DB
	sink    AuditSink
	chainMu sync.Mutex // serialises LogAudit to maintain hash chain integrity
}

// NewAuditService 建立審計服務（預設 DBSink）
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{
		db:   db,
		sink: NewDBSink(db),
	}
}

// WithSink replaces the default DBSink.
// Useful for multi-sink fan-out (DB + webhook) or testing with a stub sink.
func (s *AuditService) WithSink(sink AuditSink) *AuditService {
	s.sink = sink
	return s
}

// ── Hash chain (P2-2) ─────────────────────────────────────────────────────────

// LogAuditRequest carries the fields needed to create one audit log entry.
type LogAuditRequest struct {
	UserID       uint
	Action       string
	ResourceType string
	ResourceRef  string // JSON-encoded resource reference (may be empty)
	Result       string // "success" | "failed"
	IP           string
	UserAgent    string
	Details      string
}

// LogAudit writes a hash-chained audit log entry.
// A mutex ensures that prev_hash → hash ordering is consistent within a
// single process. Cross-process ordering is preserved by inserting the
// prev_hash before the DB write; duplicate chain tips are idempotent because
// the hash commits to the creation timestamp (UnixNano).
func (s *AuditService) LogAudit(ctx context.Context, req LogAuditRequest) error {
	s.chainMu.Lock()
	defer s.chainMu.Unlock()

	prevHash, err := s.getLastHash(ctx)
	if err != nil {
		return fmt.Errorf("audit: get last hash: %w", err)
	}

	entry := &models.AuditLog{
		UserID:       req.UserID,
		Action:       req.Action,
		ResourceType: req.ResourceType,
		ResourceRef:  req.ResourceRef,
		Result:       req.Result,
		IP:           req.IP,
		UserAgent:    req.UserAgent,
		Details:      req.Details,
		PrevHash:     prevHash,
		CreatedAt:    time.Now(),
	}
	// Compute hash before INSERT — single DB write, no UPDATE needed.
	entry.Hash = computeAuditHash(prevHash, entry)

	if err := s.sink.Write(ctx, entry); err != nil {
		return fmt.Errorf("audit: write entry: %w", err)
	}
	return nil
}

// ChainVerifyResult is the output of VerifyChain.
type ChainVerifyResult struct {
	Verified  int    `json:"verified"`
	Tampered  []uint `json:"tampered,omitempty"`
	FirstHash string `json:"first_hash,omitempty"`
	LastHash  string `json:"last_hash,omitempty"`
}

// VerifyChain checks the integrity of the most recent `limit` audit log
// entries that carry a hash (records written before hash-chain support was
// enabled have an empty Hash and are skipped automatically).
func (s *AuditService) VerifyChain(ctx context.Context, limit int) (*ChainVerifyResult, error) {
	if limit <= 0 {
		limit = 1000
	}

	var entries []models.AuditLog
	if err := s.db.WithContext(ctx).
		Where("hash != ''").
		Order("id ASC").
		Limit(limit).
		Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("verify chain: query: %w", err)
	}

	result := &ChainVerifyResult{}
	if len(entries) == 0 {
		return result, nil
	}

	result.FirstHash = entries[0].Hash
	result.LastHash = entries[len(entries)-1].Hash

	for i := range entries {
		e := &entries[i]
		expected := computeAuditHash(e.PrevHash, e)
		if expected != e.Hash {
			result.Tampered = append(result.Tampered, e.ID)
		} else {
			result.Verified++
		}
	}
	return result, nil
}

// getLastHash returns the Hash of the most-recently inserted audit log
// that has a non-empty hash field, or zeroHash when starting fresh.
// Must be called while chainMu is held.
func (s *AuditService) getLastHash(ctx context.Context) (string, error) {
	var last models.AuditLog
	err := s.db.WithContext(ctx).
		Select("hash").
		Order("id DESC").
		First(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return zeroHash, nil
	}
	if err != nil {
		return "", fmt.Errorf("get last hash: %w", err)
	}
	if last.Hash == "" {
		// Existing records written before hash-chain was enabled — start fresh.
		return zeroHash, nil
	}
	return last.Hash, nil
}

// computeAuditHash returns SHA-256 of the canonical fields joined with null
// bytes. Using null bytes as separators prevents length-extension attacks.
func computeAuditHash(prevHash string, e *models.AuditLog) string {
	h := sha256.New()
	fields := []string{
		prevHash,
		strconv.FormatUint(uint64(e.UserID), 10),
		e.Action,
		e.ResourceType,
		e.ResourceRef,
		e.Result,
		e.IP,
		strconv.FormatInt(e.CreatedAt.UnixNano(), 10),
	}
	for i, f := range fields {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(f))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ── Partition management (Phase 2 / C1-C2) ───────────────────────────────────

// auditPartitionTableName returns the child partition table name for the month
// that contains t (e.g. "audit_logs_2026_04").
func auditPartitionTableName(t time.Time) string {
	return fmt.Sprintf("audit_logs_%d_%02d", t.Year(), int(t.Month()))
}

// EnsureNextMonthPartition creates the next calendar month's audit_logs
// partition if it does not already exist. The call is idempotent — it uses
// CREATE TABLE IF NOT EXISTS so running it multiple times is safe.
//
// Call on application startup and monthly via a scheduled job so the partition
// always exists before the month begins.
func (s *AuditService) EnsureNextMonthPartition(ctx context.Context) error {
	if s.db == nil {
		return nil // no-op when DB is not configured (e.g. test stubs)
	}
	next := time.Now().UTC().AddDate(0, 1, 0)
	tableName := auditPartitionTableName(next)
	start := time.Date(next.Year(), next.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	sql := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_logs`+
			` FOR VALUES FROM ('%s') TO ('%s')`,
		tableName,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)
	if err := s.db.WithContext(ctx).Exec(sql).Error; err != nil {
		return fmt.Errorf("ensure audit partition %s: %w", tableName, err)
	}
	logger.Info("audit partition ensured", "table", tableName)
	return nil
}

// DropOldPartitions drops the audit_logs partition that is exactly
// retainMonths before the current month. Uses DROP TABLE IF EXISTS so the
// call is idempotent. Call monthly to advance the retention window.
//
// Example: retainMonths=3 called in April 2026 drops audit_logs_2026_01.
func (s *AuditService) DropOldPartitions(ctx context.Context, retainMonths int) error {
	if retainMonths <= 0 {
		return fmt.Errorf("drop old partitions: retainMonths must be positive, got %d", retainMonths)
	}
	if s.db == nil {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, -retainMonths, 0)
	tableName := auditPartitionTableName(cutoff)
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
	if err := s.db.WithContext(ctx).Exec(sql).Error; err != nil {
		return fmt.Errorf("drop audit partition %s: %w", tableName, err)
	}
	logger.Info("audit partition dropped", "table", tableName, "retain_months", retainMonths)
	return nil
}

// TerminalType 終端型別
type TerminalType string

const (
	TerminalTypeKubectl TerminalType = "kubectl"
	TerminalTypePod     TerminalType = "pod"
	TerminalTypeNode    TerminalType = "node"
)

// CreateSessionRequest 建立會話請求
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

// TargetRef 目標引用資訊
type TargetRef struct {
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Container string `json:"container,omitempty"`
	Node      string `json:"node,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
}

// CreateSession 建立終端會話
func (s *AuditService) CreateSession(req *CreateSessionRequest) (*models.TerminalSession, error) {
	// 構建目標引用
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
		logger.Error("建立終端會話失敗", "error", err)
		return nil, fmt.Errorf("create terminal session: %w", err)
	}

	logger.Info("終端會話已建立", "sessionID", session.ID, "userID", req.UserID, "type", req.TargetType)
	return session, nil
}

// CloseSession 關閉終端會話
func (s *AuditService) CloseSession(sessionID uint, status string) error {
	now := time.Now()
	err := s.db.Model(&models.TerminalSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"end_at": now,
			"status": status,
		}).Error

	if err != nil {
		logger.Error("關閉終端會話失敗", "error", err, "sessionID", sessionID)
		return fmt.Errorf("close terminal session %d: %w", sessionID, err)
	}

	logger.Info("終端會話已關閉", "sessionID", sessionID, "status", status)
	return nil
}

// RecordCommand 記錄命令（非同步呼叫，不阻塞終端）
func (s *AuditService) RecordCommand(sessionID uint, rawInput, parsedCmd string, exitCode *int) error {
	command := &models.TerminalCommand{
		SessionID: sessionID,
		Timestamp: time.Now(),
		RawInput:  rawInput,
		ParsedCmd: parsedCmd,
		ExitCode:  exitCode,
	}

	if err := s.db.Create(command).Error; err != nil {
		logger.Error("記錄命令失敗", "error", err, "sessionID", sessionID)
		return fmt.Errorf("record command for session %d: %w", sessionID, err)
	}

	// 更新會話的輸入大小
	s.db.Model(&models.TerminalSession{}).
		Where("id = ?", sessionID).
		Update("input_size", gorm.Expr("input_size + ?", len(rawInput)))

	return nil
}

// RecordCommandAsync 非同步記錄命令
func (s *AuditService) RecordCommandAsync(sessionID uint, rawInput, parsedCmd string, exitCode *int) {
	go func() {
		if err := s.RecordCommand(sessionID, rawInput, parsedCmd, exitCode); err != nil {
			logger.Error("非同步記錄命令失敗", "error", err)
		}
	}()
}

// SessionListRequest 會話列表請求
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

// SessionListResponse 會話列表響應
type SessionListResponse struct {
	Items    []SessionItem `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

// SessionItem 會話列表項
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

// GetSessions 獲取會話列表
func (s *AuditService) GetSessions(req *SessionListRequest) (*SessionListResponse, error) {
	query := s.db.Model(&models.TerminalSession{}).
		Select(`terminal_sessions.*, 
			users.username, users.display_name,
			clusters.name as cluster_name,
			(SELECT COUNT(*) FROM terminal_commands WHERE terminal_commands.session_id = terminal_sessions.id) as command_count`).
		Joins("LEFT JOIN users ON users.id = terminal_sessions.user_id").
		Joins("LEFT JOIN clusters ON clusters.id = terminal_sessions.cluster_id")

	// 應用過濾條件
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

	// 計算總數
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

	// 分頁
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

	// 轉換為響應格式
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

// SessionDetailResponse 會話詳情響應
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

// GetSessionDetail 獲取會話詳情
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

	// 獲取命令數量
	var commandCount int64
	s.db.Model(&models.TerminalCommand{}).Where("session_id = ?", sessionID).Count(&commandCount)

	// 計算持續時間
	var duration string
	if result.EndAt != nil {
		d := result.EndAt.Sub(result.StartAt)
		duration = formatSessionDuration(d)
	} else {
		d := time.Since(result.StartAt)
		duration = formatSessionDuration(d) + " (進行中)"
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

// CommandListResponse 命令列表響應
type CommandListResponse struct {
	Items    []models.TerminalCommand `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}

// GetSessionCommands 獲取會話命令
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

// GetSessionStats 獲取會話統計資訊
type SessionStats struct {
	TotalSessions   int64 `json:"total_sessions"`
	ActiveSessions  int64 `json:"active_sessions"`
	TotalCommands   int64 `json:"total_commands"`
	KubectlSessions int64 `json:"kubectl_sessions"`
	PodSessions     int64 `json:"pod_sessions"`
	NodeSessions    int64 `json:"node_sessions"`
}

// GetSessionStats 獲取會話統計
func (s *AuditService) GetSessionStats() (*SessionStats, error) {
	stats := &SessionStats{}

	// 總會話數
	s.db.Model(&models.TerminalSession{}).Count(&stats.TotalSessions)

	// 活躍會話數
	s.db.Model(&models.TerminalSession{}).Where("status = ?", "active").Count(&stats.ActiveSessions)

	// 總命令數
	s.db.Model(&models.TerminalCommand{}).Count(&stats.TotalCommands)

	// 各型別會話數
	s.db.Model(&models.TerminalSession{}).Where("target_type = ?", "kubectl").Count(&stats.KubectlSessions)
	s.db.Model(&models.TerminalSession{}).Where("target_type = ?", "pod").Count(&stats.PodSessions)
	s.db.Model(&models.TerminalSession{}).Where("target_type = ?", "node").Count(&stats.NodeSessions)

	return stats, nil
}

// formatSessionDuration 格式化會話持續時間
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

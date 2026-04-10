package services

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// AuditServiceTestSuite 審計服務測試套件
type AuditServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *AuditService
}

func (s *AuditServiceTestSuite) SetupTest() {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	s.Require().NoError(err)

	s.db = gormDB
	s.mock = mock
	s.service = NewAuditService(gormDB)
}

func (s *AuditServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// TestCreateSession_Success 測試建立終端會話成功
func (s *AuditServiceTestSuite) TestCreateSession_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .terminal_sessions.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	req := &CreateSessionRequest{
		UserID:     1,
		ClusterID:  2,
		TargetType: TerminalTypePod,
		Namespace:  "default",
		Pod:        "mypod",
		Container:  "mycontainer",
		ClientIP:   "127.0.0.1",
	}

	session, err := s.service.CreateSession(req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
	assert.Equal(s.T(), uint(1), session.UserID)
	assert.Equal(s.T(), "active", session.Status)
	assert.Equal(s.T(), string(TerminalTypePod), session.TargetType)
}

// TestCreateSession_Kubectl 測試建立 kubectl 會話
func (s *AuditServiceTestSuite) TestCreateSession_Kubectl() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .terminal_sessions.`).
		WillReturnResult(sqlmock.NewResult(2, 1))
	s.mock.ExpectCommit()

	req := &CreateSessionRequest{
		UserID:     2,
		ClusterID:  1,
		TargetType: TerminalTypeKubectl,
	}

	session, err := s.service.CreateSession(req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), session)
	assert.Equal(s.T(), string(TerminalTypeKubectl), session.TargetType)
}

// TestCreateSession_DBError 測試 DB 錯誤
func (s *AuditServiceTestSuite) TestCreateSession_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .terminal_sessions.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	req := &CreateSessionRequest{UserID: 1, ClusterID: 1, TargetType: TerminalTypeNode}
	session, err := s.service.CreateSession(req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), session)
}

// TestCloseSession_Success 測試關閉終端會話成功
func (s *AuditServiceTestSuite) TestCloseSession_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .terminal_sessions.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.CloseSession(1, "closed")
	assert.NoError(s.T(), err)
}

// TestCloseSession_DBError 測試關閉會話 DB 錯誤
func (s *AuditServiceTestSuite) TestCloseSession_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .terminal_sessions.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.CloseSession(99, "error")
	assert.Error(s.T(), err)
}

// TestRecordCommand_Success 測試記錄命令成功
func (s *AuditServiceTestSuite) TestRecordCommand_Success() {
	exitCode := 0

	// INSERT terminal_command
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .terminal_commands.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	// UPDATE terminal_sessions.input_size (best-effort, can fail silently)
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .terminal_sessions.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.RecordCommand(1, "kubectl get pods", "kubectl get pods", &exitCode)
	assert.NoError(s.T(), err)
}

// TestRecordCommand_DBError 測試記錄命令 DB 錯誤
func (s *AuditServiceTestSuite) TestRecordCommand_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .terminal_commands.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.RecordCommand(1, "ls -la", "ls -la", nil)
	assert.Error(s.T(), err)
}

// TestGetSessionCommands_Success 測試獲取會話命令
func (s *AuditServiceTestSuite) TestGetSessionCommands_Success() {
	now := time.Now()

	// Count
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Find
	rows := sqlmock.NewRows([]string{"id", "session_id", "timestamp", "raw_input", "parsed_cmd", "exit_code"}).
		AddRow(1, 5, now, "ls", "ls", 0).
		AddRow(2, 5, now, "pwd", "pwd", 0)
	s.mock.ExpectQuery(`SELECT`).WillReturnRows(rows)

	resp, err := s.service.GetSessionCommands(5, 1, 20)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), int64(2), resp.Total)
	assert.Len(s.T(), resp.Items, 2)
}

// TestGetSessionCommands_DefaultPagination 測試預設分頁值
func (s *AuditServiceTestSuite) TestGetSessionCommands_DefaultPagination() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "session_id", "timestamp", "raw_input", "parsed_cmd", "exit_code"}))

	resp, err := s.service.GetSessionCommands(1, 0, 0)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, resp.Page)
	assert.Equal(s.T(), 100, resp.PageSize)
}

// TestGetSessionCommands_DBError 測試查詢失敗
func (s *AuditServiceTestSuite) TestGetSessionCommands_DBError() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrInvalidDB)

	resp, err := s.service.GetSessionCommands(1, 1, 20)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
}

// TestGetSessionStats_Success 測試獲取會話統計
func (s *AuditServiceTestSuite) TestGetSessionStats_Success() {
	// 6 COUNT queries in order:
	// TotalSessions, ActiveSessions, TotalCommands, KubectlSessions, PodSessions, NodeSessions
	s.mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	s.mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	s.mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(500))
	s.mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(40))
	s.mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(35))
	s.mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))

	stats, err := s.service.GetSessionStats()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), stats)
	assert.Equal(s.T(), int64(100), stats.TotalSessions)
	assert.Equal(s.T(), int64(5), stats.ActiveSessions)
	assert.Equal(s.T(), int64(500), stats.TotalCommands)
}

// TestFormatSessionDuration 測試持續時間格式化（純函式）
func (s *AuditServiceTestSuite) TestFormatSessionDuration() {
	// Less than a minute
	assert.Contains(s.T(), formatSessionDuration(30*time.Second), "s")
	// Minutes
	assert.Contains(s.T(), formatSessionDuration(5*time.Minute), "m")
	// Hours
	assert.Contains(s.T(), formatSessionDuration(2*time.Hour), "h")
	// Hours + minutes
	result := formatSessionDuration(1*time.Hour + 30*time.Minute)
	assert.Contains(s.T(), result, "h")
}

// TestAuditServiceSuite 執行測試套件
func TestAuditServiceSuite(t *testing.T) {
	suite.Run(t, new(AuditServiceTestSuite))
}

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

	"github.com/shaia/Synapse/internal/models"
)

// EventAlertServiceTestSuite 事件告警服務測試套件
type EventAlertServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *EventAlertService
}

func (s *EventAlertServiceTestSuite) SetupTest() {
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
	s.service = NewEventAlertService(gormDB)
}

func (s *EventAlertServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// TestListRules_Success 測試列出規則成功
func (s *EventAlertServiceTestSuite) TestListRules_Success() {
	now := time.Now()

	// Count query
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Find query
	rows := sqlmock.NewRows([]string{
		"id", "cluster_id", "name", "description", "namespace",
		"event_reason", "event_type", "min_count", "notify_type",
		"notify_url", "enabled", "created_at", "updated_at", "deleted_at",
	}).
		AddRow(1, 1, "rule-1", "desc1", "default", "OOMKilling", "Warning", 1, "webhook", "http://hook1", true, now, now, nil).
		AddRow(2, 1, "rule-2", "desc2", "kube-system", "BackOff", "Warning", 2, "slack", "http://slack", true, now, now, nil)

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(rows)

	rules, total, err := s.service.ListRules(1, 1, 20)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), total)
	assert.Len(s.T(), rules, 2)
	assert.Equal(s.T(), "rule-1", rules[0].Name)
}

// TestListRules_CountError 測試 Count 失敗
func (s *EventAlertServiceTestSuite) TestListRules_CountError() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnError(gorm.ErrInvalidDB)

	rules, total, err := s.service.ListRules(1, 1, 20)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), int64(0), total)
	assert.Nil(s.T(), rules)
}

// TestListRules_Empty 測試空結果
func (s *EventAlertServiceTestSuite) TestListRules_Empty() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "cluster_id", "name", "description", "namespace",
			"event_reason", "event_type", "min_count", "notify_type",
			"notify_url", "enabled", "created_at", "updated_at", "deleted_at",
		}))

	rules, total, err := s.service.ListRules(1, 1, 20)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), total)
	assert.Len(s.T(), rules, 0)
}

// TestCreateRule_Success 測試建立規則成功
func (s *EventAlertServiceTestSuite) TestCreateRule_Success() {
	rule := &models.EventAlertRule{
		ClusterID:   1,
		Name:        "new-rule",
		EventType:   "Warning",
		EventReason: "OOMKilling",
		MinCount:    1,
		Enabled:     true,
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .event_alert_rules.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.CreateRule(rule)
	assert.NoError(s.T(), err)
}

// TestCreateRule_DBError 測試建立規則DB錯誤
func (s *EventAlertServiceTestSuite) TestCreateRule_DBError() {
	rule := &models.EventAlertRule{
		ClusterID: 1,
		Name:      "fail-rule",
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .event_alert_rules.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.CreateRule(rule)
	assert.Error(s.T(), err)
}

// TestUpdateRule_Success 測試更新規則成功
func (s *EventAlertServiceTestSuite) TestUpdateRule_Success() {
	now := time.Now()
	rule := &models.EventAlertRule{
		ID:        1,
		ClusterID: 1,
		Name:      "updated-rule",
		Enabled:   false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .event_alert_rules.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.UpdateRule(rule)
	assert.NoError(s.T(), err)
}

// TestDeleteRule_Success 測試刪除規則成功
func (s *EventAlertServiceTestSuite) TestDeleteRule_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .event_alert_rules. SET .deleted_at.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.DeleteRule(1)
	assert.NoError(s.T(), err)
}

// TestDeleteRule_DBError 測試刪除規則DB錯誤
func (s *EventAlertServiceTestSuite) TestDeleteRule_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .event_alert_rules. SET .deleted_at.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.DeleteRule(99)
	assert.Error(s.T(), err)
}

// TestToggleRule_Enable 測試啟用規則
func (s *EventAlertServiceTestSuite) TestToggleRule_Enable() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .event_alert_rules.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.ToggleRule(1, true)
	assert.NoError(s.T(), err)
}

// TestToggleRule_Disable 測試停用規則
func (s *EventAlertServiceTestSuite) TestToggleRule_Disable() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .event_alert_rules.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.ToggleRule(1, false)
	assert.NoError(s.T(), err)
}

// TestToggleRule_DBError 測試切換規則DB錯誤
func (s *EventAlertServiceTestSuite) TestToggleRule_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .event_alert_rules.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.ToggleRule(99, true)
	assert.Error(s.T(), err)
}

// TestListHistory_Success 測試列出告警歷史成功
func (s *EventAlertServiceTestSuite) TestListHistory_Success() {
	now := time.Now()

	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	histRows := sqlmock.NewRows([]string{
		"id", "rule_id", "cluster_id", "rule_name", "namespace",
		"event_reason", "event_type", "message", "involved_obj",
		"notify_result", "is_read", "triggered_at",
	}).AddRow(
		1, 1, 1, "rule-1", "default",
		"OOMKilling", "Warning", "container OOM", "Pod/mypod",
		"sent", false, now,
	)

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(histRows)

	items, total, err := s.service.ListHistory(1, 1, 20)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), total)
	assert.Len(s.T(), items, 1)
	assert.Equal(s.T(), "rule-1", items[0].RuleName)
}

// TestListHistory_CountError 測試 Count 失敗
func (s *EventAlertServiceTestSuite) TestListHistory_CountError() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnError(gorm.ErrInvalidDB)

	items, total, err := s.service.ListHistory(1, 1, 20)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), int64(0), total)
	assert.Nil(s.T(), items)
}

// TestListHistory_Pagination 測試分頁偏移計算
func (s *EventAlertServiceTestSuite) TestListHistory_Pagination() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "rule_id", "cluster_id", "rule_name", "namespace",
			"event_reason", "event_type", "message", "involved_obj",
			"notify_result", "is_read", "triggered_at",
		}))

	// page=3, pageSize=10 → offset=20
	items, total, err := s.service.ListHistory(1, 3, 10)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(50), total)
	assert.Len(s.T(), items, 0)
}

// TestEventAlertServiceSuite 執行測試套件
func TestEventAlertServiceSuite(t *testing.T) {
	suite.Run(t, new(EventAlertServiceTestSuite))
}

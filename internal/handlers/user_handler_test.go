package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/services"
)

// UserHandlerTestSuite 使用者處理器測試套件
type UserHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	router  *gin.Engine
	handler *UserHandler
}

// userCols returns the standard column list used by sqlmock row definitions.
// Keeping it in one place avoids drift between test methods.
var userCols = []string{
	"id", "username", "password_hash", "salt", "email", "display_name",
	"phone", "auth_type", "status", "system_role",
	"last_login_at", "last_login_ip", "created_at", "updated_at", "deleted_at",
}

func (s *UserHandlerTestSuite) SetupTest() {
	gin.SetMode(gin.TestMode)

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

	// nil repo → UserService falls back to the legacy *gorm.DB path.
	userSvc := services.NewUserService(gormDB, nil)
	s.handler = NewUserHandler(userSvc)

	r := gin.New()
	r.GET("/api/users", s.handler.ListUsers)
	r.GET("/api/users/:id", s.handler.GetUser)
	r.POST("/api/users", s.handler.CreateUser)
	r.PUT("/api/users/:id", s.handler.UpdateUser)
	r.DELETE("/api/users/:id", s.handler.DeleteUser)
	r.PUT("/api/users/:id/status", s.handler.UpdateUserStatus)
	r.PUT("/api/users/:id/password", s.handler.ResetPassword)
	s.router = r
}

func (s *UserHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// ─── ListUsers ───────────────────────────────────────────────────────────────

func (s *UserHandlerTestSuite) TestListUsers_Success() {
	now := time.Now()

	// COUNT query
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .users.`).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))

	// SELECT query
	s.mock.ExpectQuery(`SELECT \* FROM .users.`).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(
			1, "alice", "", "", "alice@example.com", "Alice", "", "local", "active", "user",
			nil, "", now, now, nil,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users?page=1&pageSize=20", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 1, body["total"])
}

func (s *UserHandlerTestSuite) TestListUsers_DBError() {
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .users.`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── GetUser ─────────────────────────────────────────────────────────────────

func (s *UserHandlerTestSuite) TestGetUser_Success() {
	now := time.Now()
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(
			1, "alice", "", "", "alice@example.com", "Alice", "", "local", "active", "user",
			nil, "", now, now, nil,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users/1", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(s.T(), "alice", body["username"])
}

func (s *UserHandlerTestSuite) TestGetUser_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users/abc", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestGetUser_NotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(999, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users/999", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	errObj, ok := body["error"].(map[string]interface{})
	s.Require().True(ok)
	assert.Equal(s.T(), "USER_NOT_FOUND", errObj["code"])
}

// ─── CreateUser ──────────────────────────────────────────────────────────────

func (s *UserHandlerTestSuite) TestCreateUser_Success() {
	// username uniqueness check
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .users.`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	// INSERT
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .users.`).
		WillReturnResult(sqlmock.NewResult(2, 1))
	s.mock.ExpectCommit()

	body, _ := json.Marshal(map[string]string{
		"username": "bob",
		"password": "secret123",
		"email":    "bob@example.com",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *UserHandlerTestSuite) TestCreateUser_InvalidJSON() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/users", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestCreateUser_MissingRequiredFields() {
	// username present but password missing
	body, _ := json.Marshal(map[string]string{"username": "bob"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestCreateUser_DuplicateUsername() {
	// username already exists
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .users.`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	body, _ := json.Marshal(map[string]string{
		"username": "alice",
		"password": "secret123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusConflict, w.Code)

	var respBody map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &respBody))
	errObj, ok := respBody["error"].(map[string]interface{})
	s.Require().True(ok)
	assert.Equal(s.T(), "USER_DUPLICATE_USERNAME", errObj["code"])
}

// ─── UpdateUser ──────────────────────────────────────────────────────────────

func (s *UserHandlerTestSuite) TestUpdateUser_Success() {
	now := time.Now()
	// fetchUser SELECT
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(
			1, "alice", "", "", "alice@example.com", "Alice", "", "local", "active", "user",
			nil, "", now, now, nil,
		))
	// Save UPDATE
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .users.`).WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	newEmail := "newalice@example.com"
	body, _ := json.Marshal(map[string]interface{}{"email": newEmail})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *UserHandlerTestSuite) TestUpdateUser_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/xyz", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestUpdateUser_UserNotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(99, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	body, _ := json.Marshal(map[string]interface{}{"email": "x@x.com"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/99", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

// ─── DeleteUser ──────────────────────────────────────────────────────────────

func (s *UserHandlerTestSuite) TestDeleteUser_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/users/nope", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestDeleteUser_CannotDeleteSelf() {
	// user_id context key = 5, deleting id = 5 → bad request
	selfRouter := gin.New()
	selfRouter.DELETE("/api/users/:id", func(c *gin.Context) {
		c.Set("user_id", uint(5))
		s.handler.DeleteUser(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/users/5", nil)
	selfRouter.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	errObj, ok := body["error"].(map[string]interface{})
	s.Require().True(ok)
	assert.Equal(s.T(), "BAD_REQUEST", errObj["code"])
}

func (s *UserHandlerTestSuite) TestDeleteUser_NotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(88, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/users/88", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *UserHandlerTestSuite) TestDeleteUser_AdminProtected() {
	now := time.Now()
	// fetch returns a platform_admin user
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(3, 1).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(
			3, "admin", "", "", "", "Admin", "", "local", "active", "platform_admin",
			nil, "", now, now, nil,
		))
	// best-effort cleanup deletes (UserGroupMember, ClusterPermission)
	s.mock.ExpectExec(`.*`).WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectExec(`.*`).WillReturnResult(sqlmock.NewResult(0, 0))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/users/3", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusForbidden, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	errObj, ok := body["error"].(map[string]interface{})
	s.Require().True(ok)
	assert.Equal(s.T(), "USER_ADMIN_PROTECTED", errObj["code"])
}

// ─── UpdateUserStatus ────────────────────────────────────────────────────────

func (s *UserHandlerTestSuite) TestUpdateUserStatus_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/bad/status", bytes.NewBufferString(`{"status":"active"}`))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestUpdateUserStatus_MissingBody() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/1/status", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	// status field missing → binding required → 400
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestUpdateUserStatus_Success() {
	now := time.Now()
	// fetchUser
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(
			2, "carol", "", "", "", "Carol", "", "local", "active", "user",
			nil, "", now, now, nil,
		))
	// Save
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .users.`).WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	body, _ := json.Marshal(map[string]string{"status": "inactive"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/2/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *UserHandlerTestSuite) TestUpdateUserStatus_UserNotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(77, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	body, _ := json.Marshal(map[string]string{"status": "inactive"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/77/status", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

// ─── ResetPassword ───────────────────────────────────────────────────────────

func (s *UserHandlerTestSuite) TestResetPassword_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/xyz/password", bytes.NewBufferString(`{"new_password":"newpass1"}`))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestResetPassword_MissingBody() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/1/password", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	// new_password binding:required,min=6 → 400
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestResetPassword_PasswordTooShort() {
	body, _ := json.Marshal(map[string]string{"new_password": "abc"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/1/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *UserHandlerTestSuite) TestResetPassword_Success() {
	now := time.Now()
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows(userCols).AddRow(
			1, "alice", "", "kp_alice_salt", "alice@example.com", "Alice", "", "local", "active", "user",
			nil, "", now, now, nil,
		))
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .users.`).WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	body, _ := json.Marshal(map[string]string{"new_password": "newsecret123"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/1/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *UserHandlerTestSuite) TestResetPassword_UserNotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `users`.`id` = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(55, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	body, _ := json.Marshal(map[string]string{"new_password": "newsecret123"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/users/55/password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

// ─── Suite runner ────────────────────────────────────────────────────────────

func TestUserHandlerSuite(t *testing.T) {
	suite.Run(t, new(UserHandlerTestSuite))
}

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGitProviderHandler_Get_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GitProviderHandler{} // nil svc — won't reach service call

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	h.Get(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitProviderHandler_Update_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GitProviderHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "xyz"}}

	h.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitProviderHandler_Update_NoFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GitProviderHandler{}

	body := UpdateGitProviderRequest{}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitProviderHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GitProviderHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "not-a-number"}}

	h.Delete(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitProviderHandler_RegenerateToken_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GitProviderHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad"}}

	h.RegenerateToken(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitProviderHandler_Create_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GitProviderHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewReader([]byte(`{invalid`)))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitProviderHandler_Create_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GitProviderHandler{}

	// Missing name, type, base_url
	body := map[string]string{"access_token": "tok"}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGitProviderHandler_Create_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GitProviderHandler{}

	body := CreateGitProviderRequest{
		Name:    "bad",
		Type:    "bitbucket", // not in oneof
		BaseURL: "https://example.com",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

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

func TestRegistryHandler_Get_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	h.Get(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_Create_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewReader([]byte(`{invalid`)))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_Create_MissingRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	body := map[string]string{"username": "user"}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_Create_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	body := CreateRegistryRequest{
		Name: "bad",
		Type: "quay", // not in oneof
		URL:  "https://quay.io",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_Update_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "xyz"}}

	h.Update(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_Update_NoFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	body := UpdateRegistryRequest{}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("PUT", "/", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.Update(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "not-a-number"}}

	h.Delete(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_TestConnection_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad"}}

	h.TestConnection(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_ListRepositories_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad"}}

	h.ListRepositories(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_ListTags_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/?repository=test", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad"}}

	h.ListTags(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegistryHandler_ListTags_MissingRepository(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &RegistryHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.ListTags(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

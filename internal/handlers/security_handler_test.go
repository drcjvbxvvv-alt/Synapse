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

func TestSecurityHandler_IngestScan_InvalidClusterID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SecurityHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Params = gin.Params{{Key: "clusterID", Value: "abc"}}

	h.IngestScan(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_IngestScan_MissingImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SecurityHandler{}

	body := map[string]string{"namespace": "default"}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "clusterID", Value: "1"}}

	h.IngestScan(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_IngestScan_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SecurityHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", bytes.NewReader([]byte(`{invalid`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "clusterID", Value: "1"}}

	h.IngestScan(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSecurityHandler_TriggerScan_InvalidClusterID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SecurityHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Params = gin.Params{{Key: "clusterID", Value: "xyz"}}

	h.TriggerScan(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

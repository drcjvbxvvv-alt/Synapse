package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/models"
)

// PrometheusService Prometheus 查詢服務
type PrometheusService struct {
	httpClient *http.Client
}

// NewPrometheusService 建立 Prometheus 服務
func NewPrometheusService() *PrometheusService {
	return &PrometheusService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // #nosec G402 -- 內部叢集 Prometheus 通訊，使用者可自行配置證書
				},
			},
		},
	}
}

// QueryPrometheus 查詢 Prometheus
func (s *PrometheusService) QueryPrometheus(ctx context.Context, config *models.MonitoringConfig, query *models.MetricsQuery) (*models.MetricsResponse, error) {
	if config.Type == "disabled" {
		return nil, fmt.Errorf("監控功能已禁用")
	}

	// 構建查詢 URL
	queryURL, err := s.buildQueryURL(config.Endpoint, query)
	if err != nil {
		return nil, fmt.Errorf("構建查詢URL失敗: %w", err)
	}

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return nil, fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("執行請求失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	// 檢查狀態碼
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查詢失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	// 解析響應
	var result models.MetricsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w", err)
	}

	return &result, nil
}

// TestConnection 測試監控資料來源連線
func (s *PrometheusService) TestConnection(ctx context.Context, config *models.MonitoringConfig) error {
	if config.Type == "disabled" {
		return fmt.Errorf("監控功能已禁用")
	}

	// 構建測試查詢 URL
	testURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return fmt.Errorf("無效的監控端點: %w", err)
	}
	testURL.Path = "/api/v1/query"
	testURL.RawQuery = "query=up"

	// 建立測試請求
	req, err := http.NewRequestWithContext(ctx, "GET", testURL.String(), nil)
	if err != nil {
		return fmt.Errorf("建立測試請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行測試請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("連線測試失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("監控資料來源響應異常: %s", string(body))
	}

	return nil
}

// ── Instant query helper ────────────────────────────────────────────────────

// instantResult is the minimal shape of a Prometheus /api/v1/query response.
type instantResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Value []interface{} `json:"value"` // [unixtime, "value_string"]
		} `json:"result"`
	} `json:"data"`
}

// QueryInstantScalar executes a PromQL instant query and returns the first scalar result.
// Returns (math.NaN(), nil) when the result set is empty (metric absent / no data).
func (s *PrometheusService) QueryInstantScalar(ctx context.Context, config *models.MonitoringConfig, expr string) (float64, error) {
	if config == nil || config.Type == "disabled" {
		return math.NaN(), fmt.Errorf("monitoring disabled or config nil")
	}

	base, err := url.Parse(config.Endpoint)
	if err != nil {
		return math.NaN(), fmt.Errorf("parse endpoint: %w", err)
	}
	base.Path = "/api/v1/query"
	q := base.Query()
	q.Set("query", expr)
	base.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", base.String(), nil)
	if err != nil {
		return math.NaN(), fmt.Errorf("build request: %w", err)
	}
	if err := s.setAuth(req, config.Auth); err != nil {
		return math.NaN(), fmt.Errorf("set auth: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return math.NaN(), fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return math.NaN(), fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return math.NaN(), fmt.Errorf("prometheus returned %d: %s", resp.StatusCode, string(body))
	}

	var result instantResult
	if err := json.Unmarshal(body, &result); err != nil {
		return math.NaN(), fmt.Errorf("parse response: %w", err)
	}
	if result.Status != "success" {
		return math.NaN(), fmt.Errorf("prometheus status: %s", result.Status)
	}
	if len(result.Data.Result) == 0 {
		return math.NaN(), nil // no data — caller decides what to do
	}

	vals := result.Data.Result[0].Value
	if len(vals) < 2 {
		return math.NaN(), fmt.Errorf("unexpected value array length %d", len(vals))
	}
	raw, ok := vals[1].(string)
	if !ok {
		return math.NaN(), fmt.Errorf("value is not a string")
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return math.NaN(), fmt.Errorf("parse float %q: %w", raw, err)
	}
	return v, nil
}

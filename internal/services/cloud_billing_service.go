package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// ---- 資料結構 ----

// CloudBillingOverview 帳單彙總 + 資源單位成本
type CloudBillingOverview struct {
	Month          string                      `json:"month"`
	Provider       string                      `json:"provider"`
	TotalAmount    float64                     `json:"total_amount"`
	Currency       string                      `json:"currency"`
	CPUUnitCost    float64                     `json:"cpu_unit_cost"`    // USD/core-hour（實際帳單÷資源小時）
	MemoryUnitCost float64                     `json:"memory_unit_cost"` // USD/GiB-hour
	Services       []models.CloudBillingRecord `json:"services"`
	LastSyncedAt   *time.Time                  `json:"last_synced_at,omitempty"`
	SyncError      string                      `json:"sync_error,omitempty"`
}

// UpdateBillingConfigReq 更新帳單設定請求（空字串的敏感欄位保留舊值）
type UpdateBillingConfigReq struct {
	Provider string `json:"provider"`

	AWSAccessKeyID     string `json:"aws_access_key_id"`
	AWSSecretAccessKey string `json:"aws_secret_access_key"` // 空值 = 不更新
	AWSRegion          string `json:"aws_region"`
	AWSLinkedAccountID string `json:"aws_linked_account_id"`

	GCPProjectID          string `json:"gcp_project_id"`
	GCPBillingAccountID   string `json:"gcp_billing_account_id"`
	GCPServiceAccountJSON string `json:"gcp_service_account_json"` // 空值 = 不更新
}

// ---- CloudBillingService ----

type CloudBillingService struct {
	db         *gorm.DB
	httpClient *http.Client
}

func NewCloudBillingService(db *gorm.DB) *CloudBillingService {
	return &CloudBillingService{
		db:         db,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetConfig 取得（或自動建立）叢集帳單設定
func (s *CloudBillingService) GetConfig(clusterID uint) (*models.CloudBillingConfig, error) {
	var cfg models.CloudBillingConfig
	err := s.db.Where("cluster_id = ?", clusterID).First(&cfg).Error
	if err == gorm.ErrRecordNotFound {
		cfg = models.CloudBillingConfig{ClusterID: clusterID, Provider: "disabled"}
		if err2 := s.db.Create(&cfg).Error; err2 != nil {
			return nil, err2
		}
	} else if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateConfig 更新帳單設定，敏感欄位只在非空時才覆寫
func (s *CloudBillingService) UpdateConfig(clusterID uint, req *UpdateBillingConfigReq) (*models.CloudBillingConfig, error) {
	cfg, err := s.GetConfig(clusterID)
	if err != nil {
		return nil, err
	}

	cfg.Provider = req.Provider
	cfg.AWSAccessKeyID = req.AWSAccessKeyID
	cfg.AWSRegion = req.AWSRegion
	cfg.AWSLinkedAccountID = req.AWSLinkedAccountID
	cfg.GCPProjectID = req.GCPProjectID
	cfg.GCPBillingAccountID = req.GCPBillingAccountID

	// 敏感欄位：空值表示「不更新」
	if req.AWSSecretAccessKey != "" {
		cfg.AWSSecretAccessKey = req.AWSSecretAccessKey
	}
	if req.GCPServiceAccountJSON != "" {
		cfg.GCPServiceAccountJSON = req.GCPServiceAccountJSON
	}

	if err := s.db.Save(cfg).Error; err != nil {
		return nil, err
	}
	return cfg, nil
}

// SyncBilling 觸發帳單同步（從雲端拉取最新資料存入 DB）
func (s *CloudBillingService) SyncBilling(clusterID uint, month string) error {
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	cfg, err := s.GetConfig(clusterID)
	if err != nil {
		return err
	}

	var records []models.CloudBillingRecord
	var syncErr error

	switch cfg.Provider {
	case "aws":
		records, syncErr = s.fetchAWSCost(cfg, month)
	case "gcp":
		records, syncErr = s.fetchGCPCost(cfg, month)
	default:
		return fmt.Errorf("雲端帳單未啟用（provider=%s）", cfg.Provider)
	}

	// 不論成功或失敗都更新 LastSyncedAt 和 LastError
	now := time.Now()
	cfg.LastSyncedAt = &now
	if syncErr != nil {
		cfg.LastError = syncErr.Error()
	} else {
		cfg.LastError = ""
	}
	_ = s.db.Save(cfg)

	if syncErr != nil {
		return syncErr
	}

	// 刪除舊紀錄再插入（冪等）
	if err := s.db.Where("cluster_id = ? AND month = ? AND provider = ?", clusterID, month, cfg.Provider).
		Delete(&models.CloudBillingRecord{}).Error; err != nil {
		return err
	}
	if len(records) > 0 {
		return s.db.Create(&records).Error
	}
	return nil
}

// GetBillingOverview 從 DB 讀取帳單記錄並計算資源單位成本
func (s *CloudBillingService) GetBillingOverview(clusterID uint, month string) (*CloudBillingOverview, error) {
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	cfg, err := s.GetConfig(clusterID)
	if err != nil {
		return nil, err
	}

	var records []models.CloudBillingRecord
	if err := s.db.Where("cluster_id = ? AND month = ?", clusterID, month).
		Order("amount desc").Find(&records).Error; err != nil {
		return nil, err
	}

	total := 0.0
	currency := "USD"
	for _, r := range records {
		total += r.Amount
		if r.Currency != "" {
			currency = r.Currency
		}
	}

	// 資源單位成本計算（需要叢集快照資料）
	cpuUnitCost, memUnitCost := s.calcUnitCost(clusterID, month, total)

	return &CloudBillingOverview{
		Month:          month,
		Provider:       cfg.Provider,
		TotalAmount:    total,
		Currency:       currency,
		CPUUnitCost:    cpuUnitCost,
		MemoryUnitCost: memUnitCost,
		Services:       records,
		LastSyncedAt:   cfg.LastSyncedAt,
		SyncError:      cfg.LastError,
	}, nil
}

// calcUnitCost 以實際帳單÷資源小時計算單位成本
// 當無帳單資料時回傳 0
func (s *CloudBillingService) calcUnitCost(clusterID uint, month string, totalAmount float64) (cpuUnitCost, memUnitCost float64) {
	if totalAmount == 0 {
		return 0, 0
	}

	// 查詢該月所有叢集快照，計算平均可分配資源
	var snaps []models.ClusterOccupancySnapshot
	_ = s.db.Where("cluster_id = ? AND strftime('%Y-%m', date) = ?", clusterID, month).
		Find(&snaps)

	if len(snaps) == 0 {
		// MySQL 語法
		_ = s.db.Where("cluster_id = ? AND DATE_FORMAT(date, '%Y-%m') = ?", clusterID, month).
			Find(&snaps)
	}

	if len(snaps) == 0 {
		return 0, 0
	}

	var sumCPU, sumMem float64
	for _, sn := range snaps {
		sumCPU += sn.AllocatableCPU
		sumMem += sn.AllocatableMemory
	}
	avgCPUMillicores := sumCPU / float64(len(snaps))
	avgMemMiB := sumMem / float64(len(snaps))

	// 計算月小時數（28–31 天 × 24 小時）
	t, _ := time.Parse("2006-01", month)
	nextMonth := t.AddDate(0, 1, 0)
	hoursInMonth := nextMonth.Sub(t).Hours()

	avgCPUCores := avgCPUMillicores / 1000
	avgMemGiB := avgMemMiB / 1024

	cpuResourceHours := avgCPUCores * hoursInMonth
	memResourceHours := avgMemGiB * hoursInMonth

	if cpuResourceHours > 0 {
		// 典型雲端 CPU:記憶體費用比例約 65:35
		cpuUnitCost = (totalAmount * 0.65) / cpuResourceHours
	}
	if memResourceHours > 0 {
		memUnitCost = (totalAmount * 0.35) / memResourceHours
	}
	return
}

// ---- AWS Cost Explorer（SigV4）----

func (s *CloudBillingService) fetchAWSCost(cfg *models.CloudBillingConfig, month string) ([]models.CloudBillingRecord, error) {
	if cfg.AWSAccessKeyID == "" || cfg.AWSSecretAccessKey == "" {
		return nil, fmt.Errorf("AWS Access Key ID 或 Secret Access Key 未設定")
	}

	start := month + "-01"
	t, _ := time.Parse("2006-01", month)
	end := t.AddDate(0, 1, 0).Format("2006-01-02")

	// 建立 Cost Explorer 請求 Body
	reqBody := map[string]interface{}{
		"TimePeriod":  map[string]string{"Start": start, "End": end},
		"Granularity": "MONTHLY",
		"Metrics":     []string{"UnblendedCost"},
		"GroupBy":     []map[string]string{{"Type": "DIMENSION", "Key": "SERVICE"}},
	}
	if cfg.AWSLinkedAccountID != "" {
		reqBody["Filter"] = map[string]interface{}{
			"Dimensions": map[string]interface{}{
				"Key":    "LINKED_ACCOUNT",
				"Values": []string{cfg.AWSLinkedAccountID},
			},
		}
	}
	bodyBytes, _ := json.Marshal(reqBody)

	endpoint := "https://ce.us-east-1.amazonaws.com"
	region := cfg.AWSRegion
	if region == "" {
		region = "us-east-1"
	}

	// SigV4 簽名
	reqHeaders := awsSigV4Headers(
		"POST", endpoint, region, "ce",
		cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey,
		string(bodyBytes),
	)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	for k, v := range reqHeaders {
		req.Header.Set(k, v)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AWS Cost Explorer 請求失敗: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AWS Cost Explorer 回傳 %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析回應
	var ceResp struct {
		ResultsByTime []struct {
			Groups []struct {
				Keys    []string `json:"Keys"`
				Metrics map[string]struct {
					Amount string `json:"Amount"`
					Unit   string `json:"Unit"`
				} `json:"Metrics"`
			} `json:"Groups"`
		} `json:"ResultsByTime"`
	}
	if err := json.Unmarshal(respBody, &ceResp); err != nil {
		return nil, fmt.Errorf("解析 AWS CE 回應失敗: %w", err)
	}

	records := make([]models.CloudBillingRecord, 0)
	for _, rt := range ceResp.ResultsByTime {
		for _, g := range rt.Groups {
			if len(g.Keys) == 0 {
				continue
			}
			m, ok := g.Metrics["UnblendedCost"]
			if !ok {
				continue
			}
			amount, _ := strconv.ParseFloat(m.Amount, 64)
			if amount < 0.001 {
				continue // 跳過極小費用
			}
			records = append(records, models.CloudBillingRecord{
				ClusterID: cfg.ClusterID,
				Month:     month,
				Provider:  "aws",
				Service:   g.Keys[0],
				Amount:    amount,
				Currency:  m.Unit,
			})
		}
	}
	return records, nil
}

// awsSigV4Headers 產生帶 SigV4 簽名的 HTTP Header map
func awsSigV4Headers(method, endpoint, region, service, accessKeyID, secretKey, body string) map[string]string {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	// 取得 host
	host := strings.TrimPrefix(endpoint, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.Split(host, "/")[0]

	target := "AWSInsightsIndexService.GetCostAndUsage"

	// 建立 Canonical Request
	payloadHash := sha256Hex(body)
	canonicalHeaders := fmt.Sprintf(
		"content-type:application/x-amz-json-1.1\nhost:%s\nx-amz-date:%s\nx-amz-target:%s\n",
		host, amzDate, target,
	)
	signedHeaders := "content-type;host;x-amz-date;x-amz-target"
	canonicalReq := strings.Join([]string{
		method, "/", "", canonicalHeaders, signedHeaders, payloadHash,
	}, "\n")

	// 建立 String To Sign
	credScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, credScope, sha256Hex(canonicalReq),
	}, "\n")

	// 計算 Signing Key
	signingKey := hmacSHA256Bytes(
		hmacSHA256Bytes(
			hmacSHA256Bytes(
				hmacSHA256Bytes([]byte("AWS4"+secretKey), dateStamp),
				region),
			service),
		"aws4_request")

	// 計算 Signature
	signature := hex.EncodeToString(hmacSHA256Bytes(signingKey, stringToSign))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKeyID, credScope, signedHeaders, signature,
	)

	return map[string]string{
		"Authorization":    authHeader,
		"Content-Type":     "application/x-amz-json-1.1",
		"X-Amz-Date":       amzDate,
		"X-Amz-Target":     target,
		"X-Amz-Content-Sha256": payloadHash,
	}
}

func hmacSHA256Bytes(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---- GCP Cloud Billing（oauth2/google + Budget API）----

func (s *CloudBillingService) fetchGCPCost(cfg *models.CloudBillingConfig, month string) ([]models.CloudBillingRecord, error) {
	if cfg.GCPServiceAccountJSON == "" {
		return nil, fmt.Errorf("GCP Service Account JSON 未設定")
	}
	if cfg.GCPBillingAccountID == "" {
		return nil, fmt.Errorf("GCP Billing Account ID 未設定（格式：billingAccounts/XXXX-XXXX-XXXX）")
	}

	ctx := context.Background()
	creds, err := google.CredentialsFromJSON(ctx, []byte(cfg.GCPServiceAccountJSON),
		"https://www.googleapis.com/auth/cloud-billing.readonly")
	if err != nil {
		return nil, fmt.Errorf("GCP service account JSON 無效: %w", err)
	}
	token, err := creds.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("取得 GCP access token 失敗: %w", err)
	}

	// 查詢 Cloud Billing Budget API
	billingAccountID := cfg.GCPBillingAccountID
	budgetURL := fmt.Sprintf("https://billingbudgets.googleapis.com/v1/%s/budgets", billingAccountID)

	req, err := http.NewRequest("GET", budgetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GCP Budget API 請求失敗: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("GCP 權限不足（需要 billing.budgets.get 或 billing.accounts.viewer 角色）: %s", string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GCP Budget API 回傳 %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析預算列表，提取 currentSpend
	var budgetResp struct {
		Budgets []struct {
			Name         string `json:"name"`
			DisplayName  string `json:"displayName"`
			BudgetFilter struct {
				Projects []string `json:"projects"` // "projects/my-project-id"
			} `json:"budgetFilter"`
			CurrentSpend struct {
				Units         string `json:"units"`
				Nanos         int    `json:"nanos"`
				CurrencyCode  string `json:"currencyCode"`
			} `json:"currentSpend"`
		} `json:"budgets"`
	}
	if err := json.Unmarshal(respBody, &budgetResp); err != nil {
		return nil, fmt.Errorf("解析 GCP Budget 回應失敗: %w", err)
	}

	records := make([]models.CloudBillingRecord, 0)
	targetProject := ""
	if cfg.GCPProjectID != "" {
		targetProject = "projects/" + cfg.GCPProjectID
	}

	for _, budget := range budgetResp.Budgets {
		// 若設定了 Project ID，只取包含該 Project 的預算
		if targetProject != "" {
			matched := false
			for _, p := range budget.BudgetFilter.Projects {
				if p == targetProject {
					matched = true
					break
				}
			}
			if !matched && len(budget.BudgetFilter.Projects) > 0 {
				continue
			}
		}

		units, _ := strconv.ParseFloat(budget.CurrentSpend.Units, 64)
		nanos := float64(budget.CurrentSpend.Nanos) / 1e9
		amount := units + nanos
		if amount < 0.001 {
			continue
		}

		name := budget.DisplayName
		if name == "" {
			name = "GCP Budget"
		}
		records = append(records, models.CloudBillingRecord{
			ClusterID: cfg.ClusterID,
			Month:     month,
			Provider:  "gcp",
			Service:   name,
			Amount:    amount,
			Currency:  budget.CurrentSpend.CurrencyCode,
		})
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("GCP Billing Account 中未找到匹配的預算（Budget）記錄。請先在 GCP Console 建立預算，或將帳單匯出至 BigQuery 取得詳細費用資料。")
	}
	return records, nil
}

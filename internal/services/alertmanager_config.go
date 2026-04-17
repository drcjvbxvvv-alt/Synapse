package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// getRawConfig 從 Alertmanager /api/v2/status 取得原始 config YAML
func (s *AlertManagerService) getRawConfig(ctx context.Context, config *models.AlertManagerConfig) (string, error) {
	status, err := s.GetStatus(ctx, config)
	if err != nil {
		return "", fmt.Errorf("取得 Alertmanager 狀態失敗: %w", err)
	}
	return status.Config.Original, nil
}

// GetFullReceivers 取得完整 Receiver 設定列表（解析 config YAML）
func (s *AlertManagerService) GetFullReceivers(ctx context.Context, config *models.AlertManagerConfig) ([]models.ReceiverConfig, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("alertmanager 未啟用")
	}
	raw, err := s.getRawConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("get full receivers: %w", err)
	}
	var fullCfg models.AlertmanagerFullConfig
	if err := yaml.Unmarshal([]byte(raw), &fullCfg); err != nil {
		return nil, fmt.Errorf("解析 Alertmanager 配置 YAML 失敗: %w", err)
	}
	return fullCfg.Receivers, nil
}

// updateConfigMapAndReload 更新 K8s ConfigMap 並觸發 Alertmanager reload
func (s *AlertManagerService) updateConfigMapAndReload(
	ctx context.Context,
	config *models.AlertManagerConfig,
	clientset *kubernetes.Clientset,
	newYAML string,
) error {
	ns := config.ConfigMapNamespace
	name := config.ConfigMapName
	if ns == "" || name == "" {
		return fmt.Errorf("未設定 ConfigMap 命名空間或名稱，無法寫回設定")
	}

	// 更新 ConfigMap
	cm, err := clientset.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("取得 ConfigMap %s/%s 失敗: %w", ns, name, err)
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data["alertmanager.yaml"] = newYAML
	if _, err := clientset.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("更新 ConfigMap 失敗: %w", err)
	}

	// 觸發 reload
	reloadURL, _ := url.Parse(config.Endpoint)
	reloadURL.Path = "/-/reload"
	req, err := http.NewRequestWithContext(ctx, "POST", reloadURL.String(), nil)
	if err != nil {
		return fmt.Errorf("建立 reload 請求失敗: %w", err)
	}
	if err := s.setAuth(req, config.Auth); err != nil {
		return fmt.Errorf("設定認證失敗: %w", err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Warn("觸發 Alertmanager reload 失敗", "error", err)
		return nil // ConfigMap 已更新，reload 失敗不視為致命
	}
	defer func() { _ = resp.Body.Close() }()
	logger.Info("Alertmanager reload 成功")
	return nil
}

// CreateReceiver 新增 Receiver
func (s *AlertManagerService) CreateReceiver(
	ctx context.Context,
	config *models.AlertManagerConfig,
	clientset *kubernetes.Clientset,
	receiver *models.ReceiverConfig,
) error {
	if !config.Enabled {
		return fmt.Errorf("alertmanager 未啟用")
	}
	raw, err := s.getRawConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("create receiver: %w", err)
	}

	var fullCfg models.AlertmanagerFullConfig
	if err := yaml.Unmarshal([]byte(raw), &fullCfg); err != nil {
		return fmt.Errorf("解析配置失敗: %w", err)
	}
	for _, r := range fullCfg.Receivers {
		if r.Name == receiver.Name {
			return fmt.Errorf("receiver '%s' 已存在", receiver.Name)
		}
	}
	fullCfg.Receivers = append(fullCfg.Receivers, *receiver)

	newYAML, err := yaml.Marshal(fullCfg)
	if err != nil {
		return fmt.Errorf("序列化配置失敗: %w", err)
	}
	return s.updateConfigMapAndReload(ctx, config, clientset, string(newYAML))
}

// UpdateReceiver 更新 Receiver
func (s *AlertManagerService) UpdateReceiver(
	ctx context.Context,
	config *models.AlertManagerConfig,
	clientset *kubernetes.Clientset,
	name string,
	receiver *models.ReceiverConfig,
) error {
	if !config.Enabled {
		return fmt.Errorf("alertmanager 未啟用")
	}
	raw, err := s.getRawConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("update receiver: %w", err)
	}

	var fullCfg models.AlertmanagerFullConfig
	if err := yaml.Unmarshal([]byte(raw), &fullCfg); err != nil {
		return fmt.Errorf("解析配置失敗: %w", err)
	}
	found := false
	for i, r := range fullCfg.Receivers {
		if r.Name == name {
			fullCfg.Receivers[i] = *receiver
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("receiver '%s' 不存在", name)
	}

	newYAML, err := yaml.Marshal(fullCfg)
	if err != nil {
		return fmt.Errorf("序列化配置失敗: %w", err)
	}
	return s.updateConfigMapAndReload(ctx, config, clientset, string(newYAML))
}

// DeleteReceiver 刪除 Receiver
func (s *AlertManagerService) DeleteReceiver(
	ctx context.Context,
	config *models.AlertManagerConfig,
	clientset *kubernetes.Clientset,
	name string,
) error {
	if !config.Enabled {
		return fmt.Errorf("alertmanager 未啟用")
	}
	raw, err := s.getRawConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("delete receiver: %w", err)
	}

	var fullCfg models.AlertmanagerFullConfig
	if err := yaml.Unmarshal([]byte(raw), &fullCfg); err != nil {
		return fmt.Errorf("解析配置失敗: %w", err)
	}
	filtered := fullCfg.Receivers[:0]
	found := false
	for _, r := range fullCfg.Receivers {
		if r.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}
	if !found {
		return fmt.Errorf("receiver '%s' 不存在", name)
	}
	fullCfg.Receivers = filtered

	newYAML, err := yaml.Marshal(fullCfg)
	if err != nil {
		return fmt.Errorf("序列化配置失敗: %w", err)
	}
	return s.updateConfigMapAndReload(ctx, config, clientset, string(newYAML))
}

// TestReceiver 傳送測試告警至指定 Receiver
func (s *AlertManagerService) TestReceiver(
	ctx context.Context,
	config *models.AlertManagerConfig,
	receiverName string,
	req *models.TestReceiverRequest,
) error {
	if !config.Enabled {
		return fmt.Errorf("alertmanager 未啟用")
	}

	labels := map[string]string{
		"alertname": "TestAlert",
		"severity":  "info",
		"receiver":  receiverName,
	}
	if req != nil {
		for k, v := range req.Labels {
			labels[k] = v
		}
	}
	annotations := map[string]string{
		"summary":     "這是一封測試告警",
		"description": "由 Synapse 發送的測試告警，請忽略",
	}
	if req != nil {
		for k, v := range req.Annotations {
			annotations[k] = v
		}
	}

	now := time.Now()
	body := []map[string]interface{}{
		{
			"labels":       labels,
			"annotations":  annotations,
			"startsAt":     now.Format(time.RFC3339),
			"endsAt":       now.Add(5 * time.Minute).Format(time.RFC3339),
			"generatorURL": "",
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化測試告警失敗: %w", err)
	}

	alertsURL, _ := url.Parse(config.Endpoint)
	alertsURL.Path = "/api/v2/alerts"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", alertsURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("建立請求失敗: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := s.setAuth(httpReq, config.Auth); err != nil {
		return fmt.Errorf("設定認證失敗: %w", err)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("傳送測試告警失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("傳送測試告警失敗: %s (狀態碼: %d)", string(b), resp.StatusCode)
	}
	logger.Info("測試告警傳送成功", "receiver", receiverName)
	return nil
}

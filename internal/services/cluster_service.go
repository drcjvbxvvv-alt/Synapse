package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/features"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/repositories"
	"github.com/shaia/Synapse/pkg/logger"

	"gorm.io/gorm"
)

const clusterListCacheTTL = 30 * time.Second

type allClustersCache struct {
	clusters  []*models.Cluster
	expiresAt time.Time
}

// StoredCluster 儲存的叢集資訊結構體
type StoredCluster struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	ApiServer     string            `json:"apiServer"`
	Version       string            `json:"version"`
	Status        string            `json:"status"`
	Labels        map[string]string `json:"labels"`
	CreatedAt     time.Time         `json:"createdAt"`
	LastHeartbeat time.Time         `json:"lastHeartbeat"`
}

// ClusterService 叢集服務
//
// P0-4b migration status: dual-path. When features.FlagRepositoryLayer is
// enabled the service routes reads/writes through the Repository layer;
// when disabled it falls back to the legacy *gorm.DB path. The legacy path
// is kept intact so the flag can be flipped off if a production regression
// shows up. Once the flag is retired, the *gorm.DB field and all
// `if features.IsEnabled(...)` branches can be deleted together.
type ClusterService struct {
	db       *gorm.DB
	repo     repositories.ClusterRepository
	cacheMu  sync.RWMutex
	allCache *allClustersCache
}

// NewClusterService builds a ClusterService with both the legacy DB handle
// and the new Repository. Both are required during the migration — once the
// flag is retired, the *gorm.DB parameter can be removed.
func NewClusterService(db *gorm.DB, repo repositories.ClusterRepository) *ClusterService {
	return &ClusterService{db: db, repo: repo}
}

// useRepo reports whether the service should dispatch to the Repository layer.
// Centralising the check makes the branches grep-friendly (look for useRepo())
// and ensures the dual paths stay in sync.
func (s *ClusterService) useRepo() bool {
	return s.repo != nil && features.IsEnabled(features.FlagRepositoryLayer)
}

// invalidateClusterCache 清除叢集列表快取（Create/Delete 時呼叫）
func (s *ClusterService) invalidateClusterCache() {
	s.cacheMu.Lock()
	s.allCache = nil
	s.cacheMu.Unlock()
}

// CreateCluster 建立叢集
func (s *ClusterService) CreateCluster(ctx context.Context, cluster *models.Cluster) error {
	// 設定建立時間
	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()
	cluster.LastHeartbeat = &cluster.CreatedAt

	// 確保 MonitoringConfig 是有效的 JSON，避免 MySQL JSON 欄位報錯
	if cluster.MonitoringConfig == "" {
		cluster.MonitoringConfig = "{}"
	}
	if cluster.MonitoringConfig != "" {
		var testJSON interface{}
		if err := json.Unmarshal([]byte(cluster.MonitoringConfig), &testJSON); err != nil {
			cluster.MonitoringConfig = "{}"
		}
	}

	// 確保 AlertManagerConfig 是有效的 JSON，避免 MySQL JSON 欄位報錯
	if cluster.AlertManagerConfig == "" {
		cluster.AlertManagerConfig = "{}"
	}
	if cluster.AlertManagerConfig != "" {
		var testJSON interface{}
		if err := json.Unmarshal([]byte(cluster.AlertManagerConfig), &testJSON); err != nil {
			cluster.AlertManagerConfig = "{}"
		}
	}

	var err error
	if s.useRepo() {
		err = s.repo.Create(ctx, cluster)
	} else {
		err = s.db.WithContext(ctx).Create(cluster).Error
	}
	if err != nil {
		logger.Error("建立叢集失敗", "error", err)
		return fmt.Errorf("建立叢集失敗: %w", err)
	}

	s.invalidateClusterCache()
	logger.Info("叢集建立成功", "id", cluster.ID, "name", cluster.Name)
	return nil
}

// GetCluster 獲取單個叢集
//
// Note: kept ctx-less in P0-4b because this is the single hottest method in
// the codebase (150+ call sites across 40+ handlers). Pushing ctx through
// every caller is a separate refactor scoped for P0-4c. Internally the
// Repository still runs with a background context — tracing is deferred.
func (s *ClusterService) GetCluster(id uint) (*models.Cluster, error) {
	ctx := context.Background()
	if s.useRepo() {
		cluster, err := s.repo.Get(ctx, id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return nil, fmt.Errorf("叢集不存在: %d", id)
			}
			return nil, fmt.Errorf("獲取叢集失敗: %w", err)
		}
		return cluster, nil
	}

	var cluster models.Cluster
	if err := s.db.WithContext(ctx).First(&cluster, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("叢集不存在: %d", id)
		}
		return nil, fmt.Errorf("獲取叢集失敗: %w", err)
	}
	return &cluster, nil
}

// GetAllClusters 獲取所有叢集（附 30 秒 TTL 快取，Create/Delete 時自動失效）
//
// The TTL cache stays in the service layer intentionally — Repository is
// stateless, and mixing cache state into a data-access object would break
// the "same repo can be shared across requests" invariant.
func (s *ClusterService) GetAllClusters(ctx context.Context) ([]*models.Cluster, error) {
	s.cacheMu.RLock()
	if s.allCache != nil && time.Now().Before(s.allCache.expiresAt) {
		result := s.allCache.clusters
		s.cacheMu.RUnlock()
		return result, nil
	}
	s.cacheMu.RUnlock()

	var clusters []*models.Cluster
	if s.useRepo() {
		list, err := s.repo.Find(ctx)
		if err != nil {
			logger.Error("獲取叢集列表失敗", "error", err)
			return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
		}
		clusters = list
	} else {
		if err := s.db.WithContext(ctx).Find(&clusters).Error; err != nil {
			logger.Error("獲取叢集列表失敗", "error", err)
			return nil, fmt.Errorf("獲取叢集列表失敗: %w", err)
		}
	}

	s.cacheMu.Lock()
	s.allCache = &allClustersCache{clusters: clusters, expiresAt: time.Now().Add(clusterListCacheTTL)}
	s.cacheMu.Unlock()
	return clusters, nil
}

// GetClustersByIDs 根據 ID 列表獲取叢集集合（用於按權限過濾）
//
// Introduced in P0-4b to replace the raw h.db.Where("id IN ?", ids) call
// that used to live inside ClusterHandler.getAccessibleClusters. Handlers
// should not touch the DB at all, so the query is pushed down here.
func (s *ClusterService) GetClustersByIDs(ctx context.Context, ids []uint) ([]*models.Cluster, error) {
	if len(ids) == 0 {
		return []*models.Cluster{}, nil
	}
	if s.useRepo() {
		return s.repo.FindByIDs(ctx, ids)
	}
	var clusters []*models.Cluster
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&clusters).Error; err != nil {
		return nil, fmt.Errorf("依 ID 查詢叢集列表失敗: %w", err)
	}
	return clusters, nil
}

// GetConnectableClusters 獲取可連線的叢集（排除 unhealthy 狀態），用於 Informer 預熱
func (s *ClusterService) GetConnectableClusters(ctx context.Context) ([]*models.Cluster, error) {
	if s.useRepo() {
		list, err := s.repo.ListConnectable(ctx)
		if err != nil {
			logger.Error("獲取可連線叢集列表失敗", "error", err)
			return nil, fmt.Errorf("獲取可連線叢集列表失敗: %w", err)
		}
		return list, nil
	}

	var clusters []*models.Cluster
	if err := s.db.WithContext(ctx).Where("status != ?", "unhealthy").Find(&clusters).Error; err != nil {
		logger.Error("獲取可連線叢集列表失敗", "error", err)
		return nil, fmt.Errorf("獲取可連線叢集列表失敗: %w", err)
	}
	return clusters, nil
}

// UpdateClusterStatus 更新叢集狀態
func (s *ClusterService) UpdateClusterStatus(ctx context.Context, id uint, status string, version string) error {
	now := time.Now()
	fields := map[string]any{
		"status":         status,
		"version":        version,
		"last_heartbeat": &now,
		"updated_at":     now,
	}

	if s.useRepo() {
		affected, err := s.repo.UpdateFields(ctx, id, fields)
		if err != nil {
			return fmt.Errorf("更新叢集狀態失敗: %w", err)
		}
		if affected == 0 {
			return fmt.Errorf("叢集不存在: %d", id)
		}
		return nil
	}

	result := s.db.WithContext(ctx).Model(&models.Cluster{}).Where("id = ?", id).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("更新叢集狀態失敗: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("叢集不存在: %d", id)
	}
	return nil
}

// DeleteCluster 刪除叢集
//
// Still uses the raw *gorm.DB path because the transaction needs to touch
// six unrelated tables (ClusterPermission, TerminalSession, TerminalCommand,
// ArgoCDConfig, OperationLog, ClusterMetrics) — defining Repository methods
// for all of them just to cover one code-path adds more noise than value.
// When the flag is retired and the legacy DB handle is removed, the
// repository's Transaction() + WithTx() API will absorb this block.
func (s *ClusterService) DeleteCluster(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 檢查叢集是否存在
		var cluster models.Cluster
		if err := tx.First(&cluster, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("叢集不存在: %d", id)
			}
			return fmt.Errorf("查詢叢集失敗: %w", err)
		}

		// 2. 刪除關聯的叢集權限（硬刪除）
		if err := tx.Unscoped().Where("cluster_id = ?", id).Delete(&models.ClusterPermission{}).Error; err != nil {
			logger.Error("刪除叢集權限失敗", "cluster_id", id, "error", err)
			return fmt.Errorf("刪除叢集權限失敗: %w", err)
		}
		logger.Info("已刪除叢集關聯的權限", "cluster_id", id)

		// 3. 刪除關聯的終端會話（硬刪除）
		if err := tx.Unscoped().Exec(`
			DELETE FROM terminal_commands
			WHERE session_id IN (SELECT id FROM terminal_sessions WHERE cluster_id = ?)
		`, id).Error; err != nil {
			logger.Error("刪除終端命令記錄失敗", "cluster_id", id, "error", err)
			return fmt.Errorf("刪除終端命令記錄失敗: %w", err)
		}
		if err := tx.Unscoped().Where("cluster_id = ?", id).Delete(&models.TerminalSession{}).Error; err != nil {
			logger.Error("刪除終端會話失敗", "cluster_id", id, "error", err)
			return fmt.Errorf("刪除終端會話失敗: %w", err)
		}
		logger.Info("已刪除叢集關聯的終端會話", "cluster_id", id)

		// 4. 刪除關聯的 ArgoCD 配置（硬刪除）
		if err := tx.Unscoped().Where("cluster_id = ?", id).Delete(&models.ArgoCDConfig{}).Error; err != nil {
			logger.Error("刪除 ArgoCD 配置失敗", "cluster_id", id, "error", err)
			return fmt.Errorf("刪除 ArgoCD 配置失敗: %w", err)
		}
		logger.Info("已刪除叢集關聯的 ArgoCD 配置", "cluster_id", id)

		// 5. 清空關聯的操作日誌的叢集引用（保留日誌記錄，只清空叢集ID）
		if err := tx.Model(&models.OperationLog{}).Where("cluster_id = ?", id).Update("cluster_id", nil).Error; err != nil {
			logger.Error("清空操作日誌叢集引用失敗", "cluster_id", id, "error", err)
		}

		// 6. 刪除叢集監控指標
		if err := tx.Where("cluster_id = ?", id).Delete(&models.ClusterMetrics{}).Error; err != nil {
			logger.Error("刪除叢集監控指標失敗", "cluster_id", id, "error", err)
		}

		// 7. 硬刪除叢集（使用 Unscoped 繞過軟刪除）
		if err := tx.Unscoped().Delete(&cluster).Error; err != nil {
			return fmt.Errorf("刪除叢集失敗: %w", err)
		}

		s.invalidateClusterCache()
		logger.Info("叢集刪除成功", "id", id, "name", cluster.Name)
		return nil
	})
}


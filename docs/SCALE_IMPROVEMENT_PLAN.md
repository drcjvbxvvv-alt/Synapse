# Synapse 規模上限改善計畫

> 狀態：✅ Phase 1 完成，✅ Phase 2 完成，Phase 3 視需求評估
> 建立日期：2026-04-16
> 對應 PLANNING.md §3.1 規模上限

---

## 目錄

1. [現況摘要](#1-現況摘要)
2. [改善項目 A：Informer 閒置 GC + Namespace 範圍限制](#2-改善項目-a叢集-informer)
3. [改善項目 B：Web Terminal 並行上限管理](#3-改善項目-bweb-terminal)
4. [改善項目 C：稽核日誌表分區 + 資料保留](#4-改善項目-c稽核日誌)
5. [優先順序與里程碑](#5-優先順序與里程碑)
6. [風險與取捨](#6-風險與取捨)

---

## 1. 現況摘要

| 項目 | 現有上限 | 根因 | 改善後目標 |
|------|----------|------|------------|
| 叢集數 | ~20 | 每叢集獨立 Informer，無閒置 GC，記憶體 O(n) | ~100（active）/ 無限（idle GC） |
| 並行 Web Terminal | ~50 | 無 semaphore、無閒置超時、無 per-user 上限 | ~200 全局 / 5 per-user |
| 稽核日誌 DB | 無硬限，但查詢隨資料量退化 | 無分區、無保留策略、全局寫入 mutex | 穩定查詢效能 + 可控磁碟用量 |

> **注意**：資料庫已是 PostgreSQL，不是 SQLite。`PLANNING.md` 的「SQLite ~1GB」描述不準確，應更新。

---

## 2. 改善項目 A：叢集 Informer

### 2.1 根因分析

**檔案**：`internal/k8s/manager.go`

`EnsureForCluster`（L112）每次建立新 `ClusterRuntime` 時：
- 立即啟動 10 個 cluster-wide informer（pods/nodes/ns/services/configmaps/secrets/deployments/statefulsets/daemonsets/jobs）
- 大叢集（5k+ Pod）單個 runtime 記憶體佔用 200–500MB
- `lastAccessAt`（L37）已有追蹤，但**缺乏定期掃描觸發 GC**
- 無上限硬保護，叢集數增加時記憶體線性增長且 goroutine 洩漏

### 2.2 方案 A1：閒置 GC（P0，低風險）

**目標**：閒置超過 N 分鐘的叢集自動停止 Informer 並釋放記憶體。

**改動範圍**：`internal/k8s/manager.go`（新增約 30 行）

```go
// StartIdleGC 啟動背景 goroutine，定期清理閒置叢集的 Informer。
// idleTimeout：叢集最後存取後超過此時間即被回收（建議 30 分鐘）。
// 需在 main.go 或 router 初始化後呼叫一次。
func (m *ClusterInformerManager) StartIdleGC(idleTimeout time.Duration) {
    go func() {
        ticker := time.NewTicker(idleTimeout / 2)
        defer ticker.Stop()
        for range ticker.C {
            m.gcOnce(idleTimeout)
        }
    }()
}

func (m *ClusterInformerManager) gcOnce(idleTimeout time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()
    for id, rt := range m.clusters {
        if time.Since(rt.lastAccessAt) > idleTimeout {
            rt.stopOnce.Do(func() { close(rt.stopCh) })
            delete(m.clusters, id)
            logger.Info("informer GC: idle cluster evicted",
                "cluster_id", id,
                "idle_minutes", time.Since(rt.lastAccessAt).Minutes(),
            )
        }
    }
}
```

**呼叫位置**：`internal/router/router.go` 或 `cmd/server/main.go`，在 `NewClusterInformerManager()` 後：

```go
k8sMgr := k8s.NewClusterInformerManager()
k8sMgr.StartIdleGC(30 * time.Minute)
```

**效果**：實際活躍 5 個叢集時，記憶體從 O(total registered) 降至 O(5)。

---

### 2.3 方案 A2：LRU 硬上限保護（P0，低風險）

**目標**：當 active 叢集數達到上限時，主動驅逐最久未存取的叢集。

**改動範圍**：`internal/k8s/manager.go`（`EnsureForCluster` 函式內，約 20 行）

```go
const defaultMaxActiveClusters = 30

// 在 EnsureForCluster 建立新 runtime 前加入：
if len(m.clusters) >= m.maxActiveClusters {
    m.evictLRU()
}

func (m *ClusterInformerManager) evictLRU() {
    var oldestID uint
    var oldestTime time.Time
    for id, rt := range m.clusters {
        if oldestTime.IsZero() || rt.lastAccessAt.Before(oldestTime) {
            oldestID = id
            oldestTime = rt.lastAccessAt
        }
    }
    if rt, ok := m.clusters[oldestID]; ok {
        rt.stopOnce.Do(func() { close(rt.stopCh) })
        delete(m.clusters, oldestID)
        logger.Info("informer LRU evict: capacity reached",
            "evicted_cluster_id", oldestID,
        )
    }
}
```

**`ClusterInformerManager` struct 新增欄位**：

```go
maxActiveClusters int // 0 = 使用 defaultMaxActiveClusters
```

---

### 2.4 方案 A3：Namespace 範圍限制（P2，中風險）

**目標**：對指定大型叢集改用 namespace-scoped informer，降低單叢集記憶體 5–10x。

**改動範圍**：`internal/k8s/manager.go`（`EnsureForCluster`，factory 建立處）

```go
// 原有：cluster-wide factory
factory := informers.NewSharedInformerFactory(clientset, 0)

// 改為：可配置 namespace 限制
var factory informers.SharedInformerFactory
if len(cluster.WatchNamespaces) > 0 {
    // WatchNamespaces 為 Cluster model 的新選填欄位（[]string）
    factory = informers.NewSharedInformerFactoryWithOptions(
        clientset, 0,
        informers.WithNamespace(cluster.WatchNamespaces[0]), // 若只需單一 ns
    )
} else {
    factory = informers.NewSharedInformerFactory(clientset, 0)
}
```

**前置條件**：
- `models.Cluster` 需新增 `WatchNamespaces []string`（JSON 欄位，允許空值）
- 前端叢集設定頁需加入 namespace 篩選欄位
- 若 `WatchNamespaces` 為空，行為與現在相同（向後相容）

**此方案依賴 A1/A2 完成後再評估是否必要。**

---

## 3. 改善項目 B：Web Terminal

### 3.1 根因分析

**檔案**：`internal/handlers/pod_terminal_handler.go`

`PodTerminalHandler`（L19）目前：
- `sessions map[string]*PodTerminalSession`（L24）：無界 map，無容量限制
- 每個 session 消耗：1 goroutine + WebSocket + 2 pipe + channel，約 2–4MB
- 無全局並行數上限
- 無每用戶上限（單用戶可開啟任意數量 terminal）
- 無閒置超時：連線掛起時 session 永不釋放

同樣問題存在於 `kubectl_terminal_handler.go`（需同步修改）。

### 3.2 方案 B1：全局 Semaphore（P0，低風險）

**目標**：硬性限制全局並行 terminal 數，達到上限時回傳 503。

**改動範圍**：`internal/handlers/pod_terminal_handler.go`（struct + WebSocket 升級前，約 15 行）

**Struct 新增**：

```go
type PodTerminalHandler struct {
    clusterService *services.ClusterService
    auditService   *services.AuditService
    k8sMgr         *k8s.ClusterInformerManager
    upgrader       websocket.Upgrader
    sessions       map[string]*PodTerminalSession
    sessionsMutex  sync.RWMutex
    sem            chan struct{} // 全局並行 semaphore
}
```

**`NewPodTerminalHandler` 初始化**：

```go
func NewPodTerminalHandler(clusterSvc *services.ClusterService, auditSvc *services.AuditService, k8sMgr *k8s.ClusterInformerManager) *PodTerminalHandler {
    return &PodTerminalHandler{
        // ...現有欄位
        sem: make(chan struct{}, 200), // 全局上限 200，可改為從 config 讀取
    }
}
```

**WebSocket handler 升級前插入**：

```go
// 嘗試取得 semaphore slot（非阻塞）
select {
case h.sem <- struct{}{}:
    defer func() { <-h.sem }()
default:
    c.JSON(http.StatusServiceUnavailable, gin.H{
        "error": "terminal capacity reached, please try again later",
    })
    return
}
```

---

### 3.3 方案 B2：每用戶上限（P0，低風險）

**目標**：單一用戶最多同時開啟 N 個 terminal，防止濫用。

**改動範圍**：`internal/handlers/pod_terminal_handler.go`（WebSocket 升級前，semaphore 之後，約 15 行）

```go
const maxSessionsPerUser = 5

// 取得當前用戶 ID（由 auth middleware 注入）
userID := middleware.GetUserID(c)

h.sessionsMutex.RLock()
userCount := 0
for _, s := range h.sessions {
    if s.UserID == userID {
        userCount++
    }
}
h.sessionsMutex.RUnlock()

if userCount >= maxSessionsPerUser {
    c.JSON(http.StatusTooManyRequests, gin.H{
        "error": fmt.Sprintf("exceeded per-user terminal limit (%d)", maxSessionsPerUser),
    })
    return
}
```

**前置條件**：`PodTerminalSession` 需新增 `UserID uint` 欄位（建立 session 時從 middleware 取得並填入）。

---

### 3.4 方案 B3：閒置超時清理（P1，低風險）

**目標**：超過 N 分鐘無任何 stdin/stdout 活動的 session 自動關閉，釋放 goroutine 和 WebSocket。

**`PodTerminalSession` 新增欄位**：

```go
lastActivityAt time.Time // 最後 stdin/stdout 活動時間
```

**每次 stdin read 和 stdout write 時更新**：

```go
atomic.StoreInt64(&s.lastActivityNano, time.Now().UnixNano())
// 或直接 s.Mutex.Lock(); s.lastActivityAt = time.Now(); s.Mutex.Unlock()
```

**`NewPodTerminalHandler` 啟動清理 goroutine**：

```go
go func() {
    ticker := time.NewTicker(2 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        h.sessionsMutex.Lock()
        for id, s := range h.sessions {
            if time.Since(s.lastActivityAt) > 15*time.Minute {
                s.Cancel()      // 觸發 context cancel，連帶關閉 K8s exec stream
                delete(h.sessions, id)
                logger.Info("terminal idle timeout: session closed",
                    "session_id", id,
                    "user_id", s.UserID,
                )
            }
        }
        h.sessionsMutex.Unlock()
    }
}()
```

---

## 4. 改善項目 C：稽核日誌

### 4.1 根因分析

**檔案**：`internal/services/audit_service.go`、`internal/models/audit.go`

現有問題：
1. **無分區**：`audit_logs` 單表，資料量大後 `WHERE user_id = ? AND created_at > ?` 需全表掃描
2. **無保留策略**：日誌無限累積，磁碟持續增長
3. **chainMu 全局鎖**（L27）：所有稽核寫入序列化，高流量時成為瓶頸
4. 現有索引只有 `user_id`、`hash`、`created_at`（各自獨立），缺乏複合索引

### 4.2 方案 C1：PostgreSQL 月分區（P1，中風險）

**目標**：按月自動分區，查詢效能穩定，DROP 舊分區代替 DELETE（零 I/O）。

**新 Migration 檔案**：`internal/database/migrations/postgres/003_audit_partition.up.sql`

```sql
-- Step 1: 備份現有資料
CREATE TABLE audit_logs_legacy AS SELECT * FROM audit_logs;

-- Step 2: 刪除舊表（包含現有 index）
DROP TABLE audit_logs;

-- Step 3: 建立分區母表
CREATE TABLE audit_logs (
    id            bigserial,
    user_id       bigint       NOT NULL,
    action        varchar(100) NOT NULL,
    resource_type varchar(50),
    resource_ref  text,
    result        varchar(10),
    ip            varchar(45),
    user_agent    text,
    details       text,
    prev_hash     varchar(64)  NOT NULL DEFAULT '',
    hash          varchar(64)  NOT NULL DEFAULT '',
    created_at    timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Step 4: 建立當月與前後各一個月的分區
-- （應由應用程式自動建立，此處建立三個初始分區作示範）
CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE audit_logs_2026_04 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE audit_logs_2026_05 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

-- Step 5: 在每個分區建立複合索引（注意：分區索引需逐一建立）
CREATE INDEX ON audit_logs_2026_03 (user_id, created_at DESC);
CREATE INDEX ON audit_logs_2026_04 (user_id, created_at DESC);
CREATE INDEX ON audit_logs_2026_05 (user_id, created_at DESC);
CREATE INDEX ON audit_logs_2026_03 (hash);
CREATE INDEX ON audit_logs_2026_04 (hash);
CREATE INDEX ON audit_logs_2026_05 (hash);

-- Step 6: 還原歷史資料
INSERT INTO audit_logs SELECT * FROM audit_logs_legacy;
DROP TABLE audit_logs_legacy;
```

**Down migration**：`003_audit_partition.down.sql`

```sql
-- 合併回單表（僅供緊急回滾，不保留索引結構）
CREATE TABLE audit_logs_unified AS SELECT * FROM audit_logs;
DROP TABLE audit_logs;
ALTER TABLE audit_logs_unified RENAME TO audit_logs;
CREATE INDEX idx_audit_logs_user_id ON audit_logs (user_id);
CREATE INDEX idx_audit_logs_hash ON audit_logs (hash);
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at);
```

---

### 4.3 方案 C2：自動建立分區 + 保留策略（P1，低風險）

**目標**：每月自動建立下個月分區，並 DROP 超過保留期的舊分區。

**新增方法**：`internal/services/audit_service.go`

```go
// EnsureNextMonthPartition 建立下個月的分區（若不存在）。
// 應在每月 1 日由 cron job 或啟動時呼叫。
func (s *AuditService) EnsureNextMonthPartition(ctx context.Context) error {
    next := time.Now().AddDate(0, 1, 0)
    tableName := fmt.Sprintf("audit_logs_%d_%02d", next.Year(), int(next.Month()))
    start := time.Date(next.Year(), next.Month(), 1, 0, 0, 0, 0, time.UTC)
    end := start.AddDate(0, 1, 0)

    sql := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_logs
        FOR VALUES FROM ('%s') TO ('%s')`,
        tableName,
        start.Format("2006-01-02"),
        end.Format("2006-01-02"),
    )
    if err := s.db.WithContext(ctx).Exec(sql).Error; err != nil {
        return fmt.Errorf("ensure audit partition %s: %w", tableName, err)
    }

    // 建立索引（IF NOT EXISTS 確保冪等）
    for _, idx := range []string{
        fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_user_created ON %s (user_id, created_at DESC)", tableName, tableName),
        fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_hash ON %s (hash)", tableName, tableName),
    } {
        if err := s.db.WithContext(ctx).Exec(idx).Error; err != nil {
            return fmt.Errorf("create index on %s: %w", tableName, err)
        }
    }
    return nil
}

// DropOldPartitions 刪除超過保留期的分區。
// retainMonths = 90 表示保留近 3 個月的資料（DROP 更早的分區）。
func (s *AuditService) DropOldPartitions(ctx context.Context, retainMonths int) error {
    cutoff := time.Now().AddDate(0, -retainMonths, 0)
    tableName := fmt.Sprintf("audit_logs_%d_%02d", cutoff.Year(), int(cutoff.Month()))
    sql := fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)
    if err := s.db.WithContext(ctx).Exec(sql).Error; err != nil {
        return fmt.Errorf("drop audit partition %s: %w", tableName, err)
    }
    logger.Info("audit partition dropped", "table", tableName)
    return nil
}
```

**呼叫點**（建議在應用程式啟動時 + 每月 cron）：

```go
// router.go 或 main.go 啟動時
ctx := context.Background()
if err := auditSvc.EnsureNextMonthPartition(ctx); err != nil {
    logger.Warn("failed to ensure audit partition", "error", err)
}
```

---

### 4.4 方案 C3：非同步寫入（P3，需評估合規需求）

**目標**：HTTP 請求不阻塞於 `chainMu`，通過 buffered channel 非同步寫入。

**適用情境**：高並發 API 呼叫（>200 QPS），chainMu 成為可觀測瓶頸時才實施。

**⚠️ 注意**：非同步寫入在程序崩潰時可能丟失 channel buffer 中尚未 flush 的稽核記錄。若稽核合規要求（如 SOC 2、ISO 27001）不允許遺失，此方案需配合 WAL 持久化或改用同步方案。

**改動**：`internal/services/audit_service.go`

```go
type AuditService struct {
    db      *gorm.DB
    sink    AuditSink
    chainMu sync.Mutex
    queue   chan LogAuditRequest  // 非同步 buffer（1000 條）
    done    chan struct{}
}

func NewAuditService(db *gorm.DB) *AuditService {
    s := &AuditService{
        db:    db,
        sink:  NewDBSink(db),
        queue: make(chan LogAuditRequest, 1000),
        done:  make(chan struct{}),
    }
    go s.drainQueue()
    return s
}

// Shutdown 優雅關閉：等待 queue 排盡後退出
func (s *AuditService) Shutdown(ctx context.Context) {
    close(s.queue)
    select {
    case <-s.done:
    case <-ctx.Done():
        logger.Warn("audit shutdown timeout: some entries may be lost")
    }
}

func (s *AuditService) drainQueue() {
    for req := range s.queue {
        s.writeChained(req)  // 原有 chainMu 邏輯不變
    }
    close(s.done)
}

// LogAuditAsync 非同步版本，不阻塞請求 goroutine。
// 若 queue 滿，記錄 warning 並丟棄（保護系統不因日誌積壓而崩潰）。
func (s *AuditService) LogAuditAsync(req LogAuditRequest) {
    select {
    case s.queue <- req:
    default:
        logger.Warn("audit queue full: entry dropped",
            "action", req.Action,
            "user_id", req.UserID,
        )
    }
}
```

---

## 5. 優先順序與里程碑

### Phase 1（✅ 完成）

| 任務 | 檔案 | 狀態 |
|------|------|------|
| Informer 閒置 GC（方案 A1） | `internal/k8s/manager.go` | ✅ 已存在，`StartGC(30min, 2h)` 已在 router.go 啟動 |
| Informer LRU 硬上限（方案 A2） | `internal/k8s/manager.go`、`internal/router/router.go` | ✅ `SetMaxActiveClusters(50)` + `evictLRU()`，4 個單元測試通過 |
| Terminal 全局 semaphore（方案 B1） | `internal/handlers/pod_terminal_handler.go`、`kubectl_terminal_handler.go` | ✅ `sem chan struct{}`（容量 200），WS 升級前非阻塞 select |
| Terminal 每用戶上限（方案 B2） | 同上 + `pod_terminal_ws.go`、`kubectl_terminal_ws.go` | ✅ `maxPerUser=5`，WS 升級前計數檢查，`UserID` 存入 session |
| Terminal 閒置超時（方案 B3） | 同上 + `pod_terminal_helpers.go` | ✅ `idleCleanup()` goroutine（2min 掃描，15min 超時），`lastActivityAt` 在 handleInput 更新 |
| `response.TooManyRequests` | `internal/response/response.go` | ✅ 新增 HTTP 429 helper |

**Phase 1 效果**：叢集上限 ~20 → **~50+ active**（GC 後實際無硬限）；Terminal 上限 ~50 → **200 全局 / 5 per-user；閒置 15 分鐘自動釋放**

**新增測試**：`internal/k8s/health_test.go`（4 個 LRU 測試）、`internal/handlers/terminal_limits_test.go`（11 個 semaphore/limit/idle 測試）

### Phase 2（✅ 完成）

| 任務 | 檔案 | 狀態 |
|------|------|------|
| Audit log 月分區 migration（方案 C1） | `internal/database/migrations/postgres/003_audit_partition.up.sql` + `.down.sql` | ✅ RANGE 分區 + DEFAULT 分區 + parent-level 索引（自動傳播至子分區） |
| 自動建立分區 + 保留策略（方案 C2） | `internal/services/audit_service.go` | ✅ `EnsureNextMonthPartition(ctx)` + `DropOldPartitions(ctx, retainMonths)` + 啟動時非同步呼叫（router.go） |

**Phase 2 效果**：稽核日誌查詢效能穩定（partition pruning）；磁碟用量可控（按月 DROP 舊分區）；parent-level 索引自動傳播，無需每月手動建立索引

**新增測試**：`internal/services/audit_partition_test.go`（12 個測試，含 sqlmock SQL 驗證、nil-DB no-op、error 傳播、月份命名格式）

### Phase 3（視需求評估，P2–P3）

| 任務 | 檔案 | 前置條件 |
|------|------|----------|
| Informer Namespace 範圍限制（方案 A3） | `internal/k8s/manager.go`、`internal/models/cluster.go`、前端 | Phase 1 完成後觀察記憶體是否仍不足 |
| Audit 非同步寫入（方案 C3） | `internal/services/audit_service.go` | 合規需求評估通過；可觀測到 chainMu 成為瓶頸 |

---

## 6. 風險與取捨

### 方案 A1/A2（Informer GC）
- **風險**：GC 後下次存取需重新 warm up（約 5–30 秒，視叢集規模）。用戶第一次存取閒置叢集時會有延遲。
- **緩解**：在 UI 顯示「叢集連線中…」loading 狀態；`syncTimeout` 可調整（目前 30s）。

### 方案 B1/B2（Terminal 限制）
- **風險**：現有用戶可能注意到新的限制，特別是 per-user 5 個上限對重度用戶可能不夠。
- **緩解**：上限可改為從 `config` 或 admin 設定讀取；初始值設保守（5），觀察後調整。

### 方案 C1（Audit 分區）
- **風險**：Migration 為破壞性操作（DROP + 重建），**必須在維護視窗執行並提前備份**。
- **緩解**：down migration 已準備好；建議在 staging 環境完整跑過後才上 production。
- **hash chain 相容性**：分區不影響 chain 邏輯（`prev_hash` 基於 `id` 排序），月份跨越時 chain 自然延續。

### 方案 C3（Audit 非同步）
- **強烈建議僅在確認以下條件後實施**：
  1. Prometheus 指標顯示 `audit_write_latency_p99 > 50ms` 且成為 API 瓶頸
  2. 法務/合規確認允許極端情況下（程序崩潰）丟失少量稽核記錄
  3. 實作 `Shutdown()` 確保優雅關閉時 queue 排盡

---

*文件版本：v1.2 — Phase 1 + Phase 2 完成*

// Grafana Dashboard 和 Panel 配置
// ⚠️ 請根據實際情況修改 UID 和 Panel ID

export const GRAFANA_CONFIG = {
  cluster: {
    // 使用專案自帶的叢集概覽 Dashboard
    dashboardUid: 'synapse-cluster-overview',
    panels: {
      // 叢集資源總量
      cpuTotal: 85,
      cpuUsed: 87,
      memoryTotal: 89,
      memoryUsed: 90,
      podCapacity: 91,
      podRunning: 92,
      // 叢集狀態
      nodeReady: 63,
      nodeNotReady: 65,
      podHealthy: 69,
      // 資源使用趨勢
      cpuUsageTrend: 2,
      memoryUsageTrend: 4,
      // 網路
      networkReceive: 73,
      networkTransmit: 75,
    },
  },
  node: {
    // ⚠️ 修改為你匯入的節點 Dashboard UID
    dashboardUid: 'node-exporter-full',
    panels: {
      cpuUsage: 2,
      memoryUsage: 4,
      diskUsage: 6,
      networkIO: 8,
      loadAverage: 10,
    },
  },
  pod: {
    // ⚠️ 修改為你匯入的 Pod Dashboard UID
    // 推薦使用 Dashboard ID: 6417 (Kubernetes Pod Monitoring)
    dashboardUid: 'k8s-pod-monitoring',
    panels: {
      cpuUsage: 2,
      memoryUsage: 4,
      networkTraffic: 6,
      containerRestarts: 8,
      cpuThrottling: 10,
      diskIO: 12,
    },
  },
  workload: {
    // ⚠️ 修改為你匯入的工作負載 Dashboard UID
    dashboardUid: 'k8s-workload-monitoring',
    panels: {
      cpuUsageMulti: 2,
      memoryUsageMulti: 4,
      podStatus: 6,
      replicaCount: 8,
      restartCount: 10,
    },
  },
  // 工作負載詳情監控 Dashboard 配置
  workloadDetail: {
    dashboardUid: 'synapse-workload-detail',
    panels: {
      // 資源使用
      cpuUsage: 2,              // CPU 使用率
      memoryUsage: 6,           // Memory 使用率
      ioReadQps: 18,            // IO Read QPS
      ioWriteQps: 19,           // IO Write QPS
      // 容器狀態
      cpuLimit: 28,             // CPU 核限制
      memoryLimit: 30,          // 記憶體限制
      availability: 34,         // 容器整體可用率
      healthCheckFailed: 36,    // 健康檢查失敗次數
      containerRestarts: 38,    // 容器重啟情況
      // 網路流量
      networkIncoming: 4,       // Network Incoming
      networkOutgoing: 14,      // Network Outgoing
      networkInputPps: 15,      // Network Input PPS
      networkOutputPps: 16,     // Network Output PPS
      // 系統資源
      fileDescriptors: 22,      // 檔案控制代碼開啟數
      runningThreads: 23,       // Running Threads
      networkInputDropped: 12,  // Network Input Dropped
      networkOutputDropped: 20, // Network Output Dropped
      // CPU 限流
      cpuThrottleRate: 46,      // CPU限流比例
      cpuThrottleTime: 32,      // CPU節流時間
    },
  },
  // Pod 詳情監控 Dashboard 配置（按容器維度展示）
  podDetail: {
    dashboardUid: 'synapse-pod-detail',
    panels: {
      // 資源使用（按容器）
      cpuUsage: 2,              // CPU 使用率 (按容器)
      memoryUsage: 6,           // Memory 使用率 (按容器)
      ioRead: 18,               // IO Read (按容器)
      ioWrite: 19,              // IO Write (按容器)
      // 容器狀態
      cpuLimit: 28,             // CPU 核限制
      memoryLimit: 30,          // 記憶體限制
      containerRestarts: 38,    // 容器重啟次數 (stat)
      healthCheckFailed: 36,    // 健康檢查失敗次數
      containerRestartsChart: 39, // 容器重啟情況 (按容器)
      // 網路流量
      networkIncoming: 4,       // Network Incoming
      networkOutgoing: 14,      // Network Outgoing
      networkInputPps: 15,      // Network Input PPS
      networkOutputPps: 16,     // Network Output PPS
      // 系統資源
      fileDescriptors: 22,      // 檔案控制代碼開啟數 (按容器)
      runningThreads: 23,       // Running Threads (按容器)
      networkInputDropped: 12,  // Network Input Dropped
      networkOutputDropped: 20, // Network Output Dropped
      // CPU 限流
      cpuThrottleRate: 46,      // CPU限流比例 (按容器)
      cpuThrottleTime: 32,      // CPU節流時間 (按容器)
    },
  },
};

// 時間範圍對映
export const TIME_RANGE_MAP: Record<string, { from: string; to: string }> = {
  '1h': { from: 'now-1h', to: 'now' },
  '6h': { from: 'now-6h', to: 'now' },
  '24h': { from: 'now-24h', to: 'now' },
  '7d': { from: 'now-7d', to: 'now' },
};

// 根據叢集名生成 Grafana 資料來源 UID
// 與後端 GenerateDataSourceUID 保持一致
export const generateDataSourceUID = (clusterName: string): string => {
  const uid = clusterName.toLowerCase().replace(/[_ ]/g, '-');
  return `prometheus-${uid}`;
};

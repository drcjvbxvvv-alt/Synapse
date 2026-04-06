/**
 * 資料新鮮度設定
 *
 * staleTime：React Query 快取保鮮時間（毫秒）
 * POLL_INTERVALS：各資源頁面的自動輪詢間隔（毫秒）
 *
 * 原則：
 *   - 即時性敏感（Pod/Node 狀態）→ 短間隔
 *   - 低頻變動（ConfigMap/Secret/使用者）→ 長 staleTime，不輪詢
 *   - 所有數值集中於此，禁止在頁面中散落魔術數字
 */

/** React Query staleTime（不輪詢頁面使用） */
export const STALE_TIMES = {
  /** Pod / Node 狀態 */
  realtime: 5_000,
  /** Deployment / StatefulSet 列表 */
  workload: 15_000,
  /** Helm / CRD / Alert 等中頻資源 */
  default: 30_000,
  /** ConfigMap / Secret / 使用者 / 權限等低頻資源 */
  slow: 120_000,
} as const;

/** setInterval 輪詢間隔（手動 fetch 頁面使用，單位 ms） */
export const POLL_INTERVALS = {
  /** Pod 列表 */
  pod: 5_000,
  /** Node 列表 */
  node: 10_000,
  /** Deployment / StatefulSet / DaemonSet 等工作負載列表 */
  workload: 15_000,
  /** Overview 儀表板 */
  overview: 30_000,
} as const;

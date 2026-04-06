/**
 * 與當前頁面同源（含連接埠）的 WebSocket 基址，適用於：
 * - 單二進位制 / Nginx 反代（443 與頁面一致）
 * - 自定義 SERVER_PORT
 * - 開發：Vite 將 /ws 代理到後端，使用 dev server 的 host:5173
 */
export function buildWebSocketUrl(pathWithQuery: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const path = pathWithQuery.startsWith('/') ? pathWithQuery : `/${pathWithQuery}`;
  return `${protocol}//${window.location.host}${path}`;
}

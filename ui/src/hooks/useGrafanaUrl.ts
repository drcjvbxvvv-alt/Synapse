import { useState, useEffect } from 'react';
import { systemSettingService } from '../services/authService';

let cachedUrl: string | null = null;

/**
 * 獲取系統設定中配置的 Grafana 地址，用於 iframe 直連。
 * 全域性快取，避免重複請求。
 */
export function useGrafanaUrl(): { grafanaUrl: string; loading: boolean } {
  const [url, setUrl] = useState(cachedUrl ?? '');
  const [loading, setLoading] = useState(cachedUrl === null);

  useEffect(() => {
    if (cachedUrl !== null) return;

    systemSettingService.getGrafanaConfig()
      .then((res) => {
        if (res?.url) {
          const u = res.url.replace(/\/+$/, '');
          cachedUrl = u;
          setUrl(u);
        } else {
          cachedUrl = '';
          setUrl('');
        }
      })
      .catch(() => {
        cachedUrl = '';
        setUrl('');
      })
      .finally(() => setLoading(false));
  }, []);

  return { grafanaUrl: url, loading };
}

export function invalidateGrafanaUrlCache() {
  cachedUrl = null;
}

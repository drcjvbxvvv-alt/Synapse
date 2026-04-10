import axios from 'axios';
import type { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios';
import { tokenManager, silentRefresh } from '../services/authService';

// P1-6：Tiered timeouts
// GET  → 60 000 ms (handles both list and detail)
// POST / PUT / DELETE / PATCH → 45 000 ms
const TIMEOUT_GET      = 60_000;
const TIMEOUT_MUTATION = 45_000;

const api: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: TIMEOUT_GET,   // default; overridden per-method below
  headers: {
    'Content-Type': 'application/json',
  },
});

// P0-6：Access Token 從記憶體中讀取（不再觸碰 localStorage）
// P1-6：Apply tiered timeout based on HTTP method
api.interceptors.request.use(
  (config) => {
    const token = tokenManager.getToken();
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }

    // Override timeout only when the caller has not already set one explicitly
    if (config.timeout === undefined || config.timeout === TIMEOUT_GET) {
      const method = (config.method ?? 'get').toLowerCase();
      if (method !== 'get' && method !== 'head') {
        config.timeout = TIMEOUT_MUTATION;
      } else {
        config.timeout = TIMEOUT_GET;
      }
    }

    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 標記是否正在 refresh，防止多個 401 同時觸發多次 refresh
let isRefreshing = false;
let refreshQueue: Array<(token: string | null) => void> = [];

api.interceptors.response.use(
  (response: AxiosResponse) => response,
  async (error) => {
    const originalRequest = error.config;

    if (error.response?.status === 401 && !originalRequest._retried) {
      const requestUrl = originalRequest?.url || '';
      // 這些路由本身就是認證路由，401 直接導登入，不嘗試 refresh
      const skipRefreshUrls = ['/auth/login', '/auth/refresh', '/auth/change-password'];
      if (skipRefreshUrls.some(u => requestUrl.includes(u))) {
        return Promise.reject(error);
      }

      originalRequest._retried = true;

      // 若已在 refreshing，排隊等待
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          refreshQueue.push((token) => {
            if (token) {
              originalRequest.headers.Authorization = `Bearer ${token}`;
              resolve(api(originalRequest));
            } else {
              reject(error);
            }
          });
        });
      }

      isRefreshing = true;
      const ok = await silentRefresh();
      isRefreshing = false;

      if (ok) {
        const newToken = tokenManager.getToken();
        // 通知排隊中的請求
        refreshQueue.forEach(cb => cb(newToken));
        refreshQueue = [];
        // 重試原始請求
        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        return api(originalRequest);
      } else {
        // refresh 失敗，清除狀態並導向登入
        refreshQueue.forEach(cb => cb(null));
        refreshQueue = [];
        tokenManager.clear();
        if (!window.location.pathname.includes('/login')) {
          window.location.href = '/login';
        }
      }
    }

    return Promise.reject(error);
  }
);

export const request = {
  get: <T>(url: string, config?: AxiosRequestConfig): Promise<T> =>
    api.get(url, config).then(res => res.data),
  
  post: <T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> =>
    api.post(url, data, config).then(res => res.data),
  
  put: <T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> =>
    api.put(url, data, config).then(res => res.data),
  
  delete: <T>(url: string, config?: AxiosRequestConfig): Promise<T> =>
    api.delete(url, config).then(res => res.data),
  
  patch: <T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> =>
    api.patch(url, data, config).then(res => res.data),
};

export function parseApiError(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data;
    if (data?.error?.message) {
      return data.error.message;
    }
    if (data?.message) {
      return data.message;
    }
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return '未知錯誤';
}

export default api;

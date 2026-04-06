import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import './index.css'
// 初始化 i18n（必須在 App 之前匯入）
import './i18n'
import App from './App.tsx'
import { STALE_TIMES } from './config/queryConfig'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: STALE_TIMES.default,
      retry: 1,
      refetchOnWindowFocus: false,
    },
    mutations: {
      onError: (error: unknown) => {
        // 全域 mutation 錯誤記錄（各頁面仍可自行覆蓋）
        console.error('[mutation error]', error);
      },
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={queryClient}>
    <App />
  </QueryClientProvider>
)

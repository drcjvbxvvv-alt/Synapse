import { createRoot } from 'react-dom/client'
import './index.css'
// 初始化 i18n（必须在 App 之前导入）
import './i18n'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
    <App />
)

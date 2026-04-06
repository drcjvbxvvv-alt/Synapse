import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import zhTW from '../locales/zh-TW';
import enUS from '../locales/en-US';

// 支援的語言列表
export const supportedLanguages = [
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
];

// 預設語言
export const defaultLanguage = 'zh-TW';

i18n
  // 自動偵測使用者語言
  .use(LanguageDetector)
  // 將 i18n 例項傳遞給 react-i18next
  .use(initReactI18next)
  // 初始化 i18next
  .init({
    resources: {
      'zh-TW': zhTW,
      'en-US': enUS,
    },
    fallbackLng: defaultLanguage,
    defaultNS: 'common',
ns: ['common', 'cluster', 'node', 'pod', 'overview', 'workload', 'namespace', 'yaml', 'search', 'terminal', 'storage', 'permission', 'nodeOps', 'settings', 'profile', 'om', 'plugins', 'logs', 'audit', 'alert', 'network', 'config', 'components', 'helm', 'cost', 'security'],
// 語言檢測選項
    detection: {
      // 檢測順序
      order: ['localStorage', 'navigator', 'htmlTag'],
      // 快取使用者語言選擇
      caches: ['localStorage'],
      // localStorage 鍵名
      lookupLocalStorage: 'synapse-language',
    },
    
    interpolation: {
      // React 已經處理了 XSS 防護
      escapeValue: false,
    },
    
    react: {
      // 等待翻譯載入完成
      useSuspense: true,
    },
  });

export default i18n;

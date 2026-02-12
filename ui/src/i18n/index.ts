import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import zhCN from '../locales/zh-CN';
import enUS from '../locales/en-US';

// 支持的语言列表
export const supportedLanguages = [
  { code: 'zh-CN', name: '简体中文', flag: '🇨🇳' },
  { code: 'en-US', name: 'English', flag: '🇺🇸' },
];

// 默认语言
export const defaultLanguage = 'zh-CN';

i18n
  // 自动检测用户语言
  .use(LanguageDetector)
  // 将 i18n 实例传递给 react-i18next
  .use(initReactI18next)
  // 初始化 i18next
  .init({
    resources: {
      'zh-CN': zhCN,
      'en-US': enUS,
    },
    fallbackLng: defaultLanguage,
    defaultNS: 'common',
ns: ['common', 'cluster', 'node', 'pod', 'overview', 'workload', 'namespace', 'yaml', 'search', 'terminal', 'storage', 'permission', 'nodeOps', 'settings', 'profile', 'om', 'plugins', 'logs', 'audit', 'alert', 'network', 'config', 'components'],
// 语言检测选项
    detection: {
      // 检测顺序
      order: ['localStorage', 'navigator', 'htmlTag'],
      // 缓存用户语言选择
      caches: ['localStorage'],
      // localStorage 键名
      lookupLocalStorage: 'kubepolaris-language',
    },
    
    interpolation: {
      // React 已经处理了 XSS 防护
      escapeValue: false,
    },
    
    react: {
      // 等待翻译加载完成
      useSuspense: true,
    },
  });

export default i18n;

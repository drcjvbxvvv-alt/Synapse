import React, { Suspense, useEffect, useState } from 'react';
import { BrowserRouter as Router } from 'react-router-dom';
import { ConfigProvider, App as AntdApp, Spin } from 'antd';
import { useTranslation } from 'react-i18next';
import zhTW from 'antd/locale/zh_TW';
import enUS from 'antd/locale/en_US';
import { synapseTheme } from './config/theme';
import { tokenManager, silentRefresh } from './services/authService';
import { useSessionStore } from './store';
import ErrorBoundary from './components/ErrorBoundary';
import { AppRoutes } from './router/routes';
import './App.css';

// ── Ant Design locale map ──────────────────────────────────────────────────

const antdLocaleMap: Record<string, typeof zhTW> = {
  'zh-TW': zhTW,
  'en-US': enUS,
};

// ── Auth bootstrap ─────────────────────────────────────────────────────────
//
// On page refresh the in-memory accessToken is null. If localStorage has a
// stored session, attempt a silent refresh via the httpOnly refresh-token cookie.
// Show a full-page spinner during the attempt so RequireAuth doesn't
// incorrectly redirect to /login.

const useAuthInit = () => {
  const hasStoredSession = !!localStorage.getItem('user');
  const alreadyHasToken  = tokenManager.isLoggedIn();
  const setSession = useSessionStore((s) => s.setSession);

  const [authReady, setAuthReady] = useState(alreadyHasToken || !hasStoredSession);

  // Hydrate session store when a token already exists in memory
  useEffect(() => {
    if (alreadyHasToken) {
      const user = tokenManager.getUser();
      const expiresAt = tokenManager.getExpiresAt();
      if (user) setSession(user, expiresAt ?? 0);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (authReady) return;

    silentRefresh().then((ok) => {
      if (ok) {
        const user = tokenManager.getUser();
        const expiresAt = tokenManager.getExpiresAt();
        if (user) setSession(user, expiresAt ?? 0);
      }
    }).finally(() => {
      setAuthReady(true);
    });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return authReady;
};

// ── AppContent (needs i18n + auth) ─────────────────────────────────────────

const AppContent: React.FC = () => {
  const { i18n } = useTranslation();
  const currentLocale = antdLocaleMap[i18n.language] || zhTW;
  const authReady = useAuthInit();

  if (!authReady) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <ConfigProvider locale={currentLocale} theme={synapseTheme}>
      <AntdApp>
        <Router>
          <ErrorBoundary>
            <AppRoutes />
          </ErrorBoundary>
        </Router>
      </AntdApp>
    </ConfigProvider>
  );
};

// ── App root (Suspense for i18n loading) ───────────────────────────────────

const App: React.FC = () => {
  return (
    <Suspense fallback={
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    }>
      <AppContent />
    </Suspense>
  );
};

export default App;

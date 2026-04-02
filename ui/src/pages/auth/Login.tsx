import React, { useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import kubernetesLogo from '../../assets/kubernetes.png';
import { Form, Input, Button, Typography, Tabs, Space, App } from 'antd';
import {
  UserOutlined,
  LockOutlined,
  LoginOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  MonitorOutlined,
  SafetyCertificateOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import { authService, tokenManager } from '../../services/authService';
import { parseApiError } from '@/utils/api';

const { Text } = Typography;

interface LoginFormValues {
  username: string;
  password: string;
}

const isDev = import.meta.env.DEV;

const Login: React.FC = () => {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useTranslation('common');
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [ldapEnabled, setLdapEnabled] = useState(false);
  const [activeTab, setActiveTab] = useState<'local' | 'ldap'>('local');

  const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/';

  useEffect(() => {
    if (tokenManager.isLoggedIn()) navigate(from, { replace: true });
  }, [navigate, from]);

  useEffect(() => {
    authService.getAuthStatus()
      .then(r => setLdapEnabled(r.ldap_enabled))
      .catch(() => {});
  }, []);

  const handleLogin = async (values: LoginFormValues) => {
    setLoading(true);
    try {
      const response = await authService.login({
        username: values.username,
        password: values.password,
        auth_type: activeTab,
      });
      tokenManager.setToken(response.token);
      tokenManager.setUser(response.user);
      tokenManager.setExpiresAt(response.expires_at);
      if (response.permissions) tokenManager.setPermissions(response.permissions);
      message.success(t('auth.loginSuccess'));
      navigate(from, { replace: true });
    } catch (error: unknown) {
      message.error(parseApiError(error) || t('messages.networkError'));
    } finally {
      setLoading(false);
    }
  };

  const tabItems = [
    { key: 'local', label: <Space><UserOutlined />{t('auth.passwordLogin')}</Space> },
    ...(ldapEnabled ? [{ key: 'ldap', label: <Space><CloudServerOutlined />{t('auth.ldapLogin')}</Space> }] : []),
  ];

  const features = [
    { icon: <ClusterOutlined />, title: t('auth.featureMultiClusterTitle'), desc: t('auth.featureMultiClusterDesc') },
    { icon: <MonitorOutlined />, title: t('auth.featureObservabilityTitle'), desc: t('auth.featureObservabilityDesc') },
    { icon: <SafetyCertificateOutlined />, title: t('auth.featureRBACTitle'), desc: t('auth.featureRBACDesc') },
    { icon: <CodeOutlined />, title: t('auth.featureGitOpsTitle'), desc: t('auth.featureGitOpsDesc') },
  ];

  return (
    <main className="min-h-screen flex bg-[var(--color-bg-page)] dark:bg-slate-900">

      {/* ── Left: Brand Panel ── */}
      <section
        aria-hidden="true"
        className="hidden lg:flex lg:w-[52%] relative flex-col justify-center overflow-hidden
                   bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900"
      >
        {/* Subtle grid */}
        <div
          className="absolute inset-0 opacity-[0.03]"
          style={{ backgroundImage: 'linear-gradient(#fff 1px,transparent 1px),linear-gradient(90deg,#fff 1px,transparent 1px)', backgroundSize: '40px 40px' }}
        />
        {/* Glow spots */}
        <div className="absolute top-1/4 left-1/4 w-80 h-80 rounded-full bg-blue-600/20 blur-3xl" />
        <div className="absolute bottom-1/4 right-1/4 w-64 h-64 rounded-full bg-indigo-500/15 blur-3xl" />

        <div className="relative z-10 px-14 py-16 max-w-lg">
          {/* Logo */}
          <div className="flex items-center gap-3 mb-12">
            <img src={kubernetesLogo} alt="" className="w-10 h-10" />
            <span className="text-white text-2xl font-bold tracking-wide">Synapse</span>
          </div>

          <h2 className="text-white text-3xl font-semibold leading-snug mb-4">
            {t('auth.brandHeadline')}
          </h2>
          <p className="text-slate-400 text-base leading-relaxed mb-10">
            {t('auth.brandDesc')}
          </p>

          <ul className="space-y-5">
            {features.map((f, i) => (
              <li key={i} className="flex items-start gap-4">
                <div className="mt-0.5 w-9 h-9 flex items-center justify-center rounded-lg
                                bg-white/10 text-blue-400 text-base flex-shrink-0">
                  {f.icon}
                </div>
                <div>
                  <h4 className="text-white text-sm font-semibold mb-0.5">{f.title}</h4>
                  <p className="text-slate-400 text-xs leading-relaxed">{f.desc}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </section>

      {/* ── Right: Form Panel ── */}
      <section className="flex-1 flex flex-col items-center justify-center px-6 py-12
                          bg-[var(--color-bg-page)] dark:bg-slate-900">
        <div className="w-full max-w-sm">
          {/* Mobile logo */}
          <div className="flex items-center gap-2 mb-8 lg:hidden">
            <img src={kubernetesLogo} alt="" className="w-8 h-8" />
            <span className="text-[var(--color-text-primary)] font-bold text-lg">Synapse</span>
          </div>

          <div className="mb-8">
            <h1 className="text-[var(--color-text-primary)] text-2xl font-semibold mb-1">
              {t('auth.welcomeBack')}
            </h1>
            <p className="text-[var(--color-text-secondary)] text-sm">
              {t('auth.loginSubtitle')}
            </p>
          </div>

          {ldapEnabled && (
            <Tabs
              activeKey={activeTab}
              onChange={(key) => setActiveTab(key as 'local' | 'ldap')}
              items={tabItems}
              centered
              className="mb-6"
            />
          )}

          <Form
            form={form}
            onFinish={handleLogin}
            layout="vertical"
            requiredMark={false}
          >
            <Form.Item
              name="username"
              label={t('auth.username')}
              rules={[{ required: true, message: t('auth.usernameRequired') }]}
            >
              <Input
                prefix={<UserOutlined className="text-slate-400" aria-hidden="true" />}
                placeholder={`${t('auth.username')}…`}
                size="large"
                autoComplete="username"
                spellCheck={false}
                autoFocus
              />
            </Form.Item>

            <Form.Item
              name="password"
              label={t('auth.password')}
              rules={[{ required: true, message: t('auth.passwordRequired') }]}
              className="!mb-7"
            >
              <Input.Password
                prefix={<LockOutlined className="text-slate-400" aria-hidden="true" />}
                placeholder={`${t('auth.password')}…`}
                size="large"
                autoComplete="current-password"
                spellCheck={false}
              />
            </Form.Item>

            <Form.Item className="!mb-0">
              <Button
                type="primary"
                htmlType="submit"
                size="large"
                block
                loading={loading}
                icon={<LoginOutlined />}
              >
                {t('auth.login')}
              </Button>
            </Form.Item>
          </Form>

          {isDev && (
            <div className="mt-5 px-4 py-3 rounded-lg bg-amber-50 dark:bg-amber-900/20
                            border border-amber-200 dark:border-amber-700/50">
              <Text className="!text-amber-700 dark:!text-amber-400 text-xs">
                {activeTab === 'ldap' ? t('auth.ldapHint') : t('auth.defaultAdminHint')}
              </Text>
            </div>
          )}
        </div>

        <footer className="mt-12 text-center">
          <Text className="!text-[var(--color-text-muted)] text-xs">{t('auth.copyright')}</Text>
        </footer>
      </section>
    </main>
  );
};

export default Login;

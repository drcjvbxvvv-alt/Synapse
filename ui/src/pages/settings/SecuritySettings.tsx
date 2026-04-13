import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Form,
  InputNumber,
  Button,
  Space,
  Typography,
  Divider,
  Table,
  Tag,
  Modal,
  Input,
  DatePicker,
  Checkbox,
  Alert,
  App,
  Spin,
  Popconfirm,
} from 'antd';
import {
  SafetyCertificateOutlined,
  KeyOutlined,
  PlusOutlined,
  DeleteOutlined,
  CopyOutlined,
  LockOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { usePermission } from '../../hooks/usePermission';
import { securitySettingService } from '../../services/securitySettingService';
import type { SecurityConfig, APIToken, CreateAPITokenResponse } from '../../types';
import SIEMConfigPage from './SIEMConfig';

const { Text, Paragraph } = Typography;

const SCOPES = ['read', 'write', 'admin'];

const SecuritySettings: React.FC = () => {
  const { t } = useTranslation(['settings', 'common']);
  const { message } = App.useApp();
  const { canDelete } = usePermission();

  // ── Login Security State ──────────────────────────────────────────────────
  const [secForm] = Form.useForm<SecurityConfig>();
  const [secLoading, setSecLoading] = useState(true);
  const [secSaving, setSecSaving] = useState(false);

  // ── API Token State ────────────────────────────────────────────────────────
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [tokensLoading, setTokensLoading] = useState(true);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [createForm] = Form.useForm();
  const [creating, setCreating] = useState(false);
  const [newTokenResult, setNewTokenResult] = useState<CreateAPITokenResponse | null>(null);
  const [resultModalOpen, setResultModalOpen] = useState(false);

  // ── Load Data ─────────────────────────────────────────────────────────────
  useEffect(() => {
    securitySettingService.getSecurityConfig()
      .then(cfg => secForm.setFieldsValue(cfg))
      .catch(() => message.error(t('settings:security.loadConfigFailed')))
      .finally(() => setSecLoading(false));
  }, [secForm, message, t]);

  const loadTokens = useCallback(() => {
    setTokensLoading(true);
    securitySettingService.listAPITokens()
      .then(setTokens)
      .catch(() => message.error(t('settings:security.loadTokensFailed')))
      .finally(() => setTokensLoading(false));
  }, [t, message]);

  useEffect(() => {
    loadTokens();
  }, [loadTokens]);

  // ── Security Config Handlers ───────────────────────────────────────────────
  const handleSecSave = async () => {
    const values = await secForm.validateFields();
    setSecSaving(true);
    try {
      await securitySettingService.updateSecurityConfig(values);
      message.success(t('settings:security.saveConfigSuccess'));
    } catch {
      message.error(t('settings:security.saveConfigFailed'));
    } finally {
      setSecSaving(false);
    }
  };

  // ── API Token Handlers ─────────────────────────────────────────────────────
  const handleCreateToken = async () => {
    const values = await createForm.validateFields();
    setCreating(true);
    try {
      const result = await securitySettingService.createAPIToken({
        name: values.name,
        scopes: values.scopes ?? [],
        expires_at: values.expires_at
          ? dayjs(values.expires_at).toISOString()
          : undefined,
      });
      setNewTokenResult(result);
      setCreateModalOpen(false);
      setResultModalOpen(true);
      createForm.resetFields();
      loadTokens();
    } catch {
      message.error(t('settings:security.createTokenFailed'));
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteToken = async (id: number) => {
    try {
      await securitySettingService.deleteAPIToken(id);
      message.success(t('settings:security.revokeTokenSuccess'));
      loadTokens();
    } catch {
      message.error(t('settings:security.revokeTokenFailed'));
    }
  };

  const handleCopyToken = (token: string) => {
    navigator.clipboard.writeText(token)
      .then(() => message.success(t('settings:security.tokenCopied')))
      .catch(() => message.error(t('settings:security.tokenCopyFailed')));
  };

  // ── Token Table Columns ────────────────────────────────────────────────────
  const tokenColumns: ColumnsType<APIToken> = [
    {
      title: t('settings:security.tokenName'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: t('settings:security.tokenScopes'),
      dataIndex: 'scopes',
      key: 'scopes',
      render: (scopes: string[]) =>
        scopes.length > 0
          ? scopes.map(s => (
              <Tag key={s} color={s === 'admin' ? 'red' : s === 'write' ? 'orange' : 'blue'}>
                {s}
              </Tag>
            ))
          : <Text type="secondary">—</Text>,
    },
    {
      title: t('settings:security.tokenCreatedAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: t('settings:security.tokenExpiresAt'),
      dataIndex: 'expires_at',
      key: 'expires_at',
      render: (v?: string) => v ? dayjs(v).format('YYYY-MM-DD') : <Text type="secondary">—</Text>,
    },
    {
      title: t('settings:security.tokenLastUsed'),
      dataIndex: 'last_used_at',
      key: 'last_used_at',
      render: (v?: string) => v ? dayjs(v).format('YYYY-MM-DD HH:mm') : <Text type="secondary">—</Text>,
    },
    {
      title: t('common:actions'),
      key: 'actions',
      render: (_: unknown, record: APIToken) => (
        canDelete() ? (
          <Popconfirm
            title={t('settings:security.revokeTokenConfirm')}
            onConfirm={() => handleDeleteToken(record.id)}
            okText={t('common:confirm')}
            cancelText={t('common:cancel')}
          >
            <Button danger icon={<DeleteOutlined />} size="small">
              {t('settings:security.revokeToken')}
            </Button>
          </Popconfirm>
        ) : null
      ),
    },
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">

      {/* Section 1: SIEM / 稽核日誌 */}
      <Card
        title={
          <Space>
            <SafetyCertificateOutlined />
            {t('settings:security.siemTitle')}
          </Space>
        }
      >
        <SIEMConfigPage />
      </Card>

      {/* Section 2: 登入安全設定 */}
      <Card
        title={
          <Space>
            <LockOutlined />
            {t('settings:security.loginSecurityTitle')}
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 24 }}>
          {t('settings:security.loginSecurityDesc')}
        </Text>

        {secLoading ? (
          <div style={{ textAlign: 'center', padding: 32 }}><Spin /></div>
        ) : (
          <Form form={secForm} layout="vertical">
            <Divider>{t('settings:security.sessionConfig')}</Divider>
            <Form.Item
              name="session_ttl_minutes"
              label={t('settings:security.sessionTTL')}
              tooltip={t('settings:security.sessionTTLTooltip')}
              rules={[{ required: true }]}
            >
              <InputNumber min={1} max={10080} addonAfter={t('settings:security.minutes')} style={{ width: 240 }} />
            </Form.Item>

            <Divider>{t('settings:security.lockConfig')}</Divider>
            <Form.Item
              name="login_fail_lock_threshold"
              label={t('settings:security.lockThreshold')}
              tooltip={t('settings:security.lockThresholdTooltip')}
              rules={[{ required: true }]}
            >
              <InputNumber min={1} max={100} addonAfter={t('settings:security.times')} style={{ width: 240 }} />
            </Form.Item>
            <Form.Item
              name="lock_duration_minutes"
              label={t('settings:security.lockDuration')}
              tooltip={t('settings:security.lockDurationTooltip')}
              rules={[{ required: true }]}
            >
              <InputNumber min={1} max={1440} addonAfter={t('settings:security.minutes')} style={{ width: 240 }} />
            </Form.Item>

            <Divider>{t('settings:security.passwordConfig')}</Divider>
            <Form.Item
              name="password_min_length"
              label={t('settings:security.passwordMinLength')}
              tooltip={t('settings:security.passwordMinLengthTooltip')}
              rules={[{ required: true }]}
            >
              <InputNumber min={6} max={64} addonAfter={t('settings:security.chars')} style={{ width: 240 }} />
            </Form.Item>

            <Divider />
            <Form.Item>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={secSaving}
                onClick={handleSecSave}
              >
                {t('settings:security.saveConfig')}
              </Button>
            </Form.Item>
          </Form>
        )}
      </Card>

      {/* Section 3: API Token 管理 */}
      <Card
        title={
          <Space>
            <KeyOutlined />
            {t('settings:security.apiTokenTitle')}
          </Space>
        }
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => { setCreateModalOpen(true); createForm.resetFields(); }}
          >
            {t('settings:security.createToken')}
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          {t('settings:security.apiTokenDesc')}
        </Text>
        <Table
          scroll={{ x: 'max-content' }}
          columns={tokenColumns}
          dataSource={tokens}
          rowKey="id"
          loading={tokensLoading}
          pagination={{ pageSize: 10, hideOnSinglePage: true }}
        />
      </Card>

      {/* 建立 Token Modal */}
      <Modal
        title={
          <Space><KeyOutlined />{t('settings:security.createToken')}</Space>
        }
        open={createModalOpen}
        onCancel={() => setCreateModalOpen(false)}
        onOk={handleCreateToken}
        confirmLoading={creating}
        okText={t('settings:security.createToken')}
        cancelText={t('common:cancel')}
        width={480}
      >
        <Form form={createForm} layout="vertical">
          <Form.Item
            name="name"
            label={t('settings:security.tokenName')}
            rules={[{ required: true, message: t('settings:security.tokenNameRequired') }]}
          >
            <Input placeholder={t('settings:security.tokenNamePlaceholder')} maxLength={100} />
          </Form.Item>
          <Form.Item name="scopes" label={t('settings:security.tokenScopes')}>
            <Checkbox.Group options={SCOPES} />
          </Form.Item>
          <Form.Item name="expires_at" label={t('settings:security.tokenExpiresAt')}>
            <DatePicker style={{ width: '100%' }} disabledDate={d => d && d.isBefore(dayjs(), 'day')} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Token 建立成功 — 一次性顯示 Modal */}
      <Modal
        title={
          <Space><KeyOutlined />{t('settings:security.tokenCreatedTitle')}</Space>
        }
        open={resultModalOpen}
        onOk={() => setResultModalOpen(false)}
        onCancel={() => setResultModalOpen(false)}
        okText={t('common:confirm')}
        cancelButtonProps={{ style: { display: 'none' } }}
        width={560}
      >
        <Alert
          type="warning"
          message={t('settings:security.tokenOnceWarning')}
          style={{ marginBottom: 16 }}
        />
        {newTokenResult && (
          <>
            <Paragraph style={{ marginBottom: 8 }}>
              <Text strong>{t('settings:security.tokenName')}：</Text>{newTokenResult.name}
            </Paragraph>
            <Input.TextArea
              value={newTokenResult.token}
              readOnly
              rows={2}
              style={{ fontFamily: 'monospace', marginBottom: 12 }}
            />
            <Button
              icon={<CopyOutlined />}
              onClick={() => handleCopyToken(newTokenResult.token)}
            >
              {t('settings:security.copyToken')}
            </Button>
          </>
        )}
      </Modal>
    </Space>
  );
};

export default SecuritySettings;

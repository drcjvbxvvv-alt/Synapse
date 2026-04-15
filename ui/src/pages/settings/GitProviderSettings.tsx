/**
 * GitProviderSettings — Git Provider 管理（P3-4）
 *
 * 功能：
 *  - 列出所有 Git Provider（GitHub / GitLab / Gitea）
 *  - 新增 / 編輯 / 刪除 Provider
 *  - 顯示 Webhook URL + 重新生成 Token
 */
import React, { useState, useCallback } from 'react';
import {
  Table,
  Button,
  Tag,
  Space,
  Tooltip,
  Popconfirm,
  Modal,
  Form,
  Input,
  Select,
  Switch,
  Typography,
  App,
  theme,
  Flex,
  Alert,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
  LinkOutlined,
  KeyOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import gitProviderService, {
  type GitProvider,
  type CreateGitProviderRequest,
  type UpdateGitProviderRequest,
} from '../../services/gitProviderService';
import EmptyState from '../../components/EmptyState';

const { Text } = Typography;

// ─── Type colors ──────────────────────────────────────────────────────────────

const TYPE_COLORS: Record<string, string> = {
  github: 'geekblue',
  gitlab: 'orange',
  gitea: 'green',
};

// ─── Main component ───────────────────────────────────────────────────────────

const GitProviderSettings: React.FC = () => {
  const { token } = theme.useToken();
  const { message, modal } = App.useApp();
  const { t } = useTranslation(['cicd', 'common']);
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<GitProvider | null>(null);
  const [webhookInfo, setWebhookInfo] = useState<{ token: string; url: string } | null>(null);

  const [form] = Form.useForm();

  // ─── Query ──────────────────────────────────────────────────────────────────

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['git-providers'],
    queryFn: () => gitProviderService.list(),
    staleTime: 30_000,
  });

  const providers = data?.items ?? [];

  // ─── Mutations ───────────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (req: CreateGitProviderRequest) => gitProviderService.create(req),
    onSuccess: (res) => {
      message.success(t('cicd:gitProvider.messages.createSuccess'));
      setWebhookInfo({ token: res.webhook_token, url: res.webhook_url });
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['git-providers'] });
    },
    onError: () => message.error(t('cicd:gitProvider.messages.createFailed')),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateGitProviderRequest }) =>
      gitProviderService.update(id, data),
    onSuccess: () => {
      message.success(t('cicd:gitProvider.messages.updateSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['git-providers'] });
    },
    onError: () => message.error(t('cicd:gitProvider.messages.updateFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => gitProviderService.delete(id),
    onSuccess: () => {
      message.success(t('cicd:gitProvider.messages.deleteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['git-providers'] });
    },
    onError: () => message.error(t('cicd:gitProvider.messages.deleteFailed')),
  });

  const regenMutation = useMutation({
    mutationFn: (id: number) => gitProviderService.regenerateToken(id),
    onSuccess: (res) => {
      message.success(t('cicd:gitProvider.messages.regenerateSuccess'));
      setWebhookInfo({ token: res.webhook_token, url: res.webhook_url });
      queryClient.invalidateQueries({ queryKey: ['git-providers'] });
    },
    onError: () => message.error(t('cicd:gitProvider.messages.regenerateFailed')),
  });

  // ─── Handlers ────────────────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    form.resetFields();
    setFormOpen(true);
  }, [form]);

  const handleEdit = useCallback((p: GitProvider) => {
    setEditing(p);
    form.setFieldsValue({
      name: p.name,
      type: p.type,
      base_url: p.base_url,
      enabled: p.enabled,
    });
    setFormOpen(true);
  }, [form]);

  const handleSubmit = useCallback(async () => {
    const values = await form.validateFields();
    if (editing) {
      updateMutation.mutate({ id: editing.id, data: values });
    } else {
      createMutation.mutate(values);
    }
  }, [form, editing, createMutation, updateMutation]);

  const handleRegenerateToken = useCallback((p: GitProvider) => {
    modal.confirm({
      title: t('cicd:gitProvider.regenerateToken'),
      content: t('cicd:gitProvider.regenerateTokenConfirm'),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: () => regenMutation.mutate(p.id),
    });
  }, [modal, t, regenMutation]);

  // ─── Columns ─────────────────────────────────────────────────────────────────

  const columns: TableColumnsType<GitProvider> = [
    {
      title: t('cicd:gitProvider.table.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t('cicd:gitProvider.table.type'),
      dataIndex: 'type',
      key: 'type',
      width: 110,
      render: (type: string) => (
        <Tag color={TYPE_COLORS[type] ?? 'default'}>
          {t(`cicd:gitProvider.type.${type}`, { defaultValue: type })}
        </Tag>
      ),
    },
    {
      title: t('cicd:gitProvider.table.baseUrl'),
      dataIndex: 'base_url',
      key: 'base_url',
      ellipsis: true,
      render: (url: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{url}</Text>
      ),
    },
    {
      title: t('cicd:gitProvider.table.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (v: boolean) => <Tag color={v ? 'success' : 'default'}>{v ? 'ON' : 'OFF'}</Tag>,
    },
    {
      title: t('cicd:gitProvider.table.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (time: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(time).format('YYYY-MM-DD HH:mm')}
        </Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 140,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Tooltip title={t('cicd:gitProvider.webhookInfo')}>
            <Button
              type="link"
              size="small"
              icon={<LinkOutlined />}
              onClick={() => setWebhookInfo({
                token: record.webhook_token,
                url: `/api/v1/webhooks/git/${record.webhook_token}`,
              })}
            />
          </Tooltip>
          <Tooltip title={t('cicd:gitProvider.regenerateToken')}>
            <Button
              type="link"
              size="small"
              icon={<KeyOutlined />}
              onClick={() => handleRegenerateToken(record)}
            />
          </Tooltip>
          <Tooltip title={t('common:actions.edit')}>
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record)}
            />
          </Tooltip>
          <Popconfirm
            title={t('common:confirm.deleteTitle')}
            description={t('common:confirm.deleteDesc', { name: record.name })}
            onConfirm={() => deleteMutation.mutate(record.id)}
            okText={t('common:actions.delete')}
            okButtonProps={{ danger: true }}
            cancelText={t('common:actions.cancel')}
          >
            <Tooltip title={t('common:actions.delete')}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // ─── Render ──────────────────────────────────────────────────────────────────

  return (
    <>
      {/* Toolbar */}
      <Flex justify="flex-end" style={{ marginBottom: token.marginMD }}>
        <Space>
          <Tooltip title={t('common:actions.refresh')}>
            <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
          </Tooltip>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            {t('common:actions.create')}
          </Button>
        </Space>
      </Flex>

      <Table<GitProvider>
        columns={columns}
        dataSource={providers}
        rowKey="id"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (n) => t('common:pagination.total', { total: n }) }}
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
      />

      {/* Webhook URL info modal */}
      <Modal
        title={t('cicd:gitProvider.webhookInfo')}
        open={!!webhookInfo}
        onCancel={() => setWebhookInfo(null)}
        footer={<Button onClick={() => setWebhookInfo(null)}>{t('common:actions.close')}</Button>}
      >
        {webhookInfo && (
          <div>
            <Text type="secondary" style={{ display: 'block', marginBottom: token.marginXS }}>
              Webhook URL
            </Text>
            <Text
              code
              copyable
              style={{ wordBreak: 'break-all' }}
            >
              {window.location.origin}{webhookInfo.url}
            </Text>
          </div>
        )}
      </Modal>

      {/* Create / Edit modal */}
      <Modal
        title={editing ? t('cicd:gitProvider.form.editTitle') : t('cicd:gitProvider.form.createTitle')}
        open={formOpen}
        onCancel={() => setFormOpen(false)}
        onOk={handleSubmit}
        okText={editing ? t('common:actions.save') : t('common:actions.create')}
        cancelText={t('common:actions.cancel')}
        confirmLoading={createMutation.isPending || updateMutation.isPending}
        destroyOnHidden
        width={560}
      >
        <Form form={form} layout="vertical" style={{ marginTop: token.marginMD }}>
          <Form.Item
            name="name"
            label={t('cicd:gitProvider.form.name')}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:gitProvider.form.namePlaceholder')} />
          </Form.Item>

          {!editing && (
            <Form.Item
              name="type"
              label={t('cicd:gitProvider.form.type')}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Select
                options={['github', 'gitlab', 'gitea'].map((v) => ({
                  label: t(`cicd:gitProvider.type.${v}`),
                  value: v,
                }))}
              />
            </Form.Item>
          )}

          <Form.Item
            name="base_url"
            label={t('cicd:gitProvider.form.baseUrl')}
            rules={[{ required: !editing, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:gitProvider.form.baseUrlPlaceholder')} />
          </Form.Item>

          <Form.Item name="access_token" label={t('cicd:gitProvider.form.accessToken')}>
            <Input.Password placeholder={t('cicd:gitProvider.form.accessTokenPlaceholder')} />
          </Form.Item>

          <Form.Item name="webhook_secret" label={t('cicd:gitProvider.form.webhookSecret')}>
            <Input.Password placeholder={t('cicd:gitProvider.form.webhookSecretPlaceholder')} />
          </Form.Item>

          {editing && (
            <Form.Item name="enabled" label={t('cicd:gitProvider.form.enabled')} valuePropName="checked">
              <Switch />
            </Form.Item>
          )}

          {!editing && (
            <Alert
              type="info"
              showIcon
              message="建立後將顯示 Webhook URL，請妥善保管。"
              style={{ marginTop: token.marginSM }}
            />
          )}
        </Form>
      </Modal>
    </>
  );
};

export default GitProviderSettings;

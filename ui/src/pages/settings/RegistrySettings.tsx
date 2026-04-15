/**
 * RegistrySettings — 映像 Registry 管理（P3-4）
 *
 * 功能：
 *  - 列出所有 Registry（Harbor / DockerHub / ECR / GCR / ACR）
 *  - 新增 / 編輯 / 刪除 Registry
 *  - 測試連線
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
  ApiOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import registryService, {
  type Registry,
  type CreateRegistryRequest,
  type UpdateRegistryRequest,
} from '../../services/registryService';
import EmptyState from '../../components/EmptyState';

const { Text } = Typography;

// ─── Type colors ──────────────────────────────────────────────────────────────

const TYPE_COLORS: Record<string, string> = {
  harbor: 'blue',
  dockerhub: 'cyan',
  acr: 'orange',
  ecr: 'volcano',
  gcr: 'red',
};

// ─── Main component ───────────────────────────────────────────────────────────

const RegistrySettings: React.FC = () => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['cicd', 'common']);
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Registry | null>(null);
  const [testingId, setTestingId] = useState<number | null>(null);
  const [testResult, setTestResult] = useState<{ id: number; ok: boolean; error?: string } | null>(null);

  const [form] = Form.useForm();

  // ─── Query ──────────────────────────────────────────────────────────────────

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['registries'],
    queryFn: () => registryService.list(),
    staleTime: 30_000,
  });

  const registries = data?.items ?? [];

  // ─── Mutations ───────────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (req: CreateRegistryRequest) => registryService.create(req),
    onSuccess: () => {
      message.success(t('cicd:registry.messages.createSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['registries'] });
    },
    onError: () => message.error(t('cicd:registry.messages.createFailed')),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateRegistryRequest }) =>
      registryService.update(id, data),
    onSuccess: () => {
      message.success(t('cicd:registry.messages.updateSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['registries'] });
    },
    onError: () => message.error(t('cicd:registry.messages.updateFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => registryService.delete(id),
    onSuccess: () => {
      message.success(t('cicd:registry.messages.deleteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['registries'] });
    },
    onError: () => message.error(t('cicd:registry.messages.deleteFailed')),
  });

  // ─── Handlers ────────────────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    form.resetFields();
    setFormOpen(true);
  }, [form]);

  const handleEdit = useCallback((r: Registry) => {
    setEditing(r);
    form.setFieldsValue({
      name: r.name,
      type: r.type,
      url: r.url,
      username: r.username,
      insecure_tls: r.insecure_tls,
      default_project: r.default_project,
      enabled: r.enabled,
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

  const handleTestConnection = useCallback(async (r: Registry) => {
    setTestingId(r.id);
    setTestResult(null);
    try {
      const res = await registryService.testConnection(r.id);
      setTestResult({ id: r.id, ok: res.connected, error: res.error });
      if (res.connected) {
        message.success(t('cicd:registry.testSuccess'));
      } else {
        message.warning(`${t('cicd:registry.testFailed')}: ${res.error ?? 'unknown'}`);
      }
    } catch {
      setTestResult({ id: r.id, ok: false, error: 'request failed' });
      message.error(t('cicd:registry.testFailed'));
    } finally {
      setTestingId(null);
    }
  }, [t, message]);

  // ─── Columns ─────────────────────────────────────────────────────────────────

  const columns: TableColumnsType<Registry> = [
    {
      title: t('cicd:registry.table.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t('cicd:registry.table.type'),
      dataIndex: 'type',
      key: 'type',
      width: 110,
      render: (type: string) => (
        <Tag color={TYPE_COLORS[type] ?? 'default'}>
          {t(`cicd:registry.type.${type}`, { defaultValue: type })}
        </Tag>
      ),
    },
    {
      title: t('cicd:registry.table.url'),
      dataIndex: 'url',
      key: 'url',
      ellipsis: true,
      render: (url: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{url}</Text>
      ),
    },
    {
      title: t('cicd:registry.table.username'),
      dataIndex: 'username',
      key: 'username',
      width: 140,
      render: (u: string) => u ? <Text style={{ fontSize: token.fontSizeSM }}>{u}</Text> : <Text type="secondary">—</Text>,
    },
    {
      title: t('cicd:registry.table.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (v: boolean) => <Tag color={v ? 'success' : 'default'}>{v ? 'ON' : 'OFF'}</Tag>,
    },
    {
      title: t('cicd:registry.table.createdAt'),
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
      width: 160,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          {/* Test connection */}
          <Tooltip title={t('cicd:registry.testConnection')}>
            <Button
              type="link"
              size="small"
              icon={
                testingId === record.id ? (
                  <LoadingOutlined spin />
                ) : testResult?.id === record.id ? (
                  testResult.ok ? (
                    <CheckCircleOutlined style={{ color: token.colorSuccess }} />
                  ) : (
                    <CloseCircleOutlined style={{ color: token.colorError }} />
                  )
                ) : (
                  <ApiOutlined />
                )
              }
              onClick={() => handleTestConnection(record)}
              loading={false}
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

      <Table<Registry>
        columns={columns}
        dataSource={registries}
        rowKey="id"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (n) => t('common:pagination.total', { total: n }) }}
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
      />

      {/* Create / Edit modal */}
      <Modal
        title={editing ? t('cicd:registry.form.editTitle') : t('cicd:registry.form.createTitle')}
        open={formOpen}
        onCancel={() => setFormOpen(false)}
        onOk={handleSubmit}
        okText={editing ? t('common:actions.save') : t('common:actions.create')}
        cancelText={t('common:actions.cancel')}
        confirmLoading={createMutation.isPending || updateMutation.isPending}
        destroyOnHidden
        width={600}
      >
        <Form form={form} layout="vertical" style={{ marginTop: token.marginMD }}>
          <Form.Item
            name="name"
            label={t('cicd:registry.form.name')}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:registry.form.namePlaceholder')} />
          </Form.Item>

          {!editing && (
            <Form.Item
              name="type"
              label={t('cicd:registry.form.type')}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Select
                options={['harbor', 'dockerhub', 'acr', 'ecr', 'gcr'].map((v) => ({
                  label: t(`cicd:registry.type.${v}`),
                  value: v,
                }))}
              />
            </Form.Item>
          )}

          <Form.Item
            name="url"
            label={t('cicd:registry.form.url')}
            rules={[{ required: !editing, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:registry.form.urlPlaceholder')} />
          </Form.Item>

          <Form.Item name="username" label={t('cicd:registry.form.username')}>
            <Input placeholder={t('cicd:registry.form.usernamePlaceholder')} />
          </Form.Item>

          <Form.Item name="password" label={t('cicd:registry.form.password')}>
            <Input.Password placeholder={t('cicd:registry.form.passwordPlaceholder')} />
          </Form.Item>

          <Form.Item name="default_project" label={t('cicd:registry.form.defaultProject')}>
            <Input placeholder={t('cicd:registry.form.defaultProjectPlaceholder')} />
          </Form.Item>

          <Form.Item
            name="insecure_tls"
            label={t('cicd:registry.form.insecureTls')}
            tooltip={t('cicd:registry.form.insecureTlsTooltip')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.Item name="ca_bundle" label={t('cicd:registry.form.caBundle')}>
            <Input.TextArea rows={4} placeholder={t('cicd:registry.form.caBundlePlaceholder')} />
          </Form.Item>

          {editing && (
            <Form.Item name="enabled" label={t('cicd:registry.form.enabled')} valuePropName="checked">
              <Switch />
            </Form.Item>
          )}

          {testResult?.id === editing?.id && !testResult?.ok && (
            <Alert
              type="error"
              message={`${t('cicd:registry.testFailed')}: ${testResult?.error}`}
              showIcon
            />
          )}
        </Form>
      </Modal>
    </>
  );
};

export default RegistrySettings;

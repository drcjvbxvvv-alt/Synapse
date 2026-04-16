/**
 * PipelineSecretManager — Pipeline Secret 管理 Drawer
 *
 * 功能：
 *  - 列出 pipeline scope 與 global scope 的 Secrets
 *  - 新增 / 編輯（可更新值）/ 刪除 Secret
 *  - 永遠不顯示 Secret 值（建立後只能覆寫）
 *  - 使用語法：${{ secrets.NAME }} 引用
 */
import React, { useState, useCallback } from 'react';
import {
  Drawer,
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
  Typography,
  Alert,
  App,
  theme,
  Flex,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  LockOutlined,
  EyeInvisibleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import pipelineSecretService, {
  type PipelineSecret,
  type SecretScope,
} from '../../../services/pipelineSecretService';
import EmptyState from '../../../components/EmptyState';
import { type Pipeline } from '../../../services/pipelineService';

const { Text } = Typography;

// ─── Props ────────────────────────────────────────────────────────────────────

interface PipelineSecretManagerProps {
  open: boolean;
  onClose: () => void;
  pipeline: Pipeline;
}

// ─── Form values ──────────────────────────────────────────────────────────────

interface SecretFormValues {
  scope: SecretScope;
  name: string;
  value: string;
  description?: string;
}

// ─── Main component ───────────────────────────────────────────────────────────

const PipelineSecretManager: React.FC<PipelineSecretManagerProps> = ({
  open, onClose, pipeline,
}) => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['cicd', 'common']);
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<PipelineSecret | null>(null);
  const [form] = Form.useForm<SecretFormValues>();

  // ─── Queries ─────────────────────────────────────────────────────────────

  // Pipeline-scope secrets
  const { data: pipelineSecrets, isLoading: loadingPipeline } = useQuery({
    queryKey: ['pipeline-secrets', 'pipeline', pipeline.id],
    queryFn: () => pipelineSecretService.list('pipeline', pipeline.id),
    enabled: open,
    staleTime: 15_000,
  });

  // Global-scope secrets
  const { data: globalSecrets, isLoading: loadingGlobal } = useQuery({
    queryKey: ['pipeline-secrets', 'global'],
    queryFn: () => pipelineSecretService.list('global'),
    enabled: open,
    staleTime: 30_000,
  });

  const allSecrets = [
    ...(pipelineSecrets ?? []),
    ...(globalSecrets ?? []),
  ];

  // ─── Mutations ───────────────────────────────────────────────────────────

  const invalidate = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['pipeline-secrets', 'pipeline', pipeline.id] });
    queryClient.invalidateQueries({ queryKey: ['pipeline-secrets', 'global'] });
  }, [queryClient, pipeline.id]);

  const createMutation = useMutation({
    mutationFn: (vals: SecretFormValues) =>
      pipelineSecretService.create({
        scope: vals.scope,
        scope_ref: vals.scope === 'pipeline' ? pipeline.id : undefined,
        name: vals.name,
        value: vals.value,
        description: vals.description,
      }),
    onSuccess: () => {
      message.success(t('cicd:secret.messages.createSuccess'));
      setFormOpen(false);
      invalidate();
    },
    onError: () => message.error(t('cicd:secret.messages.createFailed')),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, vals }: { id: number; vals: SecretFormValues }) =>
      pipelineSecretService.update(id, {
        value: vals.value || undefined,  // 空字串 = 不更新
        description: vals.description,
      }),
    onSuccess: () => {
      message.success(t('cicd:secret.messages.updateSuccess'));
      setFormOpen(false);
      invalidate();
    },
    onError: () => message.error(t('cicd:secret.messages.updateFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => pipelineSecretService.delete(id),
    onSuccess: () => {
      message.success(t('cicd:secret.messages.deleteSuccess'));
      invalidate();
    },
    onError: () => message.error(t('cicd:secret.messages.deleteFailed')),
  });

  // ─── Handlers ────────────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ scope: 'pipeline' });
    setFormOpen(true);
  }, [form]);

  const handleEdit = useCallback((secret: PipelineSecret) => {
    setEditing(secret);
    form.resetFields();
    form.setFieldsValue({
      scope: secret.scope,
      name: secret.name,
      value: '',             // 不預填值，留空 = 保留現有
      description: secret.description,
    });
    setFormOpen(true);
  }, [form]);

  const handleSubmit = useCallback(async () => {
    const vals = await form.validateFields();
    if (editing) {
      updateMutation.mutate({ id: editing.id, vals });
    } else {
      createMutation.mutate(vals);
    }
  }, [form, editing, createMutation, updateMutation]);

  // ─── Columns ─────────────────────────────────────────────────────────────

  const columns: TableColumnsType<PipelineSecret> = [
    {
      title: t('cicd:secret.table.scope'),
      dataIndex: 'scope',
      key: 'scope',
      width: 90,
      render: (scope: SecretScope) => (
        <Tag color={scope === 'global' ? 'purple' : 'blue'}>
          {t(`cicd:secret.scope.${scope}`)}
        </Tag>
      ),
    },
    {
      title: t('cicd:secret.table.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => (
        <Flex align="center" gap={6}>
          <LockOutlined style={{ color: token.colorTextTertiary }} />
          <Text code style={{ fontSize: token.fontSizeSM }}>{name}</Text>
        </Flex>
      ),
    },
    {
      title: t('cicd:secret.table.value'),
      key: 'value',
      width: 100,
      render: () => (
        <Flex align="center" gap={4} style={{ color: token.colorTextTertiary }}>
          <EyeInvisibleOutlined />
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
            {t('cicd:secret.hidden')}
          </Text>
        </Flex>
      ),
    },
    {
      title: t('cicd:secret.table.description'),
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (desc: string) => desc
        ? <Text type="secondary">{desc}</Text>
        : <Text type="secondary" italic>—</Text>,
    },
    {
      title: t('cicd:secret.table.updatedAt'),
      dataIndex: 'updated_at',
      key: 'updated_at',
      width: 130,
      render: (t: string) => (
        <Text style={{ fontSize: 11, color: token.colorTextTertiary }}>
          {dayjs(t).format('YYYY-MM-DD HH:mm')}
        </Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 90,
      render: (_, record) => (
        <Space size={0}>
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

  // ─── Render ──────────────────────────────────────────────────────────────

  return (
    <>
      <Drawer
        title={`${pipeline.name} — ${t('cicd:secret.title')}`}
        open={open}
        onClose={onClose}
        width={760}
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            size="small"
            onClick={handleCreate}
          >
            {t('common:actions.create')}
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: token.marginMD }}>
          {t('cicd:secret.subtitle')}
        </Text>

        <Alert
          type="info"
          showIcon
          style={{ marginBottom: token.marginMD }}
          message={t('cicd:secret.usageHint')}
          description={
            <Text code style={{ fontSize: token.fontSizeSM }}>
              {'${{ secrets.SECRET_NAME }}'}
            </Text>
          }
        />

        <Table<PipelineSecret>
          columns={columns}
          dataSource={allSecrets}
          rowKey="id"
          loading={loadingPipeline || loadingGlobal}
          size="small"
          pagination={false}
          locale={{ emptyText: <EmptyState description={t('cicd:secret.noSecrets')} /> }}
        />
      </Drawer>

      {/* Create / Edit Modal */}
      <Modal
        title={
          editing
            ? t('cicd:secret.form.editTitle', { name: editing.name })
            : t('cicd:secret.form.createTitle')
        }
        open={formOpen}
        onCancel={() => setFormOpen(false)}
        onOk={handleSubmit}
        okText={editing ? t('common:actions.save') : t('common:actions.create')}
        cancelText={t('common:actions.cancel')}
        confirmLoading={createMutation.isPending || updateMutation.isPending}
        destroyOnHidden
        width={520}
      >
        <Form form={form} layout="vertical" style={{ marginTop: token.marginMD }}>
          {/* Scope selector — 編輯時鎖定 */}
          <Form.Item
            name="scope"
            label={t('cicd:secret.form.scope')}
            rules={[{ required: true }]}
          >
            <Select
              disabled={!!editing}
              options={[
                { label: t('cicd:secret.scope.pipeline'), value: 'pipeline' },
                { label: t('cicd:secret.scope.global'), value: 'global' },
              ]}
            />
          </Form.Item>

          <Form.Item
            name="name"
            label={t('cicd:secret.form.name')}
            tooltip={t('cicd:secret.form.nameTooltip')}
            rules={[
              { required: !editing, message: t('common:validation.required') },
              { pattern: /^[A-Z][A-Z0-9_]*$/, message: t('cicd:secret.form.namePattern') },
            ]}
          >
            <Input
              disabled={!!editing}
              placeholder="HARBOR_PASSWORD"
              style={{ fontFamily: 'monospace' }}
            />
          </Form.Item>

          <Form.Item
            name="value"
            label={
              editing
                ? t('cicd:secret.form.valueEdit')
                : t('cicd:secret.form.value')
            }
            tooltip={editing ? t('cicd:secret.form.valueEditTooltip') : undefined}
            rules={editing ? [] : [{ required: true, message: t('common:validation.required') }]}
          >
            <Input.Password
              placeholder={
                editing
                  ? t('cicd:secret.form.valueEditPlaceholder')
                  : t('cicd:secret.form.valuePlaceholder')
              }
              autoComplete="new-password"
            />
          </Form.Item>

          <Form.Item
            name="description"
            label={t('cicd:secret.form.description')}
          >
            <Input placeholder={t('cicd:secret.form.descriptionPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

export default PipelineSecretManager;

/**
 * PipelineEnvironments — Pipeline 部署環境管理 Drawer（P3-4）
 *
 * 功能：
 *  - 列出 Pipeline 下的所有 Environment（按晉升順序）
 *  - 新增 / 編輯 / 刪除 Environment
 *  - 顯示自動晉升 / 審核設定
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
  InputNumber,
  Select,
  Switch,
  Typography,
  App,
  theme,
  Flex,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ArrowUpOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';

import environmentService, {
  type Environment,
  type CreateEnvironmentRequest,
  type UpdateEnvironmentRequest,
} from '../../../services/environmentService';
import EmptyState from '../../../components/EmptyState';
import { type Pipeline } from '../../../services/pipelineService';

const { Text } = Typography;

// ─── Props ────────────────────────────────────────────────────────────────────

interface PipelineEnvironmentsProps {
  open: boolean;
  onClose: () => void;
  clusterId: number;
  pipeline: Pipeline;
  /** Cluster list for the cluster selector */
  clusters?: { id: number; name: string }[];
}

// ─── Main component ───────────────────────────────────────────────────────────

const PipelineEnvironments: React.FC<PipelineEnvironmentsProps> = ({
  open, onClose, clusterId, pipeline, clusters = [],
}) => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['cicd', 'common']);
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Environment | null>(null);
  const [form] = Form.useForm();

  // ─── Query ──────────────────────────────────────────────────────────────────

  const { data, isLoading } = useQuery({
    queryKey: ['environments', clusterId, pipeline.id],
    queryFn: () => environmentService.list(clusterId, pipeline.id),
    enabled: open && clusterId > 0 && pipeline.id > 0,
    staleTime: 15_000,
  });

  const environments = data?.items ?? [];

  // ─── Mutations ───────────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (req: CreateEnvironmentRequest) =>
      environmentService.create(clusterId, pipeline.id, req),
    onSuccess: () => {
      message.success(t('cicd:environment.messages.createSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['environments', clusterId, pipeline.id] });
    },
    onError: () => message.error(t('cicd:environment.messages.createFailed')),
  });

  const updateMutation = useMutation({
    mutationFn: ({ envId, data }: { envId: number; data: UpdateEnvironmentRequest }) =>
      environmentService.update(clusterId, pipeline.id, envId, data),
    onSuccess: () => {
      message.success(t('cicd:environment.messages.updateSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['environments', clusterId, pipeline.id] });
    },
    onError: () => message.error(t('cicd:environment.messages.updateFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (envId: number) =>
      environmentService.delete(clusterId, pipeline.id, envId),
    onSuccess: () => {
      message.success(t('cicd:environment.messages.deleteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['environments', clusterId, pipeline.id] });
    },
    onError: () => message.error(t('cicd:environment.messages.deleteFailed')),
  });

  // ─── Handlers ────────────────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({
      cluster_id: clusterId,
      order_index: environments.length + 1,
      auto_promote: false,
      approval_required: false,
    });
    setFormOpen(true);
  }, [form, clusterId, environments.length]);

  const handleEdit = useCallback((env: Environment) => {
    setEditing(env);
    form.setFieldsValue({
      name: env.name,
      cluster_id: env.cluster_id,
      namespace: env.namespace,
      order_index: env.order_index,
      auto_promote: env.auto_promote,
      approval_required: env.approval_required,
      smoke_test_step_name: env.smoke_test_step_name,
    });
    setFormOpen(true);
  }, [form]);

  const handleSubmit = useCallback(async () => {
    const values = await form.validateFields();
    if (editing) {
      updateMutation.mutate({ envId: editing.id, data: values });
    } else {
      createMutation.mutate(values);
    }
  }, [form, editing, createMutation, updateMutation]);

  // ─── Columns ─────────────────────────────────────────────────────────────────

  const columns: TableColumnsType<Environment> = [
    {
      title: t('cicd:environment.table.orderIndex'),
      dataIndex: 'order_index',
      key: 'order_index',
      width: 80,
      render: (idx: number) => (
        <Tag icon={<ArrowUpOutlined />} color="blue">{idx}</Tag>
      ),
    },
    {
      title: t('cicd:environment.table.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t('cicd:environment.table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (ns: string) => <Tag>{ns}</Tag>,
    },
    {
      title: t('cicd:environment.table.autoPromote'),
      dataIndex: 'auto_promote',
      key: 'auto_promote',
      width: 100,
      render: (v: boolean) => <Tag color={v ? 'success' : 'default'}>{v ? 'ON' : 'OFF'}</Tag>,
    },
    {
      title: t('cicd:environment.table.approvalRequired'),
      dataIndex: 'approval_required',
      key: 'approval_required',
      width: 100,
      render: (v: boolean) => <Tag color={v ? 'warning' : 'default'}>{v ? 'ON' : 'OFF'}</Tag>,
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 100,
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

  // ─── Render ──────────────────────────────────────────────────────────────────

  return (
    <>
      <Drawer
        title={`${pipeline.name} — ${t('cicd:environment.title')}`}
        open={open}
        onClose={onClose}
        width={720}
        extra={
          <Button type="primary" icon={<PlusOutlined />} size="small" onClick={handleCreate}>
            {t('common:actions.create')}
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: token.marginMD }}>
          {t('cicd:environment.subtitle')}
        </Text>

        <Table<Environment>
          columns={columns}
          dataSource={environments}
          rowKey="id"
          loading={isLoading}
          size="small"
          pagination={false}
          locale={{ emptyText: <EmptyState description={t('cicd:environment.noEnvironments')} /> }}
        />
      </Drawer>

      {/* Create / Edit modal */}
      <Modal
        title={editing ? t('cicd:environment.form.editTitle') : t('cicd:environment.form.createTitle')}
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
            label={t('cicd:environment.form.name')}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:environment.form.namePlaceholder')} />
          </Form.Item>

          {clusters.length > 0 && (
            <Form.Item
              name="cluster_id"
              label={t('cicd:environment.form.cluster')}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Select
                options={clusters.map((c) => ({ label: c.name, value: c.id }))}
                showSearch
                filterOption={(input, opt) =>
                  String(opt?.label ?? '').toLowerCase().includes(input.toLowerCase())
                }
              />
            </Form.Item>
          )}

          <Form.Item
            name="namespace"
            label={t('cicd:environment.form.namespace')}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:environment.form.namespacePlaceholder')} />
          </Form.Item>

          <Form.Item
            name="order_index"
            label={t('cicd:environment.form.orderIndex')}
            tooltip={t('cicd:environment.form.orderIndexTooltip')}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>

          <Flex gap={token.marginMD}>
            <Form.Item
              name="auto_promote"
              label={t('cicd:environment.form.autoPromote')}
              valuePropName="checked"
              style={{ flex: 1 }}
            >
              <Switch />
            </Form.Item>

            <Form.Item
              name="approval_required"
              label={t('cicd:environment.form.approvalRequired')}
              valuePropName="checked"
              style={{ flex: 1 }}
            >
              <Switch />
            </Form.Item>
          </Flex>

          <Form.Item
            name="smoke_test_step_name"
            label={t('cicd:environment.form.smokeTestStep')}
            tooltip={t('cicd:environment.form.smokeTestStepTooltip')}
          >
            <Input placeholder={t('cicd:environment.form.smokeTestStepPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

export default PipelineEnvironments;

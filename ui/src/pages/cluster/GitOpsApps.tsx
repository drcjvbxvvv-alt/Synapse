/**
 * GitOpsApps — GitOps 應用管理頁（M16）
 *
 * 功能：
 *  - 合併列表：原生 GitOps Apps + ArgoCD Applications
 *  - 建立 / 編輯 / 刪除原生 App
 *  - 手動觸發同步
 *  - 查看 Diff（漂移偵測）
 */
import React, { useState, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import {
  Card,
  Table,
  Button,
  Input,
  Select,
  Space,
  Flex,
  Tag,
  Tooltip,
  Popconfirm,
  Modal,
  Form,
  Typography,
  App,
  theme,
  InputNumber,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  SearchOutlined,
  SyncOutlined,
  DiffOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import gitopsService, {
  type GitOpsApp,
  type CreateGitOpsAppRequest,
  type UpdateGitOpsAppRequest,
  type DiffResult,
} from '../../services/gitopsService';
import EmptyState from '../../components/EmptyState';

const { Text } = Typography;

// ─── Status helpers ───────────────────────────────────────────────────────────

const STATUS_COLORS: Record<string, string> = {
  synced: 'success',
  drifted: 'warning',
  error: 'error',
  pending: 'processing',
  Synced: 'success',
  OutOfSync: 'warning',
  Healthy: 'success',
  Degraded: 'error',
  Progressing: 'processing',
  Missing: 'default',
};

// ─── Main Component ───────────────────────────────────────────────────────────

const GitOpsApps: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const clusterIdNum = parseInt(clusterId ?? '0', 10);

  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['cicd', 'common']);
  const queryClient = useQueryClient();

  const [search, setSearch] = useState('');
  const [sourceFilter, setSourceFilter] = useState<string | undefined>();
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<GitOpsApp | null>(null);
  const [diffOpen, setDiffOpen] = useState(false);
  const [diffApp, setDiffApp] = useState<GitOpsApp | null>(null);
  const [diffResult, setDiffResult] = useState<DiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);

  const [form] = Form.useForm();

  // ─── Query ───────────────────────────────────────────────────────────────

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['gitops-apps', clusterIdNum, sourceFilter],
    queryFn: () => gitopsService.list(clusterIdNum, sourceFilter),
    staleTime: 15_000,
  });

  const apps = (data?.items ?? []).filter((a) =>
    a.name.toLowerCase().includes(search.toLowerCase()) ||
    a.namespace.toLowerCase().includes(search.toLowerCase())
  );

  // ─── Mutations ────────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (req: CreateGitOpsAppRequest) => gitopsService.create(clusterIdNum, req),
    onSuccess: () => {
      message.success(t('cicd:gitops.messages.createSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['gitops-apps', clusterIdNum] });
    },
    onError: () => message.error(t('cicd:gitops.messages.createFailed')),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateGitOpsAppRequest }) =>
      gitopsService.update(clusterIdNum, id, data),
    onSuccess: () => {
      message.success(t('cicd:gitops.messages.updateSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['gitops-apps', clusterIdNum] });
    },
    onError: () => message.error(t('cicd:gitops.messages.updateFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => gitopsService.delete(clusterIdNum, id),
    onSuccess: () => {
      message.success(t('cicd:gitops.messages.deleteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['gitops-apps', clusterIdNum] });
    },
    onError: () => message.error(t('cicd:gitops.messages.deleteFailed')),
  });

  const syncMutation = useMutation({
    mutationFn: (id: number) => gitopsService.triggerSync(clusterIdNum, id),
    onSuccess: () => message.success(t('cicd:gitops.messages.syncTriggered')),
    onError: () => message.error(t('cicd:gitops.messages.syncFailed')),
  });

  // ─── Handlers ────────────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ source: 'native', render_type: 'raw', sync_policy: 'manual', sync_interval: 300 });
    setFormOpen(true);
  }, [form]);

  const handleEdit = useCallback((app: GitOpsApp) => {
    setEditing(app);
    form.setFieldsValue({
      repo_url: app.repo_url,
      branch: app.branch,
      path: app.path,
      render_type: app.render_type,
      sync_policy: app.sync_policy,
    });
    setFormOpen(true);
  }, [form]);

  const handleSubmit = useCallback(async () => {
    const values = await form.validateFields();
    if (editing) {
      updateMutation.mutate({ id: editing.id, data: values as UpdateGitOpsAppRequest });
    } else {
      createMutation.mutate({ ...values, source: 'native' } as CreateGitOpsAppRequest);
    }
  }, [form, editing, createMutation, updateMutation]);

  const handleDiff = useCallback(async (app: GitOpsApp) => {
    setDiffApp(app);
    setDiffResult(null);
    setDiffOpen(true);
    setDiffLoading(true);
    try {
      const result = await gitopsService.getDiff(clusterIdNum, app.id);
      setDiffResult(result);
    } catch {
      message.error(t('common:messages.failed'));
    } finally {
      setDiffLoading(false);
    }
  }, [clusterIdNum, message, t]);

  // ─── Columns ─────────────────────────────────────────────────────────────

  const columns: TableColumnsType<GitOpsApp> = [
    {
      title: t('cicd:gitops.table.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t('cicd:gitops.table.source'),
      dataIndex: 'source',
      key: 'source',
      width: 90,
      render: (s: string) => (
        <Tag color={s === 'argocd' ? 'orange' : 'blue'}>
          {t(`cicd:gitops.source.${s}`, { defaultValue: s })}
        </Tag>
      ),
    },
    {
      title: t('cicd:gitops.table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      render: (ns: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{ns}</Text>
      ),
    },
    {
      title: t('cicd:gitops.table.repoUrl'),
      dataIndex: 'repo_url',
      key: 'repo_url',
      ellipsis: true,
      render: (url?: string) => url ? (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{url}</Text>
      ) : <Text type="secondary">—</Text>,
    },
    {
      title: t('cicd:gitops.table.syncPolicy'),
      dataIndex: 'sync_policy',
      key: 'sync_policy',
      width: 90,
      render: (p?: string) => p ? (
        <Tag color={p === 'auto' ? 'green' : 'default'}>
          {t(`cicd:gitops.syncPolicy.${p}`, { defaultValue: p })}
        </Tag>
      ) : <Text type="secondary">—</Text>,
    },
    {
      title: t('cicd:gitops.table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (s?: string) => s ? (
        <Tag color={STATUS_COLORS[s] ?? 'default'}>
          {t(`cicd:gitops.status.${s}`, { defaultValue: s })}
        </Tag>
      ) : <Text type="secondary">—</Text>,
    },
    {
      title: t('cicd:gitops.table.diffSummary'),
      dataIndex: 'diff_summary',
      key: 'diff_summary',
      width: 120,
      ellipsis: true,
      render: (d?: string) => d ? (
        <Text type="warning" style={{ fontSize: token.fontSizeSM }}>{d}</Text>
      ) : <Text type="secondary">—</Text>,
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 160,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          {record.source === 'native' && (
            <>
              <Tooltip title={t('cicd:gitops.syncButton')}>
                <Button
                  type="link"
                  size="small"
                  icon={<SyncOutlined />}
                  loading={syncMutation.isPending}
                  onClick={() => syncMutation.mutate(record.id)}
                />
              </Tooltip>
              <Tooltip title={t('cicd:gitops.diffButton')}>
                <Button
                  type="link"
                  size="small"
                  icon={<DiffOutlined />}
                  onClick={() => handleDiff(record)}
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
            </>
          )}
        </Space>
      ),
    },
  ];

  // ─── Diff columns ─────────────────────────────────────────────────────────

  const diffColumns: TableColumnsType<DiffResult['resources'][number]> = [
    {
      title: 'Kind',
      dataIndex: 'kind',
      key: 'kind',
      width: 120,
      render: (k: string) => <Text strong>{k}</Text>,
    },
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
    },
    {
      title: 'Namespace',
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      render: (ns: string) => <Text type="secondary">{ns}</Text>,
    },
    {
      title: 'Action',
      dataIndex: 'action',
      key: 'action',
      width: 100,
      render: (a: string) => {
        const colorMap: Record<string, string> = {
          added: 'success', modified: 'warning', deleted: 'error', unchanged: 'default',
        };
        return <Tag color={colorMap[a] ?? 'default'}>{t(`cicd:gitops.diff.action.${a}`, { defaultValue: a })}</Tag>;
      },
    },
  ];

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <>
      <div style={{ marginBottom: token.marginLG }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('cicd:gitops.page.title')}
        </Typography.Title>
        <Text type="secondary">{t('cicd:gitops.page.subtitle')}</Text>
      </div>

      <Card variant="borderless">
        {/* Toolbar */}
        <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
          <Space>
            <Input
              prefix={<SearchOutlined />}
              placeholder={t('common:search.placeholder')}
              allowClear
              style={{ width: 240 }}
              onChange={(e) => setSearch(e.target.value)}
            />
            <Select
              allowClear
              placeholder={t('cicd:gitops.table.source')}
              style={{ width: 130 }}
              onChange={setSourceFilter}
              options={[
                { label: t('cicd:gitops.source.native'), value: 'native' },
                { label: t('cicd:gitops.source.argocd'), value: 'argocd' },
              ]}
            />
          </Space>
          <Space>
            <Tooltip title={t('common:actions.refresh')}>
              <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
            </Tooltip>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              {t('common:actions.create')}
            </Button>
          </Space>
        </Flex>

        <Table<GitOpsApp>
          columns={columns}
          dataSource={apps}
          rowKey="name"
          loading={isLoading}
          size="small"
          scroll={{ x: 'max-content' }}
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => t('common:pagination.total', { total }),
          }}
          locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
        />
      </Card>

      {/* Create / Edit Modal */}
      <Modal
        title={editing ? t('cicd:gitops.form.editTitle') : t('cicd:gitops.form.createTitle')}
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
          {!editing && (
            <Form.Item
              name="name"
              label={t('cicd:gitops.form.name')}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Input />
            </Form.Item>
          )}

          <Form.Item
            name="repo_url"
            label={t('cicd:gitops.form.repoUrl')}
            rules={[{ required: !editing, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:gitops.form.repoUrlPlaceholder')} />
          </Form.Item>

          <Form.Item
            name="branch"
            label={t('cicd:gitops.form.branch')}
            rules={[{ required: !editing, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:gitops.form.branchPlaceholder')} />
          </Form.Item>

          <Form.Item
            name="path"
            label={t('cicd:gitops.form.path')}
            rules={[{ required: !editing, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:gitops.form.pathPlaceholder')} />
          </Form.Item>

          <Form.Item
            name="render_type"
            label={t('cicd:gitops.form.renderType')}
            rules={[{ required: !editing, message: t('common:validation.required') }]}
          >
            <Select
              options={['raw', 'kustomize', 'helm'].map((v) => ({
                label: t(`cicd:gitops.renderType.${v}`),
                value: v,
              }))}
            />
          </Form.Item>

          {!editing && (
            <Form.Item
              name="namespace"
              label={t('cicd:gitops.form.namespace')}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Input placeholder={t('cicd:gitops.form.namespacePlaceholder')} />
            </Form.Item>
          )}

          <Form.Item name="sync_policy" label={t('cicd:gitops.form.syncPolicy')}>
            <Select
              options={['auto', 'manual'].map((v) => ({
                label: t(`cicd:gitops.syncPolicy.${v}`),
                value: v,
              }))}
            />
          </Form.Item>

          <Form.Item name="sync_interval" label={t('cicd:gitops.form.syncInterval')}>
            <InputNumber min={30} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Diff Modal */}
      <Modal
        title={t('cicd:gitops.diff.title', { name: diffApp?.name ?? '' })}
        open={diffOpen}
        onCancel={() => { setDiffOpen(false); setDiffApp(null); }}
        footer={<Button onClick={() => setDiffOpen(false)}>{t('common:actions.close')}</Button>}
        width={760}
      >
        {diffLoading ? (
          <div style={{ textAlign: 'center', padding: token.paddingLG }}>
            <SyncOutlined spin style={{ fontSize: 24 }} />
          </div>
        ) : diffResult ? (
          diffResult.resources.filter((r) => r.action !== 'unchanged').length === 0 ? (
            <EmptyState description={t('cicd:gitops.diff.noChanges')} />
          ) : (
            <Table
              columns={diffColumns}
              dataSource={diffResult.resources.filter((r) => r.action !== 'unchanged')}
              rowKey={(r) => `${r.kind}/${r.namespace}/${r.name}`}
              size="small"
              pagination={false}
            />
          )
        ) : null}
      </Modal>
    </>
  );
};

export default GitOpsApps;

/**
 * CIEngineSettings — CI 引擎連線設定管理（M18b/c/d/e-UI）
 *
 * 功能：
 *  - 列出所有外部 CI 引擎設定（GitLab CI / Jenkins / Tekton / Argo / GitHub Actions）
 *  - 新增 / 編輯 / 刪除引擎設定
 *  - 依引擎類型動態顯示對應欄位
 *  - 顯示引擎健康狀態（last_healthy / last_version）
 */
import React, { useState, useCallback, useEffect } from 'react';
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
  Divider,
  Badge,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
  PlayCircleOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import ciEngineService, {
  type CIEngineConfig,
  type CIEngineType,
  type CreateCIEngineRequest,
} from '../../services/ciEngineService';
import clusterService from '../../services/clusterService';
import EmptyState from '../../components/EmptyState';
import TriggerRunModal from './components/TriggerRunModal';
import CIEngineRunViewer from './components/CIEngineRunViewer';

const { Text } = Typography;

// ─── Constants ────────────────────────────────────────────────────────────────

const ENGINE_TYPE_COLORS: Record<CIEngineType, string> = {
  native:  'default',
  gitlab:  'orange',
  jenkins: 'blue',
  tekton:  'purple',
  argo:    'cyan',
  github:  'geekblue',
};

// Which engine types need an HTTP endpoint
const HAS_ENDPOINT: CIEngineType[] = ['gitlab', 'jenkins', 'github'];
// Which engine types use token auth
const HAS_TOKEN: CIEngineType[] = ['gitlab', 'github'];
// Which engine types use basic auth (username + password)
const HAS_BASIC: CIEngineType[] = ['jenkins'];
// Which engine types need a cluster reference
const HAS_CLUSTER: CIEngineType[] = ['tekton', 'argo'];

// ─── ExtraJSON helpers ────────────────────────────────────────────────────────

function parseExtra(json?: string): Record<string, string> {
  if (!json) return {};
  try { return JSON.parse(json); } catch { return {}; }
}

function buildExtra(type: CIEngineType, fields: Record<string, string>): string {
  const filtered = Object.fromEntries(
    Object.entries(fields).filter(([, v]) => v !== undefined && v !== ''),
  );
  if (Object.keys(filtered).length === 0) return '';
  return JSON.stringify(filtered);
}

// ─── Main component ───────────────────────────────────────────────────────────

const CIEngineSettings: React.FC = () => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['cicd', 'common']);
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<CIEngineConfig | null>(null);
  const [selectedType, setSelectedType] = useState<CIEngineType>('gitlab');

  const [triggerTarget, setTriggerTarget] = useState<CIEngineConfig | null>(null);
  const [runViewer, setRunViewer] = useState<{ engine: CIEngineConfig; runId: string } | null>(null);

  const [form] = Form.useForm();

  // ─── Queries ─────────────────────────────────────────────────────────────

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['ci-engines'],
    queryFn: () => ciEngineService.list(),
    staleTime: 30_000,
  });

  const { data: clustersData } = useQuery({
    queryKey: ['clusters-mini'],
    queryFn: () => clusterService.getClusters(),
    staleTime: 60_000,
  });

  const engines = data?.items ?? [];
  const clusters = clustersData?.items ?? [];

  // ─── Mutations ───────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (req: CreateCIEngineRequest) => ciEngineService.create(req),
    onSuccess: () => {
      message.success(t('cicd:ciEngine.messages.createSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['ci-engines'] });
    },
    onError: () => message.error(t('cicd:ciEngine.messages.createFailed')),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: CreateCIEngineRequest }) =>
      ciEngineService.update(id, data),
    onSuccess: () => {
      message.success(t('cicd:ciEngine.messages.updateSuccess'));
      setFormOpen(false);
      queryClient.invalidateQueries({ queryKey: ['ci-engines'] });
    },
    onError: () => message.error(t('cicd:ciEngine.messages.updateFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => ciEngineService.delete(id),
    onSuccess: () => {
      message.success(t('cicd:ciEngine.messages.deleteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['ci-engines'] });
    },
    onError: () => message.error(t('cicd:ciEngine.messages.deleteFailed')),
  });

  // ─── Form handlers ───────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    setSelectedType('gitlab');
    form.resetFields();
    form.setFieldsValue({ engine_type: 'gitlab', enabled: true });
    setFormOpen(true);
  }, [form]);

  const handleEdit = useCallback((cfg: CIEngineConfig) => {
    setEditing(cfg);
    setSelectedType(cfg.engine_type);
    const extra = parseExtra(cfg.extra_json);
    form.setFieldsValue({
      name: cfg.name,
      engine_type: cfg.engine_type,
      enabled: cfg.enabled,
      endpoint: cfg.endpoint,
      auth_type: cfg.auth_type,
      username: cfg.username,
      cluster_id: cfg.cluster_id,
      insecure_skip_verify: cfg.insecure_skip_verify,
      // extra fields
      gitlab_project_id: extra.project_id,
      gitlab_default_ref: extra.default_ref,
      jenkins_job_path: extra.job_path,
      tekton_pipeline_name: extra.pipeline_name,
      tekton_namespace: extra.namespace,
      tekton_service_account: extra.service_account_name,
      argo_namespace: extra.workflow_namespace,
      argo_template_name: extra.workflow_template_name,
      argo_service_account: extra.service_account_name,
      github_owner: extra.owner,
      github_repo: extra.repo,
      github_workflow_id: extra.workflow_id,
      github_ref: extra.ref,
    });
    setFormOpen(true);
  }, [form]);

  const handleTypeChange = useCallback((type: CIEngineType) => {
    setSelectedType(type);
    // Set default auth_type when type changes
    const authDefaults: Partial<Record<CIEngineType, string>> = {
      gitlab: 'token',
      jenkins: 'basic',
      tekton: 'kubeconfig',
      argo: 'kubeconfig',
      github: 'token',
    };
    form.setFieldValue('auth_type', authDefaults[type] ?? 'token');
  }, [form]);

  const handleSubmit = useCallback(async () => {
    const values = await form.validateFields();
    const engineType: CIEngineType = values.engine_type;

    // Build extra_json from type-specific fields
    let extra: Record<string, string> = {};
    if (engineType === 'gitlab') {
      extra = { project_id: values.gitlab_project_id ?? '', default_ref: values.gitlab_default_ref ?? 'main' };
    } else if (engineType === 'jenkins') {
      extra = { job_path: values.jenkins_job_path ?? '' };
    } else if (engineType === 'tekton') {
      extra = {
        pipeline_name: values.tekton_pipeline_name ?? '',
        namespace: values.tekton_namespace ?? 'default',
        service_account_name: values.tekton_service_account ?? 'pipeline',
      };
    } else if (engineType === 'argo') {
      extra = {
        workflow_namespace: values.argo_namespace ?? 'argo',
        workflow_template_name: values.argo_template_name ?? '',
        service_account_name: values.argo_service_account ?? 'argo',
      };
    } else if (engineType === 'github') {
      extra = {
        owner: values.github_owner ?? '',
        repo: values.github_repo ?? '',
        workflow_id: values.github_workflow_id ?? '',
        ref: values.github_ref ?? 'main',
      };
    }

    const req: CreateCIEngineRequest = {
      name: values.name,
      engine_type: engineType,
      enabled: values.enabled ?? true,
      endpoint: values.endpoint,
      auth_type: values.auth_type,
      username: values.username,
      token: values.token,
      password: values.password,
      cluster_id: values.cluster_id,
      insecure_skip_verify: values.insecure_skip_verify ?? false,
      ca_bundle: values.ca_bundle,
      extra_json: buildExtra(engineType, extra),
    };

    if (editing) {
      updateMutation.mutate({ id: editing.id, data: req });
    } else {
      createMutation.mutate(req);
    }
  }, [form, editing, createMutation, updateMutation]);

  // Sync selectedType when form engine_type changes
  useEffect(() => {
    if (!formOpen) return;
    const cur = form.getFieldValue('engine_type') as CIEngineType;
    if (cur) setSelectedType(cur);
  }, [formOpen, form]);

  // ─── Columns ─────────────────────────────────────────────────────────────

  const columns: TableColumnsType<CIEngineConfig> = [
    {
      title: t('cicd:ciEngine.table.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t('cicd:ciEngine.table.engineType'),
      dataIndex: 'engine_type',
      key: 'engine_type',
      width: 150,
      render: (type: CIEngineType) => (
        <Tag color={ENGINE_TYPE_COLORS[type] ?? 'default'}>
          {t(`cicd:ciEngine.engineType.${type}`, { defaultValue: type })}
        </Tag>
      ),
    },
    {
      title: t('cicd:ciEngine.table.endpoint'),
      dataIndex: 'endpoint',
      key: 'endpoint',
      ellipsis: true,
      render: (ep: string) =>
        ep ? (
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{ep}</Text>
        ) : (
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>—</Text>
        ),
    },
    {
      title: t('cicd:ciEngine.table.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (v: boolean) => <Tag color={v ? 'success' : 'default'}>{v ? 'ON' : 'OFF'}</Tag>,
    },
    {
      title: t('cicd:ciEngine.table.health'),
      key: 'health',
      width: 100,
      render: (_: unknown, record: CIEngineConfig) => {
        if (!record.last_checked_at) {
          return <Badge status="default" text={t('cicd:ciEngine.health.unknown')} />;
        }
        return record.last_healthy
          ? <Badge status="success" text={t('cicd:ciEngine.health.healthy')} />
          : <Badge status="error" text={t('cicd:ciEngine.health.unhealthy')} />;
      },
    },
    {
      title: t('cicd:ciEngine.table.version'),
      dataIndex: 'last_version',
      key: 'last_version',
      width: 100,
      render: (v: string) => v
        ? <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{v}</Text>
        : <Text type="secondary">—</Text>,
    },
    {
      title: t('cicd:ciEngine.table.lastChecked'),
      dataIndex: 'last_checked_at',
      key: 'last_checked_at',
      width: 150,
      render: (time?: string) => time
        ? <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{dayjs(time).format('YYYY-MM-DD HH:mm')}</Text>
        : <Text type="secondary">—</Text>,
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 100,
      fixed: 'right',
      render: (_: unknown, record: CIEngineConfig) => (
        <Space size={0}>
          <Tooltip title={t('cicd:ciEngine.triggerRun.trigger')}>
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => setTriggerTarget(record)}
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

  // ─── Engine-type specific form fields ─────────────────────────────────────

  const renderExtraFields = () => {
    switch (selectedType) {
      case 'gitlab':
        return (
          <>
            <Divider orientation="left" style={{ fontSize: token.fontSizeSM }}>
              {t('cicd:ciEngine.form.extraSection')}
            </Divider>
            <Form.Item name="gitlab_project_id" label={t('cicd:ciEngine.form.gitlabProjectId')}
              rules={[{ required: true, message: t('common:validation.required') }]}>
              <Input placeholder={t('cicd:ciEngine.form.gitlabProjectIdPlaceholder')} />
            </Form.Item>
            <Form.Item name="gitlab_default_ref" label={t('cicd:ciEngine.form.gitlabDefaultRef')}>
              <Input placeholder={t('cicd:ciEngine.form.gitlabDefaultRefPlaceholder')} />
            </Form.Item>
          </>
        );
      case 'jenkins':
        return (
          <>
            <Divider orientation="left" style={{ fontSize: token.fontSizeSM }}>
              {t('cicd:ciEngine.form.extraSection')}
            </Divider>
            <Form.Item name="jenkins_job_path" label={t('cicd:ciEngine.form.jenkinsJobPath')}
              tooltip={t('cicd:ciEngine.form.jenkinsJobPathTooltip')}
              rules={[{ required: true, message: t('common:validation.required') }]}>
              <Input placeholder={t('cicd:ciEngine.form.jenkinsJobPathPlaceholder')} />
            </Form.Item>
          </>
        );
      case 'tekton':
        return (
          <>
            <Divider orientation="left" style={{ fontSize: token.fontSizeSM }}>
              {t('cicd:ciEngine.form.extraSection')}
            </Divider>
            <Form.Item name="tekton_pipeline_name" label={t('cicd:ciEngine.form.tektonPipelineName')}
              rules={[{ required: true, message: t('common:validation.required') }]}>
              <Input placeholder={t('cicd:ciEngine.form.tektonPipelineNamePlaceholder')} />
            </Form.Item>
            <Form.Item name="tekton_namespace" label={t('cicd:ciEngine.form.tektonNamespace')}>
              <Input placeholder={t('cicd:ciEngine.form.tektonNamespacePlaceholder')} />
            </Form.Item>
            <Form.Item name="tekton_service_account" label={t('cicd:ciEngine.form.tektonServiceAccount')}>
              <Input placeholder={t('cicd:ciEngine.form.tektonServiceAccountPlaceholder')} />
            </Form.Item>
          </>
        );
      case 'argo':
        return (
          <>
            <Divider orientation="left" style={{ fontSize: token.fontSizeSM }}>
              {t('cicd:ciEngine.form.extraSection')}
            </Divider>
            <Form.Item name="argo_namespace" label={t('cicd:ciEngine.form.argoNamespace')}>
              <Input placeholder={t('cicd:ciEngine.form.argoNamespacePlaceholder')} />
            </Form.Item>
            <Form.Item name="argo_template_name" label={t('cicd:ciEngine.form.argoTemplateName')}
              rules={[{ required: true, message: t('common:validation.required') }]}>
              <Input placeholder={t('cicd:ciEngine.form.argoTemplateNamePlaceholder')} />
            </Form.Item>
            <Form.Item name="argo_service_account" label={t('cicd:ciEngine.form.argoServiceAccount')}>
              <Input placeholder={t('cicd:ciEngine.form.argoServiceAccountPlaceholder')} />
            </Form.Item>
          </>
        );
      case 'github':
        return (
          <>
            <Divider orientation="left" style={{ fontSize: token.fontSizeSM }}>
              {t('cicd:ciEngine.form.extraSection')}
            </Divider>
            <Form.Item name="github_owner" label={t('cicd:ciEngine.form.githubOwner')}
              rules={[{ required: true, message: t('common:validation.required') }]}>
              <Input placeholder={t('cicd:ciEngine.form.githubOwnerPlaceholder')} />
            </Form.Item>
            <Form.Item name="github_repo" label={t('cicd:ciEngine.form.githubRepo')}
              rules={[{ required: true, message: t('common:validation.required') }]}>
              <Input placeholder={t('cicd:ciEngine.form.githubRepoPlaceholder')} />
            </Form.Item>
            <Form.Item name="github_workflow_id" label={t('cicd:ciEngine.form.githubWorkflowId')}
              rules={[{ required: true, message: t('common:validation.required') }]}>
              <Input placeholder={t('cicd:ciEngine.form.githubWorkflowIdPlaceholder')} />
            </Form.Item>
            <Form.Item name="github_ref" label={t('cicd:ciEngine.form.githubRef')}>
              <Input placeholder={t('cicd:ciEngine.form.githubRefPlaceholder')} />
            </Form.Item>
          </>
        );
      default:
        return null;
    }
  };

  // ─── Render ───────────────────────────────────────────────────────────────

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

      <Table<CIEngineConfig>
        columns={columns}
        dataSource={engines}
        rowKey="id"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (n) => t('common:pagination.total', { total: n }),
        }}
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
      />

      {/* Trigger Run Modal */}
      {triggerTarget && (
        <TriggerRunModal
          open={!!triggerTarget}
          engineId={triggerTarget.id}
          engineName={triggerTarget.name}
          onClose={() => setTriggerTarget(null)}
          onTriggered={(runId) => {
            setTriggerTarget(null);
            setRunViewer({ engine: triggerTarget, runId });
          }}
        />
      )}

      {/* Run Viewer Drawer */}
      {runViewer && (
        <CIEngineRunViewer
          open={!!runViewer}
          engineId={runViewer.engine.id}
          engineName={runViewer.engine.name}
          runId={runViewer.runId}
          onClose={() => setRunViewer(null)}
        />
      )}

      {/* Create / Edit Modal */}
      <Modal
        title={
          editing
            ? t('cicd:ciEngine.form.editTitle', { name: editing.name })
            : t('cicd:ciEngine.form.createTitle')
        }
        open={formOpen}
        onCancel={() => setFormOpen(false)}
        onOk={handleSubmit}
        okText={editing ? t('common:actions.save') : t('common:actions.create')}
        cancelText={t('common:actions.cancel')}
        confirmLoading={createMutation.isPending || updateMutation.isPending}
        destroyOnHidden
        width={580}
      >
        <Form
          form={form}
          layout="vertical"
          style={{ marginTop: token.marginMD }}
        >
          {/* ── Basic fields ─────────────────────────────────────────────── */}
          <Form.Item
            name="name"
            label={t('cicd:ciEngine.form.name')}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:ciEngine.form.namePlaceholder')} />
          </Form.Item>

          {/* Engine type — read-only when editing */}
          <Form.Item
            name="engine_type"
            label={t('cicd:ciEngine.form.engineType')}
            tooltip={editing ? t('cicd:ciEngine.form.engineTypeTooltip') : undefined}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Select
              disabled={!!editing}
              onChange={handleTypeChange}
              options={(['gitlab', 'jenkins', 'tekton', 'argo', 'github'] as CIEngineType[]).map((v) => ({
                label: (
                  <Space>
                    <Tag color={ENGINE_TYPE_COLORS[v]} style={{ margin: 0 }}>
                      {t(`cicd:ciEngine.engineType.${v}`)}
                    </Tag>
                  </Space>
                ),
                value: v,
              }))}
            />
          </Form.Item>

          {editing && (
            <Form.Item name="enabled" label={t('cicd:ciEngine.form.enabled')} valuePropName="checked">
              <Switch />
            </Form.Item>
          )}

          {/* ── Connection ──────────────────────────────────────────────── */}
          {HAS_ENDPOINT.includes(selectedType) && (
            <Form.Item
              name="endpoint"
              label={t('cicd:ciEngine.form.endpoint')}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Input placeholder={t('cicd:ciEngine.form.endpointPlaceholder')} />
            </Form.Item>
          )}

          {/* Cluster selector for K8s-native engines */}
          {HAS_CLUSTER.includes(selectedType) && (
            <Form.Item
              name="cluster_id"
              label={t('cicd:ciEngine.form.clusterId')}
              tooltip={t('cicd:ciEngine.form.clusterIdTooltip')}
              rules={[{ required: true, message: t('common:validation.required') }]}
            >
              <Select
                placeholder={t('cicd:ciEngine.form.selectCluster')}
                options={clusters.map((c) => ({ label: c.name, value: Number(c.id) }))}
                showSearch
                filterOption={(input, option) =>
                  String(option?.label ?? '').toLowerCase().includes(input.toLowerCase())
                }
              />
            </Form.Item>
          )}

          {/* ── Auth ────────────────────────────────────────────────────── */}
          {HAS_TOKEN.includes(selectedType) && (
            <Form.Item
              name="token"
              label={t('cicd:ciEngine.form.token')}
              rules={editing ? [] : [{ required: true, message: t('common:validation.required') }]}
            >
              <Input.Password placeholder={t('cicd:ciEngine.form.tokenPlaceholder')} />
            </Form.Item>
          )}

          {HAS_BASIC.includes(selectedType) && (
            <>
              <Form.Item name="username" label={t('cicd:ciEngine.form.username')}>
                <Input placeholder={t('cicd:ciEngine.form.usernamePlaceholder')} />
              </Form.Item>
              <Form.Item
                name="token"
                label={t('cicd:ciEngine.form.token')}
                rules={editing ? [] : [{ required: true, message: t('common:validation.required') }]}
              >
                <Input.Password placeholder={t('cicd:ciEngine.form.tokenPlaceholder')} />
              </Form.Item>
            </>
          )}

          {/* ── Engine-specific extra fields ────────────────────────────── */}
          {renderExtraFields()}

          {/* ── TLS ─────────────────────────────────────────────────────── */}
          {HAS_ENDPOINT.includes(selectedType) && (
            <>
              <Divider orientation="left" style={{ fontSize: token.fontSizeSM }}>
                {t('cicd:ciEngine.form.tlsSection')}
              </Divider>
              <Form.Item
                name="insecure_skip_verify"
                label={t('cicd:ciEngine.form.insecureSkipVerify')}
                tooltip={t('cicd:ciEngine.form.insecureSkipVerifyTooltip')}
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
              <Form.Item name="ca_bundle" label={t('cicd:ciEngine.form.caBundle')}>
                <Input.TextArea
                  rows={3}
                  placeholder={t('cicd:ciEngine.form.caBundlePlaceholder')}
                />
              </Form.Item>
            </>
          )}

          {!editing && selectedType !== 'native' && (
            <Alert
              type="info"
              showIcon
              icon={<ThunderboltOutlined />}
              message={t('cicd:ciEngine.nativeNotice')}
              style={{ marginTop: token.marginSM, display: selectedType === 'native' ? 'block' : 'none' }}
            />
          )}
        </Form>
      </Modal>
    </>
  );
};

export default CIEngineSettings;

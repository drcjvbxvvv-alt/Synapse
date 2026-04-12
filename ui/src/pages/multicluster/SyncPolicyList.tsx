import React, { useEffect, useState } from 'react';
import {
  Alert,
  Badge,
  Button,
  Drawer,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  App,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  PlayCircleOutlined,
  HistoryOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { multiclusterService, type SyncPolicy, type SyncHistory } from '../../services/multiclusterService';
import { clusterService } from '../../services/clusterService';
import { namespaceService } from '../../services/namespaceService';
import type { Cluster } from '../../types';
import type { TablePaginationConfig } from 'antd/es/table';

const { Text } = Typography;

// 將 JSON 字串陣列轉換成顯示用標籤陣列
function parseJsonArray(s: string): string[] {
  try { return JSON.parse(s) ?? []; } catch { return []; }
}

const statusColor: Record<string, string> = {
  success: 'success',
  partial: 'warning',
  failed: 'error',
};

const SyncPolicyList: React.FC = () => {
  const { t } = useTranslation(['multicluster', 'common']);
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [policies, setPolicies] = useState<SyncPolicy[]>([]);
  const [clusters, setClusters] = useState<Cluster[]>([]);

  // Editor drawer
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<SyncPolicy | null>(null);
  const [form] = Form.useForm();
  const [saving, setSaving] = useState(false);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [configmapNames, setConfigmapNames] = useState<string[]>([]);
  const [secretNames, setSecretNames] = useState<string[]>([]);

  // History drawer
  const [historyOpen, setHistoryOpen] = useState(false);
  const [historyPolicy, setHistoryPolicy] = useState<SyncPolicy | null>(null);
  const [history, setHistory] = useState<SyncHistory[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);

  const fetchPolicies = async () => {
    setLoading(true);
    try {
      const res = await multiclusterService.listSyncPolicies();
      setPolicies((res as any)?.items ?? []);
    } catch {
      message.error(t('multicluster:messages.syncPolicyListError'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPolicies();
    clusterService.getClusters({ pageSize: 100 }).then((res: any) => {
      setClusters(res?.items ?? res?.data?.items ?? []);
    }).catch(() => {});
  }, []);

  const clusterName = (id: number) => clusters.find(c => Number(c.id) === id)?.name ?? String(id);

  const loadNamespaces = async (cid: number) => {
    if (!cid) return;
    try {
      const res = await namespaceService.getNamespaces(String(cid)) as any;
      setNamespaces((res?.items ?? []).map((n: any) => n.name));
    } catch { setNamespaces([]); }
  };

  const loadResourceNames = async (cid: number, ns: string, type: string) => {
    if (!cid || !ns) return;
    try {
      const { request } = await import('../../utils/api');
      if (type === 'ConfigMap') {
        const res = await request.get(`/clusters/${cid}/configmaps?namespace=${ns}&pageSize=200`) as any;
        setConfigmapNames((res?.items ?? []).map((i: any) => i.name));
      } else if (type === 'Secret') {
        const res = await request.get(`/clusters/${cid}/secrets?namespace=${ns}&pageSize=200`) as any;
        setSecretNames((res?.items ?? []).map((i: any) => i.name));
      }
    } catch { setConfigmapNames([]); setSecretNames([]); }
  };

  const handleEdit = (policy?: SyncPolicy) => {
    setEditing(policy ?? null);
    if (policy) {
      form.setFieldsValue({
        ...policy,
        resource_names: parseJsonArray(policy.resource_names),
        target_clusters: parseJsonArray(policy.target_clusters).map(Number),
      });
      loadNamespaces(policy.source_cluster_id);
      loadResourceNames(policy.source_cluster_id, policy.source_namespace, policy.resource_type);
    } else {
      form.resetFields();
    }
    setDrawerOpen(true);
  };

  const handleSave = async () => {
    let values: any;
    try { values = await form.validateFields(); } catch { return; }
    setSaving(true);
    try {
      values.resource_names = JSON.stringify(values.resource_names ?? []);
      values.target_clusters = JSON.stringify((values.target_clusters ?? []).map(Number));
      if (editing?.id) {
        await multiclusterService.updateSyncPolicy(editing.id, values);
        message.success(t('common:messages.success'));
      } else {
        await multiclusterService.createSyncPolicy(values);
        message.success(t('common:messages.success'));
      }
      setDrawerOpen(false);
      fetchPolicies();
    } catch {
      message.error(t('multicluster:messages.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = (policy: SyncPolicy) => {
    modal.confirm({
      title: t('multicluster:messages.deleteConfirmTitle'),
      content: t('multicluster:messages.deleteConfirmContent', { name: policy.name }),
      okType: 'danger',
      onOk: async () => {
        await multiclusterService.deleteSyncPolicy(policy.id!);
        message.success(t('multicluster:messages.deleteSuccess'));
        fetchPolicies();
      },
    });
  };

  const handleTrigger = async (policy: SyncPolicy) => {
    try {
      await multiclusterService.triggerSync(policy.id!);
      message.success(t('multicluster:messages.syncTriggered'));
      fetchPolicies();
    } catch {
      message.error(t('multicluster:messages.triggerFailed'));
    }
  };

  const handleHistory = async (policy: SyncPolicy) => {
    setHistoryPolicy(policy);
    setHistoryOpen(true);
    setHistoryLoading(true);
    try {
      const res = await multiclusterService.getSyncHistory(policy.id!) as any;
      setHistory(res?.items ?? []);
    } catch {
      message.error(t('multicluster:messages.historyLoadError'));
    } finally {
      setHistoryLoading(false);
    }
  };

  const resourceTypeOptions = [
    { value: 'ConfigMap', label: 'ConfigMap' },
    { value: 'Secret', label: 'Secret' },
  ];

  const columns = [
    {
      title: t('multicluster:syncPolicy.table.policyName'),
      dataIndex: 'name',
      key: 'name',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t('multicluster:syncPolicy.table.source'),
      key: 'source',
      render: (_: any, r: SyncPolicy) => (
        <Space size={4} wrap>
          <Tag color="blue">{clusterName(r.source_cluster_id)}</Tag>
          <Tag>{r.source_namespace}</Tag>
          <Tag color="purple">{r.resource_type}</Tag>
        </Space>
      ),
    },
    {
      title: t('multicluster:syncPolicy.table.targetCluster'),
      dataIndex: 'target_clusters',
      render: (v: string) => (
        <Space wrap>
          {parseJsonArray(v).map(id => (
            <Tag key={id} color="cyan">{clusterName(Number(id))}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t('multicluster:syncPolicy.table.conflictPolicy'),
      dataIndex: 'conflict_policy',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('multicluster:syncPolicy.table.schedule'),
      dataIndex: 'schedule',
      render: (v: string) => v ? <Tag color="geekblue">{v}</Tag> : <Text type="secondary">{t('multicluster:syncPolicy.table.manual')}</Text>,
    },
    {
      title: t('multicluster:syncPolicy.table.status'),
      key: 'status',
      render: (_: any, r: SyncPolicy) => (
        <Space>
          <Switch
            checked={r.enabled}
            size="small"
            onChange={async (checked) => {
              await multiclusterService.updateSyncPolicy(r.id!, { ...r, enabled: checked });
              fetchPolicies();
            }}
          />
          {r.last_sync_status && (
            <Badge status={statusColor[r.last_sync_status] as any} text={r.last_sync_status} />
          )}
        </Space>
      ),
    },
    {
      title: t('multicluster:syncPolicy.table.lastSync'),
      dataIndex: 'last_sync_at',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
    {
      title: t('multicluster:syncPolicy.table.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 160,
      render: (_: any, r: SyncPolicy) => (
        <Space>
          <Tooltip title={t('multicluster:syncPolicy.buttons.immediateSync')}>
            <Button size="small" icon={<PlayCircleOutlined />} onClick={() => handleTrigger(r)} />
          </Tooltip>
          <Tooltip title={t('multicluster:syncPolicy.buttons.history')}>
            <Button size="small" icon={<HistoryOutlined />} onClick={() => handleHistory(r)} />
          </Tooltip>
          <Tooltip title={t('multicluster:syncPolicy.buttons.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(r)} />
          </Tooltip>
          <Tooltip title={t('multicluster:syncPolicy.buttons.delete')}>
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(r)} />
          </Tooltip>
        </Space>
      ),
    },
  ];

  const historyColumns = [
    { title: t('multicluster:syncPolicy.historyTable.triggeredBy'), dataIndex: 'triggered_by', key: 'triggered_by', render: (v: string) => <Tag>{v}</Tag> },
    { title: t('multicluster:syncPolicy.historyTable.status'), dataIndex: 'status', key: 'status', render: (v: string) => <Badge status={statusColor[v] as any} text={v} /> },
    { title: t('multicluster:syncPolicy.historyTable.message'), dataIndex: 'message', key: 'message', ellipsis: true },
    { title: t('multicluster:syncPolicy.historyTable.startedAt'), dataIndex: 'started_at', key: 'started_at', render: (v: string) => new Date(v).toLocaleString() },
    {
      title: t('multicluster:syncPolicy.historyTable.duration'),
      key: 'duration',
      render: (_: any, r: SyncHistory) => {
        if (!r.finished_at) return '—';
        const ms = new Date(r.finished_at).getTime() - new Date(r.started_at).getTime();
        return `${(ms / 1000).toFixed(1)}s`;
      },
    },
  ];

  // 動態取得資源名稱清單（ConfigMap 或 Secret）
  const watchedClusterId = Form.useWatch('source_cluster_id', form);
  const watchedNS = Form.useWatch('source_namespace', form);
  const watchedType = Form.useWatch('resource_type', form);

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Button icon={<ReloadOutlined />} onClick={fetchPolicies}>{t('multicluster:syncPolicy.buttons.refresh')}</Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => handleEdit()}>{t('multicluster:syncPolicy.buttons.newPolicy')}</Button>
      </Space>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={policies}
        loading={loading}
        scroll={{ x: 1000 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      {/* 編輯 Drawer */}
      <Drawer
        title={editing ? t('multicluster:syncPolicy.drawer.editTitle', { name: editing.name }) : t('multicluster:syncPolicy.drawer.createTitle')}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={520}
        extra={
          <Button type="primary" loading={saving} onClick={handleSave}>{t('multicluster:syncPolicy.buttons.save')}</Button>
        }
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('multicluster:syncPolicy.form.policyName')} rules={[{ required: true }]}>
            <Input placeholder={t('multicluster:syncPolicy.form.policyNamePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('multicluster:syncPolicy.form.description')}>
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="source_cluster_id" label={t('multicluster:syncPolicy.form.sourceCluster')} rules={[{ required: true }]}>
            <Select
              placeholder={t('multicluster:syncPolicy.form.selectCluster')}
              options={clusters.map(c => ({ value: c.id, label: c.name }))}
              showSearch
              optionFilterProp="label"
              onChange={(v) => {
                form.setFieldValue('source_namespace', undefined);
                form.setFieldValue('resource_names', []);
                loadNamespaces(v);
              }}
            />
          </Form.Item>
          <Form.Item name="source_namespace" label={t('multicluster:syncPolicy.form.sourceNamespace')} rules={[{ required: true }]}>
            <Select
              placeholder={t('multicluster:syncPolicy.form.selectNamespace')}
              options={namespaces.map(n => ({ value: n, label: n }))}
              disabled={!watchedClusterId}
              showSearch
              onChange={(ns) => {
                form.setFieldValue('resource_names', []);
                if (watchedClusterId && watchedType) loadResourceNames(watchedClusterId, ns, watchedType);
              }}
            />
          </Form.Item>
          <Form.Item name="resource_type" label={t('multicluster:syncPolicy.form.resourceType')} rules={[{ required: true }]}>
            <Select
              options={resourceTypeOptions}
              onChange={(type) => {
                form.setFieldValue('resource_names', []);
                if (watchedClusterId && watchedNS) loadResourceNames(watchedClusterId, watchedNS, type);
              }}
            />
          </Form.Item>
          <Form.Item name="resource_names" label={t('multicluster:syncPolicy.form.resourceNames')}>
            <Select
              mode="multiple"
              placeholder={t('multicluster:syncPolicy.form.selectResources')}
              options={(watchedType === 'ConfigMap' ? configmapNames : secretNames).map(n => ({ value: n, label: n }))}
              disabled={!watchedNS}
            />
          </Form.Item>
          <Form.Item name="target_clusters" label={t('multicluster:syncPolicy.form.targetClusters')} rules={[{ required: true }]}>
            <Select
              mode="multiple"
              placeholder={t('multicluster:syncPolicy.form.selectTargetCluster')}
              options={clusters.filter(c => Number(c.id) !== watchedClusterId).map(c => ({ value: Number(c.id), label: c.name }))}
              showSearch
              optionFilterProp="label"
            />
          </Form.Item>
          <Form.Item name="conflict_policy" label={t('multicluster:syncPolicy.form.conflictPolicy')} initialValue="skip">
            <Select options={[{ value: 'skip', label: t('multicluster:syncPolicy.form.conflictPolicySkip') }, { value: 'overwrite', label: t('multicluster:syncPolicy.form.conflictPolicyOverwrite') }]} />
          </Form.Item>
          <Form.Item name="schedule" label={t('multicluster:syncPolicy.form.schedule')}>
            <Input placeholder={t('multicluster:syncPolicy.form.schedulePlaceholder')} />
          </Form.Item>
          <Form.Item name="enabled" label={t('multicluster:syncPolicy.form.enabled')} valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
        </Form>
      </Drawer>

      {/* 歷史紀錄 Drawer */}
      <Drawer
        title={t('multicluster:syncPolicy.drawer.historyTitle', { name: historyPolicy?.name ?? '' })}
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
        width={700}
      >
        <Table
          scroll={{ x: 'max-content' }}
          rowKey="id"
          columns={historyColumns}
          dataSource={history}
          loading={historyLoading}
          pagination={{ pageSize: 20 }}
          expandable={{
            expandedRowRender: (r: SyncHistory) => {
              try {
                const details = JSON.parse(r.details);
                return (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    {details.map((d: any, i: number) => (
                      <Alert
                        key={i}
                        type={d.status === 'success' ? 'success' : 'error'}
                        message={`叢集 ${d.clusterId}：${d.message}`}
                        showIcon
                      />
                    ))}
                  </Space>
                );
              } catch {
                return <Text>{r.details}</Text>;
              }
            },
          }}
        />
      </Drawer>
    </div>
  );
};

export default SyncPolicyList;

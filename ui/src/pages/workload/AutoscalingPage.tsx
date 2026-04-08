import React, { useState, useEffect, useCallback } from 'react';
import {
  Tabs, Table, Tag, Button, Space, Spin, Alert, Typography, Tooltip,
  Badge, Select, App,
} from 'antd';
import {
  ReloadOutlined, ThunderboltOutlined, NodeIndexOutlined,
  ClusterOutlined, RocketOutlined,
} from '@ant-design/icons';
import NotInstalledCard from '../../components/NotInstalledCard';
import { useTranslation } from 'react-i18next';
import { useParams } from 'react-router-dom';
import {
  autoscalingService,
  type KEDAStatus, type KarpenterStatus,
  type ScaledObjectInfo, type ScaledJobInfo,
  type NodePoolInfo, type NodeClaimInfo, type CASStatus,
} from '../../services/autoscalingService';

const { Text, Title } = Typography;

// ─── Trigger type colour map ────────────────────────────────────────────────
const TRIGGER_COLORS: Record<string, string> = {
  kafka: 'blue',
  redis: 'red',
  prometheus: 'orange',
  cron: 'purple',
  rabbitmq: 'green',
  aws: 'gold',
  azure: 'cyan',
  gcp: 'geekblue',
};
const triggerColor = (type: string) => TRIGGER_COLORS[type.toLowerCase()] ?? 'default';

// ═══════════════════════════════════════════════════════════════════════════
// Sub-panels
// ═══════════════════════════════════════════════════════════════════════════

interface PanelProps { clusterId: string }

// ─── HPA Tab ────────────────────────────────────────────────────────────────
const HPATab: React.FC<PanelProps> = ({ clusterId }) => {
  const { t } = useTranslation('workload');
  return (
    <Alert
      type="info"
      showIcon
      message={t('autoscaling.hpa.redirectHint')}
      description={t('autoscaling.hpa.redirectDesc')}
      style={{ maxWidth: 600 }}
    />
  );
};

// ─── KEDA Tab ───────────────────────────────────────────────────────────────
const KEDATab: React.FC<PanelProps> = ({ clusterId }) => {
  const { t } = useTranslation('workload');
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [installed, setInstalled] = useState<boolean | null>(null);
  const [scaledObjects, setScaledObjects] = useState<ScaledObjectInfo[]>([]);
  const [scaledJobs, setScaledJobs] = useState<ScaledJobInfo[]>([]);
  const [ns, setNs] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const statusRes = await autoscalingService.checkKEDA(clusterId);
      const isInstalled = (statusRes.data as KEDAStatus).installed ?? false;
      setInstalled(isInstalled);
      if (isInstalled) {
        const [soRes, sjRes] = await Promise.all([
          autoscalingService.listScaledObjects(clusterId, ns || undefined),
          autoscalingService.listScaledJobs(clusterId, ns || undefined),
        ]);
        setScaledObjects((soRes.data as { items: ScaledObjectInfo[] }).items ?? []);
        setScaledJobs((sjRes.data as { items: ScaledJobInfo[] }).items ?? []);
      }
    } catch {
      message.error(t('autoscaling.keda.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, ns, message, t]);

  useEffect(() => { load(); }, [load]);

  if (loading) return <Spin style={{ display: 'block', marginTop: 60 }} />;

  if (!installed) {
    return (
      <div style={{ paddingTop: 40 }}>
        <NotInstalledCard
          title={t('autoscaling.keda.notInstalled')}
          description={t('autoscaling.keda.installHint')}
          command="helm install keda kedacore/keda --namespace keda --create-namespace"
          docsUrl="https://keda.sh/docs/latest/deploy/"
          onRecheck={load}
          recheckLoading={loading}
        />
      </div>
    );
  }

  const soColumns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name', render: (v: string) => <Text strong>{v}</Text> },
    { title: t('common.namespace'), dataIndex: 'namespace', key: 'namespace', render: (v: string) => <Tag color="blue">{v}</Tag> },
    {
      title: t('autoscaling.keda.target'), key: 'target',
      render: (_: unknown, r: ScaledObjectInfo) => <Text>{r.targetKind}/{r.targetName}</Text>,
    },
    {
      title: t('autoscaling.keda.replicas'), key: 'replicas',
      render: (_: unknown, r: ScaledObjectInfo) => (
        <Space>
          <Tooltip title={t('autoscaling.keda.current')}><Tag>{r.currentReplicas}</Tag></Tooltip>
          <Text type="secondary">/</Text>
          <Tooltip title={t('autoscaling.keda.desired')}><Tag color="blue">{r.desiredReplicas}</Tag></Tooltip>
          <Text type="secondary">({r.minReplicas}–{r.maxReplicas})</Text>
        </Space>
      ),
    },
    {
      title: t('autoscaling.keda.triggers'), dataIndex: 'triggers', key: 'triggers',
      render: (triggers: ScaledObjectInfo['triggers']) => (
        <Space wrap>
          {triggers.map((tr, i) => (
            <Tag key={i} color={triggerColor(tr.type)}>{tr.type}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t('common.createdAt'), dataIndex: 'createdAt', key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
  ];

  const sjColumns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name', render: (v: string) => <Text strong>{v}</Text> },
    { title: t('common.namespace'), dataIndex: 'namespace', key: 'namespace', render: (v: string) => <Tag color="blue">{v}</Tag> },
    {
      title: t('autoscaling.keda.triggers'), dataIndex: 'triggers', key: 'triggers',
      render: (triggers: ScaledJobInfo['triggers']) => (
        <Space wrap>
          {triggers.map((tr, i) => <Tag key={i} color={triggerColor(tr.type)}>{tr.type}</Tag>)}
        </Space>
      ),
    },
    {
      title: t('autoscaling.keda.ready'), dataIndex: 'ready', key: 'ready',
      render: (v: boolean) => <Badge status={v ? 'success' : 'error'} text={v ? t('common.yes') : t('common.no')} />,
    },
    {
      title: t('common.createdAt'), dataIndex: 'createdAt', key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      <Space>
        <Select
          allowClear
          placeholder={t('common.namespace')}
          style={{ width: 200 }}
          value={ns || undefined}
          onChange={v => setNs(v ?? '')}
        />
        <Button icon={<ReloadOutlined />} onClick={load}>{t('common.refresh')}</Button>
      </Space>
      <Title level={5} style={{ marginBottom: 8 }}>{t('autoscaling.keda.scaledObjects')} ({scaledObjects.length})</Title>
      <Table
        rowKey={r => `${r.namespace}/${r.name}`}
        columns={soColumns}
        dataSource={scaledObjects}
        size="small"
        scroll={{ x: 800 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />
      <Title level={5} style={{ marginBottom: 8 }}>{t('autoscaling.keda.scaledJobs')} ({scaledJobs.length})</Title>
      <Table
        rowKey={r => `${r.namespace}/${r.name}`}
        columns={sjColumns}
        dataSource={scaledJobs}
        size="small"
        scroll={{ x: 600 }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />
    </Space>
  );
};

// ─── Karpenter Tab ──────────────────────────────────────────────────────────
const KarpenterTab: React.FC<PanelProps> = ({ clusterId }) => {
  const { t } = useTranslation('workload');
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [installed, setInstalled] = useState<boolean | null>(null);
  const [nodePools, setNodePools] = useState<NodePoolInfo[]>([]);
  const [nodeClaims, setNodeClaims] = useState<NodeClaimInfo[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const statusRes = await autoscalingService.checkKarpenter(clusterId);
      const isInstalled = (statusRes.data as KarpenterStatus).installed ?? false;
      setInstalled(isInstalled);
      if (isInstalled) {
        const [npRes, ncRes] = await Promise.all([
          autoscalingService.listNodePools(clusterId),
          autoscalingService.listNodeClaims(clusterId),
        ]);
        setNodePools((npRes.data as { items: NodePoolInfo[] }).items ?? []);
        setNodeClaims((ncRes.data as { items: NodeClaimInfo[] }).items ?? []);
      }
    } catch {
      message.error(t('autoscaling.karpenter.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  useEffect(() => { load(); }, [load]);

  if (loading) return <Spin style={{ display: 'block', marginTop: 60 }} />;

  if (!installed) {
    return (
      <div style={{ paddingTop: 40 }}>
        <NotInstalledCard
          title={t('autoscaling.karpenter.notInstalled')}
          description={t('autoscaling.karpenter.installHint')}
          command="helm install karpenter oci://public.ecr.aws/karpenter/karpenter --version 1.0.0 -n karpenter --create-namespace"
          docsUrl="https://karpenter.sh/docs/getting-started/getting-started-with-karpenter/"
          onRecheck={load}
          recheckLoading={loading}
        />
      </div>
    );
  }

  const npColumns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name', render: (v: string) => <Text strong>{v}</Text> },
    {
      title: t('autoscaling.karpenter.limits'), dataIndex: 'limits', key: 'limits',
      render: (v: Record<string, string>) => v ? (
        <Space wrap>
          {Object.entries(v).map(([k, val]) => <Tag key={k}>{k}: {val}</Tag>)}
        </Space>
      ) : '—',
    },
    {
      title: t('autoscaling.karpenter.consolidation'), dataIndex: 'consolidationPolicy', key: 'consolidationPolicy',
      render: (v: string) => v ? <Tag color="purple">{v}</Tag> : '—',
    },
    {
      title: t('common.createdAt'), dataIndex: 'createdAt', key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
  ];

  const ncColumns = [
    { title: t('common.name'), dataIndex: 'name', key: 'name', render: (v: string) => <Text strong>{v}</Text> },
    { title: t('autoscaling.karpenter.nodePool'), dataIndex: 'nodePool', key: 'nodePool', render: (v: string) => <Tag color="geekblue">{v}</Tag> },
    { title: t('autoscaling.karpenter.nodeName'), dataIndex: 'nodeName', key: 'nodeName', render: (v: string) => v || '—' },
    {
      title: t('common.createdAt'), dataIndex: 'createdAt', key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      <Button icon={<ReloadOutlined />} onClick={load}>{t('common.refresh')}</Button>
      <Title level={5} style={{ marginBottom: 8 }}>{t('autoscaling.karpenter.nodePools')} ({nodePools.length})</Title>
      <Table rowKey="name" columns={npColumns} dataSource={nodePools} size="small" pagination={{ pageSize: 20 }} />
      <Title level={5} style={{ marginBottom: 8 }}>{t('autoscaling.karpenter.nodeClaims')} ({nodeClaims.length})</Title>
      <Table rowKey="name" columns={ncColumns} dataSource={nodeClaims} size="small" pagination={{ pageSize: 20 }} />
    </Space>
  );
};

// ─── CAS Tab ────────────────────────────────────────────────────────────────
const CASTabPanel: React.FC<PanelProps> = ({ clusterId }) => {
  const { t } = useTranslation('workload');
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<CASStatus | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await autoscalingService.getCASStatus(clusterId);
      setStatus((res.data as CASStatus) ?? null);
    } catch {
      message.error(t('autoscaling.cas.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  useEffect(() => { load(); }, [load]);

  if (loading) return <Spin style={{ display: 'block', marginTop: 60 }} />;

  if (!status?.installed) {
    return (
      <div style={{ paddingTop: 40 }}>
        <NotInstalledCard
          title={t('autoscaling.cas.notInstalled')}
          description={t('autoscaling.cas.installHint')}
          command="helm install cluster-autoscaler autoscaler/cluster-autoscaler --namespace kube-system"
          docsUrl="https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler"
          onRecheck={load}
          recheckLoading={loading}
        />
      </div>
    );
  }

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      <Button icon={<ReloadOutlined />} onClick={load}>{t('common.refresh')}</Button>
      <Space size={24}>
        <div>
          <Text type="secondary">{t('autoscaling.cas.installed')}</Text>
          <div><Badge status="success" text={t('common.yes')} /></div>
        </div>
        <div>
          <Text type="secondary">{t('autoscaling.cas.nodeGroups')}</Text>
          <div><Text strong>{status.nodeGroupCount}</Text></div>
        </div>
      </Space>
      {status.status && (
        <div>
          <Text type="secondary">{t('autoscaling.cas.statusDetail')}</Text>
          <pre style={{
            marginTop: 8, background: '#fafafa', border: '1px solid #f0f0f0',
            borderRadius: 4, padding: 12, fontSize: 12, overflowX: 'auto',
            maxHeight: 400,
          }}>{status.status}</pre>
        </div>
      )}
    </Space>
  );
};

// ═══════════════════════════════════════════════════════════════════════════
// AutoscalingPage
// ═══════════════════════════════════════════════════════════════════════════

const AutoscalingPage: React.FC = () => {
  const { t } = useTranslation('workload');
  const { clusterId } = useParams<{ clusterId: string }>();

  if (!clusterId) return null;

  const items = [
    {
      key: 'hpa',
      label: <Space><RocketOutlined />{t('autoscaling.tabs.hpa')}</Space>,
      children: <HPATab clusterId={clusterId} />,
    },
    {
      key: 'keda',
      label: <Space><ThunderboltOutlined />{t('autoscaling.tabs.keda')}</Space>,
      children: <KEDATab clusterId={clusterId} />,
    },
    {
      key: 'karpenter',
      label: <Space><NodeIndexOutlined />{t('autoscaling.tabs.karpenter')}</Space>,
      children: <KarpenterTab clusterId={clusterId} />,
    },
    {
      key: 'cas',
      label: <Space><ClusterOutlined />{t('autoscaling.tabs.cas')}</Space>,
      children: <CASTabPanel clusterId={clusterId} />,
    },
  ];

  return (
    <div style={{ padding: '0 24px 24px' }}>
      <Title level={4} style={{ marginBottom: 16 }}>{t('autoscaling.pageTitle')}</Title>
      <Tabs items={items} />
    </div>
  );
};

export default AutoscalingPage;

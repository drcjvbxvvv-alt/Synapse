import React, { useState, useEffect, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import {
  Card, Table, Tag, Space, Button, Alert, Tabs, Tooltip, Badge, Empty, Spin,
} from 'antd';
import {
  ReloadOutlined, SafetyCertificateOutlined, ExclamationCircleOutlined,
  CheckCircleOutlined, CloseCircleOutlined, ClockCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import type { TFunction } from 'i18next';
import api from '@/utils/api';

// ─── Types ───────────────────────────────────────────────────────────────────

interface CertificateSummary {
  name: string;
  namespace: string;
  ready: boolean;
  secretName: string;
  issuer: string;
  issuerKind: string;
  dnsNames: string[];
  notBefore: string;
  notAfter: string;
  daysLeft: number;
  status: 'Valid' | 'Expiring' | 'Expired' | 'NotReady';
}

interface IssuerSummary {
  name: string;
  namespace?: string;
  kind: 'Issuer' | 'ClusterIssuer';
  ready: boolean;
  type: string;
}

// ─── Status helpers ───────────────────────────────────────────────────────────

const STATUS_CONFIG = {
  Valid:    { color: 'success', icon: <CheckCircleOutlined /> },
  Expiring: { color: 'warning', icon: <ClockCircleOutlined /> },
  Expired:  { color: 'error',   icon: <CloseCircleOutlined /> },
  NotReady: { color: 'default', icon: <ExclamationCircleOutlined /> },
} as const;

function StatusTag({ status }: { status: CertificateSummary['status'] }) {
  const { t } = useTranslation('security');
  const cfg = STATUS_CONFIG[status] ?? STATUS_CONFIG.NotReady;
  return (
    <Tag color={cfg.color} icon={cfg.icon}>
      {t(`cert.status.${status}`)}
    </Tag>
  );
}

function formatDate(s: string) {
  if (!s) return '—';
  try { return new Date(s).toLocaleDateString('zh-TW'); } catch { return s; }
}

// ─── Certificate columns ──────────────────────────────────────────────────────

const certColumns = (onRefresh: () => void, t: TFunction): ColumnsType<CertificateSummary> => [
  {
    title: t('cert.columns.status'),
    dataIndex: 'status',
    width: 110,
    filters: [
      { text: t('cert.status.Valid'), value: 'Valid' },
      { text: t('cert.status.Expiring'), value: 'Expiring' },
      { text: t('cert.status.Expired'), value: 'Expired' },
      { text: t('cert.status.NotReady'), value: 'NotReady' },
    ],
    onFilter: (v, r) => r.status === v,
    render: (_, r) => <StatusTag status={r.status} />,
  },
  {
    title: t('cert.columns.name'),
    dataIndex: 'name',
    render: (v, r) => (
      <Space direction="vertical" size={0}>
        <span style={{ fontWeight: 500 }}>{v}</span>
        <span style={{ fontSize: 12 }}>{r.namespace}</span>
      </Space>
    ),
  },
  {
    title: t('cert.columns.dnsNames'),
    dataIndex: 'dnsNames',
    render: (names: string[]) =>
      names?.length ? (
        <Tooltip title={names.join(', ')}>
          <span>
            {names[0]}
            {names.length > 1 && <Tag style={{ marginLeft: 4 }}>+{names.length - 1}</Tag>}
          </span>
        </Tooltip>
      ) : '—',
  },
  {
    title: t('cert.columns.issuer'),
    dataIndex: 'issuer',
    render: (v, r) => (
      <Space>
        <Tag color={r.issuerKind === 'ClusterIssuer' ? 'purple' : 'geekblue'}>
          {r.issuerKind || 'Issuer'}
        </Tag>
        <span>{v}</span>
      </Space>
    ),
  },
  { title: t('cert.columns.secret'), dataIndex: 'secretName', render: v => <code>{v}</code> },
  {
    title: t('cert.columns.expiry'),
    dataIndex: 'notAfter',
    sorter: (a, b) => new Date(a.notAfter || 0).getTime() - new Date(b.notAfter || 0).getTime(),
    render: (v, r) => (
      <Space>
        <span>{formatDate(v)}</span>
        {r.daysLeft >= 0 && (
          <Tag color={r.daysLeft <= 7 ? 'red' : r.daysLeft <= 30 ? 'orange' : 'green'}>
            {r.daysLeft}d
          </Tag>
        )}
      </Space>
    ),
  },
];

// ─── Issuer columns ───────────────────────────────────────────────────────────

const issuerColumns = (t: TFunction): ColumnsType<IssuerSummary> => [
  {
    title: t('cert.issuerColumns.ready'),
    dataIndex: 'ready',
    width: 80,
    render: v => v
      ? <Badge status="success" text={t('cert.issuerStatus.ready')} />
      : <Badge status="error" text={t('cert.issuerStatus.notReady')} />,
  },
  { title: t('cert.issuerColumns.name'), dataIndex: 'name', render: (v, r) => (
      <Space direction="vertical" size={0}>
        <span style={{ fontWeight: 500 }}>{v}</span>
        {r.namespace && <span style={{ fontSize: 12 }}>{r.namespace}</span>}
      </Space>
    ),
  },
  {
    title: t('cert.issuerColumns.kind'),
    dataIndex: 'kind',
    render: (v: string) => (
      <Tag color={v === 'ClusterIssuer' ? 'purple' : 'geekblue'}>{v}</Tag>
    ),
  },
  { title: t('cert.issuerColumns.backend'), dataIndex: 'type', render: v => v || '—' },
];

// ─── Main page ────────────────────────────────────────────────────────────────

const CertificateList: React.FC = () => {
  const { id: clusterId } = useParams<{ id: string }>();
  const { t } = useTranslation('security');

  const [installed, setInstalled] = useState<boolean | null>(null);
  const [certs, setCerts] = useState<CertificateSummary[]>([]);
  const [issuers, setIssuers] = useState<IssuerSummary[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!clusterId) return;
    setLoading(true);
    try {
      // Check if cert-manager is installed
      const statusRes = await api.get<{ installed: boolean }>(
        `/clusters/${clusterId}/cert-manager/status`
      );
      const isInstalled = statusRes.data.installed;
      setInstalled(isInstalled);

      if (!isInstalled) {
        setLoading(false);
        return;
      }

      const [certRes, issuerRes] = await Promise.all([
        api.get<{ items: CertificateSummary[] }>(`/clusters/${clusterId}/cert-manager/certificates`),
        api.get<{ items: IssuerSummary[] }>(`/clusters/${clusterId}/cert-manager/issuers`),
      ]);
      setCerts(certRes.data.items ?? []);
      setIssuers(issuerRes.data.items ?? []);
    } catch (err) {
      console.error('cert-manager load failed', err);
      setInstalled(false);
    } finally {
      setLoading(false);
    }
  }, [clusterId]);

  useEffect(() => { load(); }, [load]);

  // Stats for header
  const expiring = certs.filter(c => c.status === 'Expiring').length;
  const expired  = certs.filter(c => c.status === 'Expired').length;

  if (installed === null) {
    return (
      <div style={{ textAlign: 'center', padding: '80px 0' }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!installed) {
    return (
      <div style={{ padding: '24px' }}>
        <Alert
          type="info"
          showIcon
          icon={<SafetyCertificateOutlined />}
          message={t('cert.notDetected')}
          description={
            <div>
              <p style={{ marginBottom: 8 }}>
                {t('cert.installDesc')}
              </p>
              <p style={{ marginBottom: 12 }}>
                <code>kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml</code>
              </p>
              <Button
                icon={<ReloadOutlined />}
                loading={loading}
                onClick={load}
              >
                {t('cert.reinstallBtn')}
              </Button>
            </div>
          }
        />
      </div>
    );
  }

  const tabItems = [
    {
      key: 'certificates',
      label: (
        <Space>
          <SafetyCertificateOutlined />
          {t('cert.tabCerts')}
          {expired > 0 && <Tag color="red">{expired} {t('cert.status.Expired')}</Tag>}
          {expiring > 0 && <Tag color="orange">{expiring} {t('cert.status.Expiring')}</Tag>}
        </Space>
      ),
      children: (
        <Table<CertificateSummary>
          columns={certColumns(load, t)}
          dataSource={certs}
          rowKey={r => `${r.namespace}/${r.name}`}
          size="middle"
          loading={loading}
          locale={{ emptyText: <Empty description={t('cert.emptyCerts')} /> }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />
      ),
    },
    {
      key: 'issuers',
      label: t('cert.tabIssuers'),
      children: (
        <Table<IssuerSummary>
          columns={issuerColumns(t)}
          dataSource={issuers}
          rowKey={r => `${r.kind}/${r.namespace ?? 'cluster'}/${r.name}`}
          size="middle"
          loading={loading}
          locale={{ emptyText: <Empty description={t('cert.emptyIssuers')} /> }}
          pagination={false}
        />
      ),
    },
  ];

  return (
    <div style={{ padding: '16px 24px' }}>
      <Card
        title={
          <Space>
            <SafetyCertificateOutlined />
            <span>{t('cert.pageTitle')}</span>
          </Space>
        }
        extra={
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
            {t('common:actions.refresh')}
          </Button>
        }
        styles={{ body: { padding: 0 } }}
      >
        <Tabs items={tabItems} style={{ padding: '0 16px' }} />
      </Card>
    </div>
  );
};

export default CertificateList;

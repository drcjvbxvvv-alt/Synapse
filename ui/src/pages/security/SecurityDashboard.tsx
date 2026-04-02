import React, { useState, useEffect, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Tabs,
  Table,
  Button,
  Tag,
  Space,
  Spin,
  Alert,
  Modal,
  Input,
  Statistic,
  Row,
  Col,
  Card,
  Progress,
  Collapse,
  Typography,
  Tooltip,
  Badge,
  message,
} from 'antd';
import {
  SafetyOutlined,
  ScanOutlined,
  ReloadOutlined,
  ExclamationCircleOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import securityService from '@/services/securityService';
import type {
  ScanResult,
  ScanDetail,
  BenchResult,
  BenchDetail,
  GatekeeperSummary,
  ConstraintSummary,
  TrivyVulnerability,
  TrivyReport,
} from '@/services/securityService';

const { Title, Text } = Typography;
const { Panel } = Collapse;

const SEVERITY_COLORS: Record<string, string> = {
  CRITICAL: '#cf1322',
  HIGH: '#d46b08',
  MEDIUM: '#d48806',
  LOW: '#52c41a',
  UNKNOWN: '#8c8c8c',
};

function SeverityTag({ severity, count }: { severity: string; count: number }) {
  if (count === 0) return null;
  return (
    <Tag color={SEVERITY_COLORS[severity] ?? 'default'} style={{ fontWeight: 600 }}>
      {severity[0]}: {count}
    </Tag>
  );
}

// ─── Image Scan Tab ─────────────────────────────────────────────────────────

function ImageScanTab({ clusterId }: { clusterId: number }) {
  const { t } = useTranslation('security');
  const [results, setResults] = useState<ScanResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [scanModalOpen, setScanModalOpen] = useState(false);
  const [imageInput, setImageInput] = useState('');
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<ScanDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const fetchResults = useCallback(async () => {
    setLoading(true);
    try {
      const data = await securityService.getScanResults(clusterId);
      setResults(data);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [clusterId]);

  useEffect(() => {
    fetchResults();
  }, [fetchResults]);

  const handleTrigger = async () => {
    if (!imageInput.trim()) return;
    try {
      await securityService.triggerScan(clusterId, imageInput.trim());
      message.success(t('scan.triggerSuccess'));
      setScanModalOpen(false);
      setImageInput('');
      setTimeout(fetchResults, 1000);
    } catch (e: any) {
      message.error(e?.message ?? 'error');
    }
  };

  const handleViewDetail = async (scanId: number) => {
    setDetailOpen(true);
    setDetailLoading(true);
    try {
      const data = await securityService.getScanDetail(clusterId, scanId);
      setDetail(data);
    } catch {
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const statusColor: Record<string, string> = {
    pending: 'default',
    scanning: 'processing',
    completed: 'success',
    failed: 'error',
  };

  const columns: ColumnsType<ScanResult> = [
    {
      title: t('scan.image'),
      dataIndex: 'image',
      key: 'image',
      ellipsis: true,
    },
    {
      title: t('scan.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      render: (v) => v || '-',
    },
    {
      title: t('scan.pod'),
      dataIndex: 'pod_name',
      key: 'pod_name',
      ellipsis: true,
      render: (v) => v || '-',
    },
    {
      title: t('scan.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (v) => <Badge status={statusColor[v] as any} text={t(`scan.${v}`)} />,
    },
    {
      title: t('scan.critical') + ' / ' + t('scan.high') + ' / ' + t('scan.medium'),
      key: 'vulns',
      width: 220,
      render: (_, r) => (
        <Space>
          <SeverityTag severity="CRITICAL" count={r.critical} />
          <SeverityTag severity="HIGH" count={r.high} />
          <SeverityTag severity="MEDIUM" count={r.medium} />
          <SeverityTag severity="LOW" count={r.low} />
        </Space>
      ),
    },
    {
      title: t('scan.scannedAt'),
      dataIndex: 'scanned_at',
      key: 'scanned_at',
      width: 160,
      render: (v) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: '',
      key: 'action',
      width: 100,
      render: (_, r) =>
        r.status === 'completed' ? (
          <Button size="small" onClick={() => handleViewDetail(r.id)}>
            {t('scan.viewDetail')}
          </Button>
        ) : null,
    },
  ];

  // Parse vulnerabilities from detail JSON
  let vulns: TrivyVulnerability[] = [];
  if (detail?.result_json) {
    try {
      const report: TrivyReport = JSON.parse(detail.result_json);
      vulns = (report.Results ?? []).flatMap((r) => r.Vulnerabilities ?? []);
    } catch {
      vulns = [];
    }
  }

  const vulnColumns: ColumnsType<TrivyVulnerability> = [
    { title: t('scan.detail.vulnId'), dataIndex: 'VulnerabilityID', key: 'id', width: 160 },
    { title: t('scan.detail.package'), dataIndex: 'PkgName', key: 'pkg', width: 140 },
    { title: t('scan.detail.installedVersion'), dataIndex: 'InstalledVersion', key: 'iv', width: 120 },
    { title: t('scan.detail.fixedVersion'), dataIndex: 'FixedVersion', key: 'fv', width: 120, render: (v) => v || '-' },
    {
      title: t('scan.detail.severity'),
      dataIndex: 'Severity',
      key: 'sev',
      width: 100,
      render: (v) => <Tag color={SEVERITY_COLORS[v] ?? 'default'}>{v}</Tag>,
    },
    { title: t('scan.detail.description'), dataIndex: 'Title', key: 'title', ellipsis: true },
  ];

  return (
    <>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Button icon={<ReloadOutlined />} onClick={fetchResults} loading={loading}>
          {t('gatekeeper.refresh')}
        </Button>
        <Button type="primary" icon={<ScanOutlined />} onClick={() => setScanModalOpen(true)}>
          {t('scan.trigger')}
        </Button>
      </div>

      <Table
        dataSource={results}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="small"
        locale={{ emptyText: t('scan.noData') }}
        pagination={{ pageSize: 20 }}
      />

      <Modal
        open={scanModalOpen}
        title={t('scan.trigger')}
        onOk={handleTrigger}
        onCancel={() => { setScanModalOpen(false); setImageInput(''); }}
        okButtonProps={{ disabled: !imageInput.trim() }}
      >
        <Input
          placeholder={t('scan.imagePlaceholder')}
          value={imageInput}
          onChange={(e) => setImageInput(e.target.value)}
          onPressEnter={handleTrigger}
          style={{ marginTop: 8 }}
        />
      </Modal>

      <Modal
        open={detailOpen}
        title={t('scan.detail.title')}
        onCancel={() => { setDetailOpen(false); setDetail(null); }}
        footer={null}
        width={900}
      >
        {detailLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
        ) : (
          <Table
            dataSource={vulns}
            columns={vulnColumns}
            rowKey={(r) => r.VulnerabilityID + r.PkgName}
            size="small"
            pagination={{ pageSize: 15 }}
            scroll={{ x: 800 }}
          />
        )}
      </Modal>
    </>
  );
}

// ─── CIS Benchmark Tab ───────────────────────────────────────────────────────

function BenchTab({ clusterId }: { clusterId: number }) {
  const { t } = useTranslation('security');
  const [results, setResults] = useState<BenchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [triggering, setTriggering] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<BenchDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const fetchResults = useCallback(async () => {
    setLoading(true);
    try {
      const data = await securityService.getBenchResults(clusterId);
      setResults(data);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [clusterId]);

  useEffect(() => {
    fetchResults();
  }, [fetchResults]);

  const handleTrigger = async () => {
    setTriggering(true);
    try {
      await securityService.triggerBenchmark(clusterId);
      message.success(t('bench.triggerSuccess'));
      setTimeout(fetchResults, 1000);
    } catch (e: any) {
      message.error(e?.message ?? 'error');
    } finally {
      setTriggering(false);
    }
  };

  const handleViewDetail = async (benchId: number) => {
    setDetailOpen(true);
    setDetailLoading(true);
    try {
      const data = await securityService.getBenchDetail(clusterId, benchId);
      setDetail(data);
    } catch {
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const statusColor: Record<string, string> = {
    pending: 'default',
    running: 'processing',
    completed: 'success',
    failed: 'error',
  };

  const columns: ColumnsType<BenchResult> = [
    {
      title: t('bench.runAt'),
      dataIndex: 'run_at',
      key: 'run_at',
      width: 160,
      render: (v, r) => (v ? new Date(v).toLocaleString() : new Date(r.created_at).toLocaleString()),
    },
    {
      title: t('bench.status'),
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (v) => <Badge status={statusColor[v] as any} text={t(`bench.${v}`)} />,
    },
    {
      title: t('bench.score'),
      dataIndex: 'score',
      key: 'score',
      width: 180,
      render: (v, r) =>
        r.status === 'completed' ? (
          <Progress percent={Math.round(v)} size="small" status={v >= 80 ? 'success' : v >= 60 ? 'normal' : 'exception'} />
        ) : '-',
    },
    {
      title: t('bench.pass'),
      dataIndex: 'pass',
      key: 'pass',
      width: 70,
      render: (v) => <Text style={{ color: '#52c41a' }}>{v}</Text>,
    },
    {
      title: t('bench.fail'),
      dataIndex: 'fail',
      key: 'fail',
      width: 70,
      render: (v) => <Text style={{ color: '#cf1322' }}>{v}</Text>,
    },
    {
      title: t('bench.warn'),
      dataIndex: 'warn',
      key: 'warn',
      width: 70,
      render: (v) => <Text style={{ color: '#d48806' }}>{v}</Text>,
    },
    {
      title: '',
      key: 'action',
      width: 100,
      render: (_, r) =>
        r.status === 'completed' ? (
          <Button size="small" onClick={() => handleViewDetail(r.id)}>
            {t('bench.viewDetail')}
          </Button>
        ) : null,
    },
  ];

  // Parse kube-bench JSON for detail modal
  let sections: any[] = [];
  if (detail?.result_json) {
    try {
      const report = JSON.parse(detail.result_json);
      sections = report.Controls ?? [];
    } catch {
      sections = [];
    }
  }

  return (
    <>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Button icon={<ReloadOutlined />} onClick={fetchResults} loading={loading}>
          {t('gatekeeper.refresh')}
        </Button>
        <Button type="primary" icon={<SafetyOutlined />} onClick={handleTrigger} loading={triggering}>
          {t('bench.trigger')}
        </Button>
      </div>

      <Table
        dataSource={results}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="small"
        locale={{ emptyText: t('bench.noData') }}
        pagination={{ pageSize: 10 }}
      />

      <Modal
        open={detailOpen}
        title={t('bench.detail.title')}
        onCancel={() => { setDetailOpen(false); setDetail(null); }}
        footer={null}
        width={900}
      >
        {detailLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
        ) : (
          <Collapse>
            {sections.map((ctrl: any, idx: number) => (
              <Panel
                key={idx}
                header={
                  <Space>
                    <Text strong>{ctrl.id}</Text>
                    <Text>{ctrl.text}</Text>
                  </Space>
                }
              >
                {(ctrl.tests ?? []).map((group: any, gi: number) => (
                  <div key={gi} style={{ marginBottom: 12 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>{group.id} {group.desc}</Text>
                    {(group.results ?? []).map((res: any, ri: number) => (
                      <div key={ri} style={{ display: 'flex', gap: 8, alignItems: 'flex-start', marginTop: 6 }}>
                        {res.status === 'PASS' && <CheckCircleOutlined style={{ color: '#52c41a', marginTop: 3 }} />}
                        {res.status === 'FAIL' && <ExclamationCircleOutlined style={{ color: '#cf1322', marginTop: 3 }} />}
                        {res.status === 'WARN' && <WarningOutlined style={{ color: '#d48806', marginTop: 3 }} />}
                        {res.status === 'INFO' && <InfoCircleOutlined style={{ color: '#1677ff', marginTop: 3 }} />}
                        <div>
                          <Text style={{ fontSize: 13 }}>[{res.test_number}] {res.test_desc}</Text>
                          {res.status === 'FAIL' && res.remediation && (
                            <div style={{ marginTop: 4, padding: '4px 8px', background: '#fff7e6', borderRadius: 4, fontSize: 12, color: '#874d00' }}>
                              {res.remediation}
                            </div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                ))}
              </Panel>
            ))}
          </Collapse>
        )}
      </Modal>
    </>
  );
}

// ─── Gatekeeper Tab ──────────────────────────────────────────────────────────

function GatekeeperTab({ clusterId }: { clusterId: number }) {
  const { t } = useTranslation('security');
  const [data, setData] = useState<GatekeeperSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const summary = await securityService.getGatekeeperViolations(clusterId);
      setData(summary);
    } catch (e: any) {
      setError(e?.response?.data?.message ?? e?.message ?? t('gatekeeper.notInstalled'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const columns: ColumnsType<ConstraintSummary> = [
    {
      title: t('gatekeeper.constraintKind'),
      dataIndex: 'kind',
      key: 'kind',
      width: 200,
    },
    {
      title: t('gatekeeper.constraintName'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: t('gatekeeper.violationCount'),
      dataIndex: 'violation_count',
      key: 'count',
      width: 120,
      render: (v) => (
        v > 0
          ? <Tag color="red">{v}</Tag>
          : <Tag color="green">0</Tag>
      ),
    },
  ];

  return (
    <>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        {data && (
          <Space>
            <Text>
              {t('gatekeeper.totalViolations')}:{' '}
              <Text strong style={{ color: data.total_violations > 0 ? '#cf1322' : '#52c41a' }}>
                {data.total_violations}
              </Text>
            </Text>
          </Space>
        )}
        <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>
          {t('gatekeeper.refresh')}
        </Button>
      </div>

      {error && <Alert type="warning" message={error} style={{ marginBottom: 16 }} />}

      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
      ) : (
        <Table
          dataSource={data?.constraints ?? []}
          columns={columns}
          rowKey={(r) => r.kind + r.name}
          size="small"
          locale={{ emptyText: t('gatekeeper.noData') }}
          pagination={false}
          expandable={{
            expandedRowRender: (record) => (
              <Table
                dataSource={record.violations}
                columns={[
                  { title: t('gatekeeper.namespace'), dataIndex: 'namespace', key: 'ns', width: 140, render: (v) => v || '-' },
                  { title: t('gatekeeper.resource'), dataIndex: 'resource', key: 'res', width: 200 },
                  { title: t('gatekeeper.message'), dataIndex: 'message', key: 'msg' },
                ]}
                rowKey={(v) => v.resource + v.namespace + v.message}
                size="small"
                pagination={false}
              />
            ),
            rowExpandable: (record) => record.violation_count > 0,
          }}
        />
      )}
    </>
  );
}

// ─── Main Dashboard ──────────────────────────────────────────────────────────

const SecurityDashboard: React.FC = () => {
  const { t } = useTranslation('security');
  const { id } = useParams<{ id: string }>();
  const clusterId = Number(id);

  const tabItems = [
    {
      key: 'imageScan',
      label: (
        <Space>
          <ScanOutlined />
          {t('tabs.imageScan')}
        </Space>
      ),
      children: <ImageScanTab clusterId={clusterId} />,
    },
    {
      key: 'bench',
      label: (
        <Space>
          <SafetyOutlined />
          {t('tabs.bench')}
        </Space>
      ),
      children: <BenchTab clusterId={clusterId} />,
    },
    {
      key: 'gatekeeper',
      label: (
        <Space>
          <ExclamationCircleOutlined />
          {t('tabs.gatekeeper')}
        </Space>
      ),
      children: <GatekeeperTab clusterId={clusterId} />,
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <Title level={4} style={{ marginBottom: 24 }}>
        <SafetyOutlined style={{ marginRight: 8 }} />
        {t('title')}
      </Title>
      <Tabs items={tabItems} destroyInactiveTabPane />
    </div>
  );
};

export default SecurityDashboard;

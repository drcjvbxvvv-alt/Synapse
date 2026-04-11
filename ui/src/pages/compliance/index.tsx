import React, { useState, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import {
  Tabs,
  Table,
  Button,
  Tag,
  Space,
  Card,
  Statistic,
  Row,
  Col,
  Select,
  Progress,
  Popconfirm,
  Drawer,
  Timeline,
  Tooltip,
  App,
  theme,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  SafetyCertificateOutlined,
  PlusOutlined,
  ReloadOutlined,
  ExportOutlined,
  DeleteOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  WarningOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import {
  complianceService,
  type ComplianceReport,
  type ControlResult,
  type ViolationEvent,
  type ViolationStats,
  type ComplianceEvidence,
  type Framework,
} from '../../services/complianceService';
import EmptyState from '../../components/EmptyState';

const FRAMEWORK_OPTIONS: { label: string; value: Framework }[] = [
  { label: 'SOC 2', value: 'SOC2' },
  { label: 'ISO 27001', value: 'ISO27001' },
  { label: 'CIS Kubernetes Benchmark', value: 'CIS_K8S' },
];

const STATUS_COLORS: Record<string, string> = {
  pass: 'success',
  fail: 'error',
  warn: 'warning',
  na: 'default',
};

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'red',
  high: 'orange',
  medium: 'gold',
  low: 'green',
  info: 'blue',
};

const SOURCE_COLORS: Record<string, string> = {
  gatekeeper: 'purple',
  trivy: 'blue',
  bench: 'cyan',
  audit: 'orange',
};

// ─── Reports Tab ───────────────────────────────────────────────────────────

function ReportsTab({ clusterId }: { clusterId: string }) {
  const { t } = useTranslation(['compliance', 'common']);
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [framework, setFramework] = useState<Framework>('SOC2');
  const [detailReport, setDetailReport] = useState<ComplianceReport | null>(null);
  const [controls, setControls] = useState<ControlResult[]>([]);

  const { data: reports = [], isLoading, refetch } = useQuery({
    queryKey: ['compliance-reports', clusterId],
    queryFn: () => complianceService.listReports(clusterId),
    staleTime: 30_000,
  });

  const generateMut = useMutation({
    mutationFn: () => complianceService.generateReport(clusterId, framework),
    onSuccess: () => {
      message.success(t('common:messages.success'));
      queryClient.invalidateQueries({ queryKey: ['compliance-reports', clusterId] });
    },
    onError: () => message.error(t('common:messages.failed')),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => complianceService.deleteReport(clusterId, id),
    onSuccess: () => {
      message.success(t('common:messages.success'));
      queryClient.invalidateQueries({ queryKey: ['compliance-reports', clusterId] });
    },
  });

  const handleViewDetail = useCallback(async (report: ComplianceReport) => {
    try {
      const full = await complianceService.getReport(clusterId, report.id) as ComplianceReport;
      setDetailReport(full);
      if (full.result_json) {
        setControls(JSON.parse(full.result_json) as ControlResult[]);
      }
    } catch {
      message.error(t('common:messages.failed'));
    }
  }, [clusterId, message, t]);

  const handleExport = useCallback(async (report: ComplianceReport) => {
    try {
      const data = await complianceService.exportReport(clusterId, report.id);
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `compliance-${report.framework}-${report.id}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      message.error(t('common:messages.failed'));
    }
  }, [clusterId, message, t]);

  const columns: TableColumnsType<ComplianceReport> = [
    {
      title: t('compliance:report.selectFramework'),
      dataIndex: 'framework',
      key: 'framework',
      width: 200,
      render: (v: string) => <Tag>{t(`compliance:framework.${v}`)}</Tag>,
    },
    {
      title: t('compliance:report.score'),
      dataIndex: 'score',
      key: 'score',
      width: 180,
      render: (v: number, r: ComplianceReport) =>
        r.status === 'completed'
          ? <Progress percent={Math.round(v)} size="small" status={v >= 80 ? 'success' : v >= 50 ? 'normal' : 'exception'} />
          : '-',
    },
    {
      title: t('compliance:report.pass') + ' / ' + t('compliance:report.fail') + ' / ' + t('compliance:report.warn'),
      key: 'counts',
      width: 160,
      render: (_: unknown, r: ComplianceReport) =>
        r.status === 'completed'
          ? (
              <Space>
                <Tag color="success">{r.pass_count}</Tag>
                <Tag color="error">{r.fail_count}</Tag>
                <Tag color="warning">{r.warn_count}</Tag>
              </Space>
            )
          : '-',
    },
    {
      title: t('compliance:report.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (v: string) => {
        const color = v === 'completed' ? 'success' : v === 'generating' ? 'processing' : v === 'failed' ? 'error' : 'default';
        return <Tag color={color}>{t(`compliance:report.${v}`)}</Tag>;
      },
    },
    {
      title: t('compliance:report.generatedAt'),
      dataIndex: 'generated_at',
      key: 'generated_at',
      width: 160,
      render: (v: string | null) => v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-',
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 140,
      fixed: 'right',
      render: (_: unknown, r: ComplianceReport) => (
        <Space size={0}>
          {r.status === 'completed' && (
            <>
              <Tooltip title={t('compliance:report.viewDetail')}>
                <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handleViewDetail(r)} />
              </Tooltip>
              <Tooltip title={t('compliance:report.export')}>
                <Button type="link" size="small" icon={<ExportOutlined />} onClick={() => handleExport(r)} />
              </Tooltip>
            </>
          )}
          <Popconfirm
            title={t('compliance:report.deleteConfirm')}
            onConfirm={() => deleteMut.mutate(r.id)}
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

  const controlColumns: TableColumnsType<ControlResult> = [
    { title: t('compliance:report.controlId'), dataIndex: 'control_id', key: 'id', width: 100 },
    { title: t('compliance:report.controlTitle'), dataIndex: 'title', key: 'title', width: 280 },
    { title: t('compliance:report.controlCategory'), dataIndex: 'category', key: 'cat', width: 200 },
    {
      title: t('compliance:report.controlStatus'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (v: string) => <Tag color={STATUS_COLORS[v] ?? 'default'}>{t(`compliance:report.${v}`)}</Tag>,
    },
    { title: t('compliance:report.controlDesc'), dataIndex: 'description', key: 'desc', ellipsis: true },
  ];

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: token.marginMD }}>
        <Space>
          <Select
            value={framework}
            onChange={setFramework}
            options={FRAMEWORK_OPTIONS}
            style={{ width: 260 }}
          />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            loading={generateMut.isPending}
            onClick={() => generateMut.mutate()}
          >
            {t('compliance:report.generate')}
          </Button>
        </Space>
        <Tooltip title={t('common:actions.refresh')}>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
        </Tooltip>
      </div>

      <Table<ComplianceReport>
        columns={columns}
        dataSource={reports as ComplianceReport[]}
        rowKey="id"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: <EmptyState /> }}
      />

      <Drawer
        title={detailReport ? `${t(`compliance:framework.${detailReport.framework}`)} — ${t('compliance:report.viewDetail')}` : ''}
        open={!!detailReport}
        onClose={() => { setDetailReport(null); setControls([]); }}
        width={900}
      >
        {detailReport && (
          <>
            <Row gutter={16} style={{ marginBottom: token.marginLG }}>
              <Col span={6}>
                <Statistic
                  title={t('compliance:report.score')}
                  value={Math.round(detailReport.score)}
                  suffix="%"
                  valueStyle={{ color: detailReport.score >= 80 ? token.colorSuccess : detailReport.score >= 50 ? token.colorWarning : token.colorError }}
                />
              </Col>
              <Col span={6}><Statistic title={t('compliance:report.pass')} value={detailReport.pass_count} valueStyle={{ color: token.colorSuccess }} /></Col>
              <Col span={6}><Statistic title={t('compliance:report.fail')} value={detailReport.fail_count} valueStyle={{ color: token.colorError }} /></Col>
              <Col span={6}><Statistic title={t('compliance:report.warn')} value={detailReport.warn_count} valueStyle={{ color: token.colorWarning }} /></Col>
            </Row>
            <Table<ControlResult>
              columns={controlColumns}
              dataSource={controls}
              rowKey="control_id"
              size="small"
              pagination={false}
              scroll={{ x: 900 }}
              locale={{ emptyText: <EmptyState /> }}
            />
          </>
        )}
      </Drawer>
    </>
  );
}

// ─── Violations Tab ────────────────────────────────────────────────────────

function ViolationsTab({ clusterId }: { clusterId: string }) {
  const { t } = useTranslation(['compliance', 'common']);
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [source, setSource] = useState<string>('');
  const [severity, setSeverity] = useState<string>('');
  const [resolved, setResolved] = useState<string>('');
  const [page, setPage] = useState(1);

  const { data: violationData, isLoading } = useQuery({
    queryKey: ['compliance-violations', clusterId, source, severity, resolved, page],
    queryFn: () => complianceService.listViolations(clusterId, {
      source: source || undefined,
      severity: severity || undefined,
      resolved: resolved || undefined,
      page,
      pageSize: 20,
    }),
    staleTime: 15_000,
  });

  const { data: stats } = useQuery<ViolationStats>({
    queryKey: ['compliance-violation-stats', clusterId],
    queryFn: () => complianceService.getViolationStats(clusterId) as Promise<ViolationStats>,
    staleTime: 30_000,
  });

  const resolveMut = useMutation({
    mutationFn: (id: number) => complianceService.resolveViolation(clusterId, id),
    onSuccess: () => {
      message.success(t('common:messages.success'));
      queryClient.invalidateQueries({ queryKey: ['compliance-violations'] });
      queryClient.invalidateQueries({ queryKey: ['compliance-violation-stats'] });
    },
  });

  const violations: ViolationEvent[] = (violationData as { items?: ViolationEvent[]; data?: ViolationEvent[] })?.items
    ?? (violationData as { items?: ViolationEvent[]; data?: ViolationEvent[] })?.data
    ?? (Array.isArray(violationData) ? violationData as ViolationEvent[] : []);
  const total: number = (violationData as { total?: number })?.total ?? violations.length;

  const columns: TableColumnsType<ViolationEvent> = [
    {
      title: t('compliance:violation.severity'),
      dataIndex: 'severity',
      key: 'severity',
      width: 90,
      render: (v: string) => <Tag color={SEVERITY_COLORS[v] ?? 'default'}>{t(`compliance:violation.severities.${v}`)}</Tag>,
    },
    {
      title: t('compliance:violation.source'),
      dataIndex: 'source',
      key: 'source',
      width: 120,
      render: (v: string) => <Tag color={SOURCE_COLORS[v] ?? 'default'}>{t(`compliance:violation.sources.${v}`)}</Tag>,
    },
    {
      title: t('compliance:violation.title'),
      dataIndex: 'title',
      key: 'title',
      ellipsis: true,
    },
    {
      title: t('compliance:violation.resourceRef'),
      dataIndex: 'resource_ref',
      key: 'ref',
      width: 200,
      ellipsis: true,
    },
    {
      title: t('compliance:violation.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (v: string) => <span style={{ color: token.colorTextSecondary, fontSize: token.fontSizeSM }}>{dayjs(v).format('YYYY-MM-DD HH:mm')}</span>,
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 100,
      fixed: 'right',
      render: (_: unknown, r: ViolationEvent) =>
        r.resolved_at ? (
          <Tag color="success">{t('compliance:violation.resolved')}</Tag>
        ) : (
          <Popconfirm
            title={t('compliance:violation.resolveConfirm')}
            onConfirm={() => resolveMut.mutate(r.id)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            <Button type="link" size="small" icon={<CheckCircleOutlined />}>
              {t('compliance:violation.resolve')}
            </Button>
          </Popconfirm>
        ),
    },
  ];

  return (
    <>
      {stats && (
        <Row gutter={16} style={{ marginBottom: token.marginLG }}>
          <Col span={6}>
            <Card variant="borderless" size="small">
              <Statistic title={t('compliance:violation.stats.totalOpen')} value={stats.total_open} valueStyle={{ color: stats.total_open > 0 ? token.colorError : token.colorSuccess }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card variant="borderless" size="small">
              <Statistic title={t('compliance:violation.stats.totalResolved')} value={stats.total_resolved} valueStyle={{ color: token.colorSuccess }} />
            </Card>
          </Col>
          <Col span={6}>
            <Card variant="borderless" size="small">
              <div style={{ fontSize: token.fontSizeSM, color: token.colorTextSecondary, marginBottom: 4 }}>{t('compliance:violation.stats.bySource')}</div>
              <Space wrap>{Object.entries(stats.by_source).map(([k, v]) => <Tag key={k} color={SOURCE_COLORS[k]}>{t(`compliance:violation.sources.${k}`)}: {v}</Tag>)}</Space>
            </Card>
          </Col>
          <Col span={6}>
            <Card variant="borderless" size="small">
              <div style={{ fontSize: token.fontSizeSM, color: token.colorTextSecondary, marginBottom: 4 }}>{t('compliance:violation.stats.bySeverity')}</div>
              <Space wrap>{Object.entries(stats.by_severity).map(([k, v]) => <Tag key={k} color={SEVERITY_COLORS[k]}>{t(`compliance:violation.severities.${k}`)}: {v}</Tag>)}</Space>
            </Card>
          </Col>
        </Row>
      )}

      <div style={{ display: 'flex', gap: token.marginSM, marginBottom: token.marginMD, flexWrap: 'wrap' }}>
        <Select
          value={source}
          onChange={setSource}
          placeholder={t('compliance:violation.source')}
          style={{ width: 160 }}
          allowClear
          options={[
            { label: t('compliance:violation.all'), value: '' },
            { label: 'Gatekeeper', value: 'gatekeeper' },
            { label: 'Trivy', value: 'trivy' },
            { label: 'CIS Benchmark', value: 'bench' },
            { label: 'Audit', value: 'audit' },
          ]}
        />
        <Select
          value={severity}
          onChange={setSeverity}
          placeholder={t('compliance:violation.severity')}
          style={{ width: 140 }}
          allowClear
          options={[
            { label: t('compliance:violation.all'), value: '' },
            ...['critical', 'high', 'medium', 'low', 'info'].map((s) => ({
              label: t(`compliance:violation.severities.${s}`),
              value: s,
            })),
          ]}
        />
        <Select
          value={resolved}
          onChange={setResolved}
          placeholder={t('compliance:violation.open')}
          style={{ width: 140 }}
          allowClear
          options={[
            { label: t('compliance:violation.all'), value: '' },
            { label: t('compliance:violation.open'), value: 'false' },
            { label: t('compliance:violation.resolved'), value: 'true' },
          ]}
        />
      </div>

      <Table<ViolationEvent>
        columns={columns}
        dataSource={violations}
        rowKey="id"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{
          current: page,
          pageSize: 20,
          total,
          onChange: setPage,
          showTotal: (t2) => t('common:pagination.total', { total: t2 }),
        }}
        locale={{ emptyText: <EmptyState /> }}
      />
    </>
  );
}

// ─── Evidence Tab ──────────────────────────────────────────────────────────

function EvidenceTab({ clusterId }: { clusterId: string }) {
  const { t } = useTranslation(['compliance', 'common']);
  const { token } = theme.useToken();
  const [framework, setFramework] = useState<string>('');
  const [detailData, setDetailData] = useState<string | null>(null);

  const { data: evidence = [], isLoading } = useQuery({
    queryKey: ['compliance-evidence', clusterId, framework],
    queryFn: () => complianceService.listEvidence(clusterId, framework || undefined),
    staleTime: 30_000,
  });

  const columns: TableColumnsType<ComplianceEvidence> = [
    {
      title: t('compliance:evidence.framework'),
      dataIndex: 'framework',
      key: 'framework',
      width: 140,
      render: (v: string) => <Tag>{t(`compliance:framework.${v}`)}</Tag>,
    },
    {
      title: t('compliance:evidence.controlId'),
      dataIndex: 'control_id',
      key: 'control_id',
      width: 100,
    },
    {
      title: t('compliance:evidence.controlTitle'),
      dataIndex: 'control_title',
      key: 'control_title',
      width: 240,
      ellipsis: true,
    },
    {
      title: t('compliance:evidence.evidenceType'),
      dataIndex: 'evidence_type',
      key: 'type',
      width: 120,
      render: (v: string) => <Tag>{t(`compliance:evidence.types.${v}`)}</Tag>,
    },
    {
      title: t('compliance:evidence.capturedAt'),
      dataIndex: 'captured_at',
      key: 'captured_at',
      width: 160,
      render: (v: string) => <span style={{ color: token.colorTextSecondary, fontSize: token.fontSizeSM }}>{dayjs(v).format('YYYY-MM-DD HH:mm')}</span>,
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 80,
      render: (_: unknown, r: ComplianceEvidence) => (
        <Tooltip title={t('compliance:evidence.viewData')}>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setDetailData(r.data_json ?? null)} />
        </Tooltip>
      ),
    },
  ];

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: token.marginMD }}>
        <Select
          value={framework}
          onChange={setFramework}
          placeholder={t('compliance:evidence.framework')}
          style={{ width: 260 }}
          allowClear
          options={[
            { label: t('compliance:violation.all'), value: '' },
            ...FRAMEWORK_OPTIONS,
          ]}
        />
      </div>

      <Table<ComplianceEvidence>
        columns={columns}
        dataSource={evidence as ComplianceEvidence[]}
        rowKey="id"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{ pageSize: 20 }}
        locale={{ emptyText: <EmptyState /> }}
      />

      <Drawer
        title={t('compliance:evidence.viewData')}
        open={!!detailData}
        onClose={() => setDetailData(null)}
        width={700}
      >
        <pre style={{
          background: token.colorBgLayout,
          padding: token.paddingLG,
          borderRadius: token.borderRadius,
          fontSize: token.fontSizeSM,
          overflow: 'auto',
          maxHeight: '80vh',
        }}>
          {detailData ? JSON.stringify(JSON.parse(detailData), null, 2) : ''}
        </pre>
      </Drawer>
    </>
  );
}

// ─── Main Page ─────────────────────────────────────────────────────────────

const ComplianceDashboard: React.FC = () => {
  const { t } = useTranslation(['compliance']);
  const { id } = useParams<{ id: string }>();
  const clusterId = id ?? '';

  const tabItems = [
    {
      key: 'reports',
      label: (
        <Space>
          <SafetyCertificateOutlined />
          {t('compliance:tabs.reports')}
        </Space>
      ),
      children: <ReportsTab clusterId={clusterId} />,
    },
    {
      key: 'violations',
      label: (
        <Space>
          <CloseCircleOutlined />
          {t('compliance:tabs.violations')}
        </Space>
      ),
      children: <ViolationsTab clusterId={clusterId} />,
    },
    {
      key: 'evidence',
      label: (
        <Space>
          <EyeOutlined />
          {t('compliance:tabs.evidence')}
        </Space>
      ),
      children: <EvidenceTab clusterId={clusterId} />,
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Tabs items={tabItems} destroyInactiveTabPane />
    </div>
  );
};

export default ComplianceDashboard;

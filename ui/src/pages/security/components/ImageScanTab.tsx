import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  Table,
  Button,
  Tag,
  Space,
  Spin,
  Modal,
  Input,
  Badge,
  message,
  theme,
} from 'antd';
import EmptyState from '@/components/EmptyState';
import { ScanOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import securityService from '@/services/securityService';
import type {
  ScanResult,
  ScanDetail,
  TrivyVulnerability,
  TrivyReport,
} from '@/services/securityService';
import { SeverityTag, SEVERITY_COLORS } from './SeverityTag';

interface ImageScanTabProps {
  clusterId: number;
}

export function ImageScanTab({ clusterId }: ImageScanTabProps) {
  const { t } = useTranslation('security');
  const { token } = theme.useToken();
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
    } catch (e: unknown) {
      const err = e as { message?: string };
      message.error(err?.message ?? 'error');
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
      render: (v: string) => (
        <Badge status={statusColor[v] as 'default' | 'processing' | 'success' | 'error'} text={t(`scan.${v}`)} />
      ),
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
      render: (v) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-'),
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
      render: (v: string) => <Tag color={SEVERITY_COLORS[v] ?? 'default'}>{v}</Tag>,
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
        scroll={{ x: 'max-content' }}
        dataSource={results}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="small"
        locale={{ emptyText: <EmptyState description={t('scan.noData')} /> }}
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
          <div style={{ textAlign: 'center', padding: token.paddingXL }}><Spin /></div>
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

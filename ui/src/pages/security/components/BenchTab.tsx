import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  Table,
  Button,
  Space,
  Spin,
  Modal,
  Badge,
  Progress,
  Collapse,
  Typography,
  message,
  theme,
} from 'antd';
import EmptyState from '@/components/EmptyState';
import {
  SafetyOutlined,
  ReloadOutlined,
  ExclamationCircleOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import securityService from '@/services/securityService';
import type { BenchResult, BenchDetail } from '@/services/securityService';

const { Text } = Typography;
const { Panel } = Collapse;

interface BenchTabProps {
  clusterId: number;
}

export function BenchTab({ clusterId }: BenchTabProps) {
  const { t } = useTranslation('security');
  const { token } = theme.useToken();
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
    } catch (e: unknown) {
      const err = e as { message?: string };
      message.error(err?.message ?? 'error');
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
      render: (v, r) => dayjs(v || r.created_at).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: t('bench.status'),
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (v: string) => (
        <Badge status={statusColor[v] as 'default' | 'processing' | 'success' | 'error'} text={t(`bench.${v}`)} />
      ),
    },
    {
      title: t('bench.score'),
      dataIndex: 'score',
      key: 'score',
      width: 180,
      render: (v: number, r) =>
        r.status === 'completed' ? (
          <Progress
            percent={Math.round(v)}
            size="small"
            status={v >= 80 ? 'success' : v >= 60 ? 'normal' : 'exception'}
          />
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
  let sections: Array<{
    id: string;
    text: string;
    tests?: Array<{
      id: string;
      desc: string;
      results?: Array<{
        status: string;
        test_number: string;
        test_desc: string;
        remediation?: string;
      }>;
    }>;
  }> = [];
  if (detail?.result_json) {
    try {
      const report = JSON.parse(detail.result_json) as { Controls?: typeof sections };
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
        scroll={{ x: 'max-content' }}
        dataSource={results}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="small"
        locale={{ emptyText: <EmptyState description={t('bench.noData')} /> }}
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
          <div style={{ textAlign: 'center', padding: token.paddingXL }}><Spin /></div>
        ) : (
          <Collapse>
            {sections.map((ctrl, idx) => (
              <Panel
                key={idx}
                header={
                  <Space>
                    <Text strong>{ctrl.id}</Text>
                    <Text>{ctrl.text}</Text>
                  </Space>
                }
              >
                {(ctrl.tests ?? []).map((group, gi) => (
                  <div key={gi} style={{ marginBottom: 12 }}>
                    <Text type="secondary" style={{ fontSize: 12 }}>{group.id} {group.desc}</Text>
                    {(group.results ?? []).map((res, ri) => (
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

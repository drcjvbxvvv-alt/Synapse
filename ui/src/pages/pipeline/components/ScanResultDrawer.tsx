/**
 * ScanResultDrawer — 安全掃描結果詳情
 *
 * 顯示 Trivy 掃描的完整漏洞清單，按嚴重程度分組。
 */
import React from 'react';
import {
  Drawer,
  Table,
  Tag,
  Statistic,
  Row,
  Col,
  Card,
  Typography,
  Spin,
  theme,
  Flex,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  SecurityScanOutlined,
  ExclamationCircleOutlined,
  WarningOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { request } from '../../../utils/api';

const { Text, Link } = Typography;

// ─── Types ─────────────────────────────────────────────────────────────────

interface Vulnerability {
  VulnerabilityID: string;
  PkgName: string;
  InstalledVersion: string;
  FixedVersion: string;
  Severity: string;
  Title: string;
  PrimaryURL: string;
}

interface TrivyResult {
  Target: string;
  Class: string;
  Type: string;
  Vulnerabilities: Vulnerability[] | null;
}

interface TrivyReport {
  Results: TrivyResult[];
}

interface ScanResultResponse {
  id: number;
  image: string;
  status: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
  unknown: number;
  result_json: string;
  scanned_at: string;
}

interface ScanResultDrawerProps {
  open: boolean;
  onClose: () => void;
  pipelineId: number;
  runId: number;
}

// ─── Severity config ───────────────────────────────────────────────────────

const SEVERITY_CONFIG: Record<string, { color: string; order: number }> = {
  CRITICAL: { color: 'error', order: 0 },
  HIGH: { color: 'warning', order: 1 },
  MEDIUM: { color: 'gold', order: 2 },
  LOW: { color: 'blue', order: 3 },
  UNKNOWN: { color: 'default', order: 4 },
};

// ─── Component ─────────────────────────────────────────────────────────────

const ScanResultDrawer: React.FC<ScanResultDrawerProps> = ({
  open,
  onClose,
  pipelineId,
  runId,
}) => {
  const { t } = useTranslation(['pipeline', 'common']);
  const { token } = theme.useToken();

  const { data, isLoading } = useQuery({
    queryKey: ['scan-result', pipelineId, runId],
    queryFn: () =>
      request.get<ScanResultResponse>(
        `/pipelines/${pipelineId}/runs/${runId}/scan-result`,
      ),
    enabled: open && runId > 0,
  });

  // Parse vulnerabilities from result_json
  const vulnerabilities: Vulnerability[] = React.useMemo(() => {
    if (!data?.result_json) return [];
    try {
      const report: TrivyReport = JSON.parse(data.result_json);
      const vulns: Vulnerability[] = [];
      for (const result of report.Results ?? []) {
        for (const v of result.Vulnerabilities ?? []) {
          vulns.push(v);
        }
      }
      // Sort by severity
      vulns.sort((a, b) => {
        const orderA = SEVERITY_CONFIG[a.Severity]?.order ?? 99;
        const orderB = SEVERITY_CONFIG[b.Severity]?.order ?? 99;
        return orderA - orderB;
      });
      return vulns;
    } catch {
      return [];
    }
  }, [data]);

  const columns: TableColumnsType<Vulnerability> = [
    {
      title: t('pipeline:scan.severity', { defaultValue: '嚴重程度' }),
      dataIndex: 'Severity',
      key: 'severity',
      width: 100,
      render: (severity: string) => {
        const cfg = SEVERITY_CONFIG[severity] ?? { color: 'default' };
        return <Tag color={cfg.color}>{severity}</Tag>;
      },
    },
    {
      title: 'CVE ID',
      dataIndex: 'VulnerabilityID',
      key: 'id',
      width: 160,
      render: (id: string, record) =>
        record.PrimaryURL ? (
          <Link href={record.PrimaryURL} target="_blank">
            {id}
          </Link>
        ) : (
          id
        ),
    },
    {
      title: t('pipeline:scan.package', { defaultValue: '套件' }),
      dataIndex: 'PkgName',
      key: 'pkg',
      width: 180,
      ellipsis: true,
    },
    {
      title: t('pipeline:scan.installed', { defaultValue: '已安裝版本' }),
      dataIndex: 'InstalledVersion',
      key: 'installed',
      width: 120,
      ellipsis: true,
    },
    {
      title: t('pipeline:scan.fixed', { defaultValue: '修復版本' }),
      dataIndex: 'FixedVersion',
      key: 'fixed',
      width: 120,
      render: (v: string) =>
        v ? (
          <Text code>{v}</Text>
        ) : (
          <Text type="secondary">—</Text>
        ),
    },
    {
      title: t('pipeline:scan.title', { defaultValue: '描述' }),
      dataIndex: 'Title',
      key: 'title',
      ellipsis: true,
    },
  ];

  return (
    <Drawer
      title={
        <Flex align="center" gap={token.marginSM}>
          <SecurityScanOutlined />
          {t('pipeline:scan.drawerTitle', { defaultValue: '安全掃描結果' })}
        </Flex>
      }
      open={open}
      onClose={onClose}
      width={960}
      destroyOnClose
    >
      <Spin spinning={isLoading}>
        {data && (
          <>
            {/* 摘要卡片 */}
            <Card variant="borderless" style={{ marginBottom: token.marginMD }}>
              <Row gutter={token.marginMD}>
                <Col span={4}>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {t('pipeline:scan.image', { defaultValue: '掃描映像' })}
                  </Text>
                  <div>
                    <Text strong ellipsis style={{ maxWidth: 200 }}>
                      {data.image.split('/').pop()}
                    </Text>
                  </div>
                </Col>
                <Col span={4}>
                  <Statistic
                    title="Critical"
                    value={data.critical}
                    valueStyle={{ color: data.critical > 0 ? token.colorError : token.colorSuccess }}
                    prefix={<ExclamationCircleOutlined />}
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="High"
                    value={data.high}
                    valueStyle={{ color: data.high > 0 ? token.colorWarning : token.colorSuccess }}
                    prefix={<WarningOutlined />}
                  />
                </Col>
                <Col span={4}>
                  <Statistic
                    title="Medium"
                    value={data.medium}
                    valueStyle={{ color: data.medium > 0 ? '#faad14' : token.colorSuccess }}
                    prefix={<InfoCircleOutlined />}
                  />
                </Col>
                <Col span={4}>
                  <Statistic title="Low" value={data.low} />
                </Col>
                <Col span={4}>
                  <Statistic
                    title={t('pipeline:scan.total', { defaultValue: '總計' })}
                    value={vulnerabilities.length}
                  />
                </Col>
              </Row>
            </Card>

            {/* 漏洞清單 */}
            <Table<Vulnerability>
              columns={columns}
              dataSource={vulnerabilities}
              rowKey={(r) => `${r.VulnerabilityID}-${r.PkgName}`}
              size="small"
              pagination={{ pageSize: 50, showSizeChanger: true }}
              scroll={{ x: 'max-content' }}
            />
          </>
        )}
      </Spin>
    </Drawer>
  );
};

export default ScanResultDrawer;

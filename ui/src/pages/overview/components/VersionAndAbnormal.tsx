import React from 'react';
import { Row, Col, Card, Table, Tag, Tooltip, Button } from 'antd';
import { CheckCircleOutlined, WarningOutlined, RightOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { VersionDistribution, AbnormalWorkload } from '../../../services/overviewService';

interface VersionAndAbnormalProps {
  versionDistribution: VersionDistribution[];
  abnormalWorkloads: AbnormalWorkload[];
  onNavigate: (path: string) => void;
}

const cardStyle = { boxShadow: '0 1px 4px rgba(0,0,0,0.08)', borderRadius: 8 };
const cardHeadStyle = { borderBottom: '1px solid #f0f0f0', padding: '12px 16px', minHeight: 48 };

export const VersionAndAbnormal: React.FC<VersionAndAbnormalProps> = ({
  versionDistribution,
  abnormalWorkloads,
  onNavigate,
}) => {
  const { t } = useTranslation(['overview', 'common']);

  const versionColumns = [
    {
      title: t('distribution.versionName'),
      dataIndex: 'version',
      key: 'version',
      render: (text: string) => (
        <Tag color="volcano" style={{ borderRadius: 4, fontSize: 12, padding: '2px 8px' }}>
          {text}
        </Tag>
      ),
    },
    {
      title: t('distribution.clusterCount'),
      dataIndex: 'count',
      key: 'count',
      align: 'right' as const,
      render: (count: number, record: VersionDistribution) => (
        <Tooltip title={record.clusters?.join(', ')}>
          <span style={{ color: '#3b82f6', fontWeight: 'bold', fontSize: 14, cursor: 'pointer' }}>
            {count}
          </span>
        </Tooltip>
      ),
    },
  ];

  const abnormalColumns = [
    {
      title: t('abnormal.workload'),
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (text: string, record: AbnormalWorkload) => {
        const getDetailPath = () => {
          const { clusterId, namespace, name, type } = record;
          if (type === 'Pod') {
            return `/clusters/${clusterId}/pods/${namespace}/${name}`;
          }
          const workloadType = type.toLowerCase();
          return `/clusters/${clusterId}/workloads/${workloadType}/${namespace}/${name}`;
        };

        return (
          <Button
            type="link"
            size="small"
            style={{
              padding: 0,
              whiteSpace: 'normal',
              wordBreak: 'break-all',
              textAlign: 'left',
              height: 'auto',
              lineHeight: 1.4,
            }}
            onClick={() => onNavigate(getDetailPath())}
          >
            {text}
          </Button>
        );
      },
    },
    {
      title: t('abnormal.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 120,
      render: (text: string) => (
        <Tag color="blue" style={{ whiteSpace: 'normal', wordBreak: 'break-all' }}>
          {text}
        </Tag>
      ),
    },
    {
      title: t('abnormal.cluster'),
      dataIndex: 'clusterName',
      key: 'clusterName',
      width: 100,
    },
    {
      title: t('abnormal.type'),
      dataIndex: 'type',
      key: 'type',
      render: (text: string) => <Tag>{text}</Tag>,
    },
    {
      title: t('abnormal.reason'),
      dataIndex: 'reason',
      key: 'reason',
      render: (text: string, record: AbnormalWorkload) => (
        <Tooltip title={record.message}>
          <span style={{ color: record.severity === 'critical' ? '#ef4444' : '#f59e0b' }}>
            <WarningOutlined style={{ marginRight: 4 }} />
            {text}
          </span>
        </Tooltip>
      ),
    },
    {
      title: t('abnormal.duration'),
      dataIndex: 'duration',
      key: 'duration',
    },
  ];

  return (
    <Row gutter={16} style={{ marginBottom: 16 }}>
      <Col span={6}>
        <Card
          title={t('distribution.clusterVersion')}
          variant="borderless"
          style={{ ...cardStyle, height: 320 }}
          headStyle={cardHeadStyle}
          styles={{ body: { padding: '12px 16px' } }}
        >
          <Table
            dataSource={versionDistribution}
            columns={versionColumns}
            rowKey="version"
            pagination={false}
            size="small"
            scroll={{ y: 220 }}
          />
        </Card>
      </Col>
      <Col span={18}>
        <Card
          title={
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span><WarningOutlined style={{ color: '#ef4444', marginRight: 8 }} />{t('abnormal.title')}</span>
              <Button type="link" size="small" onClick={() => onNavigate('/clusters')}>
                {t('common:actions.viewAll')} <RightOutlined />
              </Button>
            </div>
          }
          variant="borderless"
          style={{ ...cardStyle, height: 320 }}
          headStyle={cardHeadStyle}
          styles={{ body: { padding: '12px 16px' } }}
        >
          {abnormalWorkloads.length > 0 ? (
            <Table
              dataSource={abnormalWorkloads}
              columns={abnormalColumns}
              rowKey={(record) => `${record.clusterId}-${record.namespace}-${record.name}`}
              pagination={false}
              size="small"
              scroll={{ y: 220 }}
            />
          ) : (
            <div style={{ textAlign: 'center', padding: 60, color: '#9ca3af' }}>
              <CheckCircleOutlined style={{ fontSize: 48, color: '#10b981', marginBottom: 16 }} />
              <div>{t('abnormal.allNormal')}</div>
            </div>
          )}
        </Card>
      </Col>
    </Row>
  );
};

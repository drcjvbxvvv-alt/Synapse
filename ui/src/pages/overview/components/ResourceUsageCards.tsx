import React from 'react';
import { Row, Col, Card, Progress } from 'antd';
import { ThunderboltOutlined, DatabaseOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { ResourceUsageResponse } from '../../../services/overviewService';

interface ResourceUsageCardsProps {
  cpuUsage: number;
  memoryUsage: number;
  storageUsage: number;
  totalCPU: number;
  totalMemory: number;
  resourceUsage: ResourceUsageResponse | null;
  formatNumber: (num: number, unit?: string) => string;
}

const cardStyle = { boxShadow: '0 1px 4px rgba(0,0,0,0.08)', borderRadius: 8 };
const cardHeadStyle = { borderBottom: '1px solid #f0f0f0', padding: '10px 16px', minHeight: 48 };

export const ResourceUsageCards: React.FC<ResourceUsageCardsProps> = ({
  cpuUsage,
  memoryUsage,
  storageUsage,
  totalCPU,
  totalMemory,
  resourceUsage,
  formatNumber,
}) => {
  const { t } = useTranslation(['overview', 'common']);

  return (
    <Row gutter={16} style={{ marginBottom: 16 }}>
      <Col span={8}>
        <Card
          title={<span><ThunderboltOutlined style={{ color: '#3b82f6' }} /> {t('resource.cpuUsage')}</span>}
          bordered={false}
          style={{ ...cardStyle, height: 160 }}
          headStyle={cardHeadStyle}
          styles={{ body: { padding: '16px' } }}
        >
          <Progress
            percent={cpuUsage}
            strokeColor={cpuUsage > 80 ? '#ef4444' : cpuUsage > 60 ? '#f59e0b' : '#10b981'}
            format={(percent) => (
              <span style={{ fontWeight: 600, fontSize: 18 }}>{percent?.toFixed(1)}%</span>
            )}
          />
          <div style={{ marginTop: 8, color: '#6b7280', fontSize: 13 }}>
            {t('common:resources.used')}: {formatNumber(Math.floor(totalCPU * cpuUsage / 100), t('common:units.cores'))} /
            {t('common:resources.total')}: {formatNumber(totalCPU, t('common:units.cores'))}
          </div>
        </Card>
      </Col>
      <Col span={8}>
        <Card
          title={<span><DatabaseOutlined style={{ color: '#10b981' }} /> {t('resource.memoryUsage')}</span>}
          bordered={false}
          style={{ ...cardStyle, height: 160 }}
          headStyle={cardHeadStyle}
          styles={{ body: { padding: '16px' } }}
        >
          <Progress
            percent={memoryUsage}
            strokeColor={memoryUsage > 80 ? '#ef4444' : memoryUsage > 60 ? '#f59e0b' : '#10b981'}
            format={(percent) => (
              <span style={{ fontWeight: 600, fontSize: 18 }}>{percent?.toFixed(1)}%</span>
            )}
          />
          <div style={{ marginTop: 8, color: '#6b7280', fontSize: 13 }}>
            {t('common:resources.used')}: {(totalMemory * memoryUsage / 100 / 1024).toFixed(2)}TB /
            {t('common:resources.total')}: {(totalMemory / 1024).toFixed(2)}TB
          </div>
        </Card>
      </Col>
      <Col span={8}>
        <Card
          title={<span><DatabaseOutlined style={{ color: '#8b5cf6' }} /> {t('resource.storageUsage')}</span>}
          bordered={false}
          style={{ ...cardStyle, height: 160 }}
          headStyle={cardHeadStyle}
          styles={{ body: { padding: '16px' } }}
        >
          <Progress
            percent={storageUsage}
            strokeColor={storageUsage > 80 ? '#ef4444' : storageUsage > 60 ? '#f59e0b' : '#8b5cf6'}
            format={(percent) => (
              <span style={{ fontWeight: 600, fontSize: 18 }}>{percent?.toFixed(1)}%</span>
            )}
          />
          <div style={{ marginTop: 8, color: '#6b7280', fontSize: 13 }}>
            {t('common:resources.used')}: {resourceUsage?.storage?.used?.toFixed(0) || 0}{resourceUsage?.storage?.unit || 'GB'} /
            {t('common:resources.total')}: {resourceUsage?.storage?.total?.toFixed(0) || 0}{resourceUsage?.storage?.unit || 'GB'}
          </div>
        </Card>
      </Col>
    </Row>
  );
};

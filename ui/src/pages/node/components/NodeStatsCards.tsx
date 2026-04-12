import React from 'react';
import { Row, Col, Card } from 'antd';
import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  DesktopOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import type { TFunction } from 'i18next';

interface NodeStatsCardsProps {
  totalNodes: number;
  readyNodes: number;
  notReadyNodes: number;
  maintenanceNodes: number;
  t: TFunction;
}

export const NodeStatsCards: React.FC<NodeStatsCardsProps> = ({
  totalNodes,
  readyNodes,
  notReadyNodes,
  maintenanceNodes,
  t,
}) => {
  const stats = [
    {
      label: t('overview.total'),
      value: totalNodes,
      icon: <DesktopOutlined />,
      iconBg: '#eff6ff',
      iconColor: '#3b82f6',
    },
    {
      label: t('overview.ready'),
      value: readyNodes,
      icon: <CheckCircleOutlined />,
      iconBg: '#f0fdf4',
      iconColor: '#22c55e',
    },
    {
      label: t('overview.notReady'),
      value: notReadyNodes,
      icon: <ExclamationCircleOutlined />,
      iconBg: '#fff7ed',
      iconColor: '#f97316',
    },
    {
      label: t('overview.maintenance'),
      value: maintenanceNodes,
      icon: <SettingOutlined />,
      iconBg: '#f5f3ff',
      iconColor: '#8b5cf6',
    },
  ];

  return (
    <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
      {stats.map((item) => (
        <Col key={item.label} xs={24} sm={12} lg={6}>
          <Card
            variant="borderless"
            style={{ borderRadius: 12, boxShadow: '0 1px 4px rgba(0,0,0,0.06)' }}
            styles={{ body: { padding: '20px 24px' } }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
              <div
                style={{
                  width: 44,
                  height: 44,
                  borderRadius: 10,
                  background: item.iconBg,
                  color: item.iconColor,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 20,
                  flexShrink: 0,
                }}
              >
                {item.icon}
              </div>
              <div>
                <div style={{ fontSize: 12, color: '#9ca3af', marginBottom: 2 }}>
                  {item.label}
                </div>
                <div style={{ fontSize: 28, fontWeight: 700, color: '#111827', lineHeight: 1 }}>
                  {item.value}
                </div>
              </div>
            </div>
          </Card>
        </Col>
      ))}
    </Row>
  );
};

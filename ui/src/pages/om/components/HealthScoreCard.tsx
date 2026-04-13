import EmptyState from '@/components/EmptyState';
import React from 'react';
import {
  Card,
  Row,
  Col,
  Progress,
  Tag,
  Button,
  Spin,
  Space,
  Badge,
  Typography,
  List,
  Collapse,
  Alert,
} from 'antd';
import {
  SyncOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  InfoCircleOutlined,
  ThunderboltOutlined,
  DashboardOutlined,
  HddOutlined,
  CloudServerOutlined,
  NodeIndexOutlined,
  AppstoreOutlined,
} from '@ant-design/icons';
import type { TFunction } from 'i18next';
import type { HealthDiagnosisResponse, RiskItem } from '../../../services/omService';
import { formatTime, getHealthColor } from './omUtils';

const { Title, Text, Paragraph } = Typography;
const { Panel } = Collapse;

interface HealthScoreCardProps {
  healthDiagnosis: HealthDiagnosisResponse | null;
  healthLoading: boolean;
  onRefresh: () => void;
  t: TFunction;
}

const getSeverityTag = (severity: string, t: TFunction) => {
  switch (severity) {
    case 'critical':
      return <Tag icon={<CloseCircleOutlined />} color="error">{t('om:health.severityCritical')}</Tag>;
    case 'warning':
      return <Tag icon={<WarningOutlined />} color="warning">{t('om:health.severityWarning')}</Tag>;
    case 'info':
      return <Tag icon={<InfoCircleOutlined />} color="processing">{t('om:health.severityInfo')}</Tag>;
    default:
      return <Tag>{severity}</Tag>;
  }
};

const getCategoryIcon = (category: string) => {
  switch (category) {
    case 'node': return <NodeIndexOutlined />;
    case 'workload': return <AppstoreOutlined />;
    case 'resource': return <DashboardOutlined />;
    case 'storage': return <HddOutlined />;
    case 'control_plane': return <CloudServerOutlined />;
    default: return <InfoCircleOutlined />;
  }
};

const getCategoryName = (category: string, t: TFunction): string => {
  const names: Record<string, string> = {
    node: t('om:health.categoryNode'),
    workload: t('om:health.categoryWorkload'),
    resource: t('om:health.categoryResource'),
    storage: t('om:health.categoryStorage'),
    control_plane: t('om:health.categoryControlPlane'),
    network: t('om:health.categoryNetwork'),
  };
  return names[category] || category;
};

const HealthScoreCard: React.FC<HealthScoreCardProps> = ({
  healthDiagnosis,
  healthLoading,
  onRefresh,
  t,
}) => {
  if (healthLoading) {
    return (
      <Card title={t('om:health.title')} extra={<Button icon={<SyncOutlined spin />} disabled>{t('om:refreshing')}</Button>}>
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin size="large" />
        </div>
      </Card>
    );
  }

  if (!healthDiagnosis) {
    return (
      <Card title={t('om:health.title')}>
        <EmptyState description={t('om:health.noDiagnosisData')} />
      </Card>
    );
  }

  const { health_score, status, risk_items, suggestions, category_scores, diagnosis_time } = healthDiagnosis;

  const groupedRisks = risk_items.reduce((acc, item) => {
    if (!acc[item.category]) acc[item.category] = [];
    acc[item.category].push(item);
    return acc;
  }, {} as Record<string, RiskItem[]>);

  return (
    <Card
      title={
        <Space>
          <ThunderboltOutlined style={{ color: getHealthColor(status) }} />
          <span>{t('om:health.title')}</span>
        </Space>
      }
      extra={
        <Space>
          <Text type="secondary">{t('om:health.diagnosisTime')}: {formatTime(diagnosis_time)}</Text>
          <Button icon={<SyncOutlined />} onClick={onRefresh}>{t('common:actions.refresh')}</Button>
        </Space>
      }
    >
      <Row gutter={[24, 24]}>
        <Col xs={24} md={8}>
          <div style={{ textAlign: 'center' }}>
            <Progress
              type="dashboard"
              percent={health_score}
              strokeColor={getHealthColor(status)}
              format={(percent) => (
                <div>
                  <div style={{ fontSize: 32, fontWeight: 'bold', color: getHealthColor(status) }}>{percent}</div>
                  <div style={{ fontSize: 14, color: '#666' }}>{t('om:health.healthScore')}</div>
                </div>
              )}
              size={180}
            />
            <div style={{ marginTop: 16 }}>
              <Tag color={getHealthColor(status)} style={{ fontSize: 14, padding: '4px 16px' }}>
                {status === 'healthy' ? t('om:health.statusHealthy') : status === 'warning' ? t('om:health.statusWarning') : t('om:health.statusCritical')}
              </Tag>
            </div>
          </div>
        </Col>

        <Col xs={24} md={8}>
          <Title level={5}>{t('om:health.categoryScores')}</Title>
          {Object.entries(category_scores).map(([category, score]) => (
            <div key={category} style={{ marginBottom: 12 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                <Space>
                  {getCategoryIcon(category)}
                  <span>{getCategoryName(category, t)}</span>
                </Space>
                <span style={{ color: score >= 80 ? '#52c41a' : score >= 60 ? '#faad14' : '#ff4d4f' }}>
                  {score}{t('om:health.score')}
                </span>
              </div>
              <Progress
                percent={score}
                showInfo={false}
                strokeColor={score >= 80 ? '#52c41a' : score >= 60 ? '#faad14' : '#ff4d4f'}
                size="small"
              />
            </div>
          ))}
        </Col>

        <Col xs={24} md={8}>
          <Title level={5}>{t('om:health.suggestions')}</Title>
          {suggestions.length > 0 ? (
            <List
              size="small"
              dataSource={suggestions}
              renderItem={(item, index) => (
                <List.Item style={{ padding: '8px 0' }}>
                  <Space align="start">
                    <Badge count={index + 1} style={{ backgroundColor: '#1890ff' }} />
                    <Text>{item}</Text>
                  </Space>
                </List.Item>
              )}
            />
          ) : (
            <Alert message={t('om:health.noSuggestions')} type="success" showIcon />
          )}
        </Col>
      </Row>

      {risk_items.length > 0 && (
        <div style={{ marginTop: 24 }}>
          <Title level={5}>
            <WarningOutlined style={{ marginRight: 8, color: '#faad14' }} />
            {t('om:health.riskItems')} ({risk_items.length})
          </Title>
          <Collapse accordion>
            {Object.entries(groupedRisks).map(([category, items]) => (
              <Panel
                header={
                  <Space>
                    {getCategoryIcon(category)}
                    <span>{getCategoryName(category, t)}</span>
                    <Badge count={items.length} style={{ backgroundColor: '#ff4d4f' }} />
                  </Space>
                }
                key={category}
              >
                <List
                  size="small"
                  dataSource={items}
                  renderItem={(item) => (
                    <List.Item>
                      <List.Item.Meta
                        avatar={getSeverityTag(item.severity, t)}
                        title={item.title}
                        description={
                          <div>
                            <Paragraph style={{ marginBottom: 8 }}>{item.description}</Paragraph>
                            {item.namespace && (
                              <Text type="secondary" style={{ marginRight: 16 }}>
                                {t('om:health.namespace')}: {item.namespace}
                              </Text>
                            )}
                            {item.resource && (
                              <Text type="secondary">{t('om:health.resource')}: {item.resource}</Text>
                            )}
                            <div style={{ marginTop: 8 }}>
                              <Text strong>{t('om:health.solution')}: </Text>
                              <Text>{item.solution}</Text>
                            </div>
                          </div>
                        }
                      />
                    </List.Item>
                  )}
                />
              </Panel>
            ))}
          </Collapse>
        </div>
      )}
    </Card>
  );
};

export default HealthScoreCard;

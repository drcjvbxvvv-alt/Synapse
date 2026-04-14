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
  Typography,
  Collapse,
  theme,
  Flex,
  Divider,
} from 'antd';
import {
  SyncOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  InfoCircleOutlined,
  ThunderboltOutlined,
  HddOutlined,
  CloudServerOutlined,
  NodeIndexOutlined,
  AppstoreOutlined,
  DashboardOutlined,
  CheckCircleFilled,
  ExclamationCircleFilled,
  CloseCircleFilled,
  BulbOutlined,
} from '@ant-design/icons';
import type { TFunction } from 'i18next';
import type { HealthDiagnosisResponse, RiskItem } from '../../../services/omService';
import { formatTime, getHealthColor } from './omUtils';

const { Text, Paragraph } = Typography;

interface HealthScoreCardProps {
  healthDiagnosis: HealthDiagnosisResponse | null;
  healthLoading: boolean;
  onRefresh: () => void;
  t: TFunction;
}

const getSeverityConfig = (severity: string, t: TFunction) => {
  switch (severity) {
    case 'critical':
      return {
        icon: <CloseCircleFilled />,
        color: 'error' as const,
        label: t('om:health.severityCritical'),
      };
    case 'warning':
      return {
        icon: <ExclamationCircleFilled />,
        color: 'warning' as const,
        label: t('om:health.severityWarning'),
      };
    default:
      return {
        icon: <InfoCircleOutlined />,
        color: 'processing' as const,
        label: t('om:health.severityInfo'),
      };
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

const ScoreRing: React.FC<{ score: number; status: string; t: TFunction }> = ({ score, status, t }) => {
  const { token } = theme.useToken();
  const color = getHealthColor(status);

  const statusLabel =
    status === 'healthy'
      ? t('om:health.statusHealthy')
      : status === 'warning'
      ? t('om:health.statusWarning')
      : t('om:health.statusCritical');

  const StatusIcon =
    status === 'healthy'
      ? CheckCircleFilled
      : status === 'warning'
      ? ExclamationCircleFilled
      : CloseCircleFilled;

  return (
    <div style={{ textAlign: 'center' }}>
      <Progress
        type="circle"
        percent={score}
        strokeColor={color}
        trailColor={token.colorFillSecondary}
        strokeWidth={8}
        size={160}
        format={(percent) => (
          <div>
            <div
              style={{
                fontSize: 38,
                fontWeight: 700,
                lineHeight: 1,
                color,
              }}
            >
              {percent}
            </div>
            <div
              style={{
                fontSize: token.fontSizeSM,
                color: token.colorTextTertiary,
                marginTop: token.marginXS,
              }}
            >
              {t('om:health.healthScore')}
            </div>
          </div>
        )}
      />
      <Flex
        justify="center"
        align="center"
        gap={6}
        style={{ marginTop: token.marginMD }}
      >
        <StatusIcon style={{ color, fontSize: token.fontSizeLG }} />
        <Text style={{ color, fontWeight: 600 }}>{statusLabel}</Text>
      </Flex>
    </div>
  );
};

const CategoryScores: React.FC<{
  categoryScores: Record<string, number>;
  t: TFunction;
}> = ({ categoryScores, t }) => {
  const { token } = theme.useToken();

  return (
    <div>
      <Text
        strong
        style={{ display: 'block', marginBottom: token.marginMD, fontSize: token.fontSizeLG }}
      >
        {t('om:health.categoryScores')}
      </Text>
      <Row gutter={[token.marginSM, token.marginMD]}>
        {Object.entries(categoryScores).map(([category, score]) => {
          const color =
            score >= 80
              ? token.colorSuccess
              : score >= 60
              ? token.colorWarning
              : token.colorError;
          return (
            <Col xs={12} key={category}>
              <div
                style={{
                  padding: `${token.paddingSM}px ${token.padding}px`,
                  background: token.colorFillAlter,
                  borderRadius: token.borderRadius,
                  borderLeft: `3px solid ${color}`,
                }}
              >
                <Flex justify="space-between" align="center" style={{ marginBottom: 6 }}>
                  <Flex align="center" gap={6}>
                    <span style={{ color: token.colorTextSecondary }}>
                      {getCategoryIcon(category)}
                    </span>
                    <Text style={{ fontSize: token.fontSizeSM }}>
                      {getCategoryName(category, t)}
                    </Text>
                  </Flex>
                  <Text strong style={{ color, fontSize: token.fontSizeSM }}>
                    {score}
                  </Text>
                </Flex>
                <Progress
                  percent={score}
                  showInfo={false}
                  strokeColor={color}
                  trailColor={token.colorFillSecondary}
                  size={['100%', 4]}
                />
              </div>
            </Col>
          );
        })}
      </Row>
    </div>
  );
};

const SuggestionList: React.FC<{ suggestions: string[]; t: TFunction }> = ({
  suggestions,
  t,
}) => {
  const { token } = theme.useToken();

  if (suggestions.length === 0) {
    return (
      <Flex
        align="center"
        gap={token.marginSM}
        style={{
          padding: token.padding,
          background: token.colorSuccessBg,
          borderRadius: token.borderRadius,
          border: `1px solid ${token.colorSuccessBorder}`,
        }}
      >
        <CheckCircleFilled style={{ color: token.colorSuccess }} />
        <Text style={{ color: token.colorSuccess }}>{t('om:health.noSuggestions')}</Text>
      </Flex>
    );
  }

  return (
    <div>
      <Text
        strong
        style={{ display: 'block', marginBottom: token.marginMD, fontSize: token.fontSizeLG }}
      >
        <BulbOutlined style={{ marginRight: token.marginXS, color: token.colorWarning }} />
        {t('om:health.suggestions')}
      </Text>
      <div style={{ display: 'flex', flexDirection: 'column', gap: token.marginSM }}>
        {suggestions.map((item, index) => (
          <Flex key={index} gap={token.marginSM} align="flex-start">
            <div
              style={{
                flexShrink: 0,
                width: 22,
                height: 22,
                borderRadius: '50%',
                background: token.colorPrimaryBg,
                border: `1px solid ${token.colorPrimaryBorder}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: token.fontSizeSM,
                color: token.colorPrimary,
                fontWeight: 600,
                lineHeight: 1,
              }}
            >
              {index + 1}
            </div>
            <Text style={{ fontSize: token.fontSizeSM, lineHeight: '22px' }}>{item}</Text>
          </Flex>
        ))}
      </div>
    </div>
  );
};

const HealthScoreCard: React.FC<HealthScoreCardProps> = ({
  healthDiagnosis,
  healthLoading,
  onRefresh,
  t,
}) => {
  const { token } = theme.useToken();

  if (healthLoading) {
    return (
      <Card variant="borderless">
        <Flex justify="center" style={{ padding: token.paddingXL }}>
          <Spin size="large" />
        </Flex>
      </Card>
    );
  }

  if (!healthDiagnosis) {
    return (
      <Card variant="borderless">
        <EmptyState description={t('om:health.noDiagnosisData')} />
      </Card>
    );
  }

  const { health_score, status, risk_items, suggestions, category_scores, diagnosis_time } =
    healthDiagnosis;

  const groupedRisks = risk_items.reduce(
    (acc, item) => {
      if (!acc[item.category]) acc[item.category] = [];
      acc[item.category].push(item);
      return acc;
    },
    {} as Record<string, RiskItem[]>,
  );

  const criticalCount = risk_items.filter((r) => r.severity === 'critical').length;
  const warningCount = risk_items.filter((r) => r.severity === 'warning').length;

  return (
    <Card
      variant="borderless"
      title={
        <Flex align="center" gap={token.marginSM}>
          <ThunderboltOutlined style={{ color: getHealthColor(status) }} />
          <span>{t('om:health.title')}</span>
        </Flex>
      }
      extra={
        <Flex align="center" gap={token.marginSM}>
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
            {t('om:health.diagnosisTime')}: {formatTime(diagnosis_time)}
          </Text>
          <Button size="small" icon={<SyncOutlined />} onClick={onRefresh}>
            {t('common:actions.refresh')}
          </Button>
        </Flex>
      }
    >
      <Row gutter={[token.marginXL, token.marginLG]}>
        {/* Score */}
        <Col xs={24} md={6}>
          <ScoreRing score={health_score} status={status} t={t} />

          {/* Risk summary pills */}
          {risk_items.length > 0 && (
            <Flex justify="center" gap={token.marginSM} style={{ marginTop: token.marginLG }}>
              {criticalCount > 0 && (
                <Tag
                  icon={<CloseCircleOutlined />}
                  color="error"
                  style={{ margin: 0 }}
                >
                  {criticalCount} {t('om:health.severityCritical')}
                </Tag>
              )}
              {warningCount > 0 && (
                <Tag
                  icon={<WarningOutlined />}
                  color="warning"
                  style={{ margin: 0 }}
                >
                  {warningCount} {t('om:health.severityWarning')}
                </Tag>
              )}
            </Flex>
          )}
        </Col>

        {/* Category scores */}
        <Col xs={24} md={10}>
          <CategoryScores categoryScores={category_scores} t={t} />
        </Col>

        {/* Suggestions */}
        <Col xs={24} md={8}>
          <SuggestionList suggestions={suggestions} t={t} />
        </Col>
      </Row>

      {/* Risk items */}
      {risk_items.length > 0 && (
        <>
          <Divider style={{ margin: `${token.marginLG}px 0 ${token.marginMD}px` }} />
          <div>
            <Text
              strong
              style={{ display: 'block', marginBottom: token.marginMD, fontSize: token.fontSizeLG }}
            >
              <WarningOutlined
                style={{ marginRight: token.marginXS, color: token.colorWarning }}
              />
              {t('om:health.riskItems')} ({risk_items.length})
            </Text>
            <Collapse
              size="small"
              items={Object.entries(groupedRisks).map(([category, items]) => ({
                key: category,
                label: (
                  <Flex align="center" gap={token.marginSM}>
                    <span style={{ color: token.colorTextSecondary }}>
                      {getCategoryIcon(category)}
                    </span>
                    <Text strong>{getCategoryName(category, t)}</Text>
                    <Tag
                      color={items.some((i) => i.severity === 'critical') ? 'error' : 'warning'}
                      style={{ margin: 0 }}
                    >
                      {items.length}
                    </Tag>
                  </Flex>
                ),
                children: (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: token.marginSM }}>
                    {items.map((item, i) => {
                      const sev = getSeverityConfig(item.severity, t);
                      return (
                        <div
                          key={i}
                          style={{
                            padding: token.padding,
                            background: token.colorFillAlter,
                            borderRadius: token.borderRadius,
                            borderLeft: `3px solid ${
                              item.severity === 'critical'
                                ? token.colorError
                                : item.severity === 'warning'
                                ? token.colorWarning
                                : token.colorInfo
                            }`,
                          }}
                        >
                          <Flex align="center" gap={token.marginSM} style={{ marginBottom: 6 }}>
                            <Tag icon={sev.icon} color={sev.color} style={{ margin: 0 }}>
                              {sev.label}
                            </Tag>
                            <Text strong style={{ fontSize: token.fontSizeSM }}>
                              {item.title}
                            </Text>
                          </Flex>
                          <Paragraph
                            style={{
                              margin: 0,
                              fontSize: token.fontSizeSM,
                              color: token.colorTextSecondary,
                            }}
                          >
                            {item.description}
                          </Paragraph>
                          {(item.namespace || item.resource) && (
                            <Flex gap={token.marginMD} style={{ marginTop: 6 }}>
                              {item.namespace && (
                                <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                                  {t('om:health.namespace')}: {item.namespace}
                                </Text>
                              )}
                              {item.resource && (
                                <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                                  {t('om:health.resource')}: {item.resource}
                                </Text>
                              )}
                            </Flex>
                          )}
                          <div style={{ marginTop: 8 }}>
                            <Text
                              type="secondary"
                              style={{ fontSize: token.fontSizeSM }}
                            >
                              {t('om:health.solution')}:{' '}
                            </Text>
                            <Text style={{ fontSize: token.fontSizeSM }}>{item.solution}</Text>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                ),
              }))}
            />
          </div>
        </>
      )}
    </Card>
  );
};

export default HealthScoreCard;

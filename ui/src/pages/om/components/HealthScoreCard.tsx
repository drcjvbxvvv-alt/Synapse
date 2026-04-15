import React, { useId } from 'react';
import { Button, Spin, Tag, Typography, theme, Flex, Collapse } from 'antd';
import {
  SyncOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  InfoCircleOutlined,
  HddOutlined,
  CloudServerOutlined,
  NodeIndexOutlined,
  AppstoreOutlined,
  DashboardOutlined,
  CloseCircleFilled,
  ExclamationCircleFilled,
  CheckCircleFilled,
  BulbOutlined,
  WifiOutlined,
} from '@ant-design/icons';
import type { TFunction } from 'i18next';
import type { HealthDiagnosisResponse, RiskItem } from '../../../services/omService';
import { formatTime, getHealthColor } from './omUtils';
import EmptyState from '@/components/EmptyState';
import css from '../om.module.css';

const { Text, Paragraph } = Typography;

interface HealthScoreCardProps {
  healthDiagnosis: HealthDiagnosisResponse | null;
  healthLoading: boolean;
  onRefresh: () => void;
  t: TFunction;
}

/* ── category config ───────────────────────────────────────────────────── */
const CATEGORY_CONFIG: Record<
  string,
  { icon: React.ReactNode; color: 'green' | 'blue' | 'purple' | 'amber' }
> = {
  control_plane: { icon: <CloudServerOutlined />, color: 'green' },
  node:          { icon: <NodeIndexOutlined />,   color: 'blue' },
  resource:      { icon: <DashboardOutlined />,   color: 'purple' },
  storage:       { icon: <HddOutlined />,         color: 'amber' },
  workload:      { icon: <AppstoreOutlined />,    color: 'green' },
  network:       { icon: <WifiOutlined />,        color: 'blue' },
};

const COLOR_MAP = {
  green:  { bg: 'rgba(34,197,94,0.08)',   text: '#16a34a', bar: 'linear-gradient(90deg,#22c55e,#4ade80)' },
  blue:   { bg: 'rgba(59,130,246,0.08)',  text: '#2563eb', bar: 'linear-gradient(90deg,#3b82f6,#60a5fa)' },
  purple: { bg: 'rgba(139,92,246,0.08)', text: '#7c3aed', bar: 'linear-gradient(90deg,#8b5cf6,#a78bfa)' },
  amber:  { bg: 'rgba(245,158,11,0.08)', text: '#d97706', bar: 'linear-gradient(90deg,#f59e0b,#fbbf24)' },
};

const getCategoryName = (category: string, t: TFunction): string => {
  const names: Record<string, string> = {
    node:          t('om:health.categoryNode'),
    workload:      t('om:health.categoryWorkload'),
    resource:      t('om:health.categoryResource'),
    storage:       t('om:health.categoryStorage'),
    control_plane: t('om:health.categoryControlPlane'),
    network:       t('om:health.categoryNetwork'),
  };
  return names[category] || category;
};

/* ── SVG ring ──────────────────────────────────────────────────────────── */
const HealthRing: React.FC<{ score: number; status: string; t: TFunction }> = ({
  score, status, t,
}) => {
  const gradId = useId().replace(/:/g, '');
  const R = 70;
  const CIRC = 2 * Math.PI * R; // ≈ 439.8
  const offset = CIRC * (1 - score / 100);

  const statusColor =
    status === 'healthy' ? '#22c55e' : status === 'warning' ? '#f59e0b' : '#ef4444';
  const gradEnd =
    status === 'healthy' ? '#4ade80' : status === 'warning' ? '#fbbf24' : '#f87171';

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
      <div style={{ position: 'relative', width: 160, height: 160, margin: '0 auto 16px' }}>
        <svg
          width="160"
          height="160"
          viewBox="0 0 160 160"
          style={{ transform: 'rotate(-90deg)' }}
        >
          <defs>
            <linearGradient id={gradId} x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor={statusColor} />
              <stop offset="100%" stopColor={gradEnd} />
            </linearGradient>
          </defs>
          {/* track */}
          <circle
            cx="80" cy="80" r={R}
            fill="none"
            stroke="rgba(0,0,0,0.06)"
            strokeWidth="10"
          />
          {/* progress */}
          <circle
            cx="80" cy="80" r={R}
            className={css.ringProgress}
            stroke={`url(#${gradId})`}
            strokeWidth="10"
            strokeDasharray={CIRC}
            style={
              {
                '--ring-full': CIRC,
                '--ring-offset': offset,
                filter: `drop-shadow(0 2px 6px ${statusColor}66)`,
              } as React.CSSProperties
            }
          />
        </svg>
        {/* centre label */}
        <div
          style={{
            position: 'absolute', top: '50%', left: '50%',
            transform: 'translate(-50%, -50%)', textAlign: 'center',
          }}
        >
          <div
            style={{
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: 40, fontWeight: 600,
              lineHeight: 1, color: statusColor,
            }}
          >
            {score}
          </div>
          <div style={{ fontSize: 12, color: '#999', marginTop: 4 }}>
            {t('om:health.healthScore')}
          </div>
        </div>
      </div>

      {/* status pill */}
      <div
        style={{
          display: 'inline-flex', alignItems: 'center', gap: 7,
          padding: '6px 16px',
          background: status === 'healthy' ? '#dcfce7' : status === 'warning' ? '#fef3c7' : '#fee2e2',
          borderRadius: 20,
          color: statusColor,
          fontSize: 13, fontWeight: 500,
        }}
      >
        <span
          className={css.pulseDot}
          style={{ width: 8, height: 8, background: statusColor, color: statusColor, flexShrink: 0 }}
        />
        <StatusIcon style={{ fontSize: 13 }} />
        {statusLabel}
      </div>
    </div>
  );
};

/* ── Metric card (category score) ──────────────────────────────────────── */
const MetricCard: React.FC<{
  category: string; score: number; highlight?: boolean; t: TFunction;
}> = ({ category, score, highlight, t }) => {
  const cfg = CATEGORY_CONFIG[category] ?? { icon: <InfoCircleOutlined />, color: 'blue' as const };
  const colors = COLOR_MAP[cfg.color];

  return (
    <div
      className={css.metricCard}
      style={{ background: highlight ? `${colors.bg}` : '#f7f8fa' }}
    >
      <Flex align="center" gap={10} style={{ marginBottom: 10 }}>
        <div
          style={{
            width: 32, height: 32, borderRadius: 8,
            background: colors.bg, color: colors.text,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 15, flexShrink: 0,
          }}
        >
          {cfg.icon}
        </div>
        <span style={{ fontSize: 13, color: '#666' }}>{getCategoryName(category, t)}</span>
      </Flex>
      <div
        style={{
          fontFamily: "'JetBrains Mono', monospace",
          fontSize: 26, fontWeight: 600,
          color: '#1a1a1a', marginBottom: 8,
        }}
      >
        {score}
      </div>
      <div style={{ height: 4, background: 'rgba(0,0,0,0.06)', borderRadius: 2, overflow: 'hidden' }}>
        <div
          className={css.metricFill}
          style={{ width: `${score}%`, background: colors.bar }}
        />
      </div>
    </div>
  );
};

/* ── Risk item ─────────────────────────────────────────────────────────── */
const getSeverityConfig = (severity: string, t: TFunction) => {
  switch (severity) {
    case 'critical': return { icon: <CloseCircleOutlined />, color: 'error' as const, label: t('om:health.severityCritical') };
    case 'warning':  return { icon: <WarningOutlined />,     color: 'warning' as const, label: t('om:health.severityWarning') };
    default:         return { icon: <InfoCircleOutlined />,  color: 'processing' as const, label: t('om:health.severityInfo') };
  }
};

/* ── Main component ────────────────────────────────────────────────────── */
const HealthScoreCard: React.FC<HealthScoreCardProps> = ({
  healthDiagnosis, healthLoading, onRefresh, t,
}) => {
  const { token } = theme.useToken();

  if (healthLoading) {
    return (
      <div style={cardStyle}>
        <Flex justify="center" style={{ padding: 48 }}>
          <Spin size="large" />
        </Flex>
      </div>
    );
  }

  if (!healthDiagnosis) {
    return (
      <div style={cardStyle}>
        <EmptyState description={t('om:health.noDiagnosisData')} />
      </div>
    );
  }

  const { health_score, status, risk_items, suggestions, category_scores, diagnosis_time } =
    healthDiagnosis;

  const groupedRisks = risk_items.reduce(
    (acc, item) => { (acc[item.category] ??= []).push(item); return acc; },
    {} as Record<string, RiskItem[]>,
  );

  const categoryOrder = ['control_plane', 'node', 'resource', 'storage', 'workload', 'network'];
  const sortedCategories = [
    ...categoryOrder.filter((c) => c in category_scores),
    ...Object.keys(category_scores).filter((c) => !categoryOrder.includes(c)),
  ];

  return (
    <div style={cardStyle} className={css.fadeUp}>
      {/* top accent line */}
      <div style={{
        position: 'absolute', top: 0, left: 0, right: 0, height: 3,
        background: 'linear-gradient(90deg, #22c55e, #4ade80)',
        borderRadius: '14px 14px 0 0',
      }} />

      {/* header */}
      <Flex justify="space-between" align="center" style={{ marginBottom: 24 }}>
        <Text strong style={{ fontSize: 15 }}>
          {t('om:health.title')}
        </Text>
        <Flex align="center" gap={12}>
          <Text style={{ fontSize: 12, color: '#999' }}>
            {t('om:health.diagnosisTime')}: {formatTime(diagnosis_time)}
          </Text>
          <Button size="small" icon={<SyncOutlined />} onClick={onRefresh}>
            {t('common:actions.refresh')}
          </Button>
        </Flex>
      </Flex>

      {/* body: ring left, metrics right */}
      <div style={{ display: 'grid', gridTemplateColumns: '340px 1fr', gap: 24 }}>
        {/* left — ring + suggestions */}
        <div>
          <HealthRing score={health_score} status={status} t={t} />

          {suggestions.length > 0 && (
            <div style={{ marginTop: 20 }}>
              <Flex align="center" gap={6} style={{ marginBottom: 10 }}>
                <BulbOutlined style={{ color: token.colorWarning, fontSize: 13 }} />
                <Text strong style={{ fontSize: 13 }}>{t('om:health.suggestions')}</Text>
              </Flex>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {suggestions.map((s, i) => (
                  <div
                    key={i}
                    style={{
                      background: '#f0fdf4',
                      border: '1px solid rgba(34,197,94,0.2)',
                      borderRadius: 8, padding: '10px 12px',
                      display: 'flex', gap: 10, alignItems: 'flex-start',
                    }}
                  >
                    <div style={{
                      flexShrink: 0, width: 20, height: 20, borderRadius: '50%',
                      background: 'rgba(34,197,94,0.12)', color: '#16a34a',
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                      fontSize: 11, fontWeight: 700, lineHeight: 1,
                    }}>
                      {i + 1}
                    </div>
                    <Text style={{ fontSize: 12, color: '#555', lineHeight: '20px' }}>{s}</Text>
                  </div>
                ))}
              </div>
            </div>
          )}

          {suggestions.length === 0 && (
            <div style={{
              marginTop: 16, padding: '12px 14px',
              background: '#f0fdf4', borderRadius: 10,
              border: '1px solid rgba(34,197,94,0.2)',
              display: 'flex', alignItems: 'center', gap: 8,
            }}>
              <CheckCircleFilled style={{ color: '#22c55e' }} />
              <Text style={{ fontSize: 13, color: '#16a34a' }}>{t('om:health.noSuggestions')}</Text>
            </div>
          )}
        </div>

        {/* right — category scores grid */}
        <div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
            {sortedCategories.map((cat, i) => (
              <MetricCard
                key={cat}
                category={cat}
                score={category_scores[cat]}
                highlight={i === 0}
                t={t}
              />
            ))}
          </div>
        </div>
      </div>

      {/* risk items */}
      {risk_items.length > 0 && (
        <div style={{ marginTop: 24, borderTop: '1px solid rgba(0,0,0,0.06)', paddingTop: 20 }}>
          <Flex align="center" gap={8} style={{ marginBottom: 12 }}>
            <WarningOutlined style={{ color: token.colorWarning }} />
            <Text strong style={{ fontSize: 14 }}>
              {t('om:health.riskItems')}
              <span style={{ marginLeft: 6, fontSize: 12, color: '#999', fontWeight: 400 }}>
                ({risk_items.length})
              </span>
            </Text>
          </Flex>
          <Collapse
            size="small"
            items={Object.entries(groupedRisks).map(([category, items]) => ({
              key: category,
              label: (
                <Flex align="center" gap={8}>
                  <span style={{ color: '#888', fontSize: 13 }}>
                    {CATEGORY_CONFIG[category]?.icon ?? <InfoCircleOutlined />}
                  </span>
                  <Text style={{ fontSize: 13 }}>{getCategoryName(category, t)}</Text>
                  <Tag
                    color={items.some((i) => i.severity === 'critical') ? 'error' : 'warning'}
                    style={{ margin: 0, fontSize: 11 }}
                  >
                    {items.length}
                  </Tag>
                </Flex>
              ),
              children: (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                  {items.map((item, i) => {
                    const sev = getSeverityConfig(item.severity, t);
                    const borderColor =
                      item.severity === 'critical' ? token.colorError
                      : item.severity === 'warning' ? token.colorWarning
                      : token.colorInfo;
                    return (
                      <div
                        key={i}
                        style={{
                          padding: '12px 14px',
                          background: '#fafafa',
                          borderRadius: 8,
                          borderLeft: `3px solid ${borderColor}`,
                        }}
                      >
                        <Flex align="center" gap={8} style={{ marginBottom: 6 }}>
                          <Tag icon={sev.icon} color={sev.color} style={{ margin: 0 }}>
                            {sev.label}
                          </Tag>
                          <Text strong style={{ fontSize: 13 }}>{item.title}</Text>
                        </Flex>
                        <Paragraph style={{ margin: 0, fontSize: 12, color: '#666' }}>
                          {item.description}
                        </Paragraph>
                        {(item.namespace || item.resource) && (
                          <Flex gap={16} style={{ marginTop: 4 }}>
                            {item.namespace && (
                              <Text type="secondary" style={{ fontSize: 12 }}>
                                {t('om:health.namespace')}: {item.namespace}
                              </Text>
                            )}
                            {item.resource && (
                              <Text type="secondary" style={{ fontSize: 12 }}>
                                {t('om:health.resource')}: {item.resource}
                              </Text>
                            )}
                          </Flex>
                        )}
                        <div style={{ marginTop: 8 }}>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {t('om:health.solution')}:{' '}
                          </Text>
                          <Text style={{ fontSize: 12 }}>{item.solution}</Text>
                        </div>
                      </div>
                    );
                  })}
                </div>
              ),
            }))}
          />
        </div>
      )}
    </div>
  );
};

const cardStyle: React.CSSProperties = {
  background: '#fff',
  borderRadius: 16,
  padding: 24,
  border: '1px solid rgba(0,0,0,0.06)',
  boxShadow: '0 4px 14px rgba(0,0,0,0.06)',
  position: 'relative',
  overflow: 'hidden',
};

export default HealthScoreCard;

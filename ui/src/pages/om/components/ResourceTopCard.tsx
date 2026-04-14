import React, { useState } from 'react';
import { Button, Select, Typography, theme, Flex } from 'antd';
import { SyncOutlined, BarChartOutlined } from '@ant-design/icons';
import type { TFunction } from 'i18next';
import type { ResourceTopResponse, ResourceTopItem } from '../../../services/omService';
import EmptyState from '@/components/EmptyState';
import { formatBytes, formatCPU, formatTime } from './omUtils';
import css from '../om.module.css';

const { Text } = Typography;

interface ResourceTopCardProps {
  resourceTop: ResourceTopResponse | null;
  resourceLoading: boolean;
  resourceType: 'cpu' | 'memory' | 'disk' | 'network';
  setResourceType: (type: 'cpu' | 'memory' | 'disk' | 'network') => void;
  resourceLevel: 'namespace' | 'workload' | 'pod';
  setResourceLevel: (level: 'namespace' | 'workload' | 'pod') => void;
  onRefresh: () => void;
  t: TFunction;
}

const RESOURCE_COLORS: Record<string, { from: string; to: string }> = {
  cpu:     { from: '#22c55e', to: '#4ade80' },
  memory:  { from: '#3b82f6', to: '#60a5fa' },
  disk:    { from: '#f59e0b', to: '#fbbf24' },
  network: { from: '#8b5cf6', to: '#a78bfa' },
};

function formatUsage(item: ResourceTopItem): string {
  if (item.unit === 'bytes' || item.unit === 'bytes/s') return formatBytes(item.usage);
  if (item.unit === 'cores') return formatCPU(item.usage);
  return `${item.usage.toFixed(1)} ${item.unit}`;
}

function shortName(name: string, namespace?: string): string {
  const full = namespace ? `${namespace}/${name}` : name;
  return full.length > 22 ? full.slice(0, 20) + '…' : full;
}

const ResourceTopCard: React.FC<ResourceTopCardProps> = ({
  resourceTop,
  resourceLoading,
  resourceType,
  setResourceType,
  resourceLevel,
  setResourceLevel,
  onRefresh,
  t,
}) => {
  const { token } = theme.useToken();
  const colors = RESOURCE_COLORS[resourceType];

  const items: ResourceTopItem[] = resourceTop?.items ?? [];
  const maxRate = Math.max(...items.map((i) => i.usage_rate), 1);

  return (
    <div style={cardStyle} className={css.fadeUpDelay1}>
      {/* header */}
      <Flex justify="space-between" align="center" style={{ marginBottom: 20 }}>
        <Flex align="center" gap={8}>
          <BarChartOutlined style={{ color: colors.from, fontSize: 16 }} />
          <Text strong style={{ fontSize: 15 }}>{t('om:resourceTop.title')}</Text>
        </Flex>
        <Flex align="center" gap={8}>
          {/* resource type tabs */}
          <div style={{ display: 'flex', gap: 4 }}>
            {(['cpu', 'memory', 'disk', 'network'] as const).map((type) => (
              <button
                key={type}
                onClick={() => setResourceType(type)}
                style={{
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: 11, fontWeight: 500,
                  padding: '5px 10px',
                  borderRadius: 6,
                  border: '1px solid',
                  cursor: 'pointer',
                  transition: 'all 0.2s',
                  background: resourceType === type
                    ? `rgba(${hexToRgb(RESOURCE_COLORS[type].from)},0.1)`
                    : 'transparent',
                  borderColor: resourceType === type
                    ? `rgba(${hexToRgb(RESOURCE_COLORS[type].from)},0.35)`
                    : 'rgba(0,0,0,0.1)',
                  color: resourceType === type ? RESOURCE_COLORS[type].from : '#888',
                }}
              >
                {type.toUpperCase()}
              </button>
            ))}
          </div>
          <Select
            value={resourceLevel}
            onChange={setResourceLevel}
            size="small"
            style={{ width: 96 }}
            options={[
              { label: t('om:resourceTop.namespaceLevel'), value: 'namespace' },
              { label: t('om:resourceTop.workloadLevel'), value: 'workload' },
              { label: 'Pod', value: 'pod' },
            ]}
          />
          <Button
            size="small"
            icon={<SyncOutlined spin={resourceLoading} />}
            onClick={onRefresh}
          />
        </Flex>
      </Flex>

      {/* bar chart */}
      {items.length === 0 ? (
        <EmptyState />
      ) : (
        <div
          style={{
            height: 220,
            display: 'flex',
            alignItems: 'flex-end',
            justifyContent: 'space-between',
            gap: 8,
            paddingTop: 24,
            paddingBottom: 0,
          }}
        >
          {items.slice(0, 10).map((item, idx) => {
            const heightPct = (item.usage_rate / maxRate) * 100;
            const delayMs = 80 + idx * 35;
            return (
              <div
                key={idx}
                title={`${shortName(item.name, item.namespace)}\n${formatUsage(item)} · ${item.usage_rate.toFixed(1)}%`}
                style={{
                  flex: 1,
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  gap: 6,
                  height: '100%',
                  justifyContent: 'flex-end',
                }}
              >
                {/* value label shown above bar */}
                <Text
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: 10, color: '#999',
                    opacity: heightPct > 30 ? 1 : 0,
                  }}
                >
                  {item.usage_rate.toFixed(0)}%
                </Text>
                <div
                  className={css.bar}
                  style={{
                    width: '100%',
                    maxWidth: 40,
                    height: `${Math.max(heightPct, 4)}%`,
                    background: `linear-gradient(180deg, ${colors.from} 0%, ${colors.to} 100%)`,
                    animationDelay: `${delayMs}ms`,
                  }}
                />
                <Text
                  style={{
                    fontSize: 10, color: token.colorTextTertiary,
                    textAlign: 'center', maxWidth: 48,
                    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                  }}
                >
                  {shortName(item.name)}
                </Text>
              </div>
            );
          })}
        </div>
      )}

      {resourceTop && (
        <div style={{ marginTop: 14, textAlign: 'right' }}>
          <Text style={{ fontSize: 11, color: '#bbb' }}>
            {t('om:resourceTop.queryTime')}: {formatTime(resourceTop.query_time)}
          </Text>
        </div>
      )}
    </div>
  );
};

function hexToRgb(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `${r},${g},${b}`;
}

const cardStyle: React.CSSProperties = {
  background: '#fff',
  borderRadius: 16,
  padding: 24,
  border: '1px solid rgba(0,0,0,0.06)',
  boxShadow: '0 4px 14px rgba(0,0,0,0.06)',
  height: '100%',
};

export default ResourceTopCard;

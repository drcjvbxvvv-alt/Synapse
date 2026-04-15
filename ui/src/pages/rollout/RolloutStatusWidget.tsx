/**
 * RolloutStatusWidget — 嵌入式 Rollout 狀態卡片（P3-3）
 *
 * 用途：嵌入 PipelineRunDetail 或其他頁面，顯示指定 Rollout 的即時狀態。
 * Props：
 *  - clusterId: 叢集 ID
 *  - namespace: 命名空間
 *  - name: Rollout 名稱
 *  - onViewDetail?: 點擊「查看詳情」的回呼
 */
import React from 'react';
import { Card, Tag, Flex, Progress, Typography, Button, Spin } from 'antd';
import { LinkOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';

import rolloutService from '../../services/rolloutService';

const { Text } = Typography;

interface RolloutStatusWidgetProps {
  clusterId: number;
  namespace: string;
  name: string;
  onViewDetail?: () => void;
}

const RolloutStatusWidget: React.FC<RolloutStatusWidgetProps> = ({
  clusterId, namespace, name, onViewDetail,
}) => {
  const { t } = useTranslation('rollout');

  const { data: rollout, isLoading } = useQuery({
    queryKey: ['rollout', clusterId, namespace, name],
    queryFn: () => rolloutService.get(clusterId, namespace, name),
    enabled: clusterId > 0 && !!namespace && !!name,
    refetchInterval: 5000,
    staleTime: 3000,
  });

  const statusColor: Record<string, string> = {
    Healthy: 'success',
    Stopped: 'default',
    Degraded: 'error',
  };

  if (isLoading) {
    return (
      <Card size="small" variant="borderless" title={t('rollout:widget.title')} style={{ minHeight: 80 }}>
        <Flex justify="center" align="center" style={{ height: 60 }}>
          <Spin size="small" />
        </Flex>
      </Card>
    );
  }

  if (!rollout) {
    return (
      <Card size="small" variant="borderless" title={t('rollout:widget.title')}>
        <Text type="secondary">{t('rollout:widget.noRollout')}</Text>
      </Card>
    );
  }

  return (
    <Card
      size="small"
      variant="borderless"
      title={
        <Flex align="center" gap={8}>
          <Text strong style={{ fontSize: 13 }}>{t('rollout:widget.title')}</Text>
          <Tag color={statusColor[rollout.status] ?? 'processing'} style={{ margin: 0 }}>
            {t(`rollout:status.${rollout.status}`, { defaultValue: rollout.status })}
          </Tag>
        </Flex>
      }
      extra={
        onViewDetail ? (
          <Button type="link" size="small" icon={<LinkOutlined />} onClick={onViewDetail}>
            {t('common:actions.detail', { defaultValue: 'Detail' })}
          </Button>
        ) : null
      }
    >
      <Flex gap={16} align="flex-start" wrap="wrap">
        {/* Strategy + name */}
        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>
            {name}
          </Text>
          <Tag
            color={rollout.strategy === 'Canary' ? 'blue' : 'purple'}
            style={{ marginTop: 2 }}
          >
            {t(`rollout:strategy.${rollout.strategy}`, { defaultValue: rollout.strategy })}
          </Tag>
        </div>

        {/* Replicas */}
        <div>
          <Text type="secondary" style={{ fontSize: 11 }}>Replicas</Text>
          <div>
            <Text strong>{rollout.ready_replicas}</Text>
            <Text type="secondary">/{rollout.replicas}</Text>
          </div>
        </div>

        {/* Canary weight */}
        {rollout.strategy === 'Canary' && (
          <div style={{ minWidth: 140, flex: 1 }}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t('rollout:detail.currentWeight')}
            </Text>
            <Flex align="center" gap={8} style={{ marginTop: 2 }}>
              <Progress
                percent={rollout.current_weight ?? 0}
                size="small"
                style={{ flex: 1, marginBottom: 0 }}
                status={(rollout.current_weight ?? 0) === 100 ? 'success' : 'active'}
              />
              <Text style={{ fontSize: 11, whiteSpace: 'nowrap' }}>
                {rollout.current_weight ?? 0}%
              </Text>
            </Flex>
            {rollout.current_step_count != null && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                {t('rollout:detail.stepIndex')} {rollout.current_step_index ?? 0}/{rollout.current_step_count}
              </Text>
            )}
          </div>
        )}

        {/* BlueGreen selectors */}
        {rollout.strategy === 'BlueGreen' && (
          <div>
            {rollout.active_selector && (
              <div>
                <Text type="secondary" style={{ fontSize: 11 }}>{t('rollout:detail.activeSelector')}: </Text>
                <Tag color="blue" style={{ fontSize: 11 }}>{rollout.active_selector}</Tag>
              </div>
            )}
            {rollout.preview_selector && (
              <div style={{ marginTop: 2 }}>
                <Text type="secondary" style={{ fontSize: 11 }}>{t('rollout:detail.previewSelector')}: </Text>
                <Tag color="purple" style={{ fontSize: 11 }}>{rollout.preview_selector}</Tag>
              </div>
            )}
          </div>
        )}
      </Flex>
    </Card>
  );
};

export default RolloutStatusWidget;

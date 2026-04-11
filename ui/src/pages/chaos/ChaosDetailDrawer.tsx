import React from 'react';
import { Drawer, Descriptions, Tag, Spin, Typography, theme } from 'antd';
import { useQuery } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { chaosService, type ChaosExperiment, type ChaosKind } from '../../services/chaosService';

interface Props {
  open: boolean;
  experiment: ChaosExperiment | null;
  clusterId: string;
  onClose: () => void;
}

const KIND_COLOR: Record<ChaosKind, string> = {
  PodChaos:     'volcano',
  NetworkChaos: 'geekblue',
  StressChaos:  'purple',
  HTTPChaos:    'cyan',
  IOChaos:      'gold',
};

const PHASE_COLOR: Record<string, string> = {
  Running:   'processing',
  Injecting: 'processing',
  Waiting:   'warning',
  Paused:    'default',
  Finished:  'success',
  Failed:    'error',
  Stopped:   'default',
};

const ChaosDetailDrawer: React.FC<Props> = ({ open, experiment, clusterId, onClose }) => {
  const { token } = theme.useToken();

  const { data: detail, isLoading } = useQuery({
    queryKey: ['chaos-detail', clusterId, experiment?.namespace, experiment?.kind, experiment?.name],
    queryFn: () =>
      chaosService.getExperiment(
        clusterId,
        experiment!.namespace,
        experiment!.kind,
        experiment!.name,
      ),
    enabled: open && !!experiment,
    staleTime: 10_000,
  });

  const spec = detail && typeof detail === 'object' && 'spec' in detail
    ? (detail as Record<string, unknown>).spec
    : null;

  return (
    <Drawer
      title={experiment?.name ?? '實驗詳情'}
      open={open}
      onClose={onClose}
      width={640}
    >
      {isLoading ? (
        <Spin style={{ display: 'block', marginTop: token.marginXL }} />
      ) : (
        <>
          <Descriptions column={2} bordered size="small" style={{ marginBottom: token.marginLG }}>
            <Descriptions.Item label="名稱" span={2}>
              {experiment?.name}
            </Descriptions.Item>
            <Descriptions.Item label="Namespace">
              {experiment?.namespace}
            </Descriptions.Item>
            <Descriptions.Item label="類型">
              <Tag color={KIND_COLOR[experiment?.kind as ChaosKind] ?? 'default'}>
                {experiment?.kind}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="狀態">
              <Tag color={PHASE_COLOR[experiment?.phase ?? ''] ?? 'default'}>
                {experiment?.phase || '—'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="持續時間">
              {experiment?.duration || '—'}
            </Descriptions.Item>
            <Descriptions.Item label="建立時間" span={2}>
              {experiment?.created_at
                ? dayjs(experiment.created_at).format('YYYY-MM-DD HH:mm:ss')
                : '—'}
            </Descriptions.Item>
          </Descriptions>

          {spec && (
            <>
              <Typography.Title level={5} style={{ marginBottom: token.marginSM }}>
                Spec
              </Typography.Title>
              <pre
                style={{
                  background: token.colorBgLayout,
                  padding: token.paddingSM,
                  borderRadius: token.borderRadius,
                  fontSize: token.fontSizeSM,
                  overflowX: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {JSON.stringify(spec, null, 2)}
              </pre>
            </>
          )}
        </>
      )}
    </Drawer>
  );
};

export default ChaosDetailDrawer;

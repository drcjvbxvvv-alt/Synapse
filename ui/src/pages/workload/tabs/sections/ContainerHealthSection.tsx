import React from 'react';
import { Card, Descriptions, Tag } from 'antd';
import EmptyState from '@/components/EmptyState';
import type { TFunction } from 'i18next';
import type { ContainerInfo, ProbeConfig } from '../containerTypes';

interface ContainerHealthSectionProps {
  container: ContainerInfo;
  t: TFunction;
}

function ProbeDetail({ probe, title, t }: { probe: ProbeConfig | undefined; title: string; t: TFunction }) {
  if (!probe) {
    return (
      <Card title={title} size="small" style={{ marginBottom: 16 }}>
        <EmptyState description={t('container.probe.notConfigured', { name: title })} />
      </Card>
    );
  }

  let probeType = t('container.probe.unknown');
  if (probe.httpGet) probeType = t('container.probe.httpGet');
  else if (probe.tcpSocket) probeType = t('container.probe.tcpSocket');
  else if (probe.exec) probeType = t('container.probe.exec');
  else if (probe.grpc) probeType = t('container.probe.grpc');

  return (
    <Card title={title} size="small" style={{ marginBottom: 16 }}>
      <Descriptions column={1} size="small" bordered>
        <Descriptions.Item label={t('container.probe.checkType')}>
          <Tag color="blue">{probeType}</Tag>
        </Descriptions.Item>

        {probe.httpGet && (
          <>
            <Descriptions.Item label={t('container.probe.httpPath')}>{probe.httpGet.path || '/'}</Descriptions.Item>
            <Descriptions.Item label={t('container.probe.port')}>{probe.httpGet.port}</Descriptions.Item>
            <Descriptions.Item label={t('container.probe.protocol')}>{probe.httpGet.scheme || 'HTTP'}</Descriptions.Item>
            {probe.httpGet.host && (
              <Descriptions.Item label={t('container.probe.host')}>{probe.httpGet.host}</Descriptions.Item>
            )}
            {probe.httpGet.httpHeaders && probe.httpGet.httpHeaders.length > 0 && (
              <Descriptions.Item label={t('container.probe.httpHeaders')}>
                {probe.httpGet.httpHeaders.map((header: { name: string; value: string }, idx: number) => (
                  <Tag key={idx}>{header.name}: {header.value}</Tag>
                ))}
              </Descriptions.Item>
            )}
          </>
        )}

        {probe.tcpSocket && (
          <>
            <Descriptions.Item label={t('container.probe.tcpPort')}>{probe.tcpSocket.port}</Descriptions.Item>
            {probe.tcpSocket.host && (
              <Descriptions.Item label={t('container.probe.host')}>{probe.tcpSocket.host}</Descriptions.Item>
            )}
          </>
        )}

        {probe.exec && (
          <Descriptions.Item label={t('container.probe.exec')}>
            <code style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
              {probe.exec.command?.join(' ') || '-'}
            </code>
          </Descriptions.Item>
        )}

        {probe.grpc && (
          <>
            <Descriptions.Item label={t('container.probe.grpcPort')}>{probe.grpc.port}</Descriptions.Item>
            {probe.grpc.service && (
              <Descriptions.Item label={t('container.probe.serviceName')}>{probe.grpc.service}</Descriptions.Item>
            )}
          </>
        )}

        <Descriptions.Item label={t('container.probe.initialDelay')}>{probe.initialDelaySeconds || 0} {t('container.probe.seconds')}</Descriptions.Item>
        <Descriptions.Item label={t('container.probe.checkInterval')}>{probe.periodSeconds || 10} {t('container.probe.seconds')}</Descriptions.Item>
        <Descriptions.Item label={t('container.probe.timeout')}>{probe.timeoutSeconds || 1} {t('container.probe.seconds')}</Descriptions.Item>
        <Descriptions.Item label={t('container.probe.successThreshold')}>{probe.successThreshold || 1} {t('container.probe.times')}</Descriptions.Item>
        <Descriptions.Item label={t('container.probe.failureThreshold')}>{probe.failureThreshold || 3} {t('container.probe.times')}</Descriptions.Item>
        {probe.terminationGracePeriodSeconds !== undefined && (
          <Descriptions.Item label={t('container.probe.terminationGracePeriod')}>{probe.terminationGracePeriodSeconds} {t('container.probe.seconds')}</Descriptions.Item>
        )}
      </Descriptions>
    </Card>
  );
}

export const ContainerHealthSection: React.FC<ContainerHealthSectionProps> = ({ container, t }) => {
  return (
    <div>
      <ProbeDetail probe={container.startupProbe} title={t('container.probe.startup')} t={t} />
      <ProbeDetail probe={container.livenessProbe} title={t('container.probe.liveness')} t={t} />
      <ProbeDetail probe={container.readinessProbe} title={t('container.probe.readiness')} t={t} />
    </div>
  );
};

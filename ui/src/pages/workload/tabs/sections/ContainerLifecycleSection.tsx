import React from 'react';
import { Card, Descriptions } from 'antd';
import EmptyState from '@/components/EmptyState';
import type { TFunction } from 'i18next';
import type { ContainerInfo } from '../containerTypes';

interface ContainerLifecycleSectionProps {
  container: ContainerInfo;
  t: TFunction;
}

export const ContainerLifecycleSection: React.FC<ContainerLifecycleSectionProps> = ({ container, t }) => {
  const { command, args, workingDir, lifecycle } = container;
  const hasLifecycleConfig = command || args || workingDir || lifecycle?.postStart || lifecycle?.preStop;

  if (!hasLifecycleConfig) {
    return (
      <Card title={t('container.lifecycle.title')} size="small">
        <EmptyState description={t('container.lifecycle.noConfig')} />
      </Card>
    );
  }

  return (
    <div>
      <Card title={t('container.lifecycle.command')} size="small" style={{ marginBottom: 16 }}>
        <Descriptions column={1} size="small" bordered>
          <Descriptions.Item label="Command (ENTRYPOINT)">
            {command && command.length > 0 ? (
              <code style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                {command.join(' ')}
              </code>
            ) : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="Args (CMD)">
            {args && args.length > 0 ? (
              <code style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                {args.join(' ')}
              </code>
            ) : '-'}
          </Descriptions.Item>
          <Descriptions.Item label={t('container.lifecycle.workingDir')}>
            {workingDir || '-'}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title={t('container.lifecycle.postStart')} size="small" style={{ marginBottom: 16 }}>
        {lifecycle?.postStart ? (
          <Descriptions column={1} size="small">
            {lifecycle.postStart.exec && (
              <Descriptions.Item label={t('container.lifecycle.execCommand')}>
                <code style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                  {lifecycle.postStart.exec.command?.join(' ') || '-'}
                </code>
              </Descriptions.Item>
            )}
            {lifecycle.postStart.httpGet && (
              <Descriptions.Item label={t('container.lifecycle.httpRequest')}>
                {lifecycle.postStart.httpGet.scheme || 'HTTP'}://{lifecycle.postStart.httpGet.host || 'localhost'}:{lifecycle.postStart.httpGet.port}{lifecycle.postStart.httpGet.path}
              </Descriptions.Item>
            )}
            {lifecycle.postStart.tcpSocket && (
              <Descriptions.Item label={t('container.lifecycle.tcpPort')}>
                {lifecycle.postStart.tcpSocket.host || 'localhost'}:{lifecycle.postStart.tcpSocket.port}
              </Descriptions.Item>
            )}
          </Descriptions>
        ) : (
          <EmptyState description={t('container.lifecycle.noPostStart')} />
        )}
      </Card>

      <Card title={t('container.lifecycle.preStop')} size="small">
        {lifecycle?.preStop ? (
          <Descriptions column={1} size="small">
            {lifecycle.preStop.exec && (
              <Descriptions.Item label={t('container.lifecycle.execCommand')}>
                <code style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                  {lifecycle.preStop.exec.command?.join(' ') || '-'}
                </code>
              </Descriptions.Item>
            )}
            {lifecycle.preStop.httpGet && (
              <Descriptions.Item label={t('container.lifecycle.httpRequest')}>
                {lifecycle.preStop.httpGet.scheme || 'HTTP'}://{lifecycle.preStop.httpGet.host || 'localhost'}:{lifecycle.preStop.httpGet.port}{lifecycle.preStop.httpGet.path}
              </Descriptions.Item>
            )}
            {lifecycle.preStop.tcpSocket && (
              <Descriptions.Item label={t('container.lifecycle.tcpPort')}>
                {lifecycle.preStop.tcpSocket.host || 'localhost'}:{lifecycle.preStop.tcpSocket.port}
              </Descriptions.Item>
            )}
          </Descriptions>
        ) : (
          <EmptyState description={t('container.lifecycle.noPreStop')} />
        )}
      </Card>
    </div>
  );
};

import React from 'react';
import { Card, Descriptions, Tag } from 'antd';
import EmptyState from '@/components/EmptyState';
import type { TFunction } from 'i18next';
import type { ContainerInfo } from '../containerTypes';

interface ContainerEnvSectionProps {
  container: ContainerInfo;
  t: TFunction;
}

type EnvValueFrom = {
  configMapKeyRef?: { name: string; key: string; optional?: boolean };
  secretKeyRef?: { name: string; key: string; optional?: boolean };
  fieldRef?: { fieldPath: string; apiVersion?: string };
  resourceFieldRef?: { containerName?: string; resource: string; divisor?: string };
};

function renderEnvValueFrom(valueFrom: EnvValueFrom | undefined, t: TFunction): React.ReactNode {
  if (!valueFrom) return '-';

  if (valueFrom.configMapKeyRef) {
    return (
      <span>
        <Tag color="blue">ConfigMap</Tag>
        {valueFrom.configMapKeyRef.name} / {valueFrom.configMapKeyRef.key}
        {valueFrom.configMapKeyRef.optional && <Tag>{t('container.env.optional')}</Tag>}
      </span>
    );
  }
  if (valueFrom.secretKeyRef) {
    return (
      <span>
        <Tag color="orange">Secret</Tag>
        {valueFrom.secretKeyRef.name} / {valueFrom.secretKeyRef.key}
        {valueFrom.secretKeyRef.optional && <Tag>{t('container.env.optional')}</Tag>}
      </span>
    );
  }
  if (valueFrom.fieldRef) {
    return (
      <span>
        <Tag color="green">{t('container.env.podField')}</Tag>
        {valueFrom.fieldRef.fieldPath}
      </span>
    );
  }
  if (valueFrom.resourceFieldRef) {
    return (
      <span>
        <Tag color="purple">{t('container.env.resourceField')}</Tag>
        {valueFrom.resourceFieldRef.containerName && `${valueFrom.resourceFieldRef.containerName}/`}
        {valueFrom.resourceFieldRef.resource}
      </span>
    );
  }
  return JSON.stringify(valueFrom);
}

export const ContainerEnvSection: React.FC<ContainerEnvSectionProps> = ({ container, t }) => {
  const hasEnv = container.env && container.env.length > 0;
  const hasEnvFrom = container.envFrom && container.envFrom.length > 0;

  if (!hasEnv && !hasEnvFrom) {
    return <EmptyState description={t('container.env.noEnv')} />;
  }

  return (
    <div>
      {hasEnvFrom && (
        <Card title={t('container.env.envFrom')} size="small" style={{ marginBottom: 16 }}>
          <Descriptions column={1} size="small" bordered>
            {container.envFrom!.map((envFrom, index) => {
              if (envFrom.configMapRef) {
                return (
                  <Descriptions.Item key={index} label={<Tag color="blue">ConfigMap</Tag>}>
                    {envFrom.configMapRef.name}
                    {envFrom.prefix && <span> ({t('container.env.prefix')}: {envFrom.prefix})</span>}
                    {envFrom.configMapRef.optional && <Tag>{t('container.env.optional')}</Tag>}
                  </Descriptions.Item>
                );
              }
              if (envFrom.secretRef) {
                return (
                  <Descriptions.Item key={index} label={<Tag color="orange">Secret</Tag>}>
                    {envFrom.secretRef.name}
                    {envFrom.prefix && <span> ({t('container.env.prefix')}: {envFrom.prefix})</span>}
                    {envFrom.secretRef.optional && <Tag>{t('container.env.optional')}</Tag>}
                  </Descriptions.Item>
                );
              }
              return null;
            })}
          </Descriptions>
        </Card>
      )}

      {hasEnv && (
        <Card title={t('container.env.title')} size="small">
          <Descriptions column={1} size="small" bordered>
            {container.env!.map((env, index) => (
              <Descriptions.Item key={index} label={<code>{env.name}</code>}>
                {env.value ? (
                  <code style={{ wordBreak: 'break-all' }}>{env.value}</code>
                ) : (
                  renderEnvValueFrom(env.valueFrom, t)
                )}
              </Descriptions.Item>
            ))}
          </Descriptions>
        </Card>
      )}
    </div>
  );
};

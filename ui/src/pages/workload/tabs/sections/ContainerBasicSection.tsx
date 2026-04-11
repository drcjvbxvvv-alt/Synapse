import React from 'react';
import { Card, Descriptions, Tag, Row, Col } from 'antd';
import type { TFunction } from 'i18next';
import type { ContainerInfo } from '../containerTypes';

interface ContainerBasicSectionProps {
  container: ContainerInfo;
  t: TFunction;
}

export const ContainerBasicSection: React.FC<ContainerBasicSectionProps> = ({ container, t }) => {
  return (
    <div>
      <Card title={t('container.basic.title')} size="small" style={{ marginBottom: 16 }}>
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label={t('container.basic.containerName')}>{container.name}</Descriptions.Item>
          <Descriptions.Item label={t('container.basic.imageName')}>
            <code style={{ wordBreak: 'break-all' }}>{container.image}</code>
          </Descriptions.Item>
          <Descriptions.Item label={t('container.basic.imagePullPolicy')}>
            <Tag color={
              container.imagePullPolicy === 'Always' ? 'blue' :
              container.imagePullPolicy === 'Never' ? 'red' : 'green'
            }>
              {container.imagePullPolicy || 'IfNotPresent'}
            </Tag>
          </Descriptions.Item>
          {container.workingDir && (
            <Descriptions.Item label={t('container.basic.workingDir')}>{container.workingDir}</Descriptions.Item>
          )}
          {container.command && container.command.length > 0 && (
            <Descriptions.Item label={t('container.basic.command')}>
              <code style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                {container.command.join(' ')}
              </code>
            </Descriptions.Item>
          )}
          {container.args && container.args.length > 0 && (
            <Descriptions.Item label={t('container.basic.args')}>
              <code style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                {container.args.join(' ')}
              </code>
            </Descriptions.Item>
          )}
          {container.stdin !== undefined && (
            <Descriptions.Item label={t('container.basic.stdin')}>
              {container.stdin ? t('container.basic.on') : t('container.basic.off')}
            </Descriptions.Item>
          )}
          {container.tty !== undefined && (
            <Descriptions.Item label={t('container.basic.tty')}>
              {container.tty ? t('container.basic.on') : t('container.basic.off')}
            </Descriptions.Item>
          )}
        </Descriptions>
      </Card>

      <Card title={t('container.resources.title')} size="small" style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={12}>
            <Card size="small" title={t('container.resources.requests')} type="inner">
              <Descriptions column={1} size="small">
                <Descriptions.Item label={t('container.resources.cpu')}>{container.resources?.requests?.cpu || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('container.resources.memory')}>{container.resources?.requests?.memory || '-'}</Descriptions.Item>
                {container.resources?.requests?.['ephemeral-storage'] && (
                  <Descriptions.Item label={t('container.resources.ephemeralStorage')}>{container.resources.requests['ephemeral-storage']}</Descriptions.Item>
                )}
              </Descriptions>
            </Card>
          </Col>
          <Col span={12}>
            <Card size="small" title={t('container.resources.limits')} type="inner">
              <Descriptions column={1} size="small">
                <Descriptions.Item label={t('container.resources.cpu')}>{container.resources?.limits?.cpu || '-'}</Descriptions.Item>
                <Descriptions.Item label={t('container.resources.memory')}>{container.resources?.limits?.memory || '-'}</Descriptions.Item>
                {container.resources?.limits?.['ephemeral-storage'] && (
                  <Descriptions.Item label={t('container.resources.ephemeralStorage')}>{container.resources.limits['ephemeral-storage']}</Descriptions.Item>
                )}
              </Descriptions>
            </Card>
          </Col>
        </Row>
      </Card>

      {container.ports && container.ports.length > 0 && (
        <Card title={t('container.ports.title')} size="small" style={{ marginBottom: 16 }}>
          <Descriptions column={1} bordered size="small">
            {container.ports.map((port, index) => (
              <Descriptions.Item key={index} label={port.name || `${t('container.ports.port')}${index + 1}`}>
                <Tag color="blue">{port.containerPort}</Tag>
                <Tag>{port.protocol || 'TCP'}</Tag>
              </Descriptions.Item>
            ))}
          </Descriptions>
        </Card>
      )}

      {container.securityContext && (
        <Card title={t('container.security.title')} size="small">
          <Descriptions column={1} bordered size="small">
            {container.securityContext.privileged !== undefined && (
              <Descriptions.Item label={t('container.security.privileged')}>
                <Tag color={container.securityContext.privileged ? 'red' : 'green'}>
                  {container.securityContext.privileged ? t('container.basic.on') : t('container.basic.off')}
                </Tag>
              </Descriptions.Item>
            )}
            {container.securityContext.runAsUser !== undefined && (
              <Descriptions.Item label={t('container.security.runAsUser')}>{container.securityContext.runAsUser}</Descriptions.Item>
            )}
            {container.securityContext.runAsGroup !== undefined && (
              <Descriptions.Item label={t('container.security.runAsGroup')}>{container.securityContext.runAsGroup}</Descriptions.Item>
            )}
            {container.securityContext.runAsNonRoot !== undefined && (
              <Descriptions.Item label={t('container.security.runAsNonRoot')}>
                <Tag color={container.securityContext.runAsNonRoot ? 'green' : 'orange'}>
                  {container.securityContext.runAsNonRoot ? t('container.security.yes') : t('container.security.no')}
                </Tag>
              </Descriptions.Item>
            )}
            {container.securityContext.readOnlyRootFilesystem !== undefined && (
              <Descriptions.Item label={t('container.security.readOnlyRootFs')}>
                <Tag color={container.securityContext.readOnlyRootFilesystem ? 'green' : 'orange'}>
                  {container.securityContext.readOnlyRootFilesystem ? t('container.security.yes') : t('container.security.no')}
                </Tag>
              </Descriptions.Item>
            )}
            {container.securityContext.allowPrivilegeEscalation !== undefined && (
              <Descriptions.Item label={t('container.security.allowPrivilegeEscalation')}>
                <Tag color={container.securityContext.allowPrivilegeEscalation ? 'red' : 'green'}>
                  {container.securityContext.allowPrivilegeEscalation ? t('container.security.yes') : t('container.security.no')}
                </Tag>
              </Descriptions.Item>
            )}
            {container.securityContext.capabilities?.add && container.securityContext.capabilities.add.length > 0 && (
              <Descriptions.Item label={t('container.security.addCapabilities')}>
                {container.securityContext.capabilities.add.map((cap, idx) => (
                  <Tag key={idx} color="orange">{cap}</Tag>
                ))}
              </Descriptions.Item>
            )}
            {container.securityContext.capabilities?.drop && container.securityContext.capabilities.drop.length > 0 && (
              <Descriptions.Item label={t('container.security.dropCapabilities')}>
                {container.securityContext.capabilities.drop.map((cap, idx) => (
                  <Tag key={idx} color="green">{cap}</Tag>
                ))}
              </Descriptions.Item>
            )}
          </Descriptions>
        </Card>
      )}
    </div>
  );
};

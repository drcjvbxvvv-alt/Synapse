import React from 'react';
import {
  Drawer,
  Button,
  Space,
  Tabs,
  Descriptions,
  Tag,
  Badge,
  Table,
  Timeline,
} from 'antd';
import {
  SyncOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  ClockCircleOutlined,
  QuestionCircleOutlined,
  RollbackOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type {
  ArgoCDApplication,
  ArgoCDResource,
} from '../../services/argoCDService';

export interface ArgoCDAppDetailDrawerProps {
  open: boolean;
  selectedApp: ArgoCDApplication | null;
  onClose: () => void;
  onSync: (appName: string) => void;
  onRollback: (appName: string, revisionId: number) => void;
}

const ArgoCDAppDetailDrawer: React.FC<ArgoCDAppDetailDrawerProps> = ({
  open,
  selectedApp,
  onClose,
  onSync,
  onRollback,
}) => {
  const { t } = useTranslation(['plugins', 'common']);

  const getSyncStatusTag = (status: string) => {
    const config: Record<string, { color: string; icon: React.ReactNode }> = {
      'Synced': { color: 'success', icon: <CheckCircleOutlined /> },
      'OutOfSync': { color: 'warning', icon: <ExclamationCircleOutlined /> },
      'Unknown': { color: 'default', icon: <QuestionCircleOutlined /> },
    };
    const cfg = config[status] || config['Unknown'];
    return <Tag color={cfg.color} icon={cfg.icon}>{status || 'Unknown'}</Tag>;
  };

  const getHealthStatusBadge = (status: string) => {
    const config: Record<string, { status: 'success' | 'error' | 'processing' | 'warning' | 'default'; icon?: React.ReactNode }> = {
      'Healthy': { status: 'success', icon: <CheckCircleOutlined /> },
      'Degraded': { status: 'error', icon: <CloseCircleOutlined /> },
      'Progressing': { status: 'processing', icon: <LoadingOutlined /> },
      'Suspended': { status: 'warning', icon: <ClockCircleOutlined /> },
      'Missing': { status: 'default' },
      'Unknown': { status: 'default' },
    };
    const cfg = config[status] || config['Unknown'];
    return <Badge status={cfg.status} text={status || 'Unknown'} />;
  };

  return (
    <Drawer
      title={`${t('plugins:argocd.appDetail')}: ${selectedApp?.name || ''}`}
      open={open}
      onClose={onClose}
      width={700}
      extra={
        <Space>
          <Button
            type="primary"
            icon={<SyncOutlined />}
            onClick={() => selectedApp && onSync(selectedApp.name)}
          >
            {t('plugins:argocd.sync')}
          </Button>
        </Space>
      }
    >
      {selectedApp && (
        <Tabs
          items={[
            {
              key: 'overview',
              label: t('plugins:argocd.overview'),
              children: (
                <div>
                  <Descriptions column={2} bordered size="small">
                    <Descriptions.Item label={t('plugins:argocd.appName')}>{selectedApp.name}</Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.project')}>{selectedApp.project}</Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.syncStatus')}>{getSyncStatusTag(selectedApp.sync_status)}</Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.healthStatus')}>{getHealthStatusBadge(selectedApp.health_status)}</Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.gitRepo')} span={2}>
                      <a href={selectedApp.source?.repo_url} target="_blank" rel="noopener noreferrer">
                        {selectedApp.source?.repo_url}
                      </a>
                    </Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.gitPath')}>{selectedApp.source?.path}</Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.targetRevision')}>{selectedApp.target_revision}</Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.currentVersion')}>
                      <code>{selectedApp.synced_revision?.substring(0, 12)}</code>
                    </Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.destNamespace')}>
                      <Tag color="blue">{selectedApp.destination?.namespace}</Tag>
                    </Descriptions.Item>
                    <Descriptions.Item label={t('common:table.createdAt')}>{selectedApp.created_at}</Descriptions.Item>
                    <Descriptions.Item label={t('plugins:argocd.lastSync')}>{selectedApp.reconciled_at}</Descriptions.Item>
                  </Descriptions>
                </div>
              ),
            },
            {
              key: 'resources',
              label: t('plugins:argocd.resourceList'),
              children: (
                <Table
                  scroll={{ x: 'max-content' }}
                  size="small"
                  dataSource={selectedApp.resources || []}
                  rowKey={(record: ArgoCDResource) => `${record.kind}-${record.namespace}-${record.name}`}
                  columns={[
                    { title: 'Kind', dataIndex: 'kind', key: 'kind', width: 120 },
                    { title: t('common:table.namespace'), dataIndex: 'namespace', key: 'namespace', width: 120 },
                    { title: t('common:table.name'), dataIndex: 'name', key: 'name' },
                    {
                      title: t('plugins:argocd.healthStatus'),
                      dataIndex: 'health',
                      key: 'health',
                      width: 100,
                      render: (text: string) => getHealthStatusBadge(text)
                    },
                  ]}
                  pagination={false}
                />
              ),
            },
            {
              key: 'history',
              label: t('plugins:argocd.syncHistory'),
              children: (
                <Timeline
                  items={(selectedApp.history || []).slice(0, 10).map((h) => ({
                    color: 'green',
                    children: (
                      <div>
                        <div>
                          <strong>{t('plugins:argocd.version')}:</strong> <code>{h.revision?.substring(0, 12)}</code>
                          <Button
                            type="link"
                            size="small"
                            icon={<RollbackOutlined />}
                            onClick={() => onRollback(selectedApp.name, h.id)}
                          >
                            {t('plugins:argocd.rollbackToVersion')}
                          </Button>
                        </div>
                        <div style={{ color: '#999', fontSize: 12 }}>
                          {t('plugins:argocd.deployTime')}: {h.deployed_at}
                        </div>
                      </div>
                    ),
                  }))}
                />
              ),
            },
          ]}
        />
      )}
    </Drawer>
  );
};

export default ArgoCDAppDetailDrawer;

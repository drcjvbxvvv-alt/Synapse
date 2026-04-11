import React from 'react';
import {
  Modal,
  Table,
  Button,
  Tag,
  Row,
  Col,
  Card,
  Typography,
} from 'antd';
import {
  SyncOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { Cluster } from '../../../types';
import type { SyncStatusResult } from '../../../services/rbacService';

const { Title, Paragraph } = Typography;

interface SyncModalProps {
  visible: boolean;
  clusters: Cluster[];
  syncStatus: Record<string, SyncStatusResult>;
  syncLoading: boolean;
  selectedClusterForSync: string | null;
  onCancel: () => void;
  onSync: (clusterId: string) => void;
}

export const SyncModal: React.FC<SyncModalProps> = ({
  visible,
  clusters,
  syncStatus,
  syncLoading,
  selectedClusterForSync,
  onCancel,
  onSync,
}) => {
  const { t } = useTranslation(['permission', 'common']);

  const columns = [
    {
      title: t('permission:sync.clusterName'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'API Server',
      dataIndex: 'api_server',
      key: 'api_server',
      ellipsis: true,
    },
    {
      title: t('permission:sync.syncStatus'),
      key: 'status',
      width: 120,
      render: (_: unknown, record: Cluster) => {
        const status = syncStatus[record.id];
        if (!status) {
          return <Tag>{t('permission:sync.notChecked')}</Tag>;
        }
        return status.synced ? (
          <Tag icon={<CheckCircleOutlined />} color="success">{t('permission:sync.synced')}</Tag>
        ) : (
          <Tag icon={<CloseCircleOutlined />} color="warning">{t('permission:sync.notSynced')}</Tag>
        );
      },
    },
    {
      title: t('common:table.actions'),
      key: 'action',
      width: 120,
      render: (_: unknown, record: Cluster) => (
        <Button
          type="primary"
          size="small"
          icon={<SyncOutlined spin={syncLoading && selectedClusterForSync === record.id} />}
          loading={syncLoading && selectedClusterForSync === record.id}
          onClick={() => onSync(record.id)}
        >
          {syncStatus[record.id]?.synced ? t('permission:sync.resyncBtn') : t('permission:sync.syncBtn')}
        </Button>
      ),
    },
  ];

  return (
    <Modal
      title={t('permission:sync.title')}
      open={visible}
      onCancel={onCancel}
      footer={null}
      width={800}
    >
      <div style={{ marginBottom: 16 }}>
        <Paragraph type="secondary">
          {t('permission:sync.description')}
        </Paragraph>
      </div>
      <Table
        scroll={{ x: 'max-content' }}
        rowKey="id"
        dataSource={clusters}
        pagination={false}
        columns={columns}
      />
      <div style={{ marginTop: 16 }}>
        <Title level={5}>{t('permission:sync.resourcesTitle')}</Title>
        <Row gutter={16}>
          <Col span={8}>
            <Card size="small" title="ClusterRole" styles={{ body: { padding: 12 } }}>
              <ul style={{ margin: 0, paddingLeft: 20, fontSize: 12 }}>
                <li>synapse-cluster-admin</li>
                <li>synapse-ops</li>
                <li>synapse-dev</li>
                <li>synapse-readonly</li>
              </ul>
            </Card>
          </Col>
          <Col span={8}>
            <Card size="small" title="ServiceAccount" styles={{ body: { padding: 12 } }}>
              <ul style={{ margin: 0, paddingLeft: 20, fontSize: 12 }}>
                <li>synapse-admin-sa</li>
                <li>synapse-ops-sa</li>
                <li>synapse-dev-sa</li>
                <li>synapse-readonly-sa</li>
              </ul>
            </Card>
          </Col>
          <Col span={8}>
            <Card size="small" title="ClusterRoleBinding" styles={{ body: { padding: 12 } }}>
              <ul style={{ margin: 0, paddingLeft: 20, fontSize: 12 }}>
                <li>synapse-admin-binding</li>
                <li>synapse-ops-binding</li>
              </ul>
            </Card>
          </Col>
        </Row>
      </div>
    </Modal>
  );
};

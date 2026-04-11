import EmptyState from '@/components/EmptyState';
import React, { useState } from 'react';
import {
  Drawer,
  Descriptions,
  Table,
  Tag,
  Space,
  Button,
  Modal,
  Typography,
  Empty,
  Divider,
  App,
} from 'antd';
import { CheckCircleOutlined, QuestionCircleOutlined, CodeOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import type { GatewayItem, GatewayK8sCondition, GatewayListener } from './gatewayTypes';

interface GatewayDrawerProps {
  open: boolean;
  clusterId: string;
  item: GatewayItem | null;
  onClose: () => void;
}

const conditionTag = (cond: GatewayK8sCondition) => {
  const color = cond.status === 'True' ? 'success' : cond.status === 'False' ? 'error' : 'default';
  const icon = cond.status === 'True' ? <CheckCircleOutlined /> : <QuestionCircleOutlined />;
  return (
    <Tag key={cond.type} icon={icon} color={color}>
      {cond.type}
    </Tag>
  );
};

const listenerStatusTag = (status: string) => {
  if (status === 'Ready') return <Tag color="success">{status}</Tag>;
  if (status === 'Unknown') return <Tag color="default">{status}</Tag>;
  return <Tag color="warning">{status}</Tag>;
};

const GatewayDrawer: React.FC<GatewayDrawerProps> = ({ open, clusterId, item, onClose }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const [yamlVisible, setYamlVisible] = useState(false);
  const [yaml, setYaml] = useState('');
  const [yamlLoading, setYamlLoading] = useState(false);

  const handleViewYAML = async () => {
    if (!item) return;
    setYamlLoading(true);
    try {
      const res = await gatewayService.getGatewayYAML(clusterId, item.namespace, item.name);
      setYaml(res.yaml);
      setYamlVisible(true);
    } catch {
      message.error(t('network:gatewayapi.messages.fetchYAMLError'));
    } finally {
      setYamlLoading(false);
    }
  };

  if (!item) return null;

  const listenerColumns = [
    { title: t('network:gatewayapi.columns.name'), dataIndex: 'name', key: 'name' },
    {
      title: t('network:gatewayapi.drawer.port'),
      dataIndex: 'port',
      key: 'port',
      render: (v: number) => <Tag>{v}</Tag>,
    },
    {
      title: t('network:gatewayapi.drawer.protocol'),
      dataIndex: 'protocol',
      key: 'protocol',
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: t('network:gatewayapi.drawer.tlsMode'),
      dataIndex: 'tlsMode',
      key: 'tlsMode',
      render: (v: string) => v || '-',
    },
    {
      title: t('network:gatewayapi.columns.status'),
      dataIndex: 'status',
      key: 'status',
      render: (v: string) => listenerStatusTag(v),
    },
  ];

  return (
    <>
      <Drawer
        title={
          <Space>
            <span>{item.name}</span>
            <Tag color="default">{item.namespace}</Tag>
          </Space>
        }
        open={open}
        onClose={onClose}
        width={680}
        extra={
          <Button
            icon={<CodeOutlined />}
            onClick={handleViewYAML}
            loading={yamlLoading}
          >
            {t('network:gatewayapi.drawer.viewYAML')}
          </Button>
        }
      >
        {/* 基本資訊 */}
        <Descriptions column={2} size="small" bordered>
          <Descriptions.Item label="GatewayClass" span={2}>
            <Tag color="purple">{item.gatewayClass}</Tag>
          </Descriptions.Item>
          {item.addresses.length > 0 && (
            <Descriptions.Item label={t('network:gatewayapi.drawer.addresses')} span={2}>
              <Space wrap>
                {item.addresses.map((a, i) => (
                  <Tag key={i} color="cyan">
                    {a.value}
                    <Typography.Text type="secondary" style={{ marginLeft: 4, fontSize: 11 }}>
                      ({a.type})
                    </Typography.Text>
                  </Tag>
                ))}
              </Space>
            </Descriptions.Item>
          )}
        </Descriptions>

        {/* Conditions */}
        <Divider orientation="left" plain>{t('network:gatewayapi.drawer.conditions')}</Divider>
        {item.conditions.length > 0 ? (
          <Space wrap>
            {item.conditions.map((c) => conditionTag(c))}
          </Space>
        ) : (
          <Typography.Text type="secondary">{t('network:gatewayapi.drawer.noConditions')}</Typography.Text>
        )}

        {/* Listeners */}
        <Divider orientation="left" plain>{t('network:gatewayapi.drawer.listeners')}</Divider>
        {item.listeners.length > 0 ? (
          <Table
            rowKey="name"
            dataSource={item.listeners as GatewayListener[]}
            columns={listenerColumns}
            pagination={false}
            size="small"
          />
        ) : (
          <EmptyState description={t('network:gatewayapi.drawer.noListeners')} />
        )}
      </Drawer>

      {/* YAML Modal */}
      <Modal
        title={`YAML — ${item.name}`}
        open={yamlVisible}
        onCancel={() => setYamlVisible(false)}
        footer={null}
        width={800}
      >
        <pre
          style={{
            background: '#1e1e1e',
            color: '#d4d4d4',
            padding: 16,
            borderRadius: 6,
            overflow: 'auto',
            maxHeight: 500,
            fontSize: 13,
          }}
        >
          {yaml}
        </pre>
      </Modal>
    </>
  );
};

export default GatewayDrawer;

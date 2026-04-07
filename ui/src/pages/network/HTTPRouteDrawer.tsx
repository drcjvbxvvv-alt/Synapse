import React, { useState } from 'react';
import {
  Drawer,
  Table,
  Tag,
  Space,
  Button,
  Modal,
  Typography,
  Empty,
  Divider,
  Collapse,
  App,
} from 'antd';
import { CheckCircleOutlined, QuestionCircleOutlined, CodeOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { gatewayService } from '../../services/gatewayService';
import type {
  HTTPRouteItem,
  HTTPRouteBackend,
  HTTPRouteParentRef,
  GatewayK8sCondition,
} from './gatewayTypes';

interface HTTPRouteDrawerProps {
  open: boolean;
  clusterId: string;
  item: HTTPRouteItem | null;
  onClose: () => void;
}

const conditionTag = (cond: GatewayK8sCondition) => {
  const color = cond.status === 'True' ? 'success' : cond.status === 'False' ? 'error' : 'default';
  const icon = cond.status === 'True' ? <CheckCircleOutlined /> : <QuestionCircleOutlined />;
  return (
    <Tag key={cond.type} icon={icon} color={color} title={cond.message}>
      {cond.type}
    </Tag>
  );
};

const HTTPRouteDrawer: React.FC<HTTPRouteDrawerProps> = ({ open, clusterId, item, onClose }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const [yamlVisible, setYamlVisible] = useState(false);
  const [yaml, setYaml] = useState('');
  const [yamlLoading, setYamlLoading] = useState(false);

  const handleViewYAML = async () => {
    if (!item) return;
    setYamlLoading(true);
    try {
      const res = await gatewayService.getHTTPRouteYAML(clusterId, item.namespace, item.name);
      setYaml(res.yaml);
      setYamlVisible(true);
    } catch {
      message.error(t('network:gatewayapi.messages.fetchYAMLError'));
    } finally {
      setYamlLoading(false);
    }
  };

  if (!item) return null;

  const backendColumns = [
    { title: t('network:gatewayapi.columns.name'), dataIndex: 'name', key: 'name' },
    {
      title: t('network:gatewayapi.columns.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: t('network:gatewayapi.drawer.port'),
      dataIndex: 'port',
      key: 'port',
      render: (v: number) => <Tag>{v}</Tag>,
    },
    {
      title: t('network:gatewayapi.drawer.weight'),
      dataIndex: 'weight',
      key: 'weight',
    },
  ];

  const parentRefColumns = [
    {
      title: t('network:gatewayapi.columns.name'),
      key: 'name',
      render: (_: unknown, r: HTTPRouteParentRef) => `${r.gatewayNamespace}/${r.gatewayName}`,
    },
    {
      title: 'Listener',
      dataIndex: 'sectionName',
      key: 'sectionName',
      render: (v: string) => v || '-',
    },
    {
      title: t('network:gatewayapi.columns.status'),
      key: 'conditions',
      render: (_: unknown, r: HTTPRouteParentRef) => (
        <Space wrap>{r.conditions?.map((c) => conditionTag(c))}</Space>
      ),
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
        width={720}
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
        {/* 主機名稱 */}
        <Divider orientation="left" plain>{t('network:gatewayapi.drawer.hostnames')}</Divider>
        {item.hostnames.length > 0 ? (
          <Space wrap>
            {item.hostnames.map((h) => <Tag key={h} color="blue">{h}</Tag>)}
          </Space>
        ) : (
          <Typography.Text type="secondary">{t('network:gatewayapi.drawer.noHostnames')}</Typography.Text>
        )}

        {/* 父 Gateway */}
        <Divider orientation="left" plain>{t('network:gatewayapi.drawer.parentGateway')}</Divider>
        {item.parentRefs.length > 0 ? (
          <Table
            rowKey={(r) => `${r.gatewayNamespace}/${r.gatewayName}`}
            dataSource={item.parentRefs}
            columns={parentRefColumns}
            pagination={false}
            size="small"
          />
        ) : (
          <Empty description={t('network:gatewayapi.drawer.noParentRefs')} image={Empty.PRESENTED_IMAGE_SIMPLE} />
        )}

        {/* 路由規則 */}
        <Divider orientation="left" plain>{t('network:gatewayapi.drawer.rules')}</Divider>
        {item.rules.length > 0 ? (
          <Collapse
            size="small"
            items={item.rules.map((rule, i) => ({
              key: String(i),
              label: `Rule ${i + 1} — ${rule.backends.length} ${t('network:gatewayapi.drawer.backends')}`,
              children: (
                <>
                  {/* Matches */}
                  {rule.matches.length > 0 && (
                    <>
                      <Typography.Text strong style={{ display: 'block', marginBottom: 4 }}>
                        {t('network:gatewayapi.drawer.matches')}
                      </Typography.Text>
                      <pre style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, fontSize: 12 }}>
                        {JSON.stringify(rule.matches, null, 2)}
                      </pre>
                    </>
                  )}

                  {/* Backends */}
                  <Typography.Text strong style={{ display: 'block', marginBottom: 4, marginTop: 8 }}>
                    {t('network:gatewayapi.drawer.backends')}
                  </Typography.Text>
                  {rule.backends.length > 0 ? (
                    <Table
                      rowKey={(r: HTTPRouteBackend) => `${r.namespace}/${r.name}:${r.port}`}
                      dataSource={rule.backends}
                      columns={backendColumns}
                      pagination={false}
                      size="small"
                    />
                  ) : (
                    <Typography.Text type="secondary">{t('network:gatewayapi.drawer.noBackends')}</Typography.Text>
                  )}

                  {/* Filters */}
                  {rule.filters.length > 0 && (
                    <>
                      <Typography.Text strong style={{ display: 'block', marginBottom: 4, marginTop: 8 }}>
                        {t('network:gatewayapi.drawer.filters')}
                      </Typography.Text>
                      <pre style={{ background: '#f5f5f5', padding: 8, borderRadius: 4, fontSize: 12 }}>
                        {JSON.stringify(rule.filters, null, 2)}
                      </pre>
                    </>
                  )}
                </>
              ),
            }))}
          />
        ) : (
          <Empty description={t('network:gatewayapi.drawer.noRules')} image={Empty.PRESENTED_IMAGE_SIMPLE} />
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

export default HTTPRouteDrawer;

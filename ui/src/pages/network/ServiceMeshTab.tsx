import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Space,
  Spin,
  Tabs,
  Table,
  Tag,
  Popconfirm,
  App,
  Typography,
} from 'antd';
import EmptyState from '@/components/EmptyState';
import {
  PlusOutlined,
  ReloadOutlined,
  CopyOutlined,
  LinkOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import MonacoEditor from '@monaco-editor/react';
import { Modal } from 'antd';
import { MeshService, type MeshStatus, type VirtualServiceSummary } from '../../services/meshService';
import { usePermission } from '../../hooks/usePermission';
import ServiceTopologyGraph from './ServiceTopologyGraph';
import VirtualServiceForm from './VirtualServiceForm';
import DestinationRuleList from './DestinationRuleList';

const { Text } = Typography;

interface ServiceMeshTabProps {
  clusterId: string;
  namespaces: string[];
}

// ---- VirtualService list sub-tab ----
const VirtualServiceList: React.FC<{ clusterId: string; namespaces: string[] }> = ({
  clusterId,
  namespaces,
}) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['network', 'common']);
  const { canWrite } = usePermission();
  const [items, setItems] = useState<VirtualServiceSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespace] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [yamlOpen, setYamlOpen] = useState(false);
  const [yamlContent, setYamlContent] = useState('');

  const fetchList = useCallback(async () => {
    setLoading(true);
    try {
      const res = await MeshService.listVirtualServices(clusterId, namespace || undefined);
      const data = (res as unknown as { data: { items: VirtualServiceSummary[] } }).data ?? res;
      setItems((data as { items: VirtualServiceSummary[] }).items ?? []);
    } catch {
      message.error(t('network:servicemesh.fetchVSError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, message, t]);

  useEffect(() => {
    fetchList();
  }, [fetchList]);

  const handleViewYAML = async (record: VirtualServiceSummary) => {
    try {
      const res = await MeshService.getVirtualService(clusterId, record.namespace, record.name);
      const obj = (res as unknown as { data: Record<string, unknown> }).data ?? res;
      setYamlContent(JSON.stringify(obj, null, 2));
      setYamlOpen(true);
    } catch {
      message.error(t('network:servicemesh.fetchVSDetailError'));
    }
  };

  const handleDelete = async (record: VirtualServiceSummary) => {
    try {
      await MeshService.deleteVirtualService(clusterId, record.namespace, record.name);
      message.success(t('network:servicemesh.deleteVSSuccess'));
      fetchList();
    } catch {
      message.error(t('common:messages.deleteError'));
    }
  };

  const getHosts = (record: VirtualServiceSummary): string[] => {
    const spec = record.spec as { hosts?: string[] } | undefined;
    return spec?.hosts ?? [];
  };

  const columns = [
    {
      title: t('network:gatewayapi.columns.name'),
      dataIndex: 'name',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t('network:gatewayapi.columns.namespace'),
      dataIndex: 'namespace',
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: 'Hosts',
      key: 'hosts',
      render: (_: unknown, record: VirtualServiceSummary) =>
        getHosts(record).map(h => <Tag key={h}>{h}</Tag>),
    },
    {
      title: t('network:gatewayapi.columns.createdAt'),
      dataIndex: 'createdAt',
      render: (v: string) => (v ? new Date(v).toLocaleString() : '—'),
    },
    ...(canWrite() ? [{
      title: t('network:gatewayapi.columns.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 140,
      render: (_: unknown, record: VirtualServiceSummary) => (
        <Space size="small">
          <Button type="link" size="small" onClick={() => handleViewYAML(record)}>
            YAML
          </Button>
          <Popconfirm
            title={t('common:messages.confirmDelete')}
            description={t('network:servicemesh.confirmDeleteVS', { name: record.name })}
            onConfirm={() => handleDelete(record)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            <Button type="link" size="small" danger>
              {t('common:actions.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    }] : []),
  ];

  return (
    <div>
      <div style={{ marginBottom: 12, display: 'flex', gap: 8, alignItems: 'center' }}>
        <Button icon={<ReloadOutlined />} onClick={fetchList} loading={loading} />
        <div style={{ flex: 1 }} />
        {canWrite() && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            {t('network:servicemesh.createVirtualService')}
          </Button>
        )}
      </div>

      <Table
        rowKey={r => `${r.namespace}/${r.name}`}
        dataSource={items}
        columns={columns}
        loading={loading}
        size="middle"
        pagination={{ pageSize: 20 }}
        scroll={{ x: 700 }}
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
      />

      {/* YAML Viewer */}
      <Modal
        title="VirtualService YAML"
        open={yamlOpen}
        onCancel={() => setYamlOpen(false)}
        footer={null}
        width={700}
      >
        <MonacoEditor
          height={400}
          language="json"
          value={yamlContent}
          options={{ readOnly: true, minimap: { enabled: false }, fontSize: 12 }}
        />
      </Modal>

      {/* Create Form */}
      <VirtualServiceForm
        clusterId={clusterId}
        namespaces={namespaces}
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => { setCreateOpen(false); fetchList(); }}
      />
    </div>
  );
};

// ---- Main ServiceMeshTab ----
const ServiceMeshTab: React.FC<ServiceMeshTabProps> = ({ clusterId, namespaces }) => {
  const { message } = App.useApp();
  const [status, setStatus] = useState<MeshStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(true);
  const [statusError, setStatusError] = useState<string | null>(null);

  const loadStatus = useCallback(async () => {
    setStatusLoading(true);
    setStatusError(null);
    try {
      const res = await MeshService.getStatus(clusterId);
      const s = (res as unknown as { data: MeshStatus }).data ?? res;
      setStatus(s as MeshStatus);
    } catch {
      setStatusError('取得 Istio 狀態失敗');
    } finally {
      setStatusLoading(false);
    }
  }, [clusterId]);

  useEffect(() => {
    loadStatus();
  }, [loadStatus]);

  if (statusLoading) {
    return (
      <div style={{ padding: 40, textAlign: 'center' }}>
        <Spin tip="檢查 Istio 狀態..." />
      </div>
    );
  }

  if (statusError) {
    return (
      <Alert type="error" message={statusError} action={
        <Button size="small" onClick={loadStatus}>重試</Button>
      } />
    );
  }

  if (!status?.installed) {
    const installCmd = 'istioctl install --set profile=default';
    const handleCopyCmd = () => {
      navigator.clipboard.writeText(installCmd).then(() => {
        message.success('已複製');
      });
    };
    return (
      <div style={{ maxWidth: 640, margin: '48px auto', padding: '0 24px' }}>
        <Alert
          type="warning"
          showIcon
          message="此叢集尚未安裝 Service Mesh（Istio）"
          description={
            <Space direction="vertical" style={{ width: '100%', marginTop: 8 }}>
              <Text>{status?.reason ?? 'Istio Service Mesh 提供流量管理、可觀測性與安全策略（mTLS），是雲原生微服務架構的核心基礎設施。'}</Text>
              <Text strong style={{ display: 'block', marginTop: 8 }}>安裝指令：</Text>
              <pre
                style={{
                  background: '#1e1e1e',
                  color: '#d4d4d4',
                  padding: '12px 16px',
                  borderRadius: 6,
                  fontSize: 13,
                  margin: 0,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {installCmd}
              </pre>
              <Space style={{ marginTop: 8 }}>
                <Button icon={<CopyOutlined />} onClick={handleCopyCmd}>
                  複製安裝指令
                </Button>
                <Button
                  icon={<LinkOutlined />}
                  href="https://istio.io/latest/docs/setup/getting-started/"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  查看官方文件
                </Button>
                <Button icon={<ReloadOutlined />} onClick={loadStatus}>
                  重新偵測
                </Button>
              </Space>
            </Space>
          }
        />
      </div>
    );
  }

  const subTabs = [
    {
      key: 'topology',
      label: '服務拓撲',
      children: <ServiceTopologyGraph clusterId={clusterId} namespaces={namespaces} />,
    },
    {
      key: 'virtualservices',
      label: 'VirtualService',
      children: <VirtualServiceList clusterId={clusterId} namespaces={namespaces} />,
    },
    {
      key: 'destinationrules',
      label: 'DestinationRule',
      children: (
        <DestinationRuleList clusterId={clusterId} namespaces={namespaces} />
      ),
    },
  ];

  return (
    <div>
      {status.version && status.version !== 'unknown' && (
        <Alert
          type="success"
          message={`Istio ${status.version} 已安裝並執行中`}
          style={{ marginBottom: 12 }}
          showIcon
        />
      )}
      <Tabs items={subTabs} />
    </div>
  );
};

export default ServiceMeshTab;

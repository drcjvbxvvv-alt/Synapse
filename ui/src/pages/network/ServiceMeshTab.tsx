import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Space,
  Spin,
  Tabs,
  Table,
  Tag,
  Tooltip,
  App,
  Typography,
  Empty,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import { Modal } from 'antd';
import { MeshService, type MeshStatus, type VirtualServiceSummary } from '../../services/meshService';
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
  const { message, modal } = App.useApp();
  const [items, setItems] = useState<VirtualServiceSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespace, setNamespace] = useState('');
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
      message.error('取得 VirtualService 列表失敗');
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, message]);

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
      message.error('取得 VirtualService 詳情失敗');
    }
  };

  const handleDelete = (record: VirtualServiceSummary) => {
    modal.confirm({
      title: '確認刪除',
      content: `確定要刪除 VirtualService "${record.name}" 嗎？`,
      okType: 'danger',
      onOk: async () => {
        try {
          await MeshService.deleteVirtualService(clusterId, record.namespace, record.name);
          message.success('VirtualService 刪除成功');
          fetchList();
        } catch {
          message.error('刪除失敗');
        }
      },
    });
  };

  const getHosts = (record: VirtualServiceSummary): string[] => {
    const spec = record.spec as { hosts?: string[] } | undefined;
    return spec?.hosts ?? [];
  };

  const columns = [
    {
      title: '名稱',
      dataIndex: 'name',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: '命名空間',
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
      title: '建立時間',
      dataIndex: 'createdAt',
      render: (v: string) => (v ? new Date(v).toLocaleString() : '—'),
    },
    {
      title: '操作',
      key: 'actions',
      fixed: 'right' as const,
      width: 120,
      render: (_: unknown, record: VirtualServiceSummary) => (
        <Space>
          <Tooltip title="檢視 YAML">
            <Button size="small" icon={<EyeOutlined />} onClick={() => handleViewYAML(record)} />
          </Tooltip>
          <Tooltip title="刪除">
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record)} />
          </Tooltip>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Space style={{ marginBottom: 12 }} wrap>
        <Button icon={<ReloadOutlined />} onClick={fetchList}>重新整理</Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          建立 VirtualService
        </Button>
      </Space>

      <Table
        rowKey={r => `${r.namespace}/${r.name}`}
        dataSource={items}
        columns={columns}
        loading={loading}
        size="middle"
        pagination={{ pageSize: 20 }}
        scroll={{ x: 700 }}
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
    return (
      <Card>
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
            <div>
              <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 8 }}>
                Istio 未安裝
              </div>
              <div style={{ color: '#666', marginBottom: 12 }}>
                {status?.reason ?? '叢集中未偵測到 Istio Service Mesh'}
              </div>
              <div style={{ color: '#999', fontSize: 12 }}>
                安裝提示：<code>istioctl install --set profile=default</code>
              </div>
            </div>
          }
        />
      </Card>
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

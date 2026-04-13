import React, { useState, useCallback, useEffect } from 'react';
import {
  Table,
  Button,
  Space,
  Tag,
  Input,
  Select,
  Drawer,
  Typography,
  App,
  Tooltip,
  Segmented,
} from 'antd';
import EmptyState from '@/components/EmptyState';
import {
  ReloadOutlined,
  SearchOutlined,
  PlusOutlined,
  UnorderedListOutlined,
  ApartmentOutlined,
  ToolOutlined,
  SafetyCertificateOutlined,
  EditOutlined,
  PlayCircleOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { ActionButtons } from '../../components/ActionButtons';
import { useTranslation } from 'react-i18next';
import { usePermission } from '../../hooks/usePermission';
import type { TablePaginationConfig } from 'antd/es/table';
import MonacoEditor from '@monaco-editor/react';
import { NetworkPolicyService, type NetworkPolicyInfo } from '../../services/networkPolicyService';
import NetworkPolicyTopology from './NetworkPolicyTopology';
import NetworkPolicyWizard from './NetworkPolicyWizard';
import NetworkPolicySimulator from './NetworkPolicySimulator';
import NetworkPolicyForm from './NetworkPolicyForm';
import { namespaceService } from '../../services/namespaceService';

const { Text } = Typography;
const { Option } = Select;

interface NetworkPolicyTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}


const NetworkPolicyTab: React.FC<NetworkPolicyTabProps> = ({ clusterId, onCountChange }) => {
  const { t } = useTranslation(['network', 'common']);
  const { message } = App.useApp();
  const { canWrite, canDelete } = usePermission();

  const [viewMode, setViewMode] = useState<'list' | 'topology' | 'simulate'>('list');
  const [wizardOpen, setWizardOpen] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [policies, setPolicies] = useState<NetworkPolicyInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [namespace, setNamespace] = useState('');
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');

  // YAML drawer state (view / edit)
  const [yamlDrawerOpen, setYamlDrawerOpen] = useState(false);
  const [yamlContent, setYamlContent] = useState('');
  const [yamlMode, setYamlMode] = useState<'view' | 'edit'>('view');
  const [selectedPolicy, setSelectedPolicy] = useState<NetworkPolicyInfo | null>(null);
  const [yamlSaving, setYamlSaving] = useState(false);

  const fetchPolicies = useCallback(async (page = currentPage, ps = pageSize) => {
    setLoading(true);
    try {
      const res = await NetworkPolicyService.list(clusterId, namespace || undefined, search || undefined, page, ps);
      setPolicies(res.items ?? []);
      setTotal(res.total ?? 0);
      onCountChange?.(res.total ?? 0);
    } catch {
      message.error(t('network:networkpolicy.messages.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, search, currentPage, pageSize, message, t, onCountChange]);

  useEffect(() => {
    fetchPolicies(1, pageSize);
    setCurrentPage(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clusterId, namespace, search]);

  useEffect(() => {
    namespaceService.getNamespaces(clusterId)
      .then(res => setNamespaces(res.map(n => n.name)))
      .catch(() => {});
  }, [clusterId]);

  const handleTableChange = (pagination: TablePaginationConfig) => {
    const p = pagination.current ?? 1;
    const ps = pagination.pageSize ?? pageSize;
    setCurrentPage(p);
    setPageSize(ps);
    fetchPolicies(p, ps);
  };

  const handleSearch = () => {
    setSearch(searchInput);
  };

  const _handleViewYAML = async (policy: NetworkPolicyInfo) => {
    try {
      const res = await NetworkPolicyService.getYAML(clusterId, policy.namespace, policy.name);
      setYamlContent(res.yaml);
      setSelectedPolicy(policy);
      setYamlMode('view');
      setYamlDrawerOpen(true);
    } catch {
      message.error(t('network:networkpolicy.messages.fetchYAMLError'));
    }
  };

  const handleEditYAML = async (policy: NetworkPolicyInfo) => {
    try {
      const res = await NetworkPolicyService.getYAML(clusterId, policy.namespace, policy.name);
      setYamlContent(res.yaml);
      setSelectedPolicy(policy);
      setYamlMode('edit');
      setYamlDrawerOpen(true);
    } catch {
      message.error(t('network:networkpolicy.messages.fetchYAMLError'));
    }
  };

  const handleCreateClick = () => {
    setCreateModalOpen(true);
  };

  const handleYAMLSave = async () => {
    if (yamlMode !== 'edit' || !selectedPolicy) return;
    setYamlSaving(true);
    try {
      await NetworkPolicyService.update(clusterId, selectedPolicy.namespace, selectedPolicy.name, yamlContent);
      message.success(t('network:networkpolicy.messages.updateSuccess'));
      setYamlDrawerOpen(false);
      fetchPolicies();
    } catch {
      message.error(t('common:messages.error'));
    } finally {
      setYamlSaving(false);
    }
  };

  const handleSimulate = (_policy: NetworkPolicyInfo) => {
    setViewMode('simulate');
  };

  const handleDelete = async (policy: NetworkPolicyInfo) => {
    try {
      await NetworkPolicyService.delete(clusterId, policy.namespace, policy.name);
      message.success(t('common:messages.deleteSuccess'));
      fetchPolicies();
    } catch {
      message.error(t('common:messages.deleteError'));
    }
  };

  const columns = [
    {
      title: t('common:table.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t('common:table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      render: (ns: string) => <Tag color="blue">{ns}</Tag>,
    },
    {
      title: t('network:networkpolicy.columns.policyTypes'),
      dataIndex: 'policyTypes',
      key: 'policyTypes',
      render: (types: string[]) => (
        <Space>
          {(types ?? []).map(t => (
            <Tag key={t} color={t === 'Ingress' ? 'green' : 'orange'}>{t}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t('network:networkpolicy.columns.podSelector'),
      dataIndex: 'podSelector',
      key: 'podSelector',
      render: (selector: Record<string, string>) => {
        const entries = Object.entries(selector ?? {});
        if (entries.length === 0) return <Tag>{t('network:networkpolicy.columns.allPods')}</Tag>;
        return (
          <Space wrap>
            {entries.map(([k, v]) => (
              <Tag key={k}>{k}={v}</Tag>
            ))}
          </Space>
        );
      },
    },
    {
      title: t('network:networkpolicy.columns.rules'),
      key: 'rules',
      render: (_: unknown, record: NetworkPolicyInfo) => (
        <Space>
          {record.ingressRules > 0 && (
            <Tooltip title="Ingress rules">
              <Tag color="green">↓ {record.ingressRules}</Tag>
            </Tooltip>
          )}
          {record.egressRules > 0 && (
            <Tooltip title="Egress rules">
              <Tag color="orange">↑ {record.egressRules}</Tag>
            </Tooltip>
          )}
          {record.ingressRules === 0 && record.egressRules === 0 && <Tag>—</Tag>}
        </Space>
      ),
    },
    {
      title: t('common:table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 90,
      render: (_: unknown, record: NetworkPolicyInfo) => (
        <ActionButtons
          primary={[
            { key: 'edit', label: t('common:actions.edit'), icon: <EditOutlined />, onClick: () => handleEditYAML(record) },
            { key: 'simulate', label: t('network:networkpolicy.columns.simulate', '策略模擬'), icon: <PlayCircleOutlined />, onClick: () => handleSimulate(record) },
          ]}
          more={[
            ...(canDelete() ? [{
              key: 'delete', label: t('common:actions.delete'), icon: <DeleteOutlined />, danger: true as const,
              onClick: () => handleDelete(record),
              confirm: {
                title: t('network:networkpolicy.messages.confirmDeleteTitle'),
                description: t('network:networkpolicy.messages.confirmDeleteDesc', { name: record.name }),
              },
            }] : []),
          ]}
        />
      ),
    },
  ];

  const drawerTitle = yamlMode === 'edit'
    ? t('network:networkpolicy.editTitle', { name: selectedPolicy?.name })
    : t('network:networkpolicy.viewTitle', { name: selectedPolicy?.name });

  return (
    <div>
      {/* 檢視切換 + 精靈按鈕 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Segmented
          value={viewMode}
          onChange={v => setViewMode(v as 'list' | 'topology' | 'simulate')}
          options={[
            { value: 'list', label: '列表', icon: <UnorderedListOutlined /> },
            { value: 'topology', label: '拓撲圖', icon: <ApartmentOutlined /> },
            { value: 'simulate', label: '策略模擬', icon: <SafetyCertificateOutlined /> },
          ]}
        />
        <Button icon={<ToolOutlined />} onClick={() => setWizardOpen(true)}>
          精靈建立
        </Button>
      </div>

      {/* 拓撲檢視 */}
      {viewMode === 'topology' && (
        <NetworkPolicyTopology clusterId={clusterId} namespaces={namespaces} />
      )}

      {/* 策略模擬檢視 */}
      {viewMode === 'simulate' && (
        <NetworkPolicySimulator clusterId={clusterId} namespaces={namespaces} />
      )}

      {/* 列表檢視 */}
      {viewMode === 'list' && (<>
      {/* 工具列 */}
      <Space style={{ marginBottom: 16 }} wrap>
        <Select
          allowClear
          placeholder={t('common:table.namespace')}
          style={{ width: 180 }}
          value={namespace || undefined}
          onChange={v => setNamespace(v ?? '')}
        >
          <Option value="_all_">{t('common:status.all')}</Option>
        </Select>
        <Input
          placeholder={t('common:actions.search')}
          prefix={<SearchOutlined />}
          value={searchInput}
          onChange={e => setSearchInput(e.target.value)}
          onPressEnter={handleSearch}
          style={{ width: 220 }}
        />
        <Button icon={<SearchOutlined />} onClick={handleSearch}>
          {t('common:actions.search')}
        </Button>
        <Button icon={<ReloadOutlined />} onClick={() => fetchPolicies()}>
          {t('common:actions.refresh')}
        </Button>
        {canWrite() && (
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateClick}>
            {t('network:networkpolicy.create')}
          </Button>
        )}
      </Space>

      <Table
        rowKey={r => `${r.namespace}/${r.name}`}
        columns={columns}
        dataSource={policies}
        loading={loading}
        scroll={{ x: 'max-content' }}
        virtual
        pagination={{
          current: currentPage,
          pageSize,
          total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (tot) => t('network:networkpolicy.pagination.total', { total: tot }),
        }}
        onChange={handleTableChange}
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
      />

      {/* YAML Drawer */}
      <Drawer
        title={drawerTitle}
        open={yamlDrawerOpen}
        onClose={() => setYamlDrawerOpen(false)}
        width={720}
        extra={
          yamlMode !== 'view' && (
            <Button type="primary" loading={yamlSaving} onClick={handleYAMLSave}>
              {t('common:actions.save')}
            </Button>
          )
        }
      >
        <MonacoEditor
          height="calc(100vh - 160px)"
          language="yaml"
          value={yamlContent}
          onChange={v => setYamlContent(v ?? '')}
          options={{
            readOnly: yamlMode === 'view',
            minimap: { enabled: false },
            fontSize: 13,
            scrollBeyondLastLine: false,
          }}
        />
      </Drawer>
      </>)}

      {/* 建立精靈 */}
      <NetworkPolicyWizard
        clusterId={clusterId}
        namespaces={namespaces}
        open={wizardOpen}
        onClose={() => setWizardOpen(false)}
        onCreated={() => { setWizardOpen(false); fetchPolicies(1, pageSize); }}
      />

      {/* 建立 Modal */}
      <NetworkPolicyForm
        open={createModalOpen}
        clusterId={clusterId}
        onClose={() => setCreateModalOpen(false)}
        onSuccess={() => { setCreateModalOpen(false); fetchPolicies(1, pageSize); setCurrentPage(1); }}
      />
    </div>
  );
};

export default NetworkPolicyTab;

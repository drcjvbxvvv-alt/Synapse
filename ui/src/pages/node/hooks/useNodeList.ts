import { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { App, Form, message } from 'antd';
import type { Node } from '../../../types';
import { nodeService, type NodeListParams, type NodeOverview } from '../../../services/nodeService';
import { POLL_INTERVALS } from '../../../config/queryConfig';

export interface SearchCondition {
  field: 'name' | 'status' | 'version' | 'roles';
  value: string;
}

export function useNodeList() {
  const { clusterId: routeClusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  const { message: appMessage, modal } = App.useApp();
  const { t } = useTranslation('node');
  const { t: tc } = useTranslation('common');

  // Data states
  const [loading, setLoading] = useState(false);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [allNodes, setAllNodes] = useState<Node[]>([]);
  const [overview, setOverview] = useState<NodeOverview | null>(null);
  const [selectedClusterId, setSelectedClusterId] = useState<string>(routeClusterId || '1');

  // Pagination states
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [selectedNodes, setSelectedNodes] = useState<React.Key[]>([]);

  // Search states
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'status' | 'version' | 'roles'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // Label modal states
  const [labelModalOpen, setLabelModalOpen] = useState(false);
  const [labelForm] = Form.useForm<{ entries: { key: string; value: string }[] }>();
  const [labelSubmitting, setLabelSubmitting] = useState(false);

  // Column settings states
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'status', 'name', 'roles', 'version', 'readyStatus', 'cpuUsage', 'memoryUsage', 'podCount', 'taints', 'createdAt'
  ]);

  // Sorting states
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);

  const isBatch = selectedNodes.length > 1;

  // Fetch nodes
  const fetchNodes = useCallback(async (params: NodeListParams = { clusterId: selectedClusterId }) => {
    if (!params.clusterId) return;
    setLoading(true);
    try {
      const response = await nodeService.getNodes({
        ...params,
        page: 1,
        pageSize: 10000,
      });
      setAllNodes(response.items || []);
    } catch (error) {
      console.error('Failed to fetch nodes:', error);
      message.error(t('list.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [selectedClusterId, t]);

  // Fetch node overview
  const fetchNodeOverview = useCallback(async () => {
    if (!selectedClusterId) return;
    try {
      const response = await nodeService.getNodeOverview(selectedClusterId);
      setOverview(response);
    } catch (error) {
      console.error('Failed to fetch node overview:', error);
    }
  }, [selectedClusterId]);

  // Handle refresh
  const handleRefresh = useCallback(() => {
    setLoading(true);
    fetchNodes({ clusterId: selectedClusterId });
    if (selectedClusterId) {
      fetchNodeOverview();
    }
  }, [selectedClusterId, fetchNodes, fetchNodeOverview]);

  // Selection change
  const handleSelectionChange = useCallback((selectedRowKeys: React.Key[]) => {
    setSelectedNodes(selectedRowKeys);
  }, []);

  // Batch cordon
  const handleBatchCordon = useCallback(async () => {
    if (selectedNodes.length === 0) return;
    try {
      await Promise.all(selectedNodes.map(n => nodeService.cordonNode(selectedClusterId, String(n))));
      message.success(isBatch ? `已封鎖 ${selectedNodes.length} 個節點` : t('messages.cordonSuccess'));
      setSelectedNodes([]);
      handleRefresh();
    } catch {
      message.error(t('messages.cordonError'));
    }
  }, [selectedNodes, selectedClusterId, isBatch, t, handleRefresh]);

  // Batch uncordon
  const handleBatchUncordon = useCallback(async () => {
    if (selectedNodes.length === 0) return;
    try {
      await Promise.all(selectedNodes.map(n => nodeService.uncordonNode(selectedClusterId, String(n))));
      message.success(isBatch ? `已解封 ${selectedNodes.length} 個節點` : t('messages.uncordonSuccess'));
      setSelectedNodes([]);
      handleRefresh();
    } catch {
      message.error(t('messages.uncordonError'));
    }
  }, [selectedNodes, selectedClusterId, isBatch, t, handleRefresh]);

  // Batch drain
  const handleBatchDrain = useCallback(() => {
    if (selectedNodes.length === 0) return;
    const names = selectedNodes.map(String);
    modal.confirm({
      title: isBatch ? `批次驅逐 ${names.length} 個節點` : t('actions.drain'),
      content: isBatch
        ? `確定要驅逐這 ${names.length} 個節點上的所有 Pod 嗎？此操作可能導致服務中斷。`
        : t('actions.confirmDrain', { name: names[0] }),
      okText: tc('actions.confirm'),
      cancelText: tc('actions.cancel'),
      okType: 'danger',
      onOk: async () => {
        try {
          await Promise.all(names.map(name =>
            nodeService.drainNode(selectedClusterId, name, {
              ignoreDaemonSets: true,
              deleteLocalData: true,
              gracePeriodSeconds: 30,
            })
          ));
          message.success(isBatch ? `已驅逐 ${names.length} 個節點的 Pod` : t('messages.drainSuccess'));
          setSelectedNodes([]);
          handleRefresh();
        } catch {
          message.error(t('messages.drainError'));
        }
      },
    });
  }, [selectedNodes, selectedClusterId, isBatch, t, tc, modal, handleRefresh]);

  // Open label modal
  const handleBatchLabel = useCallback(() => {
    labelForm.setFieldsValue({ entries: [{ key: '', value: '' }] });
    setLabelModalOpen(true);
  }, [labelForm]);

  // Submit labels
  const handleLabelSubmit = useCallback(async () => {
    const values = await labelForm.validateFields();
    const labelsMap: Record<string, string> = {};
    for (const { key, value } of values.entries) {
      if (key.trim()) labelsMap[key.trim()] = value.trim();
    }
    if (Object.keys(labelsMap).length === 0) return;
    setLabelSubmitting(true);
    try {
      await Promise.all(
        selectedNodes.map(n => nodeService.patchNodeLabels(selectedClusterId, String(n), labelsMap))
      );
      message.success(selectedNodes.length > 1 ? `已為 ${selectedNodes.length} 個節點新增標籤` : '標籤已更新');
      setLabelModalOpen(false);
      labelForm.resetFields();
      setSelectedNodes([]);
      handleRefresh();
    } catch {
      message.error('更新標籤失敗');
    } finally {
      setLabelSubmitting(false);
    }
  }, [labelForm, selectedNodes, selectedClusterId, handleRefresh]);

  // Single node operations
  const handleViewDetail = useCallback((name: string) => {
    navigate(`/clusters/${selectedClusterId}/nodes/${name}`);
  }, [navigate, selectedClusterId]);

  const handleNodeTerminal = useCallback((name: string) => {
    navigate(`/clusters/${selectedClusterId}/nodes/${name}?tab=terminal`);
  }, [navigate, selectedClusterId]);

  const handleCordon = useCallback(async (name: string) => {
    try {
      await nodeService.cordonNode(selectedClusterId, name);
      message.success(t('messages.cordonSuccess'));
      handleRefresh();
    } catch (error) {
      console.error('Failed to cordon node:', error);
      message.error(t('messages.cordonError'));
    }
  }, [selectedClusterId, t, handleRefresh]);

  const handleUncordon = useCallback(async (name: string) => {
    try {
      await nodeService.uncordonNode(selectedClusterId, name);
      message.success(t('messages.uncordonSuccess'));
      handleRefresh();
    } catch (error) {
      console.error('Failed to uncordon node:', error);
      message.error(t('messages.uncordonError'));
    }
  }, [selectedClusterId, t, handleRefresh]);

  const handleDrain = useCallback((name: string) => {
    modal.confirm({
      title: t('actions.drain'),
      content: t('actions.confirmDrain', { name }),
      okText: tc('actions.confirm'),
      cancelText: tc('actions.cancel'),
      okType: 'danger',
      onOk: async () => {
        try {
          await nodeService.drainNode(selectedClusterId, name, {
            ignoreDaemonSets: true,
            deleteLocalData: true,
            gracePeriodSeconds: 30,
          });
          message.success(t('messages.drainSuccess'));
          handleRefresh();
        } catch (error) {
          console.error('Failed to drain node:', error);
          message.error(t('messages.drainError'));
        }
      },
    });
  }, [selectedClusterId, t, tc, modal, handleRefresh]);

  // Search condition handlers
  const addSearchCondition = useCallback(() => {
    if (!currentSearchValue.trim()) return;
    const newCondition: SearchCondition = {
      field: currentSearchField,
      value: currentSearchValue.trim(),
    };
    setSearchConditions(prev => [...prev, newCondition]);
    setCurrentSearchValue('');
  }, [currentSearchField, currentSearchValue]);

  const removeSearchCondition = useCallback((index: number) => {
    setSearchConditions(prev => prev.filter((_, i) => i !== index));
  }, []);

  const clearAllConditions = useCallback(() => {
    setSearchConditions([]);
    setCurrentSearchValue('');
  }, []);

  const getFieldLabel = useCallback((field: string): string => {
    const labels: Record<string, string> = {
      name: t('columns.name'),
      status: t('columns.status'),
      version: t('columns.version'),
      roles: t('columns.roles'),
    };
    return labels[field] || field;
  }, [t]);

  // Client-side filtering
  const filterNodes = useCallback((items: Node[]): Node[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(node => {
      const conditionsByField = searchConditions.reduce((acc, condition) => {
        if (!acc[condition.field]) {
          acc[condition.field] = [];
        }
        acc[condition.field].push(condition.value.toLowerCase());
        return acc;
      }, {} as Record<string, string[]>);

      return Object.entries(conditionsByField).every(([field, values]) => {
        if (field === 'roles') {
          return values.some(searchValue =>
            node.roles.some(role => role.toLowerCase().includes(searchValue))
          );
        }
        const nodeValue = node[field as keyof Node];
        const itemStr = String(nodeValue || '').toLowerCase();
        return values.some(searchValue => itemStr.includes(searchValue));
      });
    });
  }, [searchConditions]);

  // Export function
  const handleExport = useCallback(() => {
    try {
      const filteredData = filterNodes(allNodes);
      const sourceData = selectedNodes.length > 0
        ? filteredData.filter(node => selectedNodes.includes(node.id))
        : filteredData;
      if (sourceData.length === 0) {
        appMessage.warning(tc('messages.noData'));
        return;
      }

      const dataToExport = sourceData.map(node => ({
        [t('columns.name')]: node.name,
        [t('columns.status')]: node.status,
        [t('columns.roles')]: node.roles?.join(', ') || '-',
        [t('columns.version')]: node.version || '-',
        [t('columns.cpu')]: `${node.cpuUsage || 0}%`,
        [t('columns.memory')]: `${node.memoryUsage || 0}%`,
        [t('columns.pods')]: `${node.podCount || 0}/${node.maxPods || 0}`,
        [t('detail.taints')]: node.taints?.length || 0,
        [tc('table.createdAt')]: node.creationTimestamp ? new Date(node.creationTimestamp).toLocaleString() : '-',
      }));

      const headers = Object.keys(dataToExport[0]);
      const csvContent = [
        headers.join(','),
        ...dataToExport.map(row =>
          headers.map(header => {
            const value = row[header as keyof typeof row];
            return `"${value}"`;
          }).join(',')
        )
      ].join('\n');

      const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `node-list-${Date.now()}.csv`;
      link.click();
      appMessage.success(tc('messages.exportSuccess'));
    } catch (error) {
      console.error('Failed to export:', error);
      appMessage.error(tc('messages.exportError'));
    }
  }, [filterNodes, allNodes, selectedNodes, appMessage, t, tc]);

  // Column settings save
  const handleColumnSettingsSave = useCallback(() => {
    setColumnSettingsVisible(false);
    appMessage.success(tc('messages.saveSuccess'));
  }, [appMessage, tc]);

  // Route param change effect
  useEffect(() => {
    if (routeClusterId && routeClusterId !== selectedClusterId) {
      setSelectedClusterId(routeClusterId);
      setCurrentPage(1);
      setSearchConditions([]);
      setCurrentSearchValue('');
    }
  }, [routeClusterId, selectedClusterId]);

  // Reset page on search change
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // Update displayed nodes when data/filters/sorting changes
  useEffect(() => {
    if (allNodes.length === 0) {
      setNodes([]);
      setTotal(0);
      return;
    }

    let filteredItems = filterNodes(allNodes);

    // Apply sorting
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof Node];
        const bValue = b[sortField as keyof Node];

        if (aValue === undefined && bValue === undefined) return 0;
        if (aValue === undefined) return sortOrder === 'ascend' ? 1 : -1;
        if (bValue === undefined) return sortOrder === 'ascend' ? -1 : 1;

        if (typeof aValue === 'number' && typeof bValue === 'number') {
          return sortOrder === 'ascend' ? aValue - bValue : bValue - aValue;
        }

        const aStr = String(aValue);
        const bStr = String(bValue);

        if (sortOrder === 'ascend') {
          return aStr > bStr ? 1 : aStr < bStr ? -1 : 0;
        } else {
          return bStr > aStr ? 1 : bStr < aStr ? -1 : 0;
        }
      });
    }

    // Apply pagination
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    const paginatedItems = filteredItems.slice(startIndex, endIndex);

    setNodes(paginatedItems);
    setTotal(filteredItems.length);
  }, [allNodes, filterNodes, currentPage, pageSize, sortField, sortOrder]);

  // Initial load
  useEffect(() => {
    if (selectedClusterId) {
      fetchNodes({ clusterId: selectedClusterId });
      fetchNodeOverview();
    }
  }, [selectedClusterId, fetchNodes, fetchNodeOverview]);

  // Polling
  useEffect(() => {
    if (!selectedClusterId) return;
    const timer = setInterval(() => {
      fetchNodes({ clusterId: selectedClusterId });
      fetchNodeOverview();
    }, POLL_INTERVALS.node);
    return () => clearInterval(timer);
  }, [selectedClusterId, fetchNodes, fetchNodeOverview]);

  // Statistics
  const totalNodes = overview?.totalNodes || 0;
  const readyNodes = overview?.readyNodes || 0;
  const notReadyNodes = overview?.notReadyNodes || 0;
  const maintenanceNodes = overview?.maintenanceNodes || 0;

  // Row selection config
  const nodeRowSelection = useMemo(() => ({
    type: 'checkbox' as const,
    selectedRowKeys: selectedNodes,
    onChange: handleSelectionChange,
  }), [selectedNodes, handleSelectionChange]);

  return {
    // State
    loading,
    nodes,
    selectedNodes,
    selectedClusterId,
    currentPage,
    pageSize,
    total,
    searchConditions,
    currentSearchField,
    currentSearchValue,
    labelModalOpen,
    labelForm,
    labelSubmitting,
    columnSettingsVisible,
    visibleColumns,
    sortField,
    sortOrder,
    isBatch,
    nodeRowSelection,

    // Stats
    totalNodes,
    readyNodes,
    notReadyNodes,
    maintenanceNodes,

    // Setters
    setCurrentSearchField,
    setCurrentSearchValue,
    setLabelModalOpen,
    setColumnSettingsVisible,
    setVisibleColumns,
    setSortField,
    setSortOrder,
    setCurrentPage,
    setPageSize,

    // Handlers
    handleRefresh,
    handleBatchCordon,
    handleBatchUncordon,
    handleBatchDrain,
    handleBatchLabel,
    handleLabelSubmit,
    handleViewDetail,
    handleNodeTerminal,
    handleCordon,
    handleUncordon,
    handleDrain,
    handleExport,
    handleColumnSettingsSave,
    addSearchCondition,
    removeSearchCondition,
    clearAllConditions,
    getFieldLabel,

    // i18n
    t,
    tc,
  };
}

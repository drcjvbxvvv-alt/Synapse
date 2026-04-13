import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { App } from 'antd';
import { WorkloadService, type WorkloadInfo } from '../../../services/workloadService';
import { POLL_INTERVALS } from '../../../config/queryConfig';
import { useMultiSearch, applyMultiSearch } from '../../../hooks/useMultiSearch';

export type WorkloadType = 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Job' | 'CronJob' | 'ArgoRollout';

export interface UseWorkloadTabOptions {
  clusterId: string;
  workloadType: WorkloadType;
  onCountChange?: (count: number) => void;
}

export function useWorkloadTab({ clusterId, workloadType, onCountChange }: UseWorkloadTabOptions) {
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
  const { t } = useTranslation(['workload', 'common']);

  // Data states
  const [allWorkloads, setAllWorkloads] = useState<WorkloadInfo[]>([]);
  const [workloads, setWorkloads] = useState<WorkloadInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);

  // Pagination states
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // Selection state
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  // Scale modal states
  const [scaleModalVisible, setScaleModalVisible] = useState(false);
  const [scaleWorkload, setScaleWorkload] = useState<WorkloadInfo | null>(null);
  const [scaleReplicas, setScaleReplicas] = useState(1);

  // Create modal state
  const [createModalVisible, setCreateModalVisible] = useState(false);

  // Search states
  const {
    conditions: searchConditions,
    currentField: currentSearchField,
    currentValue: currentSearchValue,
    setCurrentField: setCurrentSearchField,
    setCurrentValue: setCurrentSearchValue,
    addCondition: addSearchCondition,
    removeCondition: removeSearchCondition,
    clearAll: clearAllConditions,
  } = useMultiSearch('name');

  // Column settings states
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'namespace', 'status', 'replicas', 'images', 'createdAt'
  ]);

  // Sorting states
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);

  // Load workloads
  // Pass silent=true to suppress the error toast (used for background polling).
  // Accepts unknown first arg so React's SyntheticEvent (from onClick) is treated as non-silent.
  const loadWorkloads = useCallback(async (silent?: unknown) => {
    if (!clusterId) return;
    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloads(
        clusterId,
        undefined,
        workloadType,
        1,
        10000,
        undefined
      );
      setAllWorkloads(response.items || []);
    } catch (error) {
      console.error(`fetch ${workloadType} list failed:`, error);
      // Only suppress when explicitly called with true (background poll).
      // A SyntheticEvent passed via onClick={loadWorkloads} is not === true, so toast still shows.
      if (silent !== true) {
        message.error(t('messages.fetchError', { type: workloadType }));
      }
    } finally {
      setLoading(false);
    }
  }, [clusterId, workloadType, message, t]);

  // Scale workload
  const handleScale = useCallback(async () => {
    if (!scaleWorkload || !clusterId) return;
    try {
      await WorkloadService.scaleWorkload(
        clusterId,
        scaleWorkload.namespace,
        scaleWorkload.name,
        scaleWorkload.type,
        scaleReplicas
      );
      message.success(t('messages.scaleSuccess'));
      setScaleModalVisible(false);
      loadWorkloads();
    } catch (error) {
      console.error('擴縮容失敗:', error);
      message.error(t('messages.scaleError'));
    }
  }, [clusterId, scaleWorkload, scaleReplicas, message, t, loadWorkloads]);

  // Delete workload
  const handleDelete = useCallback(async (workload: WorkloadInfo) => {
    if (!clusterId) return;
    try {
      await WorkloadService.deleteWorkload(
        clusterId,
        workload.namespace,
        workload.name,
        workload.type
      );
      message.success(t('messages.deleteSuccess'));
      loadWorkloads();
    } catch (error) {
      console.error('刪除失敗:', error);
      message.error(t('messages.deleteError'));
    }
  }, [clusterId, message, t, loadWorkloads]);

  // Restart workload
  const handleRestart = useCallback(async (workload: WorkloadInfo) => {
    if (!clusterId) return;
    try {
      await WorkloadService.restartWorkload(clusterId, workload.namespace, workload.name, workload.type);
      message.success(t('actions.restartSuccess', { name: workload.name }));
      loadWorkloads();
    } catch {
      message.error(t('actions.restartError'));
    }
  }, [clusterId, message, t, loadWorkloads]);

  // Open scale modal
  const openScaleModal = useCallback((workload: WorkloadInfo) => {
    setScaleWorkload(workload);
    setScaleReplicas(workload.replicas || 1);
    setScaleModalVisible(true);
  }, []);

  // Navigate to monitor
  const handleMonitor = useCallback((workload: WorkloadInfo) => {
    const typePath = workloadType.toLowerCase();
    navigate(`/clusters/${clusterId}/workloads/${typePath}/${workload.namespace}/${workload.name}?tab=monitoring`);
  }, [clusterId, workloadType, navigate]);

  // Navigate to edit
  const handleEdit = useCallback((workload: WorkloadInfo) => {
    navigate(`/clusters/${clusterId}/workloads/create?type=${workloadType}&namespace=${workload.namespace}&name=${workload.name}`);
  }, [clusterId, workloadType, navigate]);

  // Navigate to detail
  const navigateToDetail = useCallback((workload: WorkloadInfo) => {
    const typePath = workloadType.toLowerCase();
    navigate(`/clusters/${clusterId}/workloads/${typePath}/${workload.namespace}/${workload.name}`);
  }, [clusterId, workloadType, navigate]);

  // Batch redeploy
  const handleBatchRedeploy = useCallback(async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('actions.selectRedeploy', { type: workloadType }));
      return;
    }

    modal.confirm({
      title: t('actions.confirmRedeploy'),
      content: t('actions.confirmRedeployDesc', { count: selectedRowKeys.length, type: workloadType }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          const selectedWorkloads = workloads.filter(w =>
            selectedRowKeys.includes(`${w.namespace}/${w.name}`)
          );

          const redeployPromises = selectedWorkloads.map(workload =>
            WorkloadService.restartWorkload(clusterId, workload.namespace, workload.name, workload.type)
          );

          const results = await Promise.allSettled(redeployPromises);
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;

          if (failCount === 0) {
            message.success(t('common:messages.batchRedeploySuccess', { count: successCount }));
          } else {
            message.warning(t('common:messages.batchRedeployPartial', { success: successCount, fail: failCount }));
          }

          setSelectedRowKeys([]);
          loadWorkloads();
        } catch (error) {
          console.error('批次重新部署失敗:', error);
          message.error(t('messages.redeployError'));
        }
      }
    });
  }, [selectedRowKeys, workloads, clusterId, workloadType, message, modal, t, loadWorkloads]);

  // Export
  const handleExport = useCallback(() => {
    try {
      const filteredData = filterWorkloads(allWorkloads);
      const sourceData = selectedRowKeys.length > 0
        ? filteredData.filter(w => selectedRowKeys.includes(`${w.namespace}-${w.name}-${w.type}`))
        : filteredData;

      if (sourceData.length === 0) {
        message.warning(t('messages.noExportData'));
        return;
      }

      const dataToExport = sourceData.map(w => ({
        [t('columns.name')]: w.name,
        [t('columns.namespace')]: w.namespace,
        [t('columns.status')]: w.status,
        [t('columns.replicas')]: `="${w.readyReplicas || 0}/${w.replicas || 0}"`,
        [t('columns.cpuLimit')]: w.cpuLimit || '-',
        [t('columns.cpuRequest')]: w.cpuRequest || '-',
        [t('columns.memoryLimit')]: w.memoryLimit || '-',
        [t('columns.memoryRequest')]: w.memoryRequest || '-',
        [t('columns.images')]: w.images?.join(', ') || '-',
        [t('columns.createdAt')]: w.createdAt ? new Date(w.createdAt).toLocaleString('zh-TW', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        }).replace(/\//g, '-') : '-',
      }));

      const headers = Object.keys(dataToExport[0]);
      const csvContent = [
        headers.join(','),
        ...dataToExport.map(row =>
          headers.map(header => {
            const value = row[header as keyof typeof row];
            if (String(value).startsWith('="')) {
              return value;
            }
            return `"${value}"`;
          }).join(',')
        )
      ].join('\n');

      const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `${workloadType.toLowerCase()}-list-${Date.now()}.csv`;
      link.click();
      message.success(t('messages.exportSuccess', { count: sourceData.length }));
    } catch (error) {
      console.error('匯出失敗:', error);
      message.error(t('messages.exportError'));
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allWorkloads, selectedRowKeys, workloadType, message, t]);

  const getFieldLabel = useCallback((field: string): string => {
    const labels: Record<string, string> = {
      name: t('search.workloadName'),
      namespace: t('search.namespace'),
      image: t('search.image'),
      status: t('search.status'),
      cpuLimit: t('search.cpuLimit'),
      cpuRequest: t('search.cpuRequest'),
      memoryLimit: t('search.memoryLimit'),
      memoryRequest: t('search.memoryRequest'),
    };
    return labels[field] || field;
  }, [t]);

  // Client-side filtering
  const filterWorkloads = useCallback((items: WorkloadInfo[]): WorkloadInfo[] =>
    applyMultiSearch(items, searchConditions, (workload, field) => {
      const workloadValue = workload[field as keyof WorkloadInfo];
      if (Array.isArray(workloadValue)) return workloadValue.join(' ');
      return String(workloadValue ?? '');
    }),
  [searchConditions]);

  // Column settings save
  const handleColumnSettingsSave = useCallback(() => {
    setColumnSettingsVisible(false);
    message.success(t('messages.columnSettingsSaved'));
  }, [message, t]);

  // Reset page on search change
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // Update displayed workloads when data/filters/sorting changes
  useEffect(() => {
    if (allWorkloads.length === 0) return;

    let filteredItems = filterWorkloads(allWorkloads);

    // Apply sorting
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof WorkloadInfo];
        const bValue = b[sortField as keyof WorkloadInfo];

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

    setWorkloads(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allWorkloads, filterWorkloads, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  // Initial load
  useEffect(() => {
    loadWorkloads();
  }, [loadWorkloads]);

  // Polling — silent to avoid toast spam when optional components (e.g. Argo Rollout) are not installed
  useEffect(() => {
    if (!clusterId) return;
    const timer = setInterval(() => loadWorkloads(true), POLL_INTERVALS.workload);
    return () => clearInterval(timer);
  }, [clusterId, loadWorkloads]);

  // Row selection config
  const rowSelection = {
    columnWidth: 48,
    selectedRowKeys,
    onChange: (keys: React.Key[]) => setSelectedRowKeys(keys as string[]),
  };

  return {
    // State
    workloads,
    loading,
    total,
    selectedRowKeys,
    currentPage,
    pageSize,
    searchConditions,
    currentSearchField,
    currentSearchValue,
    scaleModalVisible,
    scaleWorkload,
    scaleReplicas,
    createModalVisible,
    columnSettingsVisible,
    visibleColumns,
    sortField,
    sortOrder,
    rowSelection,

    // Setters
    setCurrentPage,
    setPageSize,
    setCurrentSearchField,
    setCurrentSearchValue,
    setScaleModalVisible,
    setScaleReplicas,
    setCreateModalVisible,
    setColumnSettingsVisible,
    setVisibleColumns,
    setSortField,
    setSortOrder,

    // Handlers
    loadWorkloads,
    handleScale,
    handleDelete,
    handleRestart,
    openScaleModal,
    handleMonitor,
    handleEdit,
    navigateToDetail,
    handleBatchRedeploy,
    handleExport,
    addSearchCondition,
    removeSearchCondition,
    clearAllConditions,
    getFieldLabel,
    handleColumnSettingsSave,

    // i18n
    t,
  };
}

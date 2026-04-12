import { useState, useCallback, useMemo, useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { STALE_TIMES, POLL_INTERVALS } from '../../../config/queryConfig';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { App } from 'antd';
import { PodService } from '../../../services/podService';
import type { PodInfo } from '../../../services/podService';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { getPodResources } from '../podUtils';
import { useMultiSearch, applyMultiSearch } from '../../../hooks/useMultiSearch';

export function usePodList() {
  const { clusterId: routeClusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
  const { t } = useTranslation('pod');
  const { t: tc } = useTranslation('common');

  const clusterId = routeClusterId || '1';
  const queryClient = useQueryClient();

  const [pods, setPods] = useState<PodInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
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
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'status', 'namespace', 'podIP', 'nodeName', 'restartCount', 'createdAt', 'age',
  ]);
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);

  // 獲取搜尋欄位的顯示名稱
  const getFieldLabel = useCallback((field: string): string => {
    const labels: Record<string, string> = {
      name: t('columns.name'),
      namespace: tc('table.namespace'),
      status: tc('table.status'),
      podIP: t('columns.podIP'),
      nodeName: t('columns.nodeName'),
      cpuRequest: 'CPU Request',
      cpuLimit: 'CPU Limit',
      memoryRequest: 'MEM Request',
      memoryLimit: 'MEM Limit',
    };
    return labels[field] || field;
  }, [t, tc]);

  // 客戶端過濾Pod列表
  const filterPods = useCallback((items: PodInfo[]): PodInfo[] =>
    applyMultiSearch(items, searchConditions, (pod, field) => {
      const resourceFields = ['cpuRequest', 'cpuLimit', 'memoryRequest', 'memoryLimit'];
      if (resourceFields.includes(field)) {
        const resources = getPodResources(pod);
        return String(resources[field as keyof ReturnType<typeof getPodResources>] ?? '');
      }
      return String(pod[field as keyof PodInfo] ?? '');
    }),
  [searchConditions]);

  // React Query：載入所有 Pod
  const {
    data: podData,
    isLoading: loading,
    isError: podError,
  } = useQuery({
    queryKey: ['pods', clusterId],
    queryFn: () => PodService.getPods(clusterId, undefined, undefined, undefined, undefined, undefined, 1, 10000),
    enabled: !!clusterId,
    staleTime: STALE_TIMES.realtime,
    refetchInterval: POLL_INTERVALS.pod,
    refetchOnWindowFocus: true,
  });

  if (podError) {
    message.error(t('list.fetchError'));
  }

  const allPods: PodInfo[] = podData?.items || [];

  const loadPods = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: ['pods', clusterId] });
  }, [queryClient, clusterId]);

  // 刪除Pod
  const handleDelete = async (pod: PodInfo) => {
    if (!clusterId) return;
    try {
      await PodService.deletePod(clusterId, pod.namespace, pod.name);
      message.success(tc('messages.deleteSuccess'));
      loadPods();
    } catch (error) {
      console.error('Failed to delete pod:', error);
      message.error(tc('messages.deleteError'));
    }
  };

  // 批次刪除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('messages.selectPodsFirst'));
      return;
    }

    modal.confirm({
      title: t('actions.confirmBatchDelete'),
      content: t('actions.batchDeleteContent', { count: selectedRowKeys.length }),
      okText: tc('actions.confirm'),
      cancelText: tc('actions.cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const podsToDelete = selectedRowKeys.map(key => {
            const [namespace, name] = key.split('/');
            return { namespace, name };
          });

          const results = await PodService.batchDeletePods(clusterId, podsToDelete);
          const successCount = results.filter(r => r.success).length;
          const failCount = results.length - successCount;

          if (failCount === 0) {
            message.success(t('messages.batchDeleteSuccess', { count: successCount }));
          } else {
            message.warning(t('messages.batchDeletePartial', { success: successCount, fail: failCount }));
          }

          setSelectedRowKeys([]);
          loadPods();
        } catch (error) {
          console.error('Failed to batch delete pods:', error);
          message.error(t('messages.batchDeleteError'));
        }
      },
    });
  };

  // 匯出功能
  const handleExport = () => {
    try {
      const filteredData = filterPods(allPods);
      const sourceData = selectedRowKeys.length > 0
        ? filteredData.filter(pod => selectedRowKeys.includes(`${pod.namespace}/${pod.name}`))
        : filteredData;

      if (sourceData.length === 0) {
        message.warning(tc('messages.noData'));
        return;
      }

      const dataToExport = sourceData.map(pod => {
        const resources = getPodResources(pod);
        return {
          [t('columns.name')]: pod.name,
          [tc('table.status')]: pod.status,
          [tc('table.namespace')]: pod.namespace,
          [t('columns.podIP')]: pod.podIP || '-',
          [t('columns.nodeName')]: pod.nodeName || '-',
          [t('columns.restarts')]: pod.restartCount,
          'CPU Request': resources.cpuRequest,
          'CPU Limit': resources.cpuLimit,
          'MEM Request': resources.memoryRequest,
          'MEM Limit': resources.memoryLimit,
          [tc('table.createdAt')]: pod.createdAt ? new Date(pod.createdAt).toLocaleString() : '-',
          [t('columns.age')]: PodService.getAge(pod.createdAt),
        };
      });

      const headers = Object.keys(dataToExport[0]);
      const csvContent = [
        headers.join(','),
        ...dataToExport.map(row =>
          headers.map(header => `"${row[header as keyof typeof row]}"`).join(',')
        ),
      ].join('\n');

      const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `pod-list-${Date.now()}.csv`;
      link.click();
      message.success(tc('messages.exportSuccess'));
    } catch (error) {
      console.error('Failed to export:', error);
      message.error(tc('messages.exportError'));
    }
  };

  // 列設定儲存
  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(tc('messages.saveSuccess'));
  };

  const handleLogs = (pod: PodInfo) => {
    navigate(`/clusters/${clusterId}/pods/${pod.namespace}/${pod.name}/logs`);
  };

  const handleTerminal = (pod: PodInfo) => {
    window.open(`/clusters/${clusterId}/pods/${pod.namespace}/${pod.name}/terminal`, '_blank');
  };

  const handleViewDetail = (pod: PodInfo) => {
    navigate(`/clusters/${clusterId}/pods/${pod.namespace}/${pod.name}`);
  };

  const handleViewEvents = (pod: PodInfo) => {
    navigate(`/clusters/${clusterId}/pods/${pod.namespace}/${pod.name}?tab=events`);
  };

  // 確認刪除對話框
  const confirmDelete = (record: PodInfo) => {
    modal.confirm({
      title: tc('messages.confirmDelete'),
      content: t('actions.confirmDeleteContent', { name: record.name }),
      okText: tc('actions.confirm'),
      cancelText: tc('actions.cancel'),
      okButtonProps: { danger: true },
      onOk: () => handleDelete(record),
    });
  };

  // 當搜尋條件改變時重置到第一頁
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 當allPods、搜尋條件、分頁參數、排序參數改變時，重新計算顯示資料
  useEffect(() => {
    if (allPods.length === 0) {
      setPods([]);
      setTotal(0);
      return;
    }

    let filteredItems = filterPods(allPods);

    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        let aValue: string | number;
        let bValue: string | number;

        if (['cpuRequest', 'cpuLimit', 'memoryRequest', 'memoryLimit'].includes(sortField)) {
          const aResources = getPodResources(a);
          const bResources = getPodResources(b);
          aValue = aResources[sortField as keyof typeof aResources] || '';
          bValue = bResources[sortField as keyof typeof bResources] || '';
        } else {
          aValue = a[sortField as keyof PodInfo] as string | number;
          bValue = b[sortField as keyof PodInfo] as string | number;
        }

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

    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    setPods(filteredItems.slice(startIndex, endIndex));
    setTotal(filteredItems.length);
  }, [allPods, filterPods, currentPage, pageSize, sortField, sortOrder]);

  // 叢集切換時重新載入
  useEffect(() => {
    if (routeClusterId) {
      setCurrentPage(1);
      clearAllConditions();
      setSelectedRowKeys([]);
      loadPods();
    }
  }, [routeClusterId, loadPods, clearAllConditions]);

  const rowSelection = useMemo(() => ({
    columnWidth: 48,
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  }), [selectedRowKeys]);

  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<PodInfo> | SorterResult<PodInfo>[]
  ) => {
    const singleSorter = Array.isArray(sorter) ? sorter[0] : sorter;
    if (singleSorter && singleSorter.field) {
      setSortField(String(singleSorter.field));
      setSortOrder(singleSorter.order || null);
    } else {
      setSortField('');
      setSortOrder(null);
    }
  };

  return {
    // data
    clusterId,
    pods,
    allPods,
    total,
    loading,
    // pagination
    currentPage,
    pageSize,
    setCurrentPage,
    setPageSize,
    // selection
    selectedRowKeys,
    rowSelection,
    // search
    searchConditions,
    currentSearchField,
    setCurrentSearchField,
    currentSearchValue,
    setCurrentSearchValue,
    addSearchCondition,
    removeSearchCondition,
    clearAllConditions,
    getFieldLabel,
    // column settings
    columnSettingsVisible,
    setColumnSettingsVisible,
    visibleColumns,
    setVisibleColumns,
    handleColumnSettingsSave,
    // sort
    sortField,
    sortOrder,
    handleTableChange,
    // actions
    loadPods,
    handleDelete,
    confirmDelete,
    handleBatchDelete,
    handleExport,
    handleLogs,
    handleTerminal,
    handleViewDetail,
    handleViewEvents,
    // i18n
    t,
    tc,
  };
}

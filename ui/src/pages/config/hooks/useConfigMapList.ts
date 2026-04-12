import { useState, useEffect, useCallback } from 'react';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import { configMapService, type ConfigMapListItem, type NamespaceItem } from '../../../services/configService';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { useMultiSearch, applyMultiSearch } from '../../../hooks/useMultiSearch';

export function useConfigMapList(clusterId: string, onCountChange?: (count: number) => void) {
  const { message, modal } = App.useApp();
  const { t } = useTranslation(['config', 'common']);

  const [allConfigMaps, setAllConfigMaps] = useState<ConfigMapListItem[]>([]);
  const [configMaps, setConfigMaps] = useState<ConfigMapListItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [, setNamespaces] = useState<NamespaceItem[]>([]);

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
    'name', 'namespace', 'labels', 'dataCount', 'creationTimestamp', 'age',
  ]);

  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);

  const [createModalOpen, setCreateModalOpen] = useState(false);

  // ── Filtering ────────────────────────────────────────────────────────────

  const filterConfigMaps = useCallback((items: ConfigMapListItem[]): ConfigMapListItem[] =>
    applyMultiSearch(items, searchConditions, (item, field) => {
      if (field === 'label') {
        return Object.entries(item.labels || {}).map(([k, v]) => `${k}=${v}`).join(' ');
      }
      return String(item[field as keyof ConfigMapListItem] ?? '');
    }),
  [searchConditions]);

  const getFieldLabel = (field: string): string => {
    const labels: Record<string, string> = {
      name: t('config:list.searchFields.name'),
      namespace: t('config:list.searchFields.namespace'),
      label: t('config:list.searchFields.label'),
    };
    return labels[field] || field;
  };

  // ── Data fetching ─────────────────────────────────────────────────────────

  const loadNamespaces = useCallback(async () => {
    if (!clusterId) return;
    try {
      const data = await configMapService.getConfigMapNamespaces(Number(clusterId));
      setNamespaces(data);
    } catch (error) {
      console.error('載入命名空間失敗:', error);
    }
  }, [clusterId]);

  const loadConfigMaps = useCallback(async () => {
    if (!clusterId) return;
    setLoading(true);
    try {
      const response = await configMapService.getConfigMaps(Number(clusterId), {
        page: 1,
        pageSize: 10000,
      });
      setAllConfigMaps(response.items || []);
    } catch (error) {
      console.error('獲取ConfigMap列表失敗:', error);
      message.error(t('config:list.messages.fetchConfigMapError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  // ── Delete handlers ───────────────────────────────────────────────────────

  const handleDelete = async (namespace: string, name: string) => {
    if (!clusterId) return;
    try {
      await configMapService.deleteConfigMap(Number(clusterId), namespace, name);
      message.success(t('common:messages.deleteSuccess'));
      loadConfigMaps();
    } catch (error) {
      console.error('刪除失敗:', error);
      message.error(t('common:messages.deleteError'));
    }
  };

  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('config:list.messages.selectDeleteConfigMap'));
      return;
    }
    modal.confirm({
      title: t('common:messages.confirmDelete'),
      content: t('config:list.messages.confirmBatchDeleteConfigMap', { count: selectedRowKeys.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          for (const key of selectedRowKeys) {
            const [namespace, name] = key.split('/');
            await configMapService.deleteConfigMap(Number(clusterId), namespace, name);
          }
          message.success(t('config:list.messages.batchDeleteSuccess'));
          setSelectedRowKeys([]);
          loadConfigMaps();
        } catch (error) {
          console.error('批次刪除失敗:', error);
          message.error(t('config:list.messages.batchDeleteError'));
        }
      },
    });
  };

  // ── Export ────────────────────────────────────────────────────────────────

  const handleExport = () => {
    try {
      const filteredData = filterConfigMaps(allConfigMaps);
      const sourceData = selectedRowKeys.length > 0
        ? filteredData.filter(item => selectedRowKeys.includes(`${item.namespace}/${item.name}`))
        : filteredData;
      if (sourceData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }
      const dataToExport = sourceData.map(item => ({
        [t('config:list.export.name')]: item.name,
        [t('config:list.export.namespace')]: item.namespace,
        [t('config:list.export.labels')]: Object.entries(item.labels || {}).map(([k, v]) => `${k}=${v}`).join(', ') || '-',
        [t('config:list.export.dataCount')]: item.dataCount,
        [t('config:list.export.createdAt')]: item.creationTimestamp
          ? new Date(item.creationTimestamp).toLocaleString('zh-TW', {
              year: 'numeric', month: '2-digit', day: '2-digit',
              hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
            }).replace(/\//g, '-')
          : '-',
        [t('config:list.export.age')]: item.age || '-',
      }));
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
      link.download = `configmap-list-${Date.now()}.csv`;
      link.click();
      message.success(t('config:list.messages.exportSuccess', { count: sourceData.length }));
    } catch (error) {
      console.error('匯出失敗:', error);
      message.error(t('common:messages.exportError'));
    }
  };

  // ── Column settings ───────────────────────────────────────────────────────

  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(t('config:list.messages.columnSettingsSaved'));
  };

  const toggleColumn = (key: string, checked: boolean) => {
    setVisibleColumns(prev =>
      checked ? [...prev, key] : prev.filter(c => c !== key)
    );
  };

  // ── Table change ──────────────────────────────────────────────────────────

  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<ConfigMapListItem> | SorterResult<ConfigMapListItem>[]
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

  // ── Effects ───────────────────────────────────────────────────────────────

  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  useEffect(() => {
    if (allConfigMaps.length === 0) {
      setConfigMaps([]);
      setTotal(0);
      onCountChange?.(0);
      return;
    }
    let filteredItems = filterConfigMaps(allConfigMaps);
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof ConfigMapListItem];
        const bValue = b[sortField as keyof ConfigMapListItem];
        if (aValue === undefined && bValue === undefined) return 0;
        if (aValue === undefined) return sortOrder === 'ascend' ? 1 : -1;
        if (bValue === undefined) return sortOrder === 'ascend' ? -1 : 1;
        if (typeof aValue === 'number' && typeof bValue === 'number') {
          return sortOrder === 'ascend' ? aValue - bValue : bValue - aValue;
        }
        const aStr = String(aValue);
        const bStr = String(bValue);
        if (sortOrder === 'ascend') return aStr > bStr ? 1 : aStr < bStr ? -1 : 0;
        return bStr > aStr ? 1 : bStr < aStr ? -1 : 0;
      });
    }
    const startIndex = (currentPage - 1) * pageSize;
    const paginatedItems = filteredItems.slice(startIndex, startIndex + pageSize);
    setConfigMaps(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allConfigMaps, filterConfigMaps, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  useEffect(() => {
    loadNamespaces();
    loadConfigMaps();
  }, [loadNamespaces, loadConfigMaps]);

  return {
    // Data
    configMaps,
    allConfigMaps,
    loading,
    total,
    // Pagination
    currentPage,
    pageSize,
    setCurrentPage,
    setPageSize,
    // Selection
    selectedRowKeys,
    setSelectedRowKeys,
    // Search
    searchConditions,
    currentSearchField,
    setCurrentSearchField,
    currentSearchValue,
    setCurrentSearchValue,
    addSearchCondition,
    removeSearchCondition,
    clearAllConditions,
    getFieldLabel,
    // Column settings
    columnSettingsVisible,
    setColumnSettingsVisible,
    visibleColumns,
    toggleColumn,
    handleColumnSettingsSave,
    // Sorting
    sortField,
    sortOrder,
    handleTableChange,
    // Modals
    createModalOpen,
    setCreateModalOpen,
    // Actions
    loadConfigMaps,
    handleDelete,
    handleBatchDelete,
    handleExport,
  };
}

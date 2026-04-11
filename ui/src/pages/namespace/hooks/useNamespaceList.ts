import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { App, Form } from 'antd';
import { useTranslation } from 'react-i18next';
import {
  getNamespaces,
  createNamespace,
  deleteNamespace,
  type NamespaceData,
  type CreateNamespaceRequest,
} from '../../../services/namespaceService';

export interface SearchCondition {
  field: 'name' | 'status' | 'label';
  value: string;
}

const SYSTEM_NAMESPACES = ['default', 'kube-system', 'kube-public', 'kube-node-lease', 'cert-manager'];

export function useNamespaceList() {
  const { clusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
  const { t } = useTranslation(['namespace', 'common']);
  const [form] = Form.useForm();

  // Data states
  const [allNamespaces, setAllNamespaces] = useState<NamespaceData[]>([]);
  const [namespaces, setNamespaces] = useState<NamespaceData[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);

  // Pagination states
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // Operation states
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  // Search states
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'status' | 'label'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // Column settings
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'status', 'labels', 'creationTimestamp',
  ]);

  // Sort states
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);

  const addSearchCondition = () => {
    if (!currentSearchValue.trim()) return;
    const newCondition: SearchCondition = {
      field: currentSearchField,
      value: currentSearchValue.trim(),
    };
    setSearchConditions(prev => [...prev, newCondition]);
    setCurrentSearchValue('');
  };

  const removeSearchCondition = (index: number) => {
    setSearchConditions(prev => prev.filter((_, i) => i !== index));
  };

  const clearAllConditions = () => {
    setSearchConditions([]);
    setCurrentSearchValue('');
  };

  const getFieldLabel = (field: string): string => {
    const labels: Record<string, string> = {
      name: t('list.fieldName'),
      status: t('list.fieldStatus'),
      label: t('list.fieldLabel'),
    };
    return labels[field] || field;
  };

  const filterNamespaces = useCallback((items: NamespaceData[]): NamespaceData[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(namespace => {
      const conditionsByField = searchConditions.reduce((acc, condition) => {
        if (!acc[condition.field]) {
          acc[condition.field] = [];
        }
        acc[condition.field].push(condition.value.toLowerCase());
        return acc;
      }, {} as Record<string, string[]>);

      return Object.entries(conditionsByField).every(([field, values]) => {
        if (field === 'label') {
          const labels = namespace.labels || {};
          const labelStr = Object.entries(labels)
            .map(([k, v]) => `${k}:${v}`)
            .join(' ')
            .toLowerCase();
          return values.some(searchValue => labelStr.includes(searchValue));
        }
        const namespaceValue = namespace[field as keyof NamespaceData];
        const itemStr = String(namespaceValue || '').toLowerCase();
        return values.some(searchValue => itemStr.includes(searchValue));
      });
    });
  }, [searchConditions]);

  const loadNamespaces = useCallback(async () => {
    if (!clusterId) return;
    setLoading(true);
    try {
      const data = await getNamespaces(Number(clusterId));
      setAllNamespaces(data);
    } catch (error) {
      console.error('獲取命名空間列表失敗:', error);
      message.error(t('list.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  const handleCreate = async (values: CreateNamespaceRequest) => {
    if (!clusterId) return;
    try {
      await createNamespace(Number(clusterId), values);
      message.success(t('messages.createSuccess'));
      setCreateModalVisible(false);
      form.resetFields();
      loadNamespaces();
    } catch (error) {
      message.error(t('messages.createError'));
      console.error('Error creating namespace:', error);
    }
  };

  const handleDelete = async (namespace: string) => {
    if (!clusterId) return;
    try {
      await deleteNamespace(Number(clusterId), namespace);
      message.success(t('messages.deleteSuccess'));
      loadNamespaces();
    } catch (error) {
      message.error(t('messages.deleteError'));
      console.error('Error deleting namespace:', error);
    }
  };

  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('common:messages.selectFirst'));
      return;
    }

    const toDelete = selectedRowKeys.filter(ns => !SYSTEM_NAMESPACES.includes(ns));

    if (toDelete.length === 0) {
      message.warning(t('common:messages.cannotDeleteSystem'));
      return;
    }

    modal.confirm({
      title: t('actions.confirmBatchDelete'),
      content: t('actions.confirmBatchDeleteDesc', { count: toDelete.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const deletePromises = toDelete.map(ns => deleteNamespace(Number(clusterId), ns));
          const results = await Promise.allSettled(deletePromises);
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;

          if (failCount === 0) {
            message.success(t('common:messages.batchDeleteSuccess', { count: successCount }));
          } else {
            message.warning(t('common:messages.batchDeletePartial', { success: successCount, fail: failCount }));
          }

          setSelectedRowKeys([]);
          loadNamespaces();
        } catch (error) {
          console.error('批次刪除失敗:', error);
          message.error(t('messages.batchDeleteError'));
        }
      },
    });
  };

  const handleExport = () => {
    try {
      const filteredData = filterNamespaces(allNamespaces);
      const sourceData = selectedRowKeys.length > 0
        ? filteredData.filter(ns => selectedRowKeys.includes(ns.name))
        : filteredData;
      if (sourceData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }

      const dataToExport = sourceData.map(ns => ({
        [t('columns.name')]: ns.name,
        [t('columns.status')]: ns.status === 'Active' ? t('common:status.active') : ns.status,
        [t('columns.labels')]: ns.labels
          ? Object.entries(ns.labels).map(([k, v]) => `${k}=${v}`).join('; ')
          : '-',
        [t('columns.createdAt')]: ns.creationTimestamp
          ? new Date(ns.creationTimestamp).toLocaleString('zh-TW', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false,
          }).replace(/\//g, '-')
          : '-',
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
      link.download = `namespace-list-${Date.now()}.csv`;
      link.click();
      message.success(t('common:messages.exportCount', { count: sourceData.length }));
    } catch (error) {
      console.error('匯出失敗:', error);
      message.error(t('common:messages.exportError'));
    }
  };

  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(t('common:messages.columnSettingsSaved'));
  };

  const handleViewDetail = (namespace: string) => {
    navigate(`/clusters/${clusterId}/namespaces/${namespace}`);
  };

  // Reset to page 1 when search conditions change
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // Recompute displayed data when data, filters, pagination, or sort change
  useEffect(() => {
    if (allNamespaces.length === 0) {
      setNamespaces([]);
      setTotal(0);
      return;
    }

    let filteredItems = filterNamespaces(allNamespaces);

    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof NamespaceData];
        const bValue = b[sortField as keyof NamespaceData];

        if (aValue === undefined && bValue === undefined) return 0;
        if (aValue === undefined) return sortOrder === 'ascend' ? 1 : -1;
        if (bValue === undefined) return sortOrder === 'ascend' ? -1 : 1;

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
    setNamespaces(filteredItems.slice(startIndex, endIndex));
    setTotal(filteredItems.length);
  }, [allNamespaces, filterNamespaces, currentPage, pageSize, sortField, sortOrder]);

  // Initial load
  useEffect(() => {
    loadNamespaces();
  }, [loadNamespaces]);

  return {
    // data
    namespaces,
    loading,
    total,
    allNamespaces,
    // pagination
    currentPage,
    setCurrentPage,
    pageSize,
    setPageSize,
    // selection
    selectedRowKeys,
    setSelectedRowKeys,
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
    // columns
    columnSettingsVisible,
    setColumnSettingsVisible,
    visibleColumns,
    setVisibleColumns,
    // sort
    sortField,
    setSortField,
    sortOrder,
    setSortOrder,
    // actions
    loadNamespaces,
    handleCreate,
    handleDelete,
    handleBatchDelete,
    handleExport,
    handleColumnSettingsSave,
    handleViewDetail,
    // form
    form,
    createModalVisible,
    setCreateModalVisible,
    // i18n
    t,
    // constants
    SYSTEM_NAMESPACES,
  };
}

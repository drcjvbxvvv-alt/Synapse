import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Table,
  Button,
  Space,
  App,
} from 'antd';
import EmptyState from '@/components/EmptyState';
import {
  ReloadOutlined,
  PlusOutlined,
  SettingOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { ServiceService } from '../../services/serviceService';
import type { Service } from '../../types';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import ServiceCreateModal from './ServiceCreateModal';
import ServiceForm from './ServiceForm';
import { YAMLViewModal, EndpointsViewModal, ColumnSettingsDrawer } from './ServiceDrawer';
import { getServiceColumns } from './serviceColumns';
import type { ServiceTabProps, EndpointsData } from './serviceTypes';
import { useTranslation } from 'react-i18next';
import { useMultiSearch, applyMultiSearch } from '../../hooks/useMultiSearch';
import { usePermission } from '../../hooks/usePermission';
import { MultiSearchBar } from '../../components/MultiSearchBar';

const ServiceTab: React.FC<ServiceTabProps> = ({ clusterId, onCountChange }) => {
  const { hasFeature, canWrite } = usePermission();
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
  const { t } = useTranslation(['network', 'common']);

  // 資料狀態
  const [allServices, setAllServices] = useState<Service[]>([]);
  const [services, setServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);

  // 分頁狀態
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // 選擇行狀態
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  // 多條件搜尋狀態
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

  // 列設定狀態
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'type', 'access', 'ports', 'selector', 'createdAt'
  ]);

  // 排序狀態
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);

  // YAML檢視Modal
  const [yamlModalVisible, setYamlModalVisible] = useState(false);
  const [currentYaml, setCurrentYaml] = useState('');
  const [yamlLoading, setYamlLoading] = useState(false);

  // Endpoints檢視Modal
  const [endpointsModalVisible, setEndpointsModalVisible] = useState(false);
  const [currentEndpoints, setCurrentEndpoints] = useState<EndpointsData | null>(null);
  const [endpointsLoading, setEndpointsLoading] = useState(false);

  // 編輯Modal
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editYaml, setEditYaml] = useState('');
  const [editingService, setEditingService] = useState<Service | null>(null);

  // 建立Modal
  const [createModalVisible, setCreateModalVisible] = useState(false);

  // 命名空間列表
  const [namespaces, setNamespaces] = useState<{ name: string; count: number }[]>([]);

  const getFieldLabel = (field: string): string => {
    const labels: Record<string, string> = {
      name: t('network:service.search.name'),
      namespace: t('network:service.search.namespace'),
      type: t('network:service.search.type'),
      clusterIP: t('network:service.search.clusterIP'),
      selector: t('network:service.search.selector'),
    };
    return labels[field] || field;
  };

  // 客戶端過濾
  const filterServices = useCallback((items: Service[]): Service[] =>
    applyMultiSearch(items, searchConditions, (service, field) => {
      if (field === 'selector') return ServiceService.formatSelector(service.selector);
      const value = service[field as keyof Service];
      return typeof value === 'object' && value !== null
        ? JSON.stringify(value)
        : String(value ?? '');
    }),
  [searchConditions]);

  // 載入命名空間列表
  useEffect(() => {
    const loadNamespaces = async () => {
      if (!clusterId) return;
      try {
        const nsList = await ServiceService.getServiceNamespaces(clusterId);
        setNamespaces(nsList);
      } catch (error) {
        console.error('載入命名空間失敗:', error);
      }
    };
    loadNamespaces();
  }, [clusterId]);

  // 獲取Service列表
  const loadServices = useCallback(async () => {
    if (!clusterId) return;
    setLoading(true);
    try {
      const response = await ServiceService.getServices(clusterId, '_all_', '', undefined, 1, 10000);
      setAllServices(response.items || []);
    } catch (error) {
      console.error('Failed to fetch Service list:', error);
      message.error(t('network:service.messages.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 過濾、排序、分頁
  useEffect(() => {
    if (allServices.length === 0) {
      setServices([]);
      setTotal(0);
      onCountChange?.(0);
      return;
    }

    let filteredItems = filterServices(allServices);

    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof Service];
        const bValue = b[sortField as keyof Service];
        if (aValue === undefined && bValue === undefined) return 0;
        if (aValue === undefined) return sortOrder === 'ascend' ? 1 : -1;
        if (bValue === undefined) return sortOrder === 'ascend' ? -1 : 1;
        const aStr = String(aValue);
        const bStr = String(bValue);
        return sortOrder === 'ascend'
          ? (aStr > bStr ? 1 : aStr < bStr ? -1 : 0)
          : (bStr > aStr ? 1 : bStr < aStr ? -1 : 0);
      });
    }

    const startIndex = (currentPage - 1) * pageSize;
    const paginatedItems = filteredItems.slice(startIndex, startIndex + pageSize);

    setServices(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allServices, filterServices, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  useEffect(() => {
    loadServices();
  }, [loadServices]);

  // --- 操作回撥 ---

  const handleViewYAML = useCallback(async (service: Service) => {
    setYamlModalVisible(true);
    setYamlLoading(true);
    try {
      const response = await ServiceService.getServiceYAML(clusterId, service.namespace, service.name);
      setCurrentYaml(response.yaml);
    } catch (error) {
      console.error('Failed to fetch YAML:', error);
      message.error(t('network:service.messages.fetchYAMLError'));
    } finally {
      setYamlLoading(false);
    }
  }, [clusterId, message, t]);

  const handleViewEndpoints = useCallback(async (service: Service) => {
    setEndpointsModalVisible(true);
    setEndpointsLoading(true);
    try {
      const response = await ServiceService.getServiceEndpoints(clusterId, service.namespace, service.name);
      setCurrentEndpoints(response);
    } catch (error) {
      console.error('Failed to fetch Endpoints:', error);
      message.error(t('network:service.messages.fetchEndpointsError'));
    } finally {
      setEndpointsLoading(false);
    }
  }, [clusterId, message, t]);

  const handleDelete = useCallback(async (service: Service) => {
    try {
      await ServiceService.deleteService(clusterId, service.namespace, service.name);
      message.success(t('common:messages.deleteSuccess'));
      loadServices();
    } catch (error) {
      console.error('Failed to delete:', error);
      message.error(t('common:messages.deleteError'));
    }
  }, [clusterId, message, t, loadServices]);

  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('network:service.messages.selectDelete'));
      return;
    }

    modal.confirm({
      title: t('common:messages.confirmDelete'),
      content: t('network:service.messages.confirmDeleteBatch', { count: selectedRowKeys.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          const selectedServices = services.filter(s =>
            selectedRowKeys.includes(`${s.namespace}/${s.name}`)
          );
          const results = await Promise.allSettled(
            selectedServices.map(s => ServiceService.deleteService(clusterId, s.namespace, s.name))
          );
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;

          if (failCount === 0) {
            message.success(t('network:service.messages.batchDeleteSuccess', { count: successCount }));
          } else {
            message.warning(t('network:service.messages.batchDeletePartial', { success: successCount, fail: failCount }));
          }
          setSelectedRowKeys([]);
          loadServices();
        } catch (error) {
          console.error('Batch delete failed:', error);
          message.error(t('network:service.messages.batchDeleteError'));
        }
      }
    });
  };

  const handleExport = () => {
    try {
      const filteredData = filterServices(allServices);
      const sourceData = selectedRowKeys.length > 0
        ? filteredData.filter(s => selectedRowKeys.includes(`${s.namespace}/${s.name}`))
        : filteredData;
      if (sourceData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }

      const dataToExport = sourceData.map(s => ({
        [t('network:service.export.name')]: s.name,
        [t('network:service.export.namespace')]: s.namespace,
        [t('network:service.export.type')]: s.type,
        'ClusterIP': s.clusterIP || '-',
        [t('network:service.export.ports')]: ServiceService.formatPorts(s),
        [t('network:service.export.selector')]: ServiceService.formatSelector(s.selector),
        [t('network:service.export.createdAt')]: s.createdAt ? new Date(s.createdAt).toLocaleString(undefined, {
          year: 'numeric', month: '2-digit', day: '2-digit',
          hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false
        }).replace(/\//g, '-') : '-',
      }));

      const headers = Object.keys(dataToExport[0]);
      const csvContent = [
        headers.join(','),
        ...dataToExport.map(row =>
          headers.map(header => `"${row[header as keyof typeof row]}"`).join(',')
        )
      ].join('\n');

      const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `service-list-${Date.now()}.csv`;
      link.click();
      message.success(t('common:messages.exportCount', { count: sourceData.length }));
    } catch (error) {
      console.error('Export failed:', error);
      message.error(t('common:messages.exportError'));
    }
  };

  const handleEdit = useCallback((service: Service) => {
    navigate(`/clusters/${clusterId}/network/service/${service.namespace}/${service.name}/edit`);
  }, [clusterId, navigate]);

  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(t('common:messages.columnSettingsSaved'));
  };

  // 構建列定義
  const allColumns = useMemo(() => getServiceColumns({
    sortField,
    sortOrder,
    onViewYAML: handleViewYAML,
    onEdit: handleEdit,
    onViewEndpoints: handleViewEndpoints,
    onDelete: handleDelete,
    t,
    canDelete: canWrite(),
    showActions: canWrite(),
  }), [sortField, sortOrder, t, canWrite, handleDelete, handleEdit, handleViewEndpoints, handleViewYAML]);

  const columns = allColumns.filter(col => {
    if (col.key === 'actions' || col.key === 'name') return true;
    return visibleColumns.includes(col.key as string);
  });

  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<Service> | SorterResult<Service>[]
  ) => {
    const singleSorter = Array.isArray(sorter) ? sorter[0] : sorter;
    if (singleSorter?.field) {
      setSortField(String(singleSorter.field));
      setSortOrder(singleSorter.order || null);
    } else {
      setSortField('');
      setSortOrder(null);
    }
  };

  const rowSelection = {
    columnWidth: 48,
    selectedRowKeys,
    onChange: (keys: React.Key[]) => setSelectedRowKeys(keys as string[]),
  };

  return (
    <div>
      {/* 操作按鈕欄 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Space>
          {canWrite() && (
            <Button disabled={selectedRowKeys.length === 0} onClick={handleBatchDelete} danger icon={<DeleteOutlined />}>
              {selectedRowKeys.length > 1
                ? `${t('common:actions.batchDelete')} (${selectedRowKeys.length})`
                : t('common:actions.delete')}
            </Button>
          )}
          {hasFeature('export') && (
            <Button disabled={selectedRowKeys.length === 0} onClick={handleExport}>
              {selectedRowKeys.length > 1
                ? `${t('common:actions.batchExport')} (${selectedRowKeys.length})`
                : t('common:actions.export')}
            </Button>
          )}
        </Space>
        {canWrite() && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalVisible(true)}>
            {t('network:service.createService')}
          </Button>
        )}
      </div>

      {/* 多條件搜尋欄 */}
      <MultiSearchBar
        fieldOptions={[
          { value: 'name', label: t('network:service.search.name') },
          { value: 'namespace', label: t('network:service.search.namespace') },
          { value: 'type', label: t('network:service.search.type') },
          { value: 'clusterIP', label: t('network:service.search.clusterIP') },
          { value: 'selector', label: t('network:service.search.selector') },
        ]}
        conditions={searchConditions}
        currentField={currentSearchField}
        currentValue={currentSearchValue}
        onFieldChange={setCurrentSearchField}
        onValueChange={setCurrentSearchValue}
        onAdd={addSearchCondition}
        onRemove={removeSearchCondition}
        onClear={clearAllConditions}
        getFieldLabel={getFieldLabel}
        extra={
          <>
            <Button icon={<ReloadOutlined />} onClick={() => loadServices()} />
            <Button icon={<SettingOutlined />} onClick={() => setColumnSettingsVisible(true)} />
          </>
        }
      />

      <Table
        columns={columns}
        dataSource={services}
        rowKey={(record) => `${record.namespace}/${record.name}`}
        rowSelection={rowSelection}
        loading={loading}
        virtual
        scroll={{ x: 1200, y: 600 }}
        size="small"
        onChange={handleTableChange}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t('network:service.pagination.total', { total }),
          onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
      />

      <YAMLViewModal
        visible={yamlModalVisible}
        yaml={currentYaml}
        loading={yamlLoading}
        onClose={() => setYamlModalVisible(false)}
      />

      <EndpointsViewModal
        visible={endpointsModalVisible}
        endpoints={currentEndpoints}
        loading={endpointsLoading}
        onClose={() => setEndpointsModalVisible(false)}
      />

      <ServiceCreateModal
        visible={createModalVisible}
        clusterId={clusterId}
        onClose={() => setCreateModalVisible(false)}
        onSuccess={() => loadServices()}
      />

      <ServiceForm
        visible={editModalVisible}
        clusterId={clusterId}
        editingService={editingService}
        initialYaml={editYaml}
        namespaces={namespaces}
        onCancel={() => {
          setEditModalVisible(false);
          setEditYaml('');
          setEditingService(null);
        }}
        onSuccess={() => {
          message.success(t('common:messages.saveSuccess'));
          loadServices();
        }}
      />

      <ColumnSettingsDrawer
        visible={columnSettingsVisible}
        visibleColumns={visibleColumns}
        onVisibleColumnsChange={setVisibleColumns}
        onClose={() => setColumnSettingsVisible(false)}
        onSave={handleColumnSettingsSave}
      />
    </div>
  );
};

export default ServiceTab;

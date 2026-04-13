import React, { useState, useEffect, useCallback } from 'react';
import {
  Table,
  Button,
  Space,
  Tag,
  Typography,
  Tooltip,
  App,
  theme,
} from 'antd';
import {
  ReloadOutlined,
  SettingOutlined,
  DeleteOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { StorageService } from '../../services/storageService';
import type { StorageClass } from '../../types';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { useTranslation } from 'react-i18next';
import { StatusTag } from '../../components/StatusTag';
import { ActionButtons } from '../../components/ActionButtons';
import EmptyState from '../../components/EmptyState';
import { usePermission } from '../../hooks/usePermission';
import { useMultiSearch, applyMultiSearch } from '../../hooks/useMultiSearch';
import { MultiSearchBar } from '../../components/MultiSearchBar';
import StorageClassYAMLModal from './StorageClassYAMLModal';
import StorageClassColumnSettingsDrawer from './StorageClassColumnSettingsDrawer';

const { Link } = Typography;

interface StorageClassTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const StorageClassTab: React.FC<StorageClassTabProps> = ({ clusterId, onCountChange }) => {
  const { hasFeature } = usePermission();
  const { message, modal } = App.useApp();
  const { token } = theme.useToken();

  // 資料狀態
const { t } = useTranslation(['storage', 'common']);
const [allStorageClasses, setAllStorageClasses] = useState<StorageClass[]>([]);
  const [storageClasses, setStorageClasses] = useState<StorageClass[]>([]);
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
    'name', 'provisioner', 'reclaimPolicy', 'volumeBindingMode', 'allowVolumeExpansion', 'isDefault', 'createdAt'
  ]);
  
  // 排序狀態
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);
  
  // YAML檢視Modal
  const [yamlModalVisible, setYamlModalVisible] = useState(false);
  const [currentYaml, setCurrentYaml] = useState('');
  const [yamlLoading, setYamlLoading] = useState(false);

  // 獲取搜尋欄位的顯示名稱
  const getFieldLabel = (field: string): string => {
    const labels: Record<string, string> = {
      name: t('storage:search.fieldName'),
      provisioner: t('storage:search.fieldProvisioner'),
      reclaimPolicy: t('storage:search.fieldReclaimPolicy'),
      volumeBindingMode: t('storage:search.fieldVolumeBindingMode'),
    };
    return labels[field] || field;
  };

  // 客戶端過濾StorageClass列表
  const filterStorageClasses = useCallback((items: StorageClass[]): StorageClass[] =>
    applyMultiSearch(items, searchConditions, (sc, field) =>
      String(sc[field as keyof StorageClass] ?? '')
    ),
  [searchConditions]);

  // 獲取StorageClass列表
  const loadStorageClasses = useCallback(async () => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      const response = await StorageService.getStorageClasses(
        clusterId,
        undefined,
        1,
        10000
      );
      
      const items = response.items || [];
      setAllStorageClasses(items);
    } catch (error) {
      console.error('Failed to fetch StorageClass list:', error);
      message.error(t('storage:messages.fetchStorageClassError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  // 當搜尋條件改變時重置到第一頁
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 當allStorageClasses、搜尋條件、分頁參數、排序參數改變時，重新計算顯示資料
  useEffect(() => {
    if (allStorageClasses.length === 0) {
      setStorageClasses([]);
      setTotal(0);
      onCountChange?.(0);
      return;
    }
    
    let filteredItems = filterStorageClasses(allStorageClasses);
    
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof StorageClass];
        const bValue = b[sortField as keyof StorageClass];
        
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
    const paginatedItems = filteredItems.slice(startIndex, endIndex);
    
    setStorageClasses(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allStorageClasses, filterStorageClasses, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  // 初始載入資料
  useEffect(() => {
    loadStorageClasses();
  }, [loadStorageClasses]);

  // 檢視YAML
  const handleViewYAML = async (sc: StorageClass) => {
    setYamlModalVisible(true);
    setYamlLoading(true);
    try {
      const response = await StorageService.getStorageClassYAML(
        clusterId,
        sc.name
      );
      
      setCurrentYaml(response.yaml);
    } catch (error) {
      console.error('Failed to fetch YAML:', error);
      message.error(t('storage:messages.fetchYAMLError'));
    } finally {
      setYamlLoading(false);
    }
  };

  // 刪除StorageClass
  const handleDelete = async (sc: StorageClass) => {
    try {
      await StorageService.deleteStorageClass(
        clusterId,
        sc.name
      );
      
      message.success(t('common:messages.deleteSuccess'));
      loadStorageClasses();
    } catch (error) {
      console.error('Failed to delete:', error);
      message.error(t('common:messages.deleteError'));
    }
  };

  // 批次刪除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('storage:messages.selectDeleteStorageClass'));
      return;
    }

    modal.confirm({
      title: t('common:messages.confirmDelete'),
      content: t('storage:messages.confirmDeleteStorageClass', { count: selectedRowKeys.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          const selectedSCs = storageClasses.filter(s => 
            selectedRowKeys.includes(s.name)
          );
          
          const deletePromises = selectedSCs.map(sc =>
            StorageService.deleteStorageClass(clusterId, sc.name)
          );
          
          const results = await Promise.allSettled(deletePromises);
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;
          
          if (failCount === 0) {
            message.success(t('storage:messages.batchDeleteSuccess', { count: successCount, type: 'StorageClass' }));
          } else {
            message.warning(t('storage:messages.batchDeletePartial', { success: successCount, fail: failCount }));
          }
          
          setSelectedRowKeys([]);
          loadStorageClasses();
        } catch (error) {
          console.error('Batch delete failed:', error);
          message.error(t('storage:messages.batchDeleteError'));
        }
      }
    });
  };

  // 匯出功能
  const handleExport = () => {
    try {
      const filteredData = filterStorageClasses(allStorageClasses);
      const sourceData = selectedRowKeys.length > 0
        ? filteredData.filter(s => selectedRowKeys.includes(s.name))
        : filteredData;
      if (sourceData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }

      const dataToExport = sourceData.map(s => ({
        [t('storage:export.nameLabel')]: s.name,
        [t('storage:export.provisionerLabel')]: s.provisioner,
        [t('storage:export.reclaimPolicyLabel')]: s.reclaimPolicy || '-',
        [t('storage:export.bindingModeLabel')]: s.volumeBindingMode || '-',
        [t('storage:export.allowExpansionLabel')]: s.allowVolumeExpansion ? t('storage:yes') : t('storage:no'),
        [t('storage:export.defaultLabel')]: s.isDefault ? t('storage:yes') : t('storage:no'),
        [t('storage:export.createdAtLabel')]: s.createdAt ? new Date(s.createdAt).toLocaleString() : '-',
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
      link.download = `storageclass-list-${Date.now()}.csv`;
      link.click();
      message.success(t('common:messages.exportCount', { count: sourceData.length }));
    } catch (error) {
      console.error('Export failed:', error);
      message.error(t('common:messages.exportError'));
    }
  };

  // 列設定儲存
  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(t('common:messages.columnSettingsSaved'));
  };

  // 行選擇配置
  const rowSelection = {
    columnWidth: 48,
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

  // 定義所有可用列
  const allColumns: ColumnsType<StorageClass> = [
    {
      title: t('common:table.name'),
      dataIndex: 'name',
      key: 'name',
      fixed: 'left' as const,
      width: 220,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (name: string, record: StorageClass) => (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Link strong onClick={() => handleViewYAML(record)}>
            {name}
          </Link>
          {record.isDefault && (
            <Tag color="success" style={{ marginLeft: token.marginXS }}>
              {t('storage:columns.default')}
            </Tag>
          )}
        </div>
      ),
    },
    {
      title: t('storage:columns.provisioner'),
      dataIndex: 'provisioner',
      key: 'provisioner',
      width: 250,
      ellipsis: true,
      render: (provisioner: string) => (
        <Tooltip title={provisioner}>
          <span>{provisioner}</span>
        </Tooltip>
      ),
    },
    {
      title: t('storage:columns.reclaimPolicy'),
      dataIndex: 'reclaimPolicy',
      key: 'reclaimPolicy',
      width: 100,
      render: (policy: string) => <StatusTag status={policy} />,
    },
    {
      title: t('storage:columns.volumeBindingMode'),
      dataIndex: 'volumeBindingMode',
      key: 'volumeBindingMode',
      width: 150,
      render: (mode: string) => <StatusTag status={mode} />,
    },
    {
      title: t('storage:columns.allowVolumeExpansion'),
      dataIndex: 'allowVolumeExpansion',
      key: 'allowVolumeExpansion',
      width: 100,
      render: (v: boolean) => <StatusTag status={v ? 'true_yes' : 'false_no'} />,
    },
    {
      title: t('storage:columns.isDefault'),
      dataIndex: 'isDefault',
      key: 'isDefault',
      width: 100,
      render: (v: boolean) => <StatusTag status={v ? 'true_yes' : 'false_no'} />,
    },
    {
      title: t('common:table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 160,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (time: string) => time ? dayjs(time).format('YYYY-MM-DD HH:mm') : '-',
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 90,
      render: (_: unknown, record: StorageClass) => (
        <ActionButtons
          primary={[
            {
              key: 'yaml',
              label: 'YAML',
              icon: <CodeOutlined />,
              onClick: () => handleViewYAML(record),
            },
          ]}
          more={[
            ...(hasFeature('storage:delete') ? [{
              key: 'delete',
              label: t('common:actions.delete'),
              icon: <DeleteOutlined />,
              danger: true as const,
              confirm: {
                title: t('storage:messages.confirmDeleteSC'),
                description: t('storage:messages.confirmDeleteSCDesc', { name: record.name }),
              },
              onClick: () => handleDelete(record),
            }] : []),
          ]}
        />
      ),
    },
  ];

  // 根據可見性過濾列
  const columns = allColumns.filter(col => {
    if (col.key === 'actions') return hasFeature('storage:delete');
    if (col.key === 'name') return true;
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序處理
  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<StorageClass> | SorterResult<StorageClass>[]
  ) => {
    const singleSorter = Array.isArray(sorter) ? sorter[0] : sorter;
    
    if (singleSorter && singleSorter.field) {
      const fieldName = String(singleSorter.field);
      setSortField(fieldName);
      setSortOrder(singleSorter.order || null);
    } else {
      setSortField('');
      setSortOrder(null);
    }
  };

  return (
    <div>
      {/* 操作按鈕欄 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Space>
          {hasFeature('storage:delete') && (
            <Button
              disabled={selectedRowKeys.length === 0}
              onClick={handleBatchDelete}
              danger
              icon={<DeleteOutlined />}
            >
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
      </div>

      {/* 多條件搜尋欄 */}
      <MultiSearchBar
        fieldOptions={[
          { value: 'name', label: t('storage:search.fieldName') },
          { value: 'provisioner', label: t('storage:search.fieldProvisioner') },
          { value: 'reclaimPolicy', label: t('storage:search.fieldReclaimPolicy') },
          { value: 'volumeBindingMode', label: t('storage:search.fieldVolumeBindingMode') },
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
            <Button icon={<ReloadOutlined />} onClick={() => loadStorageClasses()} />
            <Button icon={<SettingOutlined />} onClick={() => setColumnSettingsVisible(true)} />
          </>
        }
      />

      <Table
        columns={columns}
        dataSource={storageClasses}
        rowKey={(record) => record.name}
        rowSelection={rowSelection}
        loading={loading}
        virtual
        scroll={{ x: 'max-content', y: 600 }}
        size="small"
        onChange={handleTableChange}
        locale={{ emptyText: <EmptyState /> }}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t('storage:pagination.totalStorageClass', { total }),
          onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />

      {/* YAML檢視Modal */}
      <StorageClassYAMLModal
        open={yamlModalVisible}
        loading={yamlLoading}
        yaml={currentYaml}
        onClose={() => setYamlModalVisible(false)}
      />

      {/* 列設定抽屜 */}
      <StorageClassColumnSettingsDrawer
        open={columnSettingsVisible}
        visibleColumns={visibleColumns}
        onVisibleColumnsChange={setVisibleColumns}
        onClose={() => setColumnSettingsVisible(false)}
        onSave={handleColumnSettingsSave}
      />
    </div>
  );
};

export default StorageClassTab;

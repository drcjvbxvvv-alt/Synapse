import React, { useState, useEffect, useCallback } from 'react';
import {
  Table,
  Button,
  Space,
  Tag,
  Typography,
  Tooltip,
  App,
} from 'antd';
import {
  ReloadOutlined,
  SettingOutlined,
  DeleteOutlined,
  CodeOutlined,
  PlusOutlined,
  EditOutlined,
} from '@ant-design/icons';
import { useMultiSearch, applyMultiSearch } from '../../hooks/useMultiSearch';
import { MultiSearchBar } from '../../components/MultiSearchBar';
import { StorageService } from '../../services/storageService';
import type { PVC } from '../../types';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { useTranslation } from 'react-i18next';
import { ActionButtons } from '../../components/ActionButtons';
import EmptyState from '../../components/EmptyState';
import YamlViewModal from '../../components/YamlViewModal';
import ColumnSettingsDrawer from '../../components/ColumnSettingsDrawer';
import { usePermission } from '../../hooks/usePermission';
import PVCForm from './PVCForm';

const { Link } = Typography;

interface PVCTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const PVCTab: React.FC<PVCTabProps> = ({ clusterId, onCountChange }) => {
  const { hasFeature } = usePermission();
  const { message, modal } = App.useApp();
  
  // 資料狀態
const { t } = useTranslation(['storage', 'common']);
const [allPVCs, setAllPVCs] = useState<PVC[]>([]);
  const [pvcs, setPVCs] = useState<PVC[]>([]);
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
    'name', 'status', 'volumeName', 'storageClassName', 'capacity', 'accessModes', 'createdAt'
  ]);
  
  // 排序狀態
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);
  
  // YAML檢視Modal
  const [yamlModalVisible, setYamlModalVisible] = useState(false);
  const [currentYaml, setCurrentYaml] = useState('');
  const [yamlLoading, setYamlLoading] = useState(false);

  // 建立/編輯表單
  const [formVisible, setFormVisible] = useState(false);
  const [editingPVC, setEditingPVC] = useState<PVC | null>(null);
  
  // 命名空間列表
  const [, setNamespaces] = useState<{ name: string; count: number }[]>([]);

  // 獲取搜尋欄位的顯示名稱
  const getFieldLabel = (field: string): string => {
    const labels: Record<string, string> = {
      name: t('storage:search.fieldPVCName'),
      namespace: t('storage:search.fieldNamespace'),
      status: t('storage:search.fieldStatus'),
      storageClassName: t('storage:search.fieldStorageClassName'),
      volumeName: t('storage:search.fieldVolumeName'),
    };
    return labels[field] || field;
  };

  // 客戶端過濾PVC列表
  const filterPVCs = useCallback((items: PVC[]): PVC[] =>
    applyMultiSearch(items, searchConditions, (pvc, field) =>
      String(pvc[field as keyof PVC] ?? '')
    ),
  [searchConditions]);

  // 載入命名空間列表
  useEffect(() => {
    const loadNamespaces = async () => {
      if (!clusterId) return;
      try {
        const response = await StorageService.getPVCNamespaces(clusterId);
        setNamespaces(response);
      } catch (error) {
        console.error('Failed to load namespaces:', error);
      }
    };

    loadNamespaces();
  }, [clusterId]);

  // 獲取PVC列表
  const loadPVCs = useCallback(async () => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      const response = await StorageService.getPVCs(
        clusterId,
        '_all_',
        undefined,
        undefined,
        1,
        10000
      );
      
      const items = response.items || [];
      setAllPVCs(items);
    } catch (error) {
      console.error('Failed to fetch PVC list:', error);
      message.error(t('storage:messages.fetchPVCError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  // 當搜尋條件改變時重置到第一頁
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 當allPVCs、搜尋條件、分頁參數、排序參數改變時，重新計算顯示資料
  useEffect(() => {
    if (allPVCs.length === 0) {
      setPVCs([]);
      setTotal(0);
      onCountChange?.(0);
      return;
    }
    
    let filteredItems = filterPVCs(allPVCs);
    
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof PVC];
        const bValue = b[sortField as keyof PVC];
        
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
    
    setPVCs(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allPVCs, filterPVCs, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  // 初始載入資料
  useEffect(() => {
    loadPVCs();
  }, [loadPVCs]);

  // 檢視YAML
  const handleViewYAML = async (pvc: PVC) => {
    setYamlModalVisible(true);
    setYamlLoading(true);
    try {
      const response = await StorageService.getPVCYAML(
        clusterId,
        pvc.namespace,
        pvc.name
      );
      
      setCurrentYaml(response.yaml);
    } catch (error) {
      console.error('Failed to fetch YAML:', error);
      message.error(t('storage:messages.fetchYAMLError'));
    } finally {
      setYamlLoading(false);
    }
  };

  // 刪除PVC
  const handleDelete = async (pvc: PVC) => {
    try {
      await StorageService.deletePVC(
        clusterId,
        pvc.namespace,
        pvc.name
      );
      
      message.success(t('common:messages.deleteSuccess'));
      loadPVCs();
    } catch (error) {
      console.error('刪除失敗:', error);
      message.error(t('common:messages.deleteError'));
    }
  };

  // 批次刪除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('storage:messages.selectDeletePVC'));
      return;
    }

    modal.confirm({
      title: t('common:messages.confirmDelete'),
      content: t('storage:messages.confirmDeletePVC', { count: selectedRowKeys.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          const selectedPVCs = pvcs.filter(p => 
            selectedRowKeys.includes(`${p.namespace}/${p.name}`)
          );
          
          const deletePromises = selectedPVCs.map(pvc =>
            StorageService.deletePVC(clusterId, pvc.namespace, pvc.name)
          );
          
          const results = await Promise.allSettled(deletePromises);
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;
          
          if (failCount === 0) {
            message.success(t('storage:messages.batchDeleteSuccess', { count: successCount, type: 'PVC' }));
          } else {
            message.warning(t('storage:messages.batchDeletePartial', { success: successCount, fail: failCount }));
          }
          
          setSelectedRowKeys([]);
          loadPVCs();
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
      const filteredData = filterPVCs(allPVCs);
      const sourceData = selectedRowKeys.length > 0
        ? filteredData.filter(p => selectedRowKeys.includes(`${p.namespace}/${p.name}`))
        : filteredData;
      if (sourceData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }

      const dataToExport = sourceData.map(p => ({
        [t('storage:export.pvcNameLabel')]: p.name,
        [t('storage:export.namespaceLabel')]: p.namespace,
        [t('storage:export.statusLabel')]: p.status,
        [t('storage:export.volumeNameLabel')]: p.volumeName || '-',
        [t('storage:export.storageClassLabel')]: p.storageClassName || '-',
        [t('storage:export.capacityLabel')]: p.capacity || '-',
        [t('storage:export.accessModesLabel')]: StorageService.formatAccessModes(p.accessModes),
        [t('storage:export.createdAtLabel')]: p.createdAt ? new Date(p.createdAt).toLocaleString() : '-',
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
      link.download = `pvc-list-${Date.now()}.csv`;
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
  const allColumns: ColumnsType<PVC> = [
    {
      title: t('storage:columns.pvcName'),
      dataIndex: 'name',
      key: 'name',
      fixed: 'left' as const,
      width: 200,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (name: string, record: PVC) => (
        <div>
          <Link strong onClick={() => handleViewYAML(record)}>
            {name}
          </Link>
          <div style={{ fontSize: 12, color: '#999' }}>
            {record.namespace}
          </div>
        </div>
      ),
    },
    {
      title: t('common:table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('common:table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Tag color={StorageService.getPVCStatusColor(status)}>
          {status}
        </Tag>
      ),
    },
    {
      title: t('storage:columns.volumeName'),
      dataIndex: 'volumeName',
      key: 'volumeName',
      width: 200,
      render: (volumeName: string) => volumeName || '-',
    },
    {
      title: t('storage:columns.storageClassName'),
      dataIndex: 'storageClassName',
      key: 'storageClassName',
      width: 150,
      render: (name: string) => name ? <Tag>{name}</Tag> : '-',
    },
    {
      title: t('storage:columns.capacity'),
      dataIndex: 'capacity',
      key: 'capacity',
      width: 100,
      render: (capacity: string) => StorageService.formatCapacity(capacity),
    },
    {
      title: t('storage:columns.accessModes'),
      dataIndex: 'accessModes',
      key: 'accessModes',
      width: 120,
      render: (modes: string[]) => (
        <Tooltip title={modes?.join(', ')}>
          <span>{StorageService.formatAccessModes(modes)}</span>
        </Tooltip>
      ),
    },
    {
      title: t('common:table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (createdAt: string) => {
        if (!createdAt) return '-';
        const date = new Date(createdAt);
        return date.toLocaleString('zh-TW');
      },
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 90,
      render: (_: unknown, record: PVC) => (
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
            ...(hasFeature('storage:write') ? [{
              key: 'edit',
              label: t('common:actions.edit'),
              icon: <EditOutlined />,
              onClick: () => { setEditingPVC(record); setFormVisible(true); },
            }] : []),
            ...(hasFeature('storage:delete') ? [{
              key: 'delete',
              label: t('common:actions.delete'),
              icon: <DeleteOutlined />,
              danger: true as const,
              confirm: {
                title: t('storage:messages.confirmDeletePVCItem'),
                description: t('storage:messages.confirmDeletePVCDesc', { name: record.name }),
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
    if (col.key === 'actions') return true;
    if (col.key === 'name') return true;
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序處理
  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<PVC> | SorterResult<PVC>[]
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
        {hasFeature('storage:write') && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => { setEditingPVC(null); setFormVisible(true); }}
          >
            {t('storage:form.createPVC')}
          </Button>
        )}
      </div>

      <MultiSearchBar
        fieldOptions={[
          { value: 'name', label: t('storage:search.fieldPVCName') },
          { value: 'namespace', label: t('storage:search.fieldNamespace') },
          { value: 'status', label: t('storage:search.fieldStatus') },
          { value: 'storageClassName', label: t('storage:search.fieldStorageClassName') },
          { value: 'volumeName', label: t('storage:search.fieldVolumeName') },
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
            <Button icon={<ReloadOutlined />} onClick={() => loadPVCs()} />
            <Button icon={<SettingOutlined />} onClick={() => setColumnSettingsVisible(true)} />
          </>
        }
      />

      <Table
        columns={columns}
        dataSource={pvcs}
        rowKey={(record) => `${record.namespace}/${record.name}`}
        rowSelection={rowSelection}
        loading={loading}
        virtual
        scroll={{ x: 'max-content', y: 600 }}
        size="middle"
        onChange={handleTableChange}
        locale={{ emptyText: <EmptyState /> }}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t('storage:pagination.totalPVC', { total }),
          onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />

      <YamlViewModal
        title="PVC YAML"
        open={yamlModalVisible}
        onCancel={() => setYamlModalVisible(false)}
        yaml={currentYaml}
        loading={yamlLoading}
      />

      <ColumnSettingsDrawer
        open={columnSettingsVisible}
        onClose={() => setColumnSettingsVisible(false)}
        onSave={handleColumnSettingsSave}
        visibleColumns={visibleColumns}
        onChange={setVisibleColumns}
        columnOptions={[
          { key: 'namespace', label: t('common:table.namespace') },
          { key: 'status', label: t('common:table.status') },
          { key: 'volumeName', label: t('storage:columns.volumeName') },
          { key: 'storageClassName', label: t('storage:columns.storageClassName') },
          { key: 'capacity', label: t('storage:columns.capacity') },
          { key: 'accessModes', label: t('storage:columns.accessModes') },
          { key: 'createdAt', label: t('common:table.createdAt') },
        ]}
      />

      <PVCForm
        open={formVisible}
        clusterId={clusterId}
        editing={editingPVC}
        onClose={() => setFormVisible(false)}
        onSuccess={loadPVCs}
      />
    </div>
  );
};

export default PVCTab;

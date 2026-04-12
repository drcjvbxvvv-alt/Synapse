import React, { useState, useEffect, useCallback } from 'react';
import {
  Table,
  Button,
  Space,
  Tag,
  Typography,
  Tooltip,
  Modal,
  App,
  Drawer,
  Checkbox,
} from 'antd';
import {
  ReloadOutlined,
  SettingOutlined,
  DeleteOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import { useMultiSearch, applyMultiSearch } from '../../hooks/useMultiSearch';
import { MultiSearchBar } from '../../components/MultiSearchBar';
import { StorageService } from '../../services/storageService';
import type { PV } from '../../types';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { useTranslation } from 'react-i18next';
import { ActionButtons } from '../../components/ActionButtons';
import EmptyState from '../../components/EmptyState';

const { Link } = Typography;

interface PVTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const PVTab: React.FC<PVTabProps> = ({ clusterId, onCountChange }) => {
  const { message, modal } = App.useApp();
  
  // 資料狀態
const { t } = useTranslation(['storage', 'common']);
const [allPVs, setAllPVs] = useState<PV[]>([]);
  const [pvs, setPVs] = useState<PV[]>([]);
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
    'name', 'status', 'capacity', 'accessModes', 'reclaimPolicy', 'storageClassName', 'claimRef', 'persistentVolumeSource', 'createdAt'
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
      name: t('storage:search.fieldPVName'),
      status: t('storage:search.fieldStatus'),
      storageClassName: t('storage:search.fieldStorageClassName'),
      persistentVolumeSource: t('storage:search.fieldPersistentVolumeSource'),
      reclaimPolicy: t('storage:search.fieldReclaimPolicy'),
    };
    return labels[field] || field;
  };

  // 客戶端過濾PV列表
  const filterPVs = useCallback((items: PV[]): PV[] =>
    applyMultiSearch(items, searchConditions, (pv, field) =>
      String(pv[field as keyof PV] ?? '')
    ),
  [searchConditions]);

  // 獲取PV列表
  const loadPVs = useCallback(async () => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      const response = await StorageService.getPVs(
        clusterId,
        undefined,
        undefined,
        1,
        10000
      );
      
      const items = response.items || [];
      setAllPVs(items);
    } catch (error) {
      console.error('Failed to fetch PV list:', error);
      message.error(t('storage:messages.fetchPVError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message]);

  // 當搜尋條件改變時重置到第一頁
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 當allPVs、搜尋條件、分頁參數、排序參數改變時，重新計算顯示資料
  useEffect(() => {
    if (allPVs.length === 0) {
      setPVs([]);
      setTotal(0);
      onCountChange?.(0);
      return;
    }
    
    let filteredItems = filterPVs(allPVs);
    
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof PV];
        const bValue = b[sortField as keyof PV];
        
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
    
    setPVs(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allPVs, filterPVs, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  // 初始載入資料
  useEffect(() => {
    loadPVs();
  }, [loadPVs]);

  // 檢視YAML
  const handleViewYAML = async (pv: PV) => {
    setYamlModalVisible(true);
    setYamlLoading(true);
    try {
      const response = await StorageService.getPVYAML(
        clusterId,
        pv.name
      );
      
      setCurrentYaml(response.yaml);
    } catch (error) {
      console.error('Failed to fetch YAML:', error);
      message.error(t('storage:messages.fetchYAMLError'));
    } finally {
      setYamlLoading(false);
    }
  };

  // 刪除PV
  const handleDelete = async (pv: PV) => {
    try {
      await StorageService.deletePV(
        clusterId,
        pv.name
      );
      
      message.success(t('common:messages.deleteSuccess'));
      loadPVs();
    } catch (error) {
      console.error('Failed to delete:', error);
      message.error(t('common:messages.deleteError'));
    }
  };

  // 批次刪除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('storage:messages.selectDeletePV'));
      return;
    }

    modal.confirm({
      title: t('common:messages.confirmDelete'),
      content: t('storage:messages.confirmDeletePV', { count: selectedRowKeys.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          const selectedPVs = pvs.filter(p => 
            selectedRowKeys.includes(p.name)
          );
          
          const deletePromises = selectedPVs.map(pv =>
            StorageService.deletePV(clusterId, pv.name)
          );
          
          const results = await Promise.allSettled(deletePromises);
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;
          
          if (failCount === 0) {
            message.success(t('storage:messages.batchDeleteSuccess', { count: successCount, type: 'PV' }));
          } else {
            message.warning(t('storage:messages.batchDeletePartial', { success: successCount, fail: failCount }));
          }
          
          setSelectedRowKeys([]);
          loadPVs();
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
      const filteredData = filterPVs(allPVs);
      const sourceData = selectedRowKeys.length > 0
        ? filteredData.filter(p => selectedRowKeys.includes(p.name))
        : filteredData;
      if (sourceData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }

      const dataToExport = sourceData.map(p => ({
        [t('storage:export.pvNameLabel')]: p.name,
        [t('storage:export.statusLabel')]: p.status,
        [t('storage:export.capacityLabel')]: p.capacity || '-',
        [t('storage:export.accessModesLabel')]: StorageService.formatAccessModes(p.accessModes),
        [t('storage:export.reclaimPolicyLabel')]: p.reclaimPolicy || '-',
        [t('storage:export.storageClassLabel')]: p.storageClassName || '-',
        [t('storage:export.claimRefLabel')]: StorageService.formatClaimRef(p.claimRef),
        [t('storage:export.sourceTypeLabel')]: p.persistentVolumeSource || '-',
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
      link.download = `pv-list-${Date.now()}.csv`;
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
  const allColumns: ColumnsType<PV> = [
    {
      title: t('storage:columns.pvName'),
      dataIndex: 'name',
      key: 'name',
      fixed: 'left' as const,
      width: 200,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (name: string, record: PV) => (
        <Link strong onClick={() => handleViewYAML(record)}>
          {name}
        </Link>
      ),
    },
    {
      title: t('common:table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Tag color={StorageService.getPVStatusColor(status)}>
          {status}
        </Tag>
      ),
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
      title: t('storage:columns.reclaimPolicy'),
      dataIndex: 'reclaimPolicy',
      key: 'reclaimPolicy',
      width: 100,
      render: (policy: string) => (
        <Tag color={StorageService.getReclaimPolicyColor(policy)}>
          {policy}
        </Tag>
      ),
    },
    {
      title: t('storage:columns.storageClassName'),
      dataIndex: 'storageClassName',
      key: 'storageClassName',
      width: 150,
      render: (name: string) => name ? <Tag>{name}</Tag> : '-',
    },
    {
      title: t('storage:columns.claimRef'),
      dataIndex: 'claimRef',
      key: 'claimRef',
      width: 200,
      render: (claimRef: { namespace: string; name: string }) => 
        StorageService.formatClaimRef(claimRef),
    },
    {
      title: t('storage:columns.persistentVolumeSource'),
      dataIndex: 'persistentVolumeSource',
      key: 'persistentVolumeSource',
      width: 120,
      render: (source: string) => <Tag color="purple">{source}</Tag>,
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
        return date.toLocaleString();
      },
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 90,
      render: (_: unknown, record: PV) => (
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
            {
              key: 'delete',
              label: t('common:actions.delete'),
              icon: <DeleteOutlined />,
              danger: true,
              confirm: {
                title: t('storage:messages.confirmDeletePVItem'),
                description: t('storage:messages.confirmDeletePVDesc', { name: record.name }),
              },
              onClick: () => handleDelete(record),
            },
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
    sorter: SorterResult<PV> | SorterResult<PV>[]
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
          <Button disabled={selectedRowKeys.length === 0} onClick={handleExport}>
            {selectedRowKeys.length > 1
              ? `${t('common:actions.batchExport')} (${selectedRowKeys.length})`
              : t('common:actions.export')}
          </Button>
        </Space>
      </div>

      <MultiSearchBar
        fieldOptions={[
          { value: 'name', label: t('storage:search.fieldPVName') },
          { value: 'status', label: t('storage:search.fieldStatus') },
          { value: 'storageClassName', label: t('storage:search.fieldStorageClassName') },
          { value: 'persistentVolumeSource', label: t('storage:search.fieldPersistentVolumeSource') },
          { value: 'reclaimPolicy', label: t('storage:search.fieldReclaimPolicy') },
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
        fieldSelectWidth={130}
        extra={
          <>
            <Button icon={<ReloadOutlined />} onClick={() => loadPVs()} />
            <Button icon={<SettingOutlined />} onClick={() => setColumnSettingsVisible(true)} />
          </>
        }
      />

      <Table
        columns={columns}
        dataSource={pvs}
        rowKey={(record) => record.name}
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
          showTotal: (total) => t('storage:pagination.totalPV', { total }),
          onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />

      {/* YAML檢視Modal */}
      <Modal
        title="PV YAML"
        open={yamlModalVisible}
        onCancel={() => setYamlModalVisible(false)}
        footer={null}
        width={800}
      >
        {yamlLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <span>{t('common:messages.loading')}</span>
          </div>
        ) : (
          <pre style={{ maxHeight: 600, overflow: 'auto', background: '#f5f5f5', padding: 16 }}>
            {currentYaml}
          </pre>
        )}
      </Modal>

      {/* 列設定抽屜 */}
      <Drawer
        title={t('storage:columnSettings.title')}
        placement="right"
        width={400}
        open={columnSettingsVisible}
        onClose={() => setColumnSettingsVisible(false)}
        footer={
          <div style={{ textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setColumnSettingsVisible(false)}>{t('common:actions.cancel')}</Button>
              <Button type="primary" onClick={handleColumnSettingsSave}>{t('storage:columnSettings.confirm')}</Button>
            </Space>
          </div>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8, color: '#666' }}>{t('storage:columnSettings.selectColumns')}</p>
          <Space direction="vertical" style={{ width: '100%' }}>
            {[
              { key: 'status', label: t('common:table.status') },
              { key: 'capacity', label: t('storage:columns.capacity') },
              { key: 'accessModes', label: t('storage:columns.accessModes') },
              { key: 'reclaimPolicy', label: t('storage:columns.reclaimPolicy') },
              { key: 'storageClassName', label: t('storage:columns.storageClassName') },
              { key: 'claimRef', label: t('storage:columns.claimRef') },
              { key: 'persistentVolumeSource', label: t('storage:columns.persistentVolumeSource') },
              { key: 'createdAt', label: t('common:table.createdAt') },
            ].map(item => (
              <Checkbox
                key={item.key}
                checked={visibleColumns.includes(item.key)}
                onChange={(e) => {
                  if (e.target.checked) {
                    setVisibleColumns([...visibleColumns, item.key]);
                  } else {
                    setVisibleColumns(visibleColumns.filter(c => c !== item.key));
                  }
                }}
              >
                {item.label}
              </Checkbox>
            ))}
          </Space>
        </div>
      </Drawer>
    </div>
  );
};

export default PVTab;

import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Table,
  Button,
  Space,
  Tag,
  Select,
  Input,
  Modal,
  Tooltip,
  Badge,
  InputNumber,
  App,
  Popconfirm,
  Checkbox,
  Drawer,
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { WorkloadService } from '../../services/workloadService';
import type { WorkloadInfo } from '../../services/workloadService';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { useTranslation } from 'react-i18next';
import { POLL_INTERVALS } from '../../config/queryConfig';
const { Option } = Select;

interface CronJobTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const CronJobTab: React.FC<CronJobTabProps> = ({ clusterId, onCountChange }) => {
  const navigate = useNavigate();
  const { message } = App.useApp();
const { t } = useTranslation(['workload', 'common']);
// 資料狀態
  const [allWorkloads, setAllWorkloads] = useState<WorkloadInfo[]>([]); // 所有原始資料
  const [workloads, setWorkloads] = useState<WorkloadInfo[]>([]); // 當前頁顯示的資料
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  
  // 分頁狀態
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  
  
  // 操作狀態
  const [scaleModalVisible, setScaleModalVisible] = useState(false);
  const [scaleWorkload, setScaleWorkload] = useState<WorkloadInfo | null>(null);
  const [scaleReplicas, setScaleReplicas] = useState(1);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  
  // 多條件搜尋狀態
  interface SearchCondition {
    field: 'name' | 'namespace' | 'image' | 'status' | 'cpuLimit' | 'cpuRequest' | 'memoryLimit' | 'memoryRequest';
    value: string;
  }
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'namespace' | 'image' | 'status' | 'cpuLimit' | 'cpuRequest' | 'memoryLimit' | 'memoryRequest'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');
  
  // 列設定狀態
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'namespace', 'status', 'replicas', 'images', 'createdAt'
  ]);
  
  // 排序狀態
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);


  // 新增搜尋條件
  const addSearchCondition = () => {
    if (!currentSearchValue.trim()) return;
    
    const newCondition: SearchCondition = {
      field: currentSearchField,
      value: currentSearchValue.trim(),
    };
    
    setSearchConditions([...searchConditions, newCondition]);
    setCurrentSearchValue('');
  };

  // 刪除搜尋條件
  const removeSearchCondition = (index: number) => {
    setSearchConditions(searchConditions.filter((_, i) => i !== index));
  };

  // 清空所有搜尋條件
  const clearAllConditions = () => {
    setSearchConditions([]);
    setCurrentSearchValue('');
  };

  // 處理回車鍵
  const handleSearchKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      addSearchCondition();
    }
  };

  // 獲取欄位的中文標籤
  const getFieldLabel = (field: string): string => {
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
  };

  // 客戶端過濾
  const filterWorkloads = useCallback((items: WorkloadInfo[]): WorkloadInfo[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(workload => {
      const conditionsByField = searchConditions.reduce((acc, condition) => {
        if (!acc[condition.field]) {
          acc[condition.field] = [];
        }
        acc[condition.field].push(condition.value.toLowerCase());
        return acc;
      }, {} as Record<string, string[]>);

      return Object.entries(conditionsByField).every(([field, values]) => {
        const workloadValue = workload[field as keyof WorkloadInfo];
        
        // CPU和記憶體欄位使用精確匹配
        const resourceFields = ['cpuLimit', 'cpuRequest', 'memoryLimit', 'memoryRequest'];
        if (resourceFields.includes(field)) {
          const itemStr = String(workloadValue || '-').toLowerCase();
          return values.some(searchValue => itemStr === searchValue);
        }
        
        if (Array.isArray(workloadValue)) {
          return values.some(searchValue =>
            workloadValue.some(item =>
              String(item).toLowerCase().includes(searchValue)
            )
          );
        }
        
        const itemStr = String(workloadValue || '').toLowerCase();
        return values.some(searchValue => itemStr.includes(searchValue));
      });
    });
  }, [searchConditions]);

  // 客戶端排序
  const sortWorkloads = useCallback((items: WorkloadInfo[]): WorkloadInfo[] => {
    if (!sortField || !sortOrder) return items;

    return [...items].sort((a, b) => {
      let aValue: unknown = a[sortField as keyof WorkloadInfo];
      let bValue: unknown = b[sortField as keyof WorkloadInfo];

      if (sortField === 'replicas') {
        // 按總副本數排序
        aValue = a.replicas || 0;
        bValue = b.replicas || 0;
      } else if (sortField === 'createdAt') {
        aValue = aValue ? new Date(aValue as string).getTime() : 0;
        bValue = bValue ? new Date(bValue as string).getTime() : 0;
      }

      // 處理 undefined 值
      if (aValue === undefined && bValue === undefined) return 0;
      if (aValue === undefined) return sortOrder === 'ascend' ? 1 : -1;
      if (bValue === undefined) return sortOrder === 'ascend' ? -1 : 1;

      // 數字型別比較
      if (typeof aValue === 'number' && typeof bValue === 'number') {
        return sortOrder === 'ascend' ? aValue - bValue : bValue - aValue;
      }

      // 字串型別比較
      const aStr = String(aValue);
      const bStr = String(bValue);

      if (sortOrder === 'ascend') {
        return aStr > bStr ? 1 : aStr < bStr ? -1 : 0;
      } else {
        return bStr > aStr ? 1 : bStr < aStr ? -1 : 0;
      }
    });
  }, [sortField, sortOrder]);

  // 載入CronJob列表
  const loadWorkloads = useCallback(async () => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      // 獲取所有資料（使用大的pageSize）
      const response = await WorkloadService.getWorkloads(
        clusterId,
        undefined,
        'CronJob',
        1,
        10000
      );
      
      const items = response.items || [];
      setAllWorkloads(items);
      
      if (onCountChange) {
        onCountChange(items.length);
      }
    } catch (error) {
      console.error('載入CronJob列表失敗:', error);
      message.error(t('messages.fetchError', { type: 'CronJob' }));
    } finally {
      setLoading(false);
    }
  }, [clusterId, onCountChange, message]);

  // 應用過濾、排序和分頁
  useEffect(() => {
    let result = [...allWorkloads];
    
    // 1. 過濾
    result = filterWorkloads(result);
    
    // 2. 排序
    result = sortWorkloads(result);
    
    // 3. 更新總數
    setTotal(result.length);
    
    // 4. 分頁
    const start = (currentPage - 1) * pageSize;
    const end = start + pageSize;
    const pagedData = result.slice(start, end);
    
    setWorkloads(pagedData);
  }, [allWorkloads, currentPage, pageSize, filterWorkloads, sortWorkloads]);

  // 初始載入
  useEffect(() => {
    loadWorkloads();
  }, [loadWorkloads]);

  // 工作負載狀態輪詢（每 15 秒）
  useEffect(() => {
    if (!clusterId) return;
    const timer = setInterval(() => {
      loadWorkloads();
    }, POLL_INTERVALS.workload);
    return () => clearInterval(timer);
  }, [clusterId, loadWorkloads]);

  // 表格列定義
  const columns: ColumnsType<WorkloadInfo> = [
    {
      title: t('columns.name'),
      dataIndex: 'name',
      key: 'name',
      width: 200,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (text: string, record: WorkloadInfo) => (
        <Button
          type="link"
          onClick={() => navigate(`/clusters/${clusterId}/workloads/cronjob/${record.namespace}/${record.name}`)}
          style={{ 
            padding: 0, 
            height: 'auto',
            whiteSpace: 'normal',
            wordBreak: 'break-all',
            textAlign: 'left'
          }}
        >
            {text}
        </Button>
      ),
    },
    {
      title: t('columns.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('columns.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => {
        let color: 'success' | 'error' | 'default' | 'warning' = 'success';
        if (status === 'Stopped') {
          color = 'default';
        } else if (status === 'Degraded') {
          color = 'warning';
        } else if (status === 'Running') {
          color = 'success';
        }
        return <Badge status={color} text={status} />;
      },
    },
    {
      title: t('columns.replicas'),
      dataIndex: 'replicas',
      key: 'replicas',
      width: 150,
      sorter: true,
      sortOrder: sortField === 'replicas' ? sortOrder : null,
      render: (_: unknown, record: WorkloadInfo) => (
        <span>
          {record.readyReplicas || 0} / {record.replicas || 0}
        </span>
      ),
    },
    {
      title: t('columns.cpuLimit'),
      dataIndex: 'cpuLimit',
      key: 'cpuLimit',
      width: 120,
      render: (value: string) => <span>{value || '-'}</span>,
    },
    {
      title: t('columns.cpuRequest'),
      dataIndex: 'cpuRequest',
      key: 'cpuRequest',
      width: 120,
      render: (value: string) => <span>{value || '-'}</span>,
    },
    {
      title: t('columns.memoryLimit'),
      dataIndex: 'memoryLimit',
      key: 'memoryLimit',
      width: 120,
      render: (value: string) => <span>{value || '-'}</span>,
    },
    {
      title: t('columns.memoryRequest'),
      dataIndex: 'memoryRequest',
      key: 'memoryRequest',
      width: 120,
      render: (value: string) => <span>{value || '-'}</span>,
    },
    {
      title: t('columns.images'),
      dataIndex: 'images',
      key: 'images',
      width: 250,
      render: (images: string[]) => {
        if (!images || images.length === 0) return '-';
        
        const firstImage = images[0];
        const imageNameVersion = firstImage.split('/').pop() || firstImage;
        
        return (
          <div>
            <Tooltip title={firstImage}>
              <Tag style={{ marginBottom: 2, maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {imageNameVersion}
              </Tag>
            </Tooltip>
            {images.length > 1 && (
              <Tooltip title={images.slice(1).map(img => img.split('/').pop()).join('\n')}>
                <Tag style={{ marginBottom: 2 }}>
                  +{images.length - 1}
                </Tag>
              </Tooltip>
            )}
          </div>
        );
      },
    },
    {
      title: t('columns.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (text: string) => {
        if (!text) return '-';
        const date = new Date(text);
        const formatted = date.toLocaleString('zh-TW', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        }).replace(/\//g, '-');
        return <span>{formatted}</span>;
      },
    },
    {
      title: t('columns.actions'),
      key: 'actions',
      width: 180,
      fixed: 'right' as const,
      render: (record: WorkloadInfo) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            onClick={() => navigate(`/clusters/${clusterId}/workloads/create?type=CronJob&namespace=${record.namespace}&name=${record.name}`)}
          >{t('actions.edit')}</Button>
          <Button
            type="link"
            size="small"
            onClick={() => {
              setScaleWorkload(record);
              setScaleReplicas(record.replicas || 1);
              setScaleModalVisible(true);
            }}
          >{t('actions.scale')}</Button>
          <Popconfirm
            title={t('actions.confirmDelete', { type: 'CronJob' })}
            onConfirm={() => handleDelete(record)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            <Button type="link" size="small" danger>{t('actions.delete')}</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // 根據可見列過濾columns
  const filteredColumns = columns.filter(col => {
    if (!col.key) return true;
    if (col.key === 'actions') return true;
    return visibleColumns.includes(col.key as string);
  });

  // 處理表格變化（分頁、排序）
  const handleTableChange = (
    pagination: TablePaginationConfig,
    filters: Record<string, FilterValue | null>,
    sorter: SorterResult<WorkloadInfo> | SorterResult<WorkloadInfo>[]
  ) => {
    const singleSorter = Array.isArray(sorter) ? sorter[0] : sorter;
    
    if (singleSorter.order) {
      setSortField(singleSorter.field as string);
      setSortOrder(singleSorter.order as 'ascend' | 'descend');
    } else {
      setSortField('');
      setSortOrder(null);
    }

    setCurrentPage(pagination.current || 1);
    setPageSize(pagination.pageSize || 20);
  };

  // 擴縮容
  const handleScale = async () => {
    if (!scaleWorkload) return;
    
    try {
      await WorkloadService.scaleWorkload(
        clusterId,
        scaleWorkload.namespace,
        scaleWorkload.name,
        'CronJob',
        scaleReplicas
      );
      message.success(t('messages.scaleSuccess'));
      setScaleModalVisible(false);
      loadWorkloads();
    } catch (error) {
      console.error('擴縮容失敗:', error);
      message.error(t('messages.scaleError'));
    }
  };

  // 刪除
  const handleDelete = async (record: WorkloadInfo) => {
    try {
      await WorkloadService.deleteWorkload(
        clusterId,
        record.namespace,
        record.name,
        'CronJob'
      );
      message.success(t('messages.deleteSuccess'));
      loadWorkloads();
    } catch (error) {
      console.error('刪除失敗:', error);
      message.error(t('messages.deleteError'));
    }
  };

  // 批次重新部署
  const handleBatchRedeploy = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('actions.selectRedeploy', { type: 'CronJob' }));
      return;
    }
    
    try {
      for (const key of selectedRowKeys) {
        const workload = workloads.find(w => `${w.namespace}/${w.name}` === key);
        if (workload) {
          await WorkloadService.restartWorkload(
            clusterId,
            workload.namespace,
            workload.name,
            'CronJob'
          );
        }
      }
      message.success(t('messages.redeploySuccess'));
      setSelectedRowKeys([]);
      loadWorkloads();
    } catch (error) {
      console.error('重新部署失敗:', error);
      message.error(t('messages.redeployError'));
    }
  };

  // 匯出
  const handleExport = () => {
    const filteredData = filterWorkloads(sortWorkloads(allWorkloads));
    const sourceData = selectedRowKeys.length > 0
      ? filteredData.filter(w => selectedRowKeys.includes(`${w.namespace}/${w.name}`))
      : filteredData;

    const headers = [
      t('columns.name'),
      t('columns.namespace'),
      t('columns.status'),
      t('columns.replicas'),
      t('columns.cpuLimit'),
      t('columns.cpuRequest'),
      t('columns.memoryLimit'),
      t('columns.memoryRequest'),
      t('columns.images'),
      t('columns.createdAt')
    ];
    
    const dataToExport = sourceData.map(workload => [
      workload.name,
      workload.namespace,
      workload.status,
      `="${(workload.readyReplicas || 0)}/${(workload.replicas || 0)}"`,
      workload.cpuLimit || '-',
      workload.cpuRequest || '-',
      workload.memoryLimit || '-',
      workload.memoryRequest || '-',
      (workload.images || []).map(img => img.split('/').pop()).join(', '),
      new Date(workload.createdAt).toLocaleString('zh-TW')
    ]);
    
    const csvContent = [
      headers.join(','),
      ...dataToExport.map(row => 
        row.map(cell => {
          const cellStr = String(cell);
          if (cellStr.startsWith('="') && cellStr.endsWith('"')) {
            return cellStr;
          }
          if (cellStr.includes(',') || cellStr.includes('"') || cellStr.includes('\n')) {
            return `"${cellStr.replace(/"/g, '""')}"`;
          }
          return cellStr;
        }).join(',')
      )
    ].join('\n');
    
    const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `cronjob_${new Date().getTime()}.csv`;
    link.click();
    URL.revokeObjectURL(link.href);
  };

  // 列設定
  const allColumns = [
    { key: 'name', title: t('columns.name'), fixed: true },
    { key: 'namespace', title: t('columns.namespace') },
    { key: 'status', title: t('columns.status') },
    { key: 'replicas', title: t('columns.replicas') },
    { key: 'cpuLimit', title: t('columns.cpuLimit') },
    { key: 'cpuRequest', title: t('columns.cpuRequest') },
    { key: 'memoryLimit', title: t('columns.memoryLimit') },
    { key: 'memoryRequest', title: t('columns.memoryRequest') },
    { key: 'images', title: t('columns.images') },
    { key: 'createdAt', title: t('columns.createdAt') },
  ];

  const handleColumnVisibilityChange = (columnKey: string, checked: boolean) => {
    if (checked) {
      setVisibleColumns([...visibleColumns, columnKey]);
    } else {
      setVisibleColumns(visibleColumns.filter(k => k !== columnKey));
    }
  };

  const handleSaveColumnSettings = () => {
    setColumnSettingsVisible(false);
    message.success(t('messages.columnSettingsSaved'));
  };

  return (
    <div>
      {/* 操作按鈕欄 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Space>
          <Button
            disabled={selectedRowKeys.length === 0}
            onClick={handleBatchRedeploy}
            icon={<ReloadOutlined />}
          >
            {selectedRowKeys.length > 1
              ? `${t('actions.batchRedeploy')} (${selectedRowKeys.length})`
              : t('actions.redeploy')}
          </Button>
          <Button disabled={selectedRowKeys.length === 0} onClick={handleExport}>
            {selectedRowKeys.length > 1
              ? `${t('actions.batchExport')} (${selectedRowKeys.length})`
              : t('actions.export')}
          </Button>
        </Space>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate(`/clusters/${clusterId}/workloads/create?type=CronJob`)}
        >
          {t('actions.create', { type: 'CronJob' })}
        </Button>
      </div>

      {/* 多條件搜尋欄 */}
      <div style={{ marginBottom: 16 }}>
        {/* 搜尋輸入框 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
          <Input
            prefix={<SearchOutlined />}
            placeholder={t('search.placeholder')}
            style={{ flex: 1 }}
            value={currentSearchValue}
            onChange={(e) => setCurrentSearchValue(e.target.value)}
            onKeyPress={handleSearchKeyPress}
            allowClear
            addonBefore={
            <Select
              value={currentSearchField}
              onChange={setCurrentSearchField}
                style={{ width: 140 }}
            >
              <Option value="name">{t('search.workloadName')}</Option>
              <Option value="namespace">{t('search.namespace')}</Option>
              <Option value="image">{t('search.image')}</Option>
              <Option value="status">{t('search.status')}</Option>
              <Option value="cpuLimit">{t('search.cpuLimit')}</Option>
              <Option value="cpuRequest">{t('search.cpuRequest')}</Option>
              <Option value="memoryLimit">{t('search.memoryLimit')}</Option>
              <Option value="memoryRequest">{t('search.memoryRequest')}</Option>
            </Select>
            }
            />
          <Button
            icon={<ReloadOutlined />}
            onClick={loadWorkloads}
          >
            </Button>
          <Button
            icon={<SettingOutlined />}
            onClick={() => setColumnSettingsVisible(true)}
          >
            </Button>
        </div>

        {/* 搜尋條件標籤 */}
        {searchConditions.length > 0 && (
          <div style={{ marginBottom: 8 }}>
          <Space wrap>
            {searchConditions.map((condition, index) => (
              <Tag
                key={index}
                closable
                onClose={() => removeSearchCondition(index)}
              >
                {getFieldLabel(condition.field)}: {condition.value}
              </Tag>
            ))}
            <Button type="link" size="small" onClick={clearAllConditions}>
                {t('common:actions.clearAllConditions')}
            </Button>
          </Space>
          </div>
        )}
      </div>

      {/* 表格 */}
      <Table
        rowSelection={{
          columnWidth: 48,
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys as string[]),
        }}
        columns={filteredColumns}
        dataSource={workloads}
        locale={{ emptyText: t('common:noData') }}
        rowKey={(record) => `${record.namespace}/${record.name}`}
        loading={loading}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showTotal: (total) => t('messages.totalItems', { count: total, type: 'CronJob' }),
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
        onChange={handleTableChange}
        virtual
        scroll={{ x: 1500, y: 600 }}
      />

      {/* 擴縮容Modal */}
      <Modal
        title={t('scale.title', { type: 'CronJob' })}
        open={scaleModalVisible}
        onOk={handleScale}
        onCancel={() => setScaleModalVisible(false)}
        okText={t('common:actions.confirm')}
        cancelText={t('common:actions.cancel')}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <div>
            <strong>{t('scale.workloadName', { type: 'CronJob' })}：</strong>{scaleWorkload?.name}
          </div>
          <div>
            <strong>{t('scale.namespace')}：</strong>{scaleWorkload?.namespace}
          </div>
          <div>
            <strong>{t('scale.currentReplicas')}：</strong>{scaleWorkload?.replicas}
          </div>
          <Space>
            <span>{t('scale.targetReplicas')}：</span>
            <InputNumber
              min={0}
              value={scaleReplicas}
              onChange={(value) => setScaleReplicas(value || 1)}
            />
          </Space>
        </Space>
      </Modal>

      {/* 列設定Drawer */}
      <Drawer
        title={t('columnSettings.title')}
        placement="right"
        onClose={() => setColumnSettingsVisible(false)}
        open={columnSettingsVisible}
        width={400}
        footer={
          <Space style={{ float: 'right' }}>
            <Button onClick={() => setColumnSettingsVisible(false)}>{t('common:actions.cancel')}</Button>
            <Button type="primary" onClick={handleSaveColumnSettings}>{t('common:actions.save')}</Button>
          </Space>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          {allColumns.map(col => (
            <Checkbox
              key={col.key}
              checked={visibleColumns.includes(col.key)}
              onChange={(e) => handleColumnVisibilityChange(col.key, e.target.checked)}
              disabled={col.fixed}
            >
              {col.title}
            </Checkbox>
          ))}
        </Space>
      </Drawer>
    </div>
  );
};

export default CronJobTab;

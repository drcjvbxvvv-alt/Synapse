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
  Dropdown,
} from 'antd';
import type { MenuProps } from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  SearchOutlined,
  AppstoreOutlined,
  DeploymentUnitOutlined,
  ArrowRightOutlined,
  ExclamationCircleOutlined,
  LineChartOutlined,
  EditOutlined,
  ColumnWidthOutlined,
  DeleteOutlined,
  MoreOutlined,
} from '@ant-design/icons';
import { Typography } from 'antd';
const { Text } = Typography;
import { WorkloadService } from '../../services/workloadService';
import type { WorkloadInfo } from '../../services/workloadService';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { useTranslation } from 'react-i18next';
import { POLL_INTERVALS } from '../../config/queryConfig';
import WorkloadCreateModal from '../../components/workload/WorkloadCreateModal';
const { Option } = Select;

interface DeploymentTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const DeploymentTab: React.FC<DeploymentTabProps> = ({ clusterId, onCountChange }) => {
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
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
  const [createModalVisible, setCreateModalVisible] = useState(false);
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

// 獲取搜尋欄位的顯示名稱
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
// 客戶端過濾工作負載列表
  const filterWorkloads = useCallback((items: WorkloadInfo[]): WorkloadInfo[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(workload => {
      // 按欄位分組條件
      const conditionsByField = searchConditions.reduce((acc, condition) => {
        if (!acc[condition.field]) {
          acc[condition.field] = [];
        }
        acc[condition.field].push(condition.value.toLowerCase());
        return acc;
      }, {} as Record<string, string[]>);

      // 不同欄位之間是 AND 關係
      // 相同欄位之間是 OR 關係
      return Object.entries(conditionsByField).every(([field, values]) => {
        const workloadValue = workload[field as keyof WorkloadInfo];
        
        // CPU和記憶體欄位使用精確匹配
        const resourceFields = ['cpuLimit', 'cpuRequest', 'memoryLimit', 'memoryRequest'];
        if (resourceFields.includes(field)) {
          const itemStr = String(workloadValue || '-').toLowerCase();
          return values.some(searchValue => itemStr === searchValue);
        }
        
        if (Array.isArray(workloadValue)) {
          // 對於陣列型別（如 images），檢查是否有任何值匹配
          return values.some(searchValue =>
            workloadValue.some(item =>
              String(item).toLowerCase().includes(searchValue)
            )
          );
        }
        
        // 對於其他字串型別，使用模糊匹配
        const itemStr = String(workloadValue || '').toLowerCase();
        return values.some(searchValue => itemStr.includes(searchValue));
      });
    });
  }, [searchConditions]);

  // 載入Deployment列表（獲取所有資料，不分頁）
  const loadWorkloads = useCallback(async () => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      // 獲取所有資料（設定一個很大的pageSize）
      const response = await WorkloadService.getWorkloads(
        clusterId,
        undefined,
        'Deployment',
        1,
        10000, // 獲取所有資料
        undefined
      );
      
      const items = response.items || [];
      setAllWorkloads(items);
    } catch (error) {
      console.error('獲取Deployment列表失敗:', error);
message.error(t('messages.fetchError', { type: 'Deployment' }));
} finally {
      setLoading(false);
    }
  }, [clusterId, message]);

  // 擴縮容
  const handleScale = async () => {
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
  };

  // 刪除
  const handleDelete = async (workload: WorkloadInfo) => {
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
  };

  // 監控
  const handleMonitor = (workload: WorkloadInfo) => {
    navigate(`/clusters/${clusterId}/workloads/deployment/${workload.namespace}/${workload.name}?tab=monitoring`);
  };

  // 編輯
  const handleEdit = (workload: WorkloadInfo) => {
    navigate(`/clusters/${clusterId}/workloads/create?type=Deployment&namespace=${workload.namespace}&name=${workload.name}`);
  };

  // 更多操作選單
  const moreActions = (record: WorkloadInfo): MenuProps['items'] => [
    {
      key: 'scale',
      label: t('actions.scale'),
      icon: <ColumnWidthOutlined />,
      onClick: () => {
        setScaleWorkload(record);
        setScaleReplicas(record.replicas || 1);
        setScaleModalVisible(true);
      },
    },
    {
      key: 'restart',
      label: t('actions.restart'),
      icon: <ReloadOutlined />,
      onClick: () => handleRestart(record),
    },
    { type: 'divider' },
    {
      key: 'delete',
      danger: true,
      icon: <DeleteOutlined />,
      label: (
        <Popconfirm
          title={t('actions.confirmDelete', { type: 'Deployment' })}
          description={t('actions.confirmDeleteDesc', { name: record.name })}
          onConfirm={() => handleDelete(record)}
          okText={t('common:actions.confirm')}
          cancelText={t('common:actions.cancel')}
        >
          {t('common:actions.delete')}
        </Popconfirm>
      ),
    },
  ];

  // 行內重啟單一 Deployment
  const handleRestart = async (workload: WorkloadInfo) => {
    if (!clusterId) return;
    try {
      await WorkloadService.restartWorkload(clusterId, workload.namespace, workload.name, workload.type);
      message.success(t('actions.restartSuccess', { name: workload.name }));
      loadWorkloads();
    } catch {
      message.error(t('actions.restartError'));
    }
  };

  // 批次重新部署
  const handleBatchRedeploy = async () => {
    if (selectedRowKeys.length === 0) {
message.warning(t('actions.selectRedeploy', { type: 'Deployment' }));
return;
    }

    modal.confirm({
title: t('actions.confirmRedeploy'),
      content: t('actions.confirmRedeployDesc', { count: selectedRowKeys.length, type: 'Deployment' }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
onOk: async () => {
        try {
    const selectedWorkloads = workloads.filter(w => 
            selectedRowKeys.includes(`${w.namespace}/${w.name}`)
    );
    
          // 重新部署：重啟所有Pod（透過更新annotation的方式）
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
  };

  // 匯出功能（匯出所有篩選後的資料，包含所有列）
  const handleExport = () => {
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

      // 匯出為CSV
      const headers = Object.keys(dataToExport[0]);
      const csvContent = [
        headers.join(','),
        ...dataToExport.map(row => 
          headers.map(header => {
            const value = row[header as keyof typeof row];
            // 對於已經包含公式的單元格（以=開頭），不再加引號
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
      link.download = `deployment-list-${Date.now()}.csv`;
      link.click();
      message.success(t('messages.exportSuccess', { count: sourceData.length }));
    } catch (error) {
      console.error('匯出失敗:', error);
      message.error(t('messages.exportError'));
}
  };

  // 列設定儲存
  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
message.success(t('messages.columnSettingsSaved'));
};

  // 當搜尋條件改變時重置到第一頁
  useEffect(() => {
      setCurrentPage(1);
  }, [searchConditions]);

  // 當allWorkloads、搜尋條件、分頁參數、排序參數改變時，重新計算顯示資料
  useEffect(() => {
    if (allWorkloads.length === 0) return;
    
    // 1. 應用客戶端過濾
    let filteredItems = filterWorkloads(allWorkloads);
    
    // 2. 應用排序
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof WorkloadInfo];
        const bValue = b[sortField as keyof WorkloadInfo];
        
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
    }
    
    // 3. 計算分頁
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    const paginatedItems = filteredItems.slice(startIndex, endIndex);
    
    setWorkloads(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allWorkloads, filterWorkloads, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  // 初始載入資料
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

  // 行選擇配置
  const rowSelection = {
    columnWidth: 48,
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

// 定義所有可用列
  const allColumns: ColumnsType<WorkloadInfo> = [
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
          onClick={() => navigate(`/clusters/${clusterId}/workloads/deployment/${record.namespace}/${record.name}`)}
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
        
        // 提取 name:version 部分（去掉 registry）
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
        // 格式化為：YYYY-MM-DD HH:mm:ss
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
      width: 120,
      fixed: 'right' as const,
      render: (record: WorkloadInfo) => (
        <Space size={0}>
          <Tooltip title={t('actions.monitoring')}>
            <Button type="link" size="small" icon={<LineChartOutlined />} onClick={() => handleMonitor(record)} />
          </Tooltip>
          <Tooltip title={t('common:actions.edit')}>
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          </Tooltip>
          <Dropdown menu={{ items: moreActions(record) }} trigger={['click']}>
            <Button type="link" size="small" icon={<MoreOutlined />} />
          </Dropdown>
        </Space>
      ),
    },
  ];
// 根據可見性過濾列
  const columns = allColumns.filter(col => {
    if (col.key === 'actions') return true; // 操作列始終顯示
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序處理（只更新排序狀態，實際排序在useEffect中處理）
  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<WorkloadInfo> | SorterResult<WorkloadInfo>[]
  ) => {
    // 處理單個排序器
    const singleSorter = Array.isArray(sorter) ? sorter[0] : sorter;
    
    if (singleSorter && singleSorter.field) {
      const fieldName = String(singleSorter.field);
      setSortField(fieldName);
      setSortOrder(singleSorter.order || null);
    } else {
      // 清除排序
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
            onClick={() => setCreateModalVisible(true)}
          >
            {t('actions.create', { type: 'Deployment' })}
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
            onPressEnter={addSearchCondition}
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
            onClick={() => {
              loadWorkloads();
            }}
          >
          </Button>
          <Button icon={<SettingOutlined />} onClick={() => setColumnSettingsVisible(true)} />
        </div>

        {/* 搜尋條件標籤 */}
        {searchConditions.length > 0 && (
          <div>
            <Space size="small" wrap>
              {searchConditions.map((condition, index) => (
                <Tag
                  key={index}
                  closable
                  onClose={() => removeSearchCondition(index)}
                  color="blue"
                >
                  {getFieldLabel(condition.field)}: {condition.value}
                </Tag>
              ))}
              <Button
                size="small"
                type="link"
                onClick={clearAllConditions}
                style={{ padding: 0 }}
              >
                {t('common:actions.clearAll')}
          </Button>
        </Space>
          </div>
        )}
      </div>

      <Table
        columns={columns}
        dataSource={workloads}
        locale={{ emptyText: t('common:messages.noData') }}
        rowKey={(record) => `${record.namespace}-${record.name}-${record.type}`}
        rowSelection={rowSelection}
        loading={loading}
        virtual
        scroll={{ x: 1400, y: 600 }}
        size="middle"
        onChange={handleTableChange}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
showTotal: (total) => t('messages.totalItems', { count: total, type: 'Deployment' }),
onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />

      {/* 擴縮容模態框 */}
      <Modal
        title={
          <Space>
            <DeploymentUnitOutlined style={{ color: '#3b82f6' }} />
            <span>{t('scale.title', { type: 'Deployment' })}</span>
          </Space>
        }
        open={scaleModalVisible}
        onOk={handleScale}
        onCancel={() => setScaleModalVisible(false)}
        okText={t('common:actions.confirm')}
        cancelText={t('common:actions.cancel')}
        width={420}
      >
        {scaleWorkload && (
          <div style={{ paddingTop: 8 }}>
            {/* 工作負載資訊卡 */}
            <div style={{
              background: 'linear-gradient(135deg, #eff6ff 0%, #f8fafc 100%)',
              border: '1px solid #dbeafe',
              borderRadius: 10,
              padding: '14px 16px',
              marginBottom: 24,
            }}>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <AppstoreOutlined style={{ color: '#3b82f6', fontSize: 14 }} />
                  <Text type="secondary" style={{ fontSize: 12, width: 80, flexShrink: 0 }}>Deployment</Text>
                  <Text strong style={{ fontSize: 13, color: '#111827' }}>{scaleWorkload.name}</Text>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <DeploymentUnitOutlined style={{ color: '#6b7280', fontSize: 14 }} />
                  <Text type="secondary" style={{ fontSize: 12, width: 80, flexShrink: 0 }}>{t('scale.namespace')}</Text>
                  <Text style={{ fontSize: 13 }}>{scaleWorkload.namespace}</Text>
                </div>
              </Space>
            </div>

            {/* 副本數調整區 */}
            <div style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 28,
              padding: '8px 0 20px',
            }}>
              {/* 目前副本數 */}
              <div style={{ textAlign: 'center', minWidth: 80 }}>
                <div style={{ fontSize: 11, color: '#9ca3af', marginBottom: 6, fontWeight: 500, letterSpacing: '0.03em' }}>
                  {t('scale.currentReplicas')}
                </div>
                <div style={{
                  fontSize: 42,
                  fontWeight: 800,
                  color: '#374151',
                  lineHeight: 1,
                  fontFamily: '"SF Mono", "Fira Code", monospace',
                }}>
                  {scaleWorkload.replicas || 0}
                </div>
              </div>

              {/* 箭頭 */}
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4, paddingTop: 20 }}>
                <ArrowRightOutlined style={{
                  fontSize: 20,
                  color: scaleReplicas !== (scaleWorkload.replicas || 0) ? '#3b82f6' : '#d1d5db',
                  transition: 'color 0.2s',
                }} />
              </div>

              {/* 目標副本數 */}
              <div style={{ textAlign: 'center', minWidth: 80 }}>
                <div style={{ fontSize: 11, color: '#9ca3af', marginBottom: 6, fontWeight: 500, letterSpacing: '0.03em' }}>
                  {t('scale.targetReplicas')}
                </div>
                <InputNumber
                  min={0}
                  max={100}
                  value={scaleReplicas}
                  onChange={(value) => setScaleReplicas(value ?? 0)}
                  controls
                  className="scale-replicas-input"
                  style={{
                    width: 96,
                    borderRadius: 8,
                    borderColor: scaleReplicas !== (scaleWorkload.replicas || 0) ? '#3b82f6' : undefined,
                  }}
                />
              </div>
            </div>

            {/* 縮容為 0 警示 */}
            {scaleReplicas === 0 && (
              <div style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                background: '#fffbeb',
                border: '1px solid #fde68a',
                borderRadius: 8,
                padding: '10px 14px',
                marginTop: 4,
              }}>
                <ExclamationCircleOutlined style={{ color: '#f59e0b', flexShrink: 0 }} />
                <Text style={{ fontSize: 13, color: '#92400e' }}>
                  副本數設為 0 將暫停所有 Pod，服務將停止對外提供。
                </Text>
              </div>
            )}
          </div>
        )}
      </Modal>

      {/* 列設定抽屜 */}
<Drawer
        title={t('columnSettings.title')}
        placement="right"
        width={400}
        open={columnSettingsVisible}
        onClose={() => setColumnSettingsVisible(false)}
        footer={
          <div style={{ textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setColumnSettingsVisible(false)}>{t('common:actions.cancel')}</Button>
              <Button type="primary" onClick={handleColumnSettingsSave}>{t('common:actions.confirm')}</Button>
            </Space>
          </div>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8, color: '#666' }}>{t('columnSettings.selectColumns')}</p>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Checkbox
              checked={visibleColumns.includes('name')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'name']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'name'));
                }
              }}
            >
              {t('columns.name')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('namespace')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'namespace']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'namespace'));
                }
              }}
            >
              {t('columns.namespace')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('status')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'status']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'status'));
                }
              }}
            >
              {t('columns.status')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('replicas')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'replicas']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'replicas'));
                }
              }}
            >
              {t('columns.replicas')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('cpuLimit')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'cpuLimit']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'cpuLimit'));
                }
              }}
            >
              {t('columns.cpuLimit')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('cpuRequest')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'cpuRequest']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'cpuRequest'));
                }
              }}
            >
              {t('columns.cpuRequest')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('memoryLimit')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'memoryLimit']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'memoryLimit'));
                }
              }}
            >
              {t('columns.memoryLimit')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('memoryRequest')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'memoryRequest']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'memoryRequest'));
                }
              }}
            >
              {t('columns.memoryRequest')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('images')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'images']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'images'));
                }
              }}
            >
              {t('columns.images')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('createdAt')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'createdAt']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'createdAt'));
                }
              }}
            >
              {t('columns.createdAt')}
            </Checkbox>
          </Space>
        </div>
      </Drawer>

      <WorkloadCreateModal
        open={createModalVisible}
        workloadType="Deployment"
        clusterId={clusterId}
        onClose={() => setCreateModalVisible(false)}
        onSuccess={() => {
          setCreateModalVisible(false);
          loadWorkloads();
        }}
      />
</div>
  );
};

export default DeploymentTab;


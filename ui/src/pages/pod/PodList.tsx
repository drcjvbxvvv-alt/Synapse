import React, { useState, useCallback, useMemo, useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { STALE_TIMES, POLL_INTERVALS } from '../../config/queryConfig';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Input,
  Select,
  Tooltip,
  Badge,
  App,
  Checkbox,
  Drawer,
  Dropdown,
  Tabs,
  Spin,
  Typography,
} from 'antd';
import type { MenuProps } from 'antd';
import {
  ReloadOutlined,
  SettingOutlined,
  SearchOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { PodService } from '../../services/podService';
import type { PodInfo } from '../../services/podService';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';

const { Option } = Select;

// 解析CPU值（轉換為毫核）
const parseCpuValue = (value: string): number => {
  if (!value) return 0;
  if (value.endsWith('m')) {
    return parseInt(value.slice(0, -1), 10);
  }
  return parseFloat(value) * 1000;
};

// 格式化CPU值
const formatCpuValue = (milliCores: number): string => {
  if (milliCores >= 1000) {
    return `${(milliCores / 1000).toFixed(1)}`;
  }
  return `${milliCores}m`;
};

// 解析記憶體值（轉換為位元組）
const parseMemoryValue = (value: string): number => {
  if (!value) return 0;
  const units: Record<string, number> = {
    'Ki': 1024,
    'Mi': 1024 * 1024,
    'Gi': 1024 * 1024 * 1024,
    'Ti': 1024 * 1024 * 1024 * 1024,
    'K': 1000,
    'M': 1000 * 1000,
    'G': 1000 * 1000 * 1000,
    'T': 1000 * 1000 * 1000 * 1000,
  };
  
  for (const [unit, multiplier] of Object.entries(units)) {
    if (value.endsWith(unit)) {
      return parseFloat(value.slice(0, -unit.length)) * multiplier;
    }
  }
  return parseFloat(value);
};

// 格式化記憶體值
const formatMemoryValue = (bytes: number): string => {
  if (bytes >= 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}Gi`;
  }
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(0)}Mi`;
  }
  if (bytes >= 1024) {
    return `${(bytes / 1024).toFixed(0)}Ki`;
  }
  return `${bytes}`;
};

// 獲取Pod的CPU和Memory資源
const getPodResources = (pod: PodInfo) => {
  let cpuRequest = 0;
  let cpuLimit = 0;
  let memoryRequest = 0;
  let memoryLimit = 0;

  pod.containers?.forEach(container => {
    // CPU Request
    if (container.resources?.requests?.cpu) {
      cpuRequest += parseCpuValue(container.resources.requests.cpu);
    }
    // CPU Limit
    if (container.resources?.limits?.cpu) {
      cpuLimit += parseCpuValue(container.resources.limits.cpu);
    }
    // Memory Request
    if (container.resources?.requests?.memory) {
      memoryRequest += parseMemoryValue(container.resources.requests.memory);
    }
    // Memory Limit
    if (container.resources?.limits?.memory) {
      memoryLimit += parseMemoryValue(container.resources.limits.memory);
    }
  });

  return {
    cpuRequest: cpuRequest > 0 ? formatCpuValue(cpuRequest) : '-',
    cpuLimit: cpuLimit > 0 ? formatCpuValue(cpuLimit) : '-',
    memoryRequest: memoryRequest > 0 ? formatMemoryValue(memoryRequest) : '-',
    memoryLimit: memoryLimit > 0 ? formatMemoryValue(memoryLimit) : '-',
  };
};

const PodList: React.FC = () => {
  const { clusterId: routeClusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
  const { t } = useTranslation('pod');
  const { t: tc } = useTranslation('common');
  
  const clusterId = routeClusterId || '1';
  
  const queryClient = useQueryClient();

  // 派生狀態：client-side 篩選和分頁後的顯示資料
  const [pods, setPods] = useState<PodInfo[]>([]);
  const [total, setTotal] = useState(0);
  
  // 分頁狀態
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  
  // 操作狀態
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  
  // 多條件搜尋狀態
  interface SearchCondition {
    field: 'name' | 'namespace' | 'status' | 'podIP' | 'nodeName' | 'cpuRequest' | 'cpuLimit' | 'memoryRequest' | 'memoryLimit';
    value: string;
  }
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<SearchCondition['field']>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // 列設定狀態
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'status', 'namespace', 'podIP', 'nodeName', 'restartCount', 'createdAt', 'age'
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
  };

  // 客戶端過濾Pod列表
  const filterPods = useCallback((items: PodInfo[]): PodInfo[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(pod => {
      const resources = getPodResources(pod);
      
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
        let podValue: string;
        
        switch (field) {
          case 'cpuRequest':
            podValue = resources.cpuRequest;
            break;
          case 'cpuLimit':
            podValue = resources.cpuLimit;
            break;
          case 'memoryRequest':
            podValue = resources.memoryRequest;
            break;
          case 'memoryLimit':
            podValue = resources.memoryLimit;
            break;
          default:
            podValue = String(pod[field as keyof PodInfo] || '');
        }
        
        // CPU和記憶體欄位使用精確匹配
        const resourceFields = ['cpuRequest', 'cpuLimit', 'memoryRequest', 'memoryLimit'];
        if (resourceFields.includes(field)) {
          return values.some(searchValue => podValue.toLowerCase() === searchValue);
        }
        
        // 對於其他字串型別，使用模糊匹配
        return values.some(searchValue => podValue.toLowerCase().includes(searchValue));
      });
    });
  }, [searchConditions]);

  // React Query：載入所有 Pod（大 pageSize，client-side 篩選）
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
      }
    });
  };

  // 匯出功能（匯出所有篩選後的資料，包含所有列）
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

      // 匯出為CSV
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

  // 檢視Pod日誌
  const handleViewLogs = (pod: PodInfo) => {
    navigate(`/clusters/${clusterId}/pods/${pod.namespace}/${pod.name}/logs`);
  };

  // 進入Pod終端 - 新視窗開啟
  const handleTerminal = (pod: PodInfo) => {
    window.open(`/clusters/${clusterId}/pods/${pod.namespace}/${pod.name}/terminal`, '_blank');
  };

  // 檢視Pod詳情（監控）
  const handleViewDetail = (pod: PodInfo) => {
    navigate(`/clusters/${clusterId}/pods/${pod.namespace}/${pod.name}`);
  };

  // 檢視Pod事件
  const handleViewEvents = (pod: PodInfo) => {
    navigate(`/clusters/${clusterId}/pods/${pod.namespace}/${pod.name}?tab=events`);
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
    
    // 1. 應用客戶端過濾
    let filteredItems = filterPods(allPods);
    
    // 2. 應用排序
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        let aValue: string | number;
        let bValue: string | number;
        
        // 處理資源欄位
        if (['cpuRequest', 'cpuLimit', 'memoryRequest', 'memoryLimit'].includes(sortField)) {
          const aResources = getPodResources(a);
          const bResources = getPodResources(b);
          aValue = aResources[sortField as keyof typeof aResources] || '';
          bValue = bResources[sortField as keyof typeof bResources] || '';
        } else {
          aValue = a[sortField as keyof PodInfo] as string | number;
          bValue = b[sortField as keyof PodInfo] as string | number;
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
    }
    
    // 3. 計算分頁
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    const paginatedItems = filteredItems.slice(startIndex, endIndex);
    
    setPods(paginatedItems);
    setTotal(filteredItems.length);
  }, [allPods, filterPods, currentPage, pageSize, sortField, sortOrder]);


  // 叢集切換時重新載入
  useEffect(() => {
    if (routeClusterId) {
      setCurrentPage(1);
      setSearchConditions([]);
      setSelectedRowKeys([]);
      loadPods();
    }
  }, [routeClusterId, loadPods]);

  const rowSelection = useMemo(() => ({
    columnWidth: 48,
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  }), [selectedRowKeys]);

  // 操作選單
  const getActionMenuItems = (record: PodInfo): MenuProps['items'] => [
    {
      key: 'monitor',
      label: tc('menu.monitoring'),
      onClick: () => handleViewDetail(record),
    },
    {
      key: 'events',
      label: t('detail.events'),
      onClick: () => handleViewEvents(record),
    },
    {
      type: 'divider',
    },
    {
      key: 'delete',
      label: tc('actions.delete'),
      danger: true,
      onClick: () => {
        modal.confirm({
          title: tc('messages.confirmDelete'),
          content: t('actions.confirmDeleteContent', { name: record.name }),
          okText: tc('actions.confirm'),
          cancelText: tc('actions.cancel'),
          okButtonProps: { danger: true },
          onOk: () => handleDelete(record),
        });
      },
    },
  ];

  const allColumns = useMemo<ColumnsType<PodInfo>>(() => [
    {
      title: t('columns.name'),
      dataIndex: 'name',
      key: 'name',
      width: 220,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (text: string, record: PodInfo) => (
        <Space size={4}>
          <Button
            type="link"
            onClick={() => handleViewDetail(record)}
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
          <Typography.Text copyable={{ text }} style={{ fontSize: 12 }} />
        </Space>
      ),
    },
    {
      title: tc('table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      sorter: true,
      sortOrder: sortField === 'status' ? sortOrder : null,
      render: (_: unknown, record: PodInfo) => {
        const { status, color } = PodService.formatStatus(record);
        // 將顏色值對映為Badge元件的status值
        const getBadgeStatus = (color: string): 'success' | 'error' | 'default' | 'processing' | 'warning' => {
          switch (color) {
            case 'green':
              return 'success';
            case 'orange':
              return 'warning';
            case 'red':
              return 'error';
            case 'blue':
              return 'processing';
            default:
              return 'default';
          }
        };
        return <Badge status={getBadgeStatus(color)} text={status} />;
      },
    },
    {
      title: tc('table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('columns.podIP'),
      dataIndex: 'podIP',
      key: 'podIP',
      width: 130,
      render: (text: string) => text || '-',
    },
    {
      title: t('columns.nodeName'),
      dataIndex: 'nodeName',
      key: 'nodeName',
      width: 150,
      sorter: true,
      sortOrder: sortField === 'nodeName' ? sortOrder : null,
      render: (text: string) => text || '-',
    },
    {
      title: t('columns.restarts'),
      dataIndex: 'restartCount',
      key: 'restartCount',
      width: 80,
      sorter: true,
      sortOrder: sortField === 'restartCount' ? sortOrder : null,
      render: (count: number) => (
        <Tag color={count > 0 ? 'orange' : 'green'}>{count}</Tag>
      ),
    },
    {
      title: 'CPU Request',
      key: 'cpuRequest',
      width: 110,
      render: (_: unknown, record: PodInfo) => {
        const resources = getPodResources(record);
        return <span>{resources.cpuRequest}</span>;
      },
    },
    {
      title: 'CPU Limit',
      key: 'cpuLimit',
      width: 100,
      render: (_: unknown, record: PodInfo) => {
        const resources = getPodResources(record);
        return <span>{resources.cpuLimit}</span>;
      },
    },
    {
      title: 'MEM Request',
      key: 'memoryRequest',
      width: 120,
      render: (_: unknown, record: PodInfo) => {
        const resources = getPodResources(record);
        return <span>{resources.memoryRequest}</span>;
      },
    },
    {
      title: 'MEM Limit',
      key: 'memoryLimit',
      width: 110,
      render: (_: unknown, record: PodInfo) => {
        const resources = getPodResources(record);
        return <span>{resources.memoryLimit}</span>;
      },
    },
    {
      title: tc('table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 150,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (text: string) => {
        if (!text) return '-';
        const date = new Date(text);
        return <span>{date.toLocaleString()}</span>;
      },
    },
    {
      title: t('columns.age'),
      key: 'age',
      width: 100,
      render: (_: unknown, record: PodInfo) => PodService.getAge(record.createdAt),
    },
    {
      title: tc('table.actions'),
      key: 'actions',
      width: 180,
      fixed: 'right' as const,
      render: (_: unknown, record: PodInfo) => (
        <Space size="small">
          <Tooltip title={t('actions.terminal')}>
            <Button
              type="link"
              size="small"
              onClick={() => handleTerminal(record)}
              disabled={record.status !== 'Running'}
            >
              {t('actions.login')}
            </Button>
          </Tooltip>
          <Tooltip title={t('actions.viewLogs')}>
            <Button
              type="link"
              size="small"
              onClick={() => handleViewLogs(record)}
            >
              {t('actions.logs')}
            </Button>
          </Tooltip>
          <Dropdown
            menu={{ items: getActionMenuItems(record) }}
            trigger={['click']}
          >
            <Button type="link" size="small">
              {tc('actions.more')}
            </Button>
          </Dropdown>
        </Space>
      ),
    },
  ], [t, tc, sortField, sortOrder, clusterId, navigate, message, loadPods]);

  const columns = useMemo(() => allColumns.filter(col => {
    if (col.key === 'actions') return true;
    return visibleColumns.includes(col.key as string);
  }), [allColumns, visibleColumns]);

  // 表格排序處理（只更新排序狀態，實際排序在useEffect中處理）
  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<PodInfo> | SorterResult<PodInfo>[]
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

  // 列設定選項
  const columnOptions = [
    { key: 'name', label: t('columns.name') },
    { key: 'status', label: tc('table.status') },
    { key: 'namespace', label: tc('table.namespace') },
    { key: 'podIP', label: t('columns.podIP') },
    { key: 'nodeName', label: t('columns.nodeName') },
    { key: 'restartCount', label: t('columns.restarts') },
    { key: 'cpuRequest', label: 'CPU Request' },
    { key: 'cpuLimit', label: 'CPU Limit' },
    { key: 'memoryRequest', label: 'MEM Request' },
    { key: 'memoryLimit', label: 'MEM Limit' },
    { key: 'createdAt', label: tc('table.createdAt') },
    { key: 'age', label: t('columns.age') },
  ];

  // Tab項配置
  const tabItems = [
    {
      key: 'pod',
      label: t('list.tabTitle'),
      children: (
        <div>
          {/* 操作按鈕欄 */}
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Space>
              <Button
                danger
                disabled={selectedRowKeys.length === 0}
                onClick={handleBatchDelete}
                icon={<DeleteOutlined />}
              >
                {selectedRowKeys.length > 1
                  ? `${t('actions.batchDelete')} (${selectedRowKeys.length})`
                  : tc('actions.delete')}
              </Button>
              <Button onClick={handleExport}>
                {selectedRowKeys.length > 1
                  ? `${tc('actions.batchExport')} (${selectedRowKeys.length})`
                  : tc('actions.export')}
              </Button>
            </Space>
          </div>

          {/* 多條件搜尋欄 */}
          <div style={{ marginBottom: 16 }}>
            {/* 搜尋輸入框 */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
              <Input
                prefix={<SearchOutlined />}
                placeholder={t('list.searchPlaceholder')}
                style={{ flex: 1 }}
                value={currentSearchValue}
                onChange={(e) => setCurrentSearchValue(e.target.value)}
                onPressEnter={addSearchCondition}
                allowClear
                addonBefore={
                  <Select 
                    value={currentSearchField} 
                    onChange={setCurrentSearchField} 
                    style={{ width: 130 }}
                  >
                    <Option value="name">{t('columns.name')}</Option>
                    <Option value="namespace">{tc('table.namespace')}</Option>
                    <Option value="status">{tc('table.status')}</Option>
                    <Option value="podIP">{t('columns.podIP')}</Option>
                    <Option value="nodeName">{t('columns.nodeName')}</Option>
                    <Option value="cpuRequest">CPU Request</Option>
                    <Option value="cpuLimit">CPU Limit</Option>
                    <Option value="memoryRequest">MEM Request</Option>
                    <Option value="memoryLimit">MEM Limit</Option>
                  </Select>
                }
              />
              <Button
                icon={<ReloadOutlined />}
                onClick={() => {
                  loadPods();
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
                    {tc('actions.clearAll')}
                  </Button>
                </Space>
              </div>
            )}
          </div>

          <Table
            columns={columns}
            dataSource={pods}
            locale={{ emptyText: tc('noData') }}
            rowKey={(record) => `${record.namespace}/${record.name}`}
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
              showTotal: (total) => `${tc('table.total')} ${total} Pod`,
              onChange: (page, size) => {
                setCurrentPage(page);
                setPageSize(size || 20);
              },
              pageSizeOptions: ['10', '20', '50', '100'],
            }}
          />

          {/* 列設定抽屜 */}
          <Drawer
            title={t('list.columnSettings')}
            placement="right"
            width={400}
            open={columnSettingsVisible}
            onClose={() => setColumnSettingsVisible(false)}
            footer={
              <div style={{ textAlign: 'right' }}>
                <Space>
                  <Button onClick={() => setColumnSettingsVisible(false)}>{tc('actions.cancel')}</Button>
                  <Button type="primary" onClick={handleColumnSettingsSave}>{tc('actions.confirm')}</Button>
                </Space>
              </div>
            }
          >
            <div style={{ marginBottom: 16 }}>
              <p style={{ marginBottom: 8, color: '#666' }}>{t('list.selectColumns')}:</p>
              <Space direction="vertical" style={{ width: '100%' }}>
                {columnOptions.map(option => (
                  <Checkbox
                    key={option.key}
                    checked={visibleColumns.includes(option.key)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        setVisibleColumns([...visibleColumns, option.key]);
                      } else {
                        setVisibleColumns(visibleColumns.filter(c => c !== option.key));
                      }
                    }}
                  >
                    {option.label}
                  </Checkbox>
                ))}
              </Space>
            </div>
          </Drawer>
        </div>
      ),
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <Card bordered={false}>
        <Spin spinning={loading && allPods.length === 0}>
          <Tabs
            activeKey="pod"
            items={tabItems}
          />
        </Spin>
      </Card>
    </div>
  );
};

export default PodList;

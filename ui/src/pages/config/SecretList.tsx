import React, { useEffect, useState, useCallback } from 'react';
import {
  Table,
  Button,
  Input,
  Space,
  Tag,
  Modal,
  Select,
  Tooltip,
  Drawer,
  Checkbox,
  App,
  theme,
} from 'antd';
import {
  PlusOutlined,
  SearchOutlined,
  ReloadOutlined,
  DeleteOutlined,
  SettingOutlined,
  LockOutlined,
  EyeOutlined,
  EditOutlined,
  HistoryOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { useNavigate } from 'react-router-dom';
import { secretService, type SecretListItem, type NamespaceItem } from '../../services/configService';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { useTranslation } from 'react-i18next';
import { ActionButtons } from '../../components/ActionButtons';
import SecretForm from './SecretForm';

const { Option } = Select;

interface SecretListProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const SecretList: React.FC<SecretListProps> = ({ clusterId, onCountChange }) => {
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
  const { token } = theme.useToken();

  // 資料狀態
const { t } = useTranslation(['config', 'common']);
const [createModalOpen, setCreateModalOpen] = useState(false);
const [allSecrets, setAllSecrets] = useState<SecretListItem[]>([]);
  const [secrets, setSecrets] = useState<SecretListItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  
  // 分頁狀態
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  
  // 命名空間
  const [, setNamespaces] = useState<NamespaceItem[]>([]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  // 多條件搜尋狀態
  interface SearchCondition {
    field: 'name' | 'namespace' | 'type' | 'label';
    value: string;
  }
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'namespace' | 'type' | 'label'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // 列設定狀態
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'namespace', 'type', 'labels', 'dataCount', 'creationTimestamp', 'age'
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
      name: t('config:list.searchFields.name'),
      namespace: t('config:list.searchFields.namespace'),
      type: t('config:list.searchFields.type'),
      label: t('config:list.searchFields.label'),
    };
    return labels[field] || field;
  };

  // Secret型別顏色對映
  const getTypeColor = (type: string) => {
    const colorMap: Record<string, string> = {
      'Opaque': 'default',
      'kubernetes.io/service-account-token': 'blue',
      'kubernetes.io/dockercfg': 'green',
      'kubernetes.io/dockerconfigjson': 'green',
      'kubernetes.io/basic-auth': 'orange',
      'kubernetes.io/ssh-auth': 'purple',
      'kubernetes.io/tls': 'red',
    };
    return colorMap[type] || 'default';
  };

  // 客戶端過濾Secret列表
  const filterSecrets = useCallback((items: SecretListItem[]): SecretListItem[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(item => {
      const conditionsByField = searchConditions.reduce((acc, condition) => {
        if (!acc[condition.field]) {
          acc[condition.field] = [];
        }
        acc[condition.field].push(condition.value.toLowerCase());
        return acc;
      }, {} as Record<string, string[]>);

      return Object.entries(conditionsByField).every(([field, values]) => {
        if (field === 'label') {
          const labelsStr = Object.entries(item.labels || {})
            .map(([k, v]) => `${k}=${v}`)
            .join(' ')
            .toLowerCase();
          return values.some(searchValue => labelsStr.includes(searchValue));
        }
        
        const itemValue = item[field as keyof SecretListItem];
        const itemStr = String(itemValue || '').toLowerCase();
        return values.some(searchValue => itemStr.includes(searchValue));
      });
    });
  }, [searchConditions]);

  // 載入命名空間列表
  const loadNamespaces = useCallback(async () => {
    if (!clusterId) return;
    try {
      const data = await secretService.getSecretNamespaces(Number(clusterId));
      setNamespaces(data);
    } catch (error) {
      console.error('載入命名空間失敗:', error);
    }
  }, [clusterId]);

  // 載入Secret列表（獲取所有資料）
  const loadSecrets = useCallback(async () => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      const response = await secretService.getSecrets(Number(clusterId), {
        page: 1,
        pageSize: 10000, // 獲取所有資料
      });
      
      setAllSecrets(response.items || []);
    } catch (error) {
      console.error('獲取Secret列表失敗:', error);
      message.error(t('config:list.messages.fetchSecretError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message]);

  // 刪除Secret
  const handleDelete = async (namespace: string, name: string) => {
    if (!clusterId) return;
    try {
      await secretService.deleteSecret(Number(clusterId), namespace, name);
      message.success(t('common:messages.deleteSuccess'));
      loadSecrets();
    } catch (error) {
      console.error('刪除失敗:', error);
      message.error(t('common:messages.deleteError'));
    }
  };

  // 批次刪除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('config:list.messages.selectDeleteSecret'));
      return;
    }

    modal.confirm({
      title: t('common:messages.confirmDelete'),
      content: t('config:list.messages.confirmBatchDeleteSecret', { count: selectedRowKeys.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          for (const key of selectedRowKeys) {
            const [namespace, name] = key.split('/');
            await secretService.deleteSecret(Number(clusterId), namespace, name);
          }
          message.success(t('config:list.messages.batchDeleteSuccess'));
          setSelectedRowKeys([]);
          loadSecrets();
        } catch (error) {
          console.error('批次刪除失敗:', error);
          message.error(t('config:list.messages.batchDeleteError'));
        }
      },
    });
  };

  // 匯出功能
  const handleExport = () => {
    try {
      const filteredData = filterSecrets(allSecrets);
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
        [t('config:list.export.type')]: item.type,
        [t('config:list.export.labels')]: Object.entries(item.labels || {}).map(([k, v]) => `${k}=${v}`).join(', ') || '-',
        [t('config:list.export.dataCount')]: item.dataCount,
        [t('config:list.export.createdAt')]: item.creationTimestamp ? new Date(item.creationTimestamp).toLocaleString('zh-TW', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        }).replace(/\//g, '-') : '-',
        [t('config:list.export.age')]: item.age || '-',
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
      link.download = `secret-list-${Date.now()}.csv`;
      link.click();
      message.success(t('config:list.messages.exportSuccess', { count: sourceData.length }));
    } catch (error) {
      console.error('匯出失敗:', error);
      message.error(t('common:messages.exportError'));
    }
  };

  // 列設定儲存
  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(t('config:list.messages.columnSettingsSaved'));
  };

  // 當搜尋條件改變時重置到第一頁
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 當allSecrets、搜尋條件、分頁參數、排序參數改變時，重新計算顯示資料
  useEffect(() => {
    if (allSecrets.length === 0) {
      setSecrets([]);
      setTotal(0);
      onCountChange?.(0);
      return;
    }
    
    // 1. 應用客戶端過濾
    let filteredItems = filterSecrets(allSecrets);
    
    // 2. 應用排序
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof SecretListItem];
        const bValue = b[sortField as keyof SecretListItem];
        
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
    
    // 3. 計算分頁
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    const paginatedItems = filteredItems.slice(startIndex, endIndex);
    
    setSecrets(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allSecrets, filterSecrets, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  // 初始載入資料
  useEffect(() => {
    loadNamespaces();
    loadSecrets();
  }, [loadNamespaces, loadSecrets]);

  // 行選擇配置
  const rowSelection = {
    selectedRowKeys,
    columnWidth: 48,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

  // 定義所有可用列
  const allColumns: ColumnsType<SecretListItem> = [
    {
      title: t('common:table.name'),
      dataIndex: 'name',
      key: 'name',
      width: 250,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (text: string, record: SecretListItem) => (
        <Space>
          <LockOutlined style={{ color: '#faad14' }} />
          <Button
            type="link"
            onClick={() => navigate(`/clusters/${clusterId}/configs/secret/${record.namespace}/${text}`)}
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
        </Space>
      ),
    },
    {
      title: t('common:table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 150,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('common:table.type'),
      dataIndex: 'type',
      key: 'type',
      width: 220,
      sorter: true,
      sortOrder: sortField === 'type' ? sortOrder : null,
      render: (type: string) => (
        <Tooltip title={type}>
          <Tag color={getTypeColor(type)} style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {type}
          </Tag>
        </Tooltip>
      ),
    },
    {
      title: t('common:table.labels'),
      dataIndex: 'labels',
      key: 'labels',
      width: 250,
      render: (labels: Record<string, string>) => {
        const entries = Object.entries(labels ?? {});
        if (entries.length === 0) {
          return <span style={{ color: token.colorTextTertiary }}>—</span>;
        }
        return (
          <Space size={4} wrap>
            {entries.slice(0, 2).map(([k, v]) => (
              <Tag key={k}>{k}={v}</Tag>
            ))}
            {entries.length > 2 && (
              <Tooltip title={entries.slice(2).map(([k, v]) => `${k}=${v}`).join('\n')}>
                <Tag>+{entries.length - 2}</Tag>
              </Tooltip>
            )}
          </Space>
        );
      },
    },
    {
      title: t('config:list.columns.dataCount'),
      dataIndex: 'dataCount',
      key: 'dataCount',
      width: 120,
      align: 'center',
      sorter: true,
      sortOrder: sortField === 'dataCount' ? sortOrder : null,
      render: (count: number) => (
        <span style={{ color: token.colorTextSecondary }}>{count}</span>
      ),
    },
    {
      title: t('common:table.createdAt'),
      dataIndex: 'creationTimestamp',
      key: 'creationTimestamp',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'creationTimestamp' ? sortOrder : null,
      render: (time: string) => {
        if (!time) return '-';
        const date = new Date(time);
        return date.toLocaleString('zh-TW', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        }).replace(/\//g, '-');
      },
    },
    {
      title: t('config:list.columns.age'),
      dataIndex: 'creationTimestamp',
      key: 'age',
      width: 100,
      render: (createdAt: string) => {
        if (!createdAt) return '-';
        const diff = dayjs().diff(dayjs(createdAt), 'minute');
        if (diff < 60) return `${diff}m`;
        if (diff < 1440) return `${Math.floor(diff / 60)}h`;
        return `${Math.floor(diff / 1440)}d`;
      },
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 90,
      fixed: 'right' as const,
      render: (_: unknown, record: SecretListItem) => (
        <ActionButtons
          primary={[
            {
              key: 'view',
              label: t('common:actions.view'),
              icon: <EyeOutlined />,
              onClick: () => navigate(`/clusters/${clusterId}/configs/secret/${record.namespace}/${record.name}`),
            },
            {
              key: 'edit',
              label: t('common:actions.edit'),
              icon: <EditOutlined />,
              onClick: () => navigate(`/clusters/${clusterId}/configs/secret/${record.namespace}/${record.name}/edit`),
            },
          ]}
          more={[
            {
              key: 'history',
              label: t('config:list.columns.history'),
              icon: <HistoryOutlined />,
              onClick: () => navigate(`/clusters/${clusterId}/configs/secret/${record.namespace}/${record.name}/history`),
            },
            {
              key: 'delete',
              label: t('common:actions.delete'),
              icon: <DeleteOutlined />,
              danger: true,
              confirm: {
                title: t('config:list.messages.confirmDeleteSecret'),
                description: t('config:list.messages.confirmDeleteDesc', { name: record.name }),
              },
              onClick: () => handleDelete(record.namespace, record.name),
            },
          ]}
        />
      ),
    },
  ];

  // 根據可見性過濾列
  const columns = allColumns.filter(col => {
    if (col.key === 'actions') return true;
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序處理
  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<SecretListItem> | SorterResult<SecretListItem>[]
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
            danger
            icon={<DeleteOutlined />}
            onClick={handleBatchDelete}
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
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setCreateModalOpen(true)}
        >
          {t('config:list.createSecret')}
        </Button>
      </div>

      {/* 多條件搜尋欄 */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
          <Input
            prefix={<SearchOutlined />}
            placeholder={t('common:search.placeholder')}
            style={{ flex: 1 }}
            value={currentSearchValue}
            onChange={(e) => setCurrentSearchValue(e.target.value)}
            onPressEnter={addSearchCondition}
            allowClear
            addonBefore={
              <Select 
                value={currentSearchField} 
                onChange={setCurrentSearchField} 
                style={{ width: 120 }}
              >
                <Option value="name">{t('config:list.searchFields.name')}</Option>
                <Option value="namespace">{t('config:list.searchFields.namespace')}</Option>
                <Option value="type">{t('config:list.searchFields.type')}</Option>
                <Option value="label">{t('config:list.searchFields.label')}</Option>
              </Select>
            }
          />
          <Button
            icon={<ReloadOutlined />}
            onClick={() => {
              loadSecrets();
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
        dataSource={secrets}
        rowKey={(record) => `${record.namespace}/${record.name}`}
        rowSelection={rowSelection}
        loading={loading}
        virtual
        scroll={{ x: 'max-content', y: 600 }}
        size="middle"
        onChange={handleTableChange}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t('config:list.pagination.totalSecret', { total }),
          onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />

      {/* 建立 Secret Modal */}
      <SecretForm
        open={createModalOpen}
        clusterId={clusterId}
        onClose={() => setCreateModalOpen(false)}
        onSuccess={() => { setCreateModalOpen(false); loadSecrets(); }}
      />

      {/* 列設定抽屜 */}
      <Drawer
        title={t('common:search.columnSettings')}
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
          <p style={{ marginBottom: 8, color: '#666' }}>{t('common:search.selectColumns')}</p>
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
              {t('config:list.columnSettings.name')}
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
              {t('config:list.columnSettings.namespace')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('type')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'type']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'type'));
                }
              }}
            >
              {t('config:list.columnSettings.type')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('labels')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'labels']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'labels'));
                }
              }}
            >
              {t('config:list.columnSettings.labels')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('dataCount')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'dataCount']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'dataCount'));
                }
              }}
            >
              {t('config:list.columnSettings.dataCount')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('creationTimestamp')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'creationTimestamp']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'creationTimestamp'));
                }
              }}
            >
              {t('config:list.columnSettings.createdAt')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('age')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'age']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'age'));
                }
              }}
            >
              {t('config:list.columnSettings.age')}
            </Checkbox>
          </Space>
        </div>
      </Drawer>
    </div>
  );
};

export default SecretList;

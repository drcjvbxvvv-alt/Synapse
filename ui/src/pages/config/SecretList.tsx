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
  Popconfirm,
  Drawer,
  Checkbox,
  App,
} from 'antd';
import {
  PlusOutlined,
  SearchOutlined,
  ReloadOutlined,
  DeleteOutlined,
  SettingOutlined,
  LockOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { secretService, type SecretListItem, type NamespaceItem } from '../../services/configService';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import { useTranslation } from 'react-i18next';

const { Option } = Select;

interface SecretListProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const SecretList: React.FC<SecretListProps> = ({ clusterId, onCountChange }) => {
  const navigate = useNavigate();
  const { message } = App.useApp();
  
  // 数据状态
const { t } = useTranslation(['config', 'common']);
const [allSecrets, setAllSecrets] = useState<SecretListItem[]>([]);
  const [secrets, setSecrets] = useState<SecretListItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  
  // 分页状态
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  
  // 命名空间
  const [, setNamespaces] = useState<NamespaceItem[]>([]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  // 多条件搜索状态
  interface SearchCondition {
    field: 'name' | 'namespace' | 'type' | 'label';
    value: string;
  }
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'namespace' | 'type' | 'label'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // 列设置状态
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'namespace', 'type', 'labels', 'dataCount', 'creationTimestamp', 'age'
  ]);

  // 排序状态
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);

  // 添加搜索条件
  const addSearchCondition = () => {
    if (!currentSearchValue.trim()) return;
    
    const newCondition: SearchCondition = {
      field: currentSearchField,
      value: currentSearchValue.trim(),
    };
    
    setSearchConditions([...searchConditions, newCondition]);
    setCurrentSearchValue('');
  };

  // 删除搜索条件
  const removeSearchCondition = (index: number) => {
    setSearchConditions(searchConditions.filter((_, i) => i !== index));
  };

  // 清空所有搜索条件
  const clearAllConditions = () => {
    setSearchConditions([]);
    setCurrentSearchValue('');
  };

  // 获取搜索字段的显示名称
  const getFieldLabel = (field: string): string => {
    const labels: Record<string, string> = {
      name: t('config:list.searchFields.name'),
      namespace: t('config:list.searchFields.namespace'),
      type: t('config:list.searchFields.type'),
      label: t('config:list.searchFields.label'),
    };
    return labels[field] || field;
  };

  // Secret类型颜色映射
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

  // 客户端过滤Secret列表
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

  // 加载命名空间列表
  const loadNamespaces = useCallback(async () => {
    if (!clusterId) return;
    try {
      const data = await secretService.getSecretNamespaces(Number(clusterId));
      setNamespaces(data);
    } catch (error) {
      console.error('加载命名空间失败:', error);
    }
  }, [clusterId]);

  // 加载Secret列表（获取所有数据）
  const loadSecrets = useCallback(async () => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      const response = await secretService.getSecrets(Number(clusterId), {
        page: 1,
        pageSize: 10000, // 获取所有数据
      });
      
      setAllSecrets(response.items || []);
    } catch (error) {
      console.error('获取Secret列表失败:', error);
      message.error(t('config:list.messages.fetchSecretError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message]);

  // 删除Secret
  const handleDelete = async (namespace: string, name: string) => {
    if (!clusterId) return;
    try {
      await secretService.deleteSecret(Number(clusterId), namespace, name);
      message.success(t('common:messages.deleteSuccess'));
      loadSecrets();
    } catch (error) {
      console.error('删除失败:', error);
      message.error(t('common:messages.deleteError'));
    }
  };

  // 批量删除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('config:list.messages.selectDeleteSecret'));
      return;
    }

    Modal.confirm({
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
          console.error('批量删除失败:', error);
          message.error(t('config:list.messages.batchDeleteError'));
        }
      },
    });
  };

  // 导出功能
  const handleExport = () => {
    try {
      const filteredData = filterSecrets(allSecrets);
      
      if (filteredData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }

      const dataToExport = filteredData.map(item => ({
        [t('config:list.export.name')]: item.name,
        [t('config:list.export.namespace')]: item.namespace,
        [t('config:list.export.type')]: item.type,
        [t('config:list.export.labels')]: Object.entries(item.labels || {}).map(([k, v]) => `${k}=${v}`).join(', ') || '-',
        [t('config:list.export.dataCount')]: item.dataCount,
        [t('config:list.export.createdAt')]: item.creationTimestamp ? new Date(item.creationTimestamp).toLocaleString('zh-CN', {
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
      message.success(t('config:list.messages.exportSuccess', { count: filteredData.length }));
    } catch (error) {
      console.error('导出失败:', error);
      message.error(t('common:messages.exportError'));
    }
  };

  // 列设置保存
  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(t('config:list.messages.columnSettingsSaved'));
  };

  // 当搜索条件改变时重置到第一页
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 当allSecrets、搜索条件、分页参数、排序参数改变时，重新计算显示数据
  useEffect(() => {
    if (allSecrets.length === 0) {
      setSecrets([]);
      setTotal(0);
      onCountChange?.(0);
      return;
    }
    
    // 1. 应用客户端过滤
    let filteredItems = filterSecrets(allSecrets);
    
    // 2. 应用排序
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
    
    // 3. 计算分页
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    const paginatedItems = filteredItems.slice(startIndex, endIndex);
    
    setSecrets(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allSecrets, filterSecrets, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  // 初始加载数据
  useEffect(() => {
    loadNamespaces();
    loadSecrets();
  }, [loadNamespaces, loadSecrets]);

  // 行选择配置
  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

  // 定义所有可用列
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
      render: (labels: Record<string, string>) => (
        <Space size={[0, 4]} wrap>
          {Object.entries(labels || {}).slice(0, 3).map(([key, value]) => (
            <Tooltip key={key} title={`${key}=${value}`}>
              <Tag style={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {`${key}=${value}`}
              </Tag>
            </Tooltip>
          ))}
          {Object.keys(labels || {}).length > 3 && (
            <Tooltip title={Object.entries(labels).slice(3).map(([k, v]) => `${k}=${v}`).join('\n')}>
              <Tag>+{Object.keys(labels).length - 3}</Tag>
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: t('config:list.columns.dataCount'),
      dataIndex: 'dataCount',
      key: 'dataCount',
      width: 120,
      align: 'center',
      sorter: true,
      sortOrder: sortField === 'dataCount' ? sortOrder : null,
      render: (count: number) => <Tag color="green">{count}</Tag>,
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
        return date.toLocaleString('zh-CN', {
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
      dataIndex: 'age',
      key: 'age',
      width: 100,
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 150,
      fixed: 'right' as const,
      render: (_: unknown, record: SecretListItem) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            onClick={() => navigate(`/clusters/${clusterId}/configs/secret/${record.namespace}/${record.name}`)}
          >
            {t('common:actions.view')}
          </Button>
          <Button
            type="link"
            size="small"
            onClick={() => navigate(`/clusters/${clusterId}/configs/secret/${record.namespace}/${record.name}/edit`)}
          >
            {t('common:actions.edit')}
          </Button>
          <Popconfirm
            title={t('config:list.messages.confirmDeleteSecret')}
            description={t('config:list.messages.confirmDeleteDesc', { name: record.name })}
            onConfirm={() => handleDelete(record.namespace, record.name)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            <Button
              type="link"
              size="small"
              danger
            >
              {t('common:actions.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // 根据可见性过滤列
  const columns = allColumns.filter(col => {
    if (col.key === 'actions') return true;
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序处理
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
      {/* 操作按钮栏 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Space>
          <Button
            disabled={selectedRowKeys.length === 0}
            danger
            icon={<DeleteOutlined />}
            onClick={handleBatchDelete}
          >
            {t('common:actions.batchDelete')} {selectedRowKeys.length > 0 && `(${selectedRowKeys.length})`}
          </Button>
          <Button onClick={handleExport}>
            {t('common:actions.export')}
          </Button>
        </Space>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate(`/clusters/${clusterId}/configs/secret/create`)}
        >
          {t('config:list.createSecret')}
        </Button>
      </div>

      {/* 多条件搜索栏 */}
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

        {/* 搜索条件标签 */}
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
        scroll={{ x: 1300 }}
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

      {/* 列设置抽屉 */}
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

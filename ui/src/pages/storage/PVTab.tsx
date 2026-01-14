import React, { useState, useEffect, useCallback } from 'react';
import {
  Table,
  Button,
  Space,
  Tag,
  Input,
  Select,
  Popconfirm,
  Typography,
  Tooltip,
  Modal,
  App,
  Drawer,
  Checkbox,
} from 'antd';
import {
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { StorageService } from '../../services/storageService';
import type { PV } from '../../types';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';

const { Link } = Typography;

interface PVTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const PVTab: React.FC<PVTabProps> = ({ clusterId, onCountChange }) => {
  const { message } = App.useApp();
  
  // 数据状态
  const [allPVs, setAllPVs] = useState<PV[]>([]);
  const [pvs, setPVs] = useState<PV[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  
  // 分页状态
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  
  // 选择行状态
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  
  // 多条件搜索状态
  interface SearchCondition {
    field: 'name' | 'status' | 'storageClassName' | 'persistentVolumeSource' | 'reclaimPolicy';
    value: string;
  }
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'status' | 'storageClassName' | 'persistentVolumeSource' | 'reclaimPolicy'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // 列设置状态
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'status', 'capacity', 'accessModes', 'reclaimPolicy', 'storageClassName', 'claimRef', 'persistentVolumeSource', 'createdAt'
  ]);
  
  // 排序状态
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);
  
  // YAML查看Modal
  const [yamlModalVisible, setYamlModalVisible] = useState(false);
  const [currentYaml, setCurrentYaml] = useState('');
  const [yamlLoading, setYamlLoading] = useState(false);

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
      name: 'PV名称',
      status: '状态',
      storageClassName: '存储类',
      persistentVolumeSource: '存储源类型',
      reclaimPolicy: '回收策略',
    };
    return labels[field] || field;
  };

  // 客户端过滤PV列表
  const filterPVs = useCallback((items: PV[]): PV[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(pv => {
      const conditionsByField = searchConditions.reduce((acc, condition) => {
        if (!acc[condition.field]) {
          acc[condition.field] = [];
        }
        acc[condition.field].push(condition.value.toLowerCase());
        return acc;
      }, {} as Record<string, string[]>);

      return Object.entries(conditionsByField).every(([field, values]) => {
        const pvValue = pv[field as keyof PV];
        const itemStr = String(pvValue || '').toLowerCase();
        return values.some(searchValue => itemStr.includes(searchValue));
      });
    });
  }, [searchConditions]);

  // 获取PV列表
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
      
      if (response.code === 200) {
        const items = response.data.items || [];
        setAllPVs(items);
      } else {
        message.error(response.message || '获取PV列表失败');
      }
    } catch (error) {
      console.error('获取PV列表失败:', error);
      message.error('获取PV列表失败');
    } finally {
      setLoading(false);
    }
  }, [clusterId, message]);

  // 当搜索条件改变时重置到第一页
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 当allPVs、搜索条件、分页参数、排序参数改变时，重新计算显示数据
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

  // 初始加载数据
  useEffect(() => {
    loadPVs();
  }, [loadPVs]);

  // 查看YAML
  const handleViewYAML = async (pv: PV) => {
    setYamlModalVisible(true);
    setYamlLoading(true);
    try {
      const response = await StorageService.getPVYAML(
        clusterId,
        pv.name
      );
      
      if (response.code === 200) {
        setCurrentYaml(response.data.yaml);
      } else {
        message.error(response.message || '获取YAML失败');
      }
    } catch (error) {
      console.error('获取YAML失败:', error);
      message.error('获取YAML失败');
    } finally {
      setYamlLoading(false);
    }
  };

  // 删除PV
  const handleDelete = async (pv: PV) => {
    try {
      const response = await StorageService.deletePV(
        clusterId,
        pv.name
      );
      
      if (response.code === 200) {
        message.success('删除成功');
        loadPVs();
      } else {
        message.error(response.message || '删除失败');
      }
    } catch (error) {
      console.error('删除失败:', error);
      message.error('删除失败');
    }
  };

  // 批量删除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请先选择要删除的PV');
      return;
    }

    Modal.confirm({
      title: '确认删除',
      content: `确定要删除选中的 ${selectedRowKeys.length} 个PV吗？`,
      okText: '确定',
      cancelText: '取消',
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
            message.success(`成功删除 ${successCount} 个PV`);
          } else {
            message.warning(`删除完成：成功 ${successCount} 个，失败 ${failCount} 个`);
          }
          
          setSelectedRowKeys([]);
          loadPVs();
        } catch (error) {
          console.error('批量删除失败:', error);
          message.error('批量删除失败');
        }
      }
    });
  };

  // 导出功能
  const handleExport = () => {
    try {
      const filteredData = filterPVs(allPVs);
      
      if (filteredData.length === 0) {
        message.warning('没有数据可导出');
        return;
      }

      const dataToExport = filteredData.map(p => ({
        'PV名称': p.name,
        '状态': p.status,
        '容量': p.capacity || '-',
        '访问模式': StorageService.formatAccessModes(p.accessModes),
        '回收策略': p.reclaimPolicy || '-',
        '存储类': p.storageClassName || '-',
        '声明': StorageService.formatClaimRef(p.claimRef),
        '存储源类型': p.persistentVolumeSource || '-',
        '创建时间': p.createdAt ? new Date(p.createdAt).toLocaleString('zh-CN') : '-',
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
      message.success(`成功导出 ${filteredData.length} 条数据`);
    } catch (error) {
      console.error('导出失败:', error);
      message.error('导出失败');
    }
  };

  // 列设置保存
  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success('列设置已保存');
  };

  // 行选择配置
  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

  // 定义所有可用列
  const allColumns: ColumnsType<PV> = [
    {
      title: 'PV名称',
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
      title: '状态',
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
      title: '容量',
      dataIndex: 'capacity',
      key: 'capacity',
      width: 100,
      render: (capacity: string) => StorageService.formatCapacity(capacity),
    },
    {
      title: '访问模式',
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
      title: '回收策略',
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
      title: '存储类',
      dataIndex: 'storageClassName',
      key: 'storageClassName',
      width: 150,
      render: (name: string) => name ? <Tag>{name}</Tag> : '-',
    },
    {
      title: '声明',
      dataIndex: 'claimRef',
      key: 'claimRef',
      width: 200,
      render: (claimRef: { namespace: string; name: string }) => 
        StorageService.formatClaimRef(claimRef),
    },
    {
      title: '存储源类型',
      dataIndex: 'persistentVolumeSource',
      key: 'persistentVolumeSource',
      width: 120,
      render: (source: string) => <Tag color="purple">{source}</Tag>,
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (createdAt: string) => {
        if (!createdAt) return '-';
        const date = new Date(createdAt);
        return date.toLocaleString('zh-CN');
      },
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right' as const,
      width: 120,
      render: (_: unknown, record: PV) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            onClick={() => handleViewYAML(record)}
          >
            YAML
          </Button>
          <Popconfirm
            title="确定要删除这个PV吗？"
            description={`确定要删除 ${record.name} 吗？`}
            onConfirm={() => handleDelete(record)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              size="small"
              danger
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // 根据可见性过滤列
  const columns = allColumns.filter(col => {
    if (col.key === 'action') return true;
    if (col.key === 'name') return true;
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序处理
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
      {/* 操作按钮栏 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Space>
          <Button
            disabled={selectedRowKeys.length === 0}
            onClick={handleBatchDelete}
            danger
          >
            批量删除
          </Button>
          <Button onClick={handleExport}>
            导出
          </Button>
        </Space>
      </div>

      {/* 多条件搜索栏 */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
          <Input
            prefix={<SearchOutlined />}
            placeholder="选择属性筛选，或输入关键字搜索"
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
                <Select.Option value="name">PV名称</Select.Option>
                <Select.Option value="status">状态</Select.Option>
                <Select.Option value="storageClassName">存储类</Select.Option>
                <Select.Option value="persistentVolumeSource">存储源类型</Select.Option>
                <Select.Option value="reclaimPolicy">回收策略</Select.Option>
              </Select>
            }
          />
          <Button
            icon={<ReloadOutlined />}
            onClick={() => loadPVs()}
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
                清空全部
              </Button>
            </Space>
          </div>
        )}
      </div>

      <Table
        columns={columns}
        dataSource={pvs}
        rowKey={(record) => record.name}
        rowSelection={rowSelection}
        loading={loading}
        scroll={{ x: 1400 }}
        size="middle"
        onChange={handleTableChange}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 个PV`,
          onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />

      {/* YAML查看Modal */}
      <Modal
        title="PV YAML"
        open={yamlModalVisible}
        onCancel={() => setYamlModalVisible(false)}
        footer={null}
        width={800}
      >
        {yamlLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <span>加载中...</span>
          </div>
        ) : (
          <pre style={{ maxHeight: 600, overflow: 'auto', background: '#f5f5f5', padding: 16 }}>
            {currentYaml}
          </pre>
        )}
      </Modal>

      {/* 列设置抽屉 */}
      <Drawer
        title="列设置"
        placement="right"
        width={400}
        open={columnSettingsVisible}
        onClose={() => setColumnSettingsVisible(false)}
        footer={
          <div style={{ textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setColumnSettingsVisible(false)}>取消</Button>
              <Button type="primary" onClick={handleColumnSettingsSave}>确定</Button>
            </Space>
          </div>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8, color: '#666' }}>选择要显示的列：</p>
          <Space direction="vertical" style={{ width: '100%' }}>
            {[
              { key: 'status', label: '状态' },
              { key: 'capacity', label: '容量' },
              { key: 'accessModes', label: '访问模式' },
              { key: 'reclaimPolicy', label: '回收策略' },
              { key: 'storageClassName', label: '存储类' },
              { key: 'claimRef', label: '声明' },
              { key: 'persistentVolumeSource', label: '存储源类型' },
              { key: 'createdAt', label: '创建时间' },
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

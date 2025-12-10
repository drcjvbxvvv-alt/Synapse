import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Table,
  Button,
  Space,
  Tag,
  Select,
  Input,
  Modal,
  Tooltip,
  Form,
  App,
  Popconfirm,
  Checkbox,
  Drawer,
  Card,
  Badge,
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  SearchOutlined,
  DeleteOutlined,
  EyeOutlined,
  TagsOutlined,
} from '@ant-design/icons';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import {
  getNamespaces,
  createNamespace,
  deleteNamespace,
  type NamespaceData,
  type CreateNamespaceRequest,
} from '../../services/namespaceService';

const { Option } = Select;

const NamespaceList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  const { message } = App.useApp();
  const [form] = Form.useForm();

  // 数据状态
  const [allNamespaces, setAllNamespaces] = useState<NamespaceData[]>([]); // 所有原始数据
  const [namespaces, setNamespaces] = useState<NamespaceData[]>([]); // 当前页显示的数据
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);

  // 分页状态
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // 操作状态
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  // 多条件搜索状态
  interface SearchCondition {
    field: 'name' | 'status' | 'label';
    value: string;
  }
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'status' | 'label'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // 列设置状态
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'status', 'labels', 'creationTimestamp'
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
      name: '名称',
      status: '状态',
      label: '标签',
    };
    return labels[field] || field;
  };

  // 客户端过滤命名空间列表
  const filterNamespaces = useCallback((items: NamespaceData[]): NamespaceData[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(namespace => {
      // 按字段分组条件
      const conditionsByField = searchConditions.reduce((acc, condition) => {
        if (!acc[condition.field]) {
          acc[condition.field] = [];
        }
        acc[condition.field].push(condition.value.toLowerCase());
        return acc;
      }, {} as Record<string, string[]>);

      // 不同字段之间是 AND 关系
      // 相同字段之间是 OR 关系
      return Object.entries(conditionsByField).every(([field, values]) => {
        if (field === 'label') {
          // 对于标签字段，检查任意标签key或value是否匹配
          const labels = namespace.labels || {};
          const labelStr = Object.entries(labels)
            .map(([k, v]) => `${k}:${v}`)
            .join(' ')
            .toLowerCase();
          return values.some(searchValue => labelStr.includes(searchValue));
        }

        const namespaceValue = namespace[field as keyof NamespaceData];
        const itemStr = String(namespaceValue || '').toLowerCase();
        return values.some(searchValue => itemStr.includes(searchValue));
      });
    });
  }, [searchConditions]);

  // 获取命名空间列表
  const loadNamespaces = useCallback(async () => {
    if (!clusterId) return;

    setLoading(true);
    try {
      const data = await getNamespaces(Number(clusterId));
      setAllNamespaces(data);
    } catch (error) {
      console.error('获取命名空间列表失败:', error);
      message.error('获取命名空间列表失败');
    } finally {
      setLoading(false);
    }
  }, [clusterId, message]);

  // 创建命名空间
  const handleCreate = async (values: CreateNamespaceRequest) => {
    if (!clusterId) return;
    try {
      await createNamespace(Number(clusterId), values);
      message.success('命名空间创建成功');
      setCreateModalVisible(false);
      form.resetFields();
      loadNamespaces();
    } catch (error) {
      message.error('创建命名空间失败');
      console.error('Error creating namespace:', error);
    }
  };

  // 删除命名空间
  const handleDelete = async (namespace: string) => {
    if (!clusterId) return;
    try {
      await deleteNamespace(Number(clusterId), namespace);
      message.success('命名空间删除成功');
      loadNamespaces();
    } catch (error) {
      message.error('删除命名空间失败');
      console.error('Error deleting namespace:', error);
    }
  };

  // 批量删除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请先选择要删除的命名空间');
      return;
    }

    // 过滤掉系统命名空间
    const systemNamespaces = ['default', 'kube-system', 'kube-public', 'kube-node-lease'];
    const toDelete = selectedRowKeys.filter(ns => !systemNamespaces.includes(ns));

    if (toDelete.length === 0) {
      message.warning('不能删除系统命名空间');
      return;
    }

    Modal.confirm({
      title: '确认批量删除',
      content: `确定要删除选中的 ${toDelete.length} 个命名空间吗？此操作将删除这些命名空间下的所有资源。`,
      okText: '确定',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const deletePromises = toDelete.map(ns => deleteNamespace(Number(clusterId), ns));
          const results = await Promise.allSettled(deletePromises);
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;

          if (failCount === 0) {
            message.success(`成功删除 ${successCount} 个命名空间`);
          } else {
            message.warning(`删除完成：成功 ${successCount} 个，失败 ${failCount} 个`);
          }

          setSelectedRowKeys([]);
          loadNamespaces();
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
      // 获取所有筛选后的数据
      const filteredData = filterNamespaces(allNamespaces);

      if (filteredData.length === 0) {
        message.warning('没有数据可导出');
        return;
      }

      // 导出筛选后的所有数据
      const dataToExport = filteredData.map(ns => ({
        '命名空间': ns.name,
        '状态': ns.status === 'Active' ? '运行中' : ns.status,
        '标签': ns.labels ? Object.entries(ns.labels).map(([k, v]) => `${k}=${v}`).join('; ') : '-',
        '创建时间': ns.creationTimestamp ? new Date(ns.creationTimestamp).toLocaleString('zh-CN', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        }).replace(/\//g, '-') : '-',
      }));

      // 导出为CSV
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
      link.download = `namespace-list-${Date.now()}.csv`;
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

  // 查看详情
  const handleViewDetail = (namespace: string) => {
    navigate(`/clusters/${clusterId}/namespaces/${namespace}`);
  };

  // 当搜索条件改变时重置到第一页
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 当allNamespaces、搜索条件、分页参数、排序参数改变时，重新计算显示数据
  useEffect(() => {
    if (allNamespaces.length === 0) {
      setNamespaces([]);
      setTotal(0);
      return;
    }

    // 1. 应用客户端过滤
    let filteredItems = filterNamespaces(allNamespaces);

    // 2. 应用排序
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof NamespaceData];
        const bValue = b[sortField as keyof NamespaceData];

        // 处理 undefined 值
        if (aValue === undefined && bValue === undefined) return 0;
        if (aValue === undefined) return sortOrder === 'ascend' ? 1 : -1;
        if (bValue === undefined) return sortOrder === 'ascend' ? -1 : 1;

        // 字符串类型比较
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

    setNamespaces(paginatedItems);
    setTotal(filteredItems.length);
  }, [allNamespaces, filterNamespaces, currentPage, pageSize, sortField, sortOrder]);

  // 初始加载数据
  useEffect(() => {
    loadNamespaces();
  }, [loadNamespaces]);

  // 行选择配置
  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

  // 定义所有可用列
  const allColumns: ColumnsType<NamespaceData> = [
    {
      title: '命名空间',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      fixed: 'left' as const,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (name: string) => (
        <Button
          type="link"
          onClick={() => handleViewDetail(name)}
          style={{
            padding: 0,
            height: 'auto',
            whiteSpace: 'normal',
            wordBreak: 'break-all',
            textAlign: 'left'
          }}
        >
          {name}
        </Button>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      sorter: true,
      sortOrder: sortField === 'status' ? sortOrder : null,
      render: (status: string) => {
        const isActive = status === 'Active';
        return (
          <Badge
            status={isActive ? 'success' : 'warning'}
            text={isActive ? '运行中' : status}
          />
        );
      },
    },
    {
      title: '标签',
      dataIndex: 'labels',
      key: 'labels',
      width: 250,
      render: (labels: Record<string, string>) => {
        if (!labels || Object.keys(labels).length === 0) {
          return <span style={{ color: '#999' }}>--</span>;
        }
        const labelArray = Object.entries(labels).slice(0, 2);
        const moreCount = Object.keys(labels).length - 2;
        return (
          <Space size={[0, 4]} wrap>
            {labelArray.map(([key, value]) => (
              <Tooltip key={key} title={`${key}: ${value}`}>
                <Tag icon={<TagsOutlined />}>{key}</Tag>
              </Tooltip>
            ))}
            {moreCount > 0 && (
              <Tooltip title={`还有 ${moreCount} 个标签`}>
                <Tag>+{moreCount}</Tag>
              </Tooltip>
            )}
          </Space>
        );
      },
    },
    {
      title: '创建时间',
      dataIndex: 'creationTimestamp',
      key: 'creationTimestamp',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'creationTimestamp' ? sortOrder : null,
      render: (text: string) => {
        if (!text) return '-';
        const date = new Date(text);
        const formatted = date.toLocaleString('zh-CN', {
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
      title: '操作',
      key: 'actions',
      width: 150,
      fixed: 'right' as const,
      render: (record: NamespaceData) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record.name)}
          >
            详情
          </Button>
          {!['default', 'kube-system', 'kube-public', 'kube-node-lease'].includes(record.name) && (
            <Popconfirm
              title="确认删除"
              description={`确定要删除命名空间 "${record.name}" 吗？此操作将删除该命名空间下的所有资源。`}
              onConfirm={() => handleDelete(record.name)}
              okText="确定"
              cancelText="取消"
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
              >
                删除
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  // 根据可见性过滤列
  const columns = allColumns.filter(col => {
    if (col.key === 'actions') return true; // 操作列始终显示
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序处理
  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<NamespaceData> | SorterResult<NamespaceData>[]
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
    <div style={{ padding: '24px' }}>
      <Card bordered={false}>
        {/* 操作按钮栏 */}
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <Space>
            <Button
              danger
              disabled={selectedRowKeys.length === 0}
              onClick={handleBatchDelete}
            >
              批量删除
            </Button>
            <Button onClick={handleExport}>
              导出
            </Button>
          </Space>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalVisible(true)}
          >
            创建命名空间
          </Button>
        </div>

        {/* 多条件搜索栏 */}
        <div style={{ marginBottom: 16 }}>
          {/* 搜索输入框 */}
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
                  style={{ width: 100 }}
                >
                  <Option value="name">名称</Option>
                  <Option value="status">状态</Option>
                  <Option value="label">标签</Option>
                </Select>
              }
            />
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                loadNamespaces();
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
                  清空全部
                </Button>
              </Space>
            </div>
          )}
        </div>

        <Table
          columns={columns}
          dataSource={namespaces}
          rowKey="name"
          rowSelection={rowSelection}
          loading={loading}
          scroll={{ x: 900 }}
          size="middle"
          onChange={handleTableChange}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 个命名空间`,
            onChange: (page, size) => {
              setCurrentPage(page);
              setPageSize(size || 20);
            },
            pageSizeOptions: ['10', '20', '50', '100'],
          }}
        />
      </Card>

      {/* 创建命名空间模态框 */}
      <Modal
        title="创建命名空间"
        open={createModalVisible}
        onCancel={() => {
          setCreateModalVisible(false);
          form.resetFields();
        }}
        onOk={() => form.submit()}
        okText="确定"
        cancelText="取消"
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleCreate}
          autoComplete="off"
        >
          <Form.Item
            name="name"
            label="命名空间名称"
            rules={[
              { required: true, message: '请输入命名空间名称' },
              {
                pattern: /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/,
                message: '名称只能包含小写字母、数字和连字符，且必须以字母或数字开头和结尾',
              },
            ]}
          >
            <Input placeholder="例如: my-namespace" />
          </Form.Item>

          <Form.Item
            name={['labels', 'description']}
            label="描述（可选）"
          >
            <Input.TextArea
              rows={3}
              placeholder="输入命名空间的描述信息"
            />
          </Form.Item>
        </Form>
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
              命名空间
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
              状态
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
              标签
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
              创建时间
            </Checkbox>
          </Space>
        </div>
      </Drawer>
    </div>
  );
};

export default NamespaceList;

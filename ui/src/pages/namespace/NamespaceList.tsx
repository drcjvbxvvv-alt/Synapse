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
import { useTranslation } from 'react-i18next';
const { Option } = Select;

const NamespaceList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
const { t } = useTranslation(["namespace", "common"]);
const [form] = Form.useForm();

  // 資料狀態
  const [allNamespaces, setAllNamespaces] = useState<NamespaceData[]>([]); // 所有原始資料
  const [namespaces, setNamespaces] = useState<NamespaceData[]>([]); // 當前頁顯示的資料
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);

  // 分頁狀態
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // 操作狀態
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  // 多條件搜尋狀態
  interface SearchCondition {
    field: 'name' | 'status' | 'label';
    value: string;
  }
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'status' | 'label'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // 列設定狀態
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'status', 'labels', 'creationTimestamp'
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
      name: t('list.fieldName'),
      status: t('list.fieldStatus'),
      label: t('list.fieldLabel'),
    };
    return labels[field] || field;
  };

  // 客戶端過濾命名空間列表
  const filterNamespaces = useCallback((items: NamespaceData[]): NamespaceData[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(namespace => {
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
        if (field === 'label') {
          // 對於標籤欄位，檢查任意標籤key或value是否匹配
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

  // 獲取命名空間列表
  const loadNamespaces = useCallback(async () => {
    if (!clusterId) return;

    setLoading(true);
    try {
      const data = await getNamespaces(Number(clusterId));
      setAllNamespaces(data);
    } catch (error) {
      console.error('獲取命名空間列表失敗:', error);
      message.error(t('list.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message]);

  // 建立命名空間
  const handleCreate = async (values: CreateNamespaceRequest) => {
    if (!clusterId) return;
    try {
      await createNamespace(Number(clusterId), values);
      message.success(t('messages.createSuccess'));
      setCreateModalVisible(false);
      form.resetFields();
      loadNamespaces();
    } catch (error) {
      message.error(t('messages.createError'));
      console.error('Error creating namespace:', error);
    }
  };

  // 刪除命名空間
  const handleDelete = async (namespace: string) => {
    if (!clusterId) return;
    try {
      await deleteNamespace(Number(clusterId), namespace);
      message.success(t('messages.deleteSuccess'));
      loadNamespaces();
    } catch (error) {
      message.error(t('messages.deleteError'));
      console.error('Error deleting namespace:', error);
    }
  };

  // 批次刪除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('common:messages.selectFirst'));
      return;
    }

    // 過濾掉系統命名空間
    const systemNamespaces = ['default', 'kube-system', 'kube-public', 'kube-node-lease'];
    const toDelete = selectedRowKeys.filter(ns => !systemNamespaces.includes(ns));

    if (toDelete.length === 0) {
      message.warning(t('common:messages.cannotDeleteSystem'));
      return;
    }

    modal.confirm({
      title: t('actions.confirmBatchDelete'),
      content: t('actions.confirmBatchDeleteDesc', { count: toDelete.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const deletePromises = toDelete.map(ns => deleteNamespace(Number(clusterId), ns));
          const results = await Promise.allSettled(deletePromises);
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;

          if (failCount === 0) {
            message.success(t('common:messages.batchDeleteSuccess', { count: successCount }));
          } else {
            message.warning(t('common:messages.batchDeletePartial', { success: successCount, fail: failCount }));
          }

          setSelectedRowKeys([]);
          loadNamespaces();
        } catch (error) {
          console.error('批次刪除失敗:', error);
          message.error(t('messages.batchDeleteError'));
        }
      }
    });
  };

  // 匯出功能
  const handleExport = () => {
    try {
      // 獲取所有篩選後的資料
      const filteredData = filterNamespaces(allNamespaces);

      if (filteredData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }

      // 匯出篩選後的所有資料
      const dataToExport = filteredData.map(ns => ({
        [t('columns.name')]: ns.name,
        [t('columns.status')]: ns.status === 'Active' ? t('common:status.active') : ns.status,
        [t('columns.labels')]: ns.labels ? Object.entries(ns.labels).map(([k, v]) => `${k}=${v}`).join('; ') : '-',
        [t('columns.createdAt')]: ns.creationTimestamp ? new Date(ns.creationTimestamp).toLocaleString('zh-TW', {
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
            return `"${value}"`;
          }).join(',')
        )
      ].join('\n');

      const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `namespace-list-${Date.now()}.csv`;
      link.click();
      message.success(t('common:messages.exportCount', { count: filteredData.length }));
    } catch (error) {
      console.error('匯出失敗:', error);
      message.error(t('common:messages.exportError'));
    }
  };

  // 列設定儲存
  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(t('common:messages.columnSettingsSaved'));
  };

  // 檢視詳情
  const handleViewDetail = (namespace: string) => {
    navigate(`/clusters/${clusterId}/namespaces/${namespace}`);
  };

  // 當搜尋條件改變時重置到第一頁
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 當allNamespaces、搜尋條件、分頁參數、排序參數改變時，重新計算顯示資料
  useEffect(() => {
    if (allNamespaces.length === 0) {
      setNamespaces([]);
      setTotal(0);
      return;
    }

    // 1. 應用客戶端過濾
    let filteredItems = filterNamespaces(allNamespaces);

    // 2. 應用排序
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof NamespaceData];
        const bValue = b[sortField as keyof NamespaceData];

        // 處理 undefined 值
        if (aValue === undefined && bValue === undefined) return 0;
        if (aValue === undefined) return sortOrder === 'ascend' ? 1 : -1;
        if (bValue === undefined) return sortOrder === 'ascend' ? -1 : 1;

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

    setNamespaces(paginatedItems);
    setTotal(filteredItems.length);
  }, [allNamespaces, filterNamespaces, currentPage, pageSize, sortField, sortOrder]);

  // 初始載入資料
  useEffect(() => {
    loadNamespaces();
  }, [loadNamespaces]);

  // 行選擇配置
  const rowSelection = {
    columnWidth: 48,
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

  // 定義所有可用列
  const allColumns: ColumnsType<NamespaceData> = [
    {
      title: t('columns.name'),
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
      title: t('columns.status'),
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
            text={isActive ? t('common:status.active') : status}
          />
        );
      },
    },
    {
      title: t('columns.labels'),
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
              <Tooltip title={t('columns.moreLabels', { count: moreCount })}>
                <Tag>+{moreCount}</Tag>
              </Tooltip>
            )}
          </Space>
        );
      },
    },
    {
      title: t('columns.createdAt'),
      dataIndex: 'creationTimestamp',
      key: 'creationTimestamp',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'creationTimestamp' ? sortOrder : null,
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
      title: t('common:table.actions'),
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
          >{t('common:actions.viewDetails')}</Button>
          {!['default', 'kube-system', 'kube-public', 'kube-node-lease'].includes(record.name) && (
            <Popconfirm
              title={t('common:actions.delete')}
              description={`確定要刪除命名空間 "${record.name}" 嗎？此操作將刪除該命名空間下的所有資源。`}
              onConfirm={() => handleDelete(record.name)}
              okText={t("common:actions.confirm")}
              cancelText={t("common:actions.cancel")}
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
              >{t('common:actions.delete')}</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  // 根據可見性過濾列
  const columns = allColumns.filter(col => {
    if (col.key === 'actions') return true; // 操作列始終顯示
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序處理
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
                ? `${t('common:actions.batchDelete')} (${selectedRowKeys.length})`
                : t('common:actions.delete')}
            </Button>
            <Button onClick={handleExport}>{t('common:actions.export')}</Button>
          </Space>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalVisible(true)}
          >{t('list.createNamespace')}</Button>
        </div>

        {/* 多條件搜尋欄 */}
        <div style={{ marginBottom: 16 }}>
          {/* 搜尋輸入框 */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
            <Input
              prefix={<SearchOutlined />}
              placeholder={t("common:search.placeholder")}
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
                  <Option value="name">{t('list.fieldName')}</Option>
                  <Option value="status">{t('list.fieldStatus')}</Option>
                  <Option value="label">{t('list.fieldLabel')}</Option>
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
                >{t('common:actions.clearAll')}</Button>
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
          virtual
          scroll={{ x: 900, y: 600 }}
          size="middle"
          onChange={handleTableChange}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => t("list.totalNamespaces", { count: total }),
            onChange: (page, size) => {
              setCurrentPage(page);
              setPageSize(size || 20);
            },
            pageSizeOptions: ['10', '20', '50', '100'],
          }}
        />
      </Card>

      {/* 建立命名空間模態框 */}
      <Modal
        title={t("list.createNamespace")}
        open={createModalVisible}
        onCancel={() => {
          setCreateModalVisible(false);
          form.resetFields();
        }}
        onOk={() => form.submit()}
        okText={t("common:actions.confirm")}
        cancelText={t("common:actions.cancel")}
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
            label={t("create.nameLabel")}
            rules={[
              { required: true, message: t('create.nameRequired') },
              {
                pattern: /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/,
                message: t('create.namePattern'),
              },
            ]}
          >
            <Input placeholder={t('namespace:create.namePlaceholder')} />
          </Form.Item>

          <Form.Item
            name={['annotations', 'description']}
            label={t("create.descriptionLabel")}
          >
            <Input.TextArea
              rows={3}
              placeholder={t("create.descriptionPlaceholder")}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 列設定抽屜 */}
      <Drawer
        title={t("common:table.columnSettings")}
        placement="right"
        width={400}
        open={columnSettingsVisible}
        onClose={() => setColumnSettingsVisible(false)}
        footer={
          <div style={{ textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setColumnSettingsVisible(false)}>{t("common:actions.cancel")}</Button>
              <Button type="primary" onClick={handleColumnSettingsSave}>{t("common:actions.confirm")}</Button>
            </Space>
          </div>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8, color: '#666' }}>{t("common:table.selectColumns")}</p>
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
            >{t('columns.name')}</Checkbox>
            <Checkbox
              checked={visibleColumns.includes('status')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'status']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'status'));
                }
              }}
            >{t('columns.status')}</Checkbox>
            <Checkbox
              checked={visibleColumns.includes('labels')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'labels']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'labels'));
                }
              }}
            >{t('columns.labels')}</Checkbox>
            <Checkbox
              checked={visibleColumns.includes('creationTimestamp')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'creationTimestamp']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'creationTimestamp'));
                }
              }}
            >{t('columns.createdAt')}</Checkbox>
          </Space>
        </div>
      </Drawer>
    </div>
  );
};

export default NamespaceList;

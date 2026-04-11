import React, { useMemo } from 'react';
import {
  Table,
  Button,
  Space,
  Tag,
  Select,
  Input,
  App,
  Card,
  theme,
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  SearchOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import type { NamespaceData } from '../../services/namespaceService';
import { useNamespaceList } from './hooks/useNamespaceList';
import { createNamespaceColumns } from './columns';
import ColumnSettingsDrawer from './components/ColumnSettingsDrawer';
import CreateNamespaceModal from './components/CreateNamespaceModal';

const { Option } = Select;

const NamespaceList: React.FC = () => {
  const state = useNamespaceList();
  const { token } = theme.useToken();

  const allColumns = useMemo(() => createNamespaceColumns({
    t: state.t,
    token,
    sortField: state.sortField,
    sortOrder: state.sortOrder,
    SYSTEM_NAMESPACES: state.SYSTEM_NAMESPACES,
    handleViewDetail: state.handleViewDetail,
    handleDelete: state.handleDelete,
  }), [state.t, token, state.sortField, state.sortOrder, state.SYSTEM_NAMESPACES, state.handleViewDetail, state.handleDelete]);

  const columns = useMemo(() => allColumns.filter(col => {
    if (col.key === 'actions') return true;
    return state.visibleColumns.includes(col.key as string);
  }), [allColumns, state.visibleColumns]);

  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<NamespaceData> | SorterResult<NamespaceData>[]
  ) => {
    const singleSorter = Array.isArray(sorter) ? sorter[0] : sorter;
    if (singleSorter && singleSorter.field) {
      state.setSortField(String(singleSorter.field));
      state.setSortOrder(singleSorter.order || null);
    } else {
      state.setSortField('');
      state.setSortOrder(null);
    }
  };

  const rowSelection = {
    columnWidth: 48,
    selectedRowKeys: state.selectedRowKeys,
    onChange: (keys: React.Key[]) => state.setSelectedRowKeys(keys as string[]),
  };

  return (
    <App>
      <div style={{ padding: '24px' }}>
        <Card bordered={false}>
          {/* Action toolbar */}
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Space>
              <Button
                danger
                disabled={state.selectedRowKeys.length === 0}
                onClick={state.handleBatchDelete}
                icon={<DeleteOutlined />}
              >
                {state.selectedRowKeys.length > 1
                  ? `${state.t('common:actions.batchDelete')} (${state.selectedRowKeys.length})`
                  : state.t('common:actions.delete')}
              </Button>
              <Button disabled={state.selectedRowKeys.length === 0} onClick={state.handleExport}>
                {state.selectedRowKeys.length > 1
                  ? `${state.t('common:actions.batchExport')} (${state.selectedRowKeys.length})`
                  : state.t('common:actions.export')}
              </Button>
            </Space>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => state.setCreateModalVisible(true)}
            >{state.t('list.createNamespace')}</Button>
          </div>

          {/* Multi-condition search bar */}
          <div style={{ marginBottom: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
              <Input
                prefix={<SearchOutlined />}
                placeholder={state.t('common:search.placeholder')}
                style={{ flex: 1 }}
                value={state.currentSearchValue}
                onChange={(e) => state.setCurrentSearchValue(e.target.value)}
                onPressEnter={state.addSearchCondition}
                allowClear
                addonBefore={
                  <Select
                    value={state.currentSearchField}
                    onChange={state.setCurrentSearchField}
                    style={{ width: 100 }}
                  >
                    <Option value="name">{state.t('list.fieldName')}</Option>
                    <Option value="status">{state.t('list.fieldStatus')}</Option>
                    <Option value="label">{state.t('list.fieldLabel')}</Option>
                  </Select>
                }
              />
              <Button icon={<ReloadOutlined />} onClick={() => state.loadNamespaces()} />
              <Button icon={<SettingOutlined />} onClick={() => state.setColumnSettingsVisible(true)} />
            </div>

            {state.searchConditions.length > 0 && (
              <Space size="small" wrap>
                {state.searchConditions.map((condition, index) => (
                  <Tag
                    key={index}
                    closable
                    onClose={() => state.removeSearchCondition(index)}
                    color="blue"
                  >
                    {state.getFieldLabel(condition.field)}: {condition.value}
                  </Tag>
                ))}
                <Button size="small" type="link" onClick={state.clearAllConditions} style={{ padding: 0 }}>
                  {state.t('common:actions.clearAll')}
                </Button>
              </Space>
            )}
          </div>

          <Table
            columns={columns}
            dataSource={state.namespaces}
            rowKey="name"
            rowSelection={rowSelection}
            loading={state.loading}
            scroll={{ x: 'max-content' }}
            size="middle"
            onChange={handleTableChange}
            pagination={{
              current: state.currentPage,
              pageSize: state.pageSize,
              total: state.total,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => state.t('list.totalNamespaces', { count: total }),
              onChange: (page, size) => {
                state.setCurrentPage(page);
                state.setPageSize(size || 20);
              },
              pageSizeOptions: ['10', '20', '50', '100'],
            }}
          />
        </Card>

        <CreateNamespaceModal
          open={state.createModalVisible}
          form={state.form}
          onCancel={() => { state.setCreateModalVisible(false); state.form.resetFields(); }}
          onFinish={state.handleCreate}
          t={state.t}
        />

        <ColumnSettingsDrawer
          open={state.columnSettingsVisible}
          visibleColumns={state.visibleColumns}
          setVisibleColumns={state.setVisibleColumns}
          onClose={() => state.setColumnSettingsVisible(false)}
          onSave={state.handleColumnSettingsSave}
          t={state.t}
        />
      </div>
    </App>
  );
};

export default NamespaceList;

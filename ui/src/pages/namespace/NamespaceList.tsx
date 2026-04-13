import React, { useMemo } from 'react';
import {
  Table,
  Button,
  Space,
  App,
  Card,
  theme,
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { MultiSearchBar } from '@/components/MultiSearchBar';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import type { NamespaceData } from '../../services/namespaceService';
import { useNamespaceList } from './hooks/useNamespaceList';
import { usePermission } from '@/hooks/usePermission';
import { createNamespaceColumns } from './columns';
import ColumnSettingsDrawer from './components/ColumnSettingsDrawer';
import CreateNamespaceModal from './components/CreateNamespaceModal';


const NamespaceList: React.FC = () => {
  const { hasFeature, canWrite, canDelete } = usePermission();
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
    canDelete: canDelete(),
    showActions: canWrite(),
  }), [state.t, token, state.sortField, state.sortOrder, state.SYSTEM_NAMESPACES, state.handleViewDetail, state.handleDelete, canWrite, canDelete]);

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
        <Card variant="borderless">
          {/* Action toolbar */}
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Space>
              {canDelete() && (
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
              )}
              {hasFeature('export') && (
                <Button disabled={state.selectedRowKeys.length === 0} onClick={state.handleExport}>
                  {state.selectedRowKeys.length > 1
                    ? `${state.t('common:actions.batchExport')} (${state.selectedRowKeys.length})`
                    : state.t('common:actions.export')}
                </Button>
              )}
            </Space>
            {canWrite() && (
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => state.setCreateModalVisible(true)}
              >{state.t('list.createNamespace')}</Button>
            )}
          </div>

          <MultiSearchBar
            fieldOptions={[
              { value: 'name', label: state.t('list.fieldName') },
              { value: 'status', label: state.t('list.fieldStatus') },
              { value: 'label', label: state.t('list.fieldLabel') },
            ]}
            conditions={state.searchConditions}
            currentField={state.currentSearchField}
            currentValue={state.currentSearchValue}
            onFieldChange={state.setCurrentSearchField}
            onValueChange={state.setCurrentSearchValue}
            onAdd={state.addSearchCondition}
            onRemove={state.removeSearchCondition}
            onClear={state.clearAllConditions}
            getFieldLabel={state.getFieldLabel}
            fieldSelectWidth={100}
            extra={
              <>
                <Button icon={<ReloadOutlined />} onClick={() => state.loadNamespaces()} />
                <Button icon={<SettingOutlined />} onClick={() => state.setColumnSettingsVisible(true)} />
              </>
            }
          />

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

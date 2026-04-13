import React, { useMemo } from 'react';
import {
  Table,
  Button,
  Divider,
  App,
  Card,
  theme,
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  DeleteOutlined,
  ExportOutlined,
  CloseOutlined,
} from '@ant-design/icons';
import { MultiSearchBar } from '@/components/MultiSearchBar';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import type { NamespaceData } from '../../services/namespaceService';
import { useParams } from 'react-router-dom';
import { useNamespaceList } from './hooks/useNamespaceList';
import { usePermission } from '@/hooks/usePermission';
import { createNamespaceColumns } from './columns';
import ColumnSettingsDrawer from './components/ColumnSettingsDrawer';
import CreateNamespaceModal from './components/CreateNamespaceModal';


const NamespaceList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { hasFeature } = usePermission();
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
    canDelete: hasFeature('namespace:delete', clusterId),
    showActions: hasFeature('namespace:write', clusterId) || hasFeature('namespace:delete', clusterId),
  }), [state.t, token, state.sortField, state.sortOrder, state.SYSTEM_NAMESPACES, state.handleViewDetail, state.handleDelete, hasFeature, clusterId]);

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
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            {/* 批次操作列 — 永遠佔位，勾選後才顯示內容 */}
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: token.marginSM,
              height: 36,
              padding: `0 ${token.paddingSM}px`,
              borderRadius: token.borderRadius,
              background: state.selectedRowKeys.length > 0 ? token.colorFillAlter : 'transparent',
              opacity: state.selectedRowKeys.length > 0 ? 1 : 0,
              pointerEvents: state.selectedRowKeys.length > 0 ? 'auto' : 'none',
              transition: 'opacity 0.2s ease, background 0.2s ease',
            }}>
              <span style={{ color: token.colorTextSecondary, fontSize: token.fontSizeSM }}>
                {state.t('common:table.selectedCount', { count: state.selectedRowKeys.length })}
              </span>
              <Divider type="vertical" style={{ margin: 0, height: 14 }} />
              {hasFeature('namespace:delete', clusterId) && (
                <Button size="small" danger type="text" icon={<DeleteOutlined />} onClick={state.handleBatchDelete}>
                  {state.t('common:actions.batchDelete')}
                </Button>
              )}
              {hasFeature('export', clusterId) && (
                <Button size="small" type="text" icon={<ExportOutlined />} onClick={state.handleExport}>
                  {state.t('common:actions.export')}
                </Button>
              )}
              <Button
                size="small" type="text"
                icon={<CloseOutlined />}
                onClick={() => state.setSelectedRowKeys([])}
                style={{ marginLeft: 'auto', color: token.colorTextTertiary }}
              />
            </div>
            {hasFeature('namespace:write', clusterId) && (
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

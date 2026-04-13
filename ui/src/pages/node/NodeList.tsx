import React, { useMemo } from 'react';
import { Card, Table, Button, Space, App } from 'antd';
import { ReloadOutlined, SettingOutlined, DatabaseOutlined } from '@ant-design/icons';
import { MultiSearchBar } from '@/components/MultiSearchBar';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import type { Node } from '../../types';

import { useNodeList } from './hooks/useNodeList';
import { createNodeColumns } from './columns';
import { NodeStatsCards, NodeLabelModal, ColumnSettingsDrawer } from './components';
import { usePermission } from '../../hooks/usePermission';


const NodeList: React.FC = () => {
  const state = useNodeList();
  const { hasFeature } = usePermission();

  // Create columns with memoization
  const allColumns = useMemo(() => createNodeColumns({
    t: state.t,
    tc: state.tc,
    sortField: state.sortField,
    sortOrder: state.sortOrder,
    handleViewDetail: state.handleViewDetail,
    handleNodeTerminal: state.handleNodeTerminal,
    handleCordon: state.handleCordon,
    handleUncordon: state.handleUncordon,
    handleDrain: state.handleDrain,
    canTerminalNode: hasFeature('terminal:node'),
    showActions: hasFeature('node:manage'),
  }), [
    state.t, state.tc, state.sortField, state.sortOrder,
    state.handleViewDetail, state.handleNodeTerminal,
    state.handleCordon, state.handleUncordon, state.handleDrain, hasFeature
  ]);

  // Filter columns by visibility
  const columns = useMemo(() => allColumns.filter(col => {
    if (col.key === 'actions') return true;
    return state.visibleColumns.includes(col.key as string);
  }), [allColumns, state.visibleColumns]);

  // Handle table change for sorting
  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<Node> | SorterResult<Node>[]
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

  return (
    <App>
      <div style={{ padding: '24px' }}>
        {/* Stats Cards */}
        <NodeStatsCards
          totalNodes={state.totalNodes}
          readyNodes={state.readyNodes}
          notReadyNodes={state.notReadyNodes}
          maintenanceNodes={state.maintenanceNodes}
          t={state.t}
        />

        {/* Node List Card */}
        <Card variant="borderless">
          {/* Action Buttons */}
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Space>
              {hasFeature('node:manage') && (
                <>
                  <Button
                    disabled={state.selectedNodes.length === 0}
                    onClick={state.handleBatchCordon}
                  >
                    {state.isBatch ? state.t('actions.batchCordon') : state.t('actions.cordon')}
                  </Button>
                  <Button
                    disabled={state.selectedNodes.length === 0}
                    onClick={state.handleBatchUncordon}
                  >
                    {state.isBatch ? state.t('actions.batchUncordon') : state.t('actions.uncordon')}
                  </Button>
                  <Button
                    disabled={state.selectedNodes.length === 0}
                    danger
                    onClick={state.handleBatchDrain}
                  >
                    {state.isBatch ? '批次驅逐' : state.t('actions.drain')}
                  </Button>
                  <Button
                    disabled={state.selectedNodes.length === 0}
                    onClick={state.handleBatchLabel}
                  >
                    {state.isBatch ? state.t('actions.batchLabel') : '新增標籤'}
                  </Button>
                </>
              )}
              {hasFeature('export') && (
                <Button disabled={state.selectedNodes.length === 0} onClick={state.handleExport}>
                  {state.selectedNodes.length > 1
                    ? `${state.tc('actions.batchExport')} (${state.selectedNodes.length})`
                    : state.tc('actions.export')}
                </Button>
              )}
            </Space>
          </div>

          <MultiSearchBar
            fieldOptions={[
              { value: 'name', label: state.t('columns.name') },
              { value: 'status', label: state.t('columns.status') },
              { value: 'version', label: state.t('columns.version') },
              { value: 'roles', label: state.t('columns.roles') },
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
            fieldSelectWidth={120}
            extra={
              <>
                <Button icon={<ReloadOutlined />} onClick={state.handleRefresh} />
                <Button icon={<SettingOutlined />} onClick={() => state.setColumnSettingsVisible(true)} />
              </>
            }
          />

          {/* Table */}
          <Table
            rowSelection={state.nodeRowSelection}
            columns={columns}
            dataSource={state.nodes}
            rowKey="id"
            loading={state.loading}
            scroll={{ x: 1400 }}
            size="middle"
            onChange={handleTableChange}
            pagination={{
              current: state.currentPage,
              pageSize: state.pageSize,
              total: state.total,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => `${state.tc('table.total')} ${total} ${state.t('list.nodes')}`,
              onChange: (page, size) => {
                state.setCurrentPage(page);
                state.setPageSize(size || 20);
              },
              pageSizeOptions: ['10', '20', '50', '100'],
            }}
            locale={{
              emptyText: (
                <div style={{ padding: '48px 0', textAlign: 'center' }}>
                  <DatabaseOutlined style={{ fontSize: 48, color: '#ccc', marginBottom: 16 }} />
                  <div style={{ fontSize: 16, color: '#666', marginBottom: 8 }}>{state.t('list.noData')}</div>
                  <div style={{ fontSize: 14, color: '#999', marginBottom: 16 }}>
                    {state.searchConditions.length > 0 ? state.tc('messages.noData') : state.t('list.selectCluster')}
                  </div>
                </div>
              )
            }}
          />
        </Card>

        {/* Label Modal */}
        <NodeLabelModal
          open={state.labelModalOpen}
          isBatch={state.isBatch}
          selectedCount={state.selectedNodes.length}
          form={state.labelForm}
          submitting={state.labelSubmitting}
          onCancel={() => {
            state.setLabelModalOpen(false);
            state.labelForm.resetFields();
          }}
          onSubmit={state.handleLabelSubmit}
          tc={state.tc}
        />

        {/* Column Settings Drawer */}
        <ColumnSettingsDrawer
          open={state.columnSettingsVisible}
          visibleColumns={state.visibleColumns}
          setVisibleColumns={state.setVisibleColumns}
          onClose={() => state.setColumnSettingsVisible(false)}
          onSave={state.handleColumnSettingsSave}
          t={state.t}
          tc={state.tc}
        />
      </div>
    </App>
  );
};

export default NodeList;

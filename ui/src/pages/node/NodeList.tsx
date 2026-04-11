import React, { useMemo } from 'react';
import { Card, Table, Button, Space, Tag, Input, Select, App } from 'antd';
import { ReloadOutlined, SearchOutlined, SettingOutlined, DatabaseOutlined } from '@ant-design/icons';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import type { Node } from '../../types';

import { useNodeList } from './hooks/useNodeList';
import { createNodeColumns } from './columns';
import { NodeStatsCards, NodeLabelModal, ColumnSettingsDrawer } from './components';

const { Option } = Select;

const NodeList: React.FC = () => {
  const state = useNodeList();

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
  }), [
    state.t, state.tc, state.sortField, state.sortOrder,
    state.handleViewDetail, state.handleNodeTerminal,
    state.handleCordon, state.handleUncordon, state.handleDrain
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
        <Card bordered={false}>
          {/* Action Buttons */}
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Space>
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
              <Button disabled={state.selectedNodes.length === 0} onClick={state.handleExport}>
                {state.selectedNodes.length > 1
                  ? `${state.tc('actions.batchExport')} (${state.selectedNodes.length})`
                  : state.tc('actions.export')}
              </Button>
            </Space>
          </div>

          {/* Search Bar */}
          <div style={{ marginBottom: 16 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
              <Input
                prefix={<SearchOutlined />}
                placeholder={state.t('list.searchPlaceholder')}
                style={{ flex: 1 }}
                value={state.currentSearchValue}
                onChange={(e) => state.setCurrentSearchValue(e.target.value)}
                onPressEnter={state.addSearchCondition}
                allowClear
                addonBefore={
                  <Select
                    value={state.currentSearchField}
                    onChange={state.setCurrentSearchField}
                    style={{ width: 120 }}
                  >
                    <Option value="name">{state.t('columns.name')}</Option>
                    <Option value="status">{state.t('columns.status')}</Option>
                    <Option value="version">{state.t('columns.version')}</Option>
                    <Option value="roles">{state.t('columns.roles')}</Option>
                  </Select>
                }
              />
              <Button icon={<ReloadOutlined />} onClick={state.handleRefresh} />
              <Button icon={<SettingOutlined />} onClick={() => state.setColumnSettingsVisible(true)} />
            </div>

            {/* Search Condition Tags */}
            {state.searchConditions.length > 0 && (
              <div>
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
                  <Button
                    size="small"
                    type="link"
                    onClick={state.clearAllConditions}
                    style={{ padding: 0 }}
                  >
                    {state.tc('actions.clearAll')}
                  </Button>
                </Space>
              </div>
            )}
          </div>

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

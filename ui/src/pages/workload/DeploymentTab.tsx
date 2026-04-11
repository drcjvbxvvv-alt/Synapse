import React, { useMemo } from 'react';
import { Table, Button, Space, Tag, Select, Input, App } from 'antd';
import { PlusOutlined, ReloadOutlined, SettingOutlined, SearchOutlined } from '@ant-design/icons';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import type { WorkloadInfo } from '../../services/workloadService';

import { useWorkloadTab } from './hooks/useWorkloadTab';
import { createWorkloadColumns } from './columns';
import { ScaleModal, WorkloadColumnSettingsDrawer } from './components';
import WorkloadCreateModal from '../../components/workload/WorkloadCreateModal';

const { Option } = Select;

interface DeploymentTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const DeploymentTab: React.FC<DeploymentTabProps> = ({ clusterId, onCountChange }) => {
  const state = useWorkloadTab({
    clusterId,
    workloadType: 'Deployment',
    onCountChange,
  });

  // Create columns with memoization
  const allColumns = useMemo(() => createWorkloadColumns({
    t: state.t,
    workloadType: 'Deployment',
    sortField: state.sortField,
    sortOrder: state.sortOrder,
    navigateToDetail: state.navigateToDetail,
    handleMonitor: state.handleMonitor,
    handleEdit: state.handleEdit,
    openScaleModal: state.openScaleModal,
    handleRestart: state.handleRestart,
    handleDelete: state.handleDelete,
  }), [
    state.t, state.sortField, state.sortOrder,
    state.navigateToDetail, state.handleMonitor, state.handleEdit,
    state.openScaleModal, state.handleRestart, state.handleDelete
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
    sorter: SorterResult<WorkloadInfo> | SorterResult<WorkloadInfo>[]
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
      <div>
        {/* Action Buttons */}
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <Space>
            <Button
              disabled={state.selectedRowKeys.length === 0}
              onClick={state.handleBatchRedeploy}
              icon={<ReloadOutlined />}
            >
              {state.selectedRowKeys.length > 1
                ? `${state.t('actions.batchRedeploy')} (${state.selectedRowKeys.length})`
                : state.t('actions.redeploy')}
            </Button>
            <Button disabled={state.selectedRowKeys.length === 0} onClick={state.handleExport}>
              {state.selectedRowKeys.length > 1
                ? `${state.t('actions.batchExport')} (${state.selectedRowKeys.length})`
                : state.t('actions.export')}
            </Button>
          </Space>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => state.setCreateModalVisible(true)}
          >
            {state.t('actions.create', { type: 'Deployment' })}
          </Button>
        </div>

        {/* Search Bar */}
        <div style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
            <Input
              prefix={<SearchOutlined />}
              placeholder={state.t('search.placeholder')}
              style={{ flex: 1 }}
              value={state.currentSearchValue}
              onChange={(e) => state.setCurrentSearchValue(e.target.value)}
              onPressEnter={state.addSearchCondition}
              allowClear
              addonBefore={
                <Select
                  value={state.currentSearchField}
                  onChange={state.setCurrentSearchField}
                  style={{ width: 140 }}
                >
                  <Option value="name">{state.t('search.workloadName')}</Option>
                  <Option value="namespace">{state.t('search.namespace')}</Option>
                  <Option value="image">{state.t('search.image')}</Option>
                  <Option value="status">{state.t('search.status')}</Option>
                  <Option value="cpuLimit">{state.t('search.cpuLimit')}</Option>
                  <Option value="cpuRequest">{state.t('search.cpuRequest')}</Option>
                  <Option value="memoryLimit">{state.t('search.memoryLimit')}</Option>
                  <Option value="memoryRequest">{state.t('search.memoryRequest')}</Option>
                </Select>
              }
            />
            <Button icon={<ReloadOutlined />} onClick={state.loadWorkloads} />
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
                  {state.t('common:actions.clearAll')}
                </Button>
              </Space>
            </div>
          )}
        </div>

        {/* Table */}
        <Table
          columns={columns}
          dataSource={state.workloads}
          locale={{ emptyText: state.t('common:messages.noData') }}
          rowKey={(record) => `${record.namespace}-${record.name}-${record.type}`}
          rowSelection={state.rowSelection}
          loading={state.loading}
          virtual
          scroll={{ x: 1400, y: 600 }}
          size="middle"
          onChange={handleTableChange}
          pagination={{
            current: state.currentPage,
            pageSize: state.pageSize,
            total: state.total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => state.t('messages.totalItems', { count: total, type: 'Deployment' }),
            onChange: (page, size) => {
              state.setCurrentPage(page);
              state.setPageSize(size || 20);
            },
            pageSizeOptions: ['10', '20', '50', '100'],
          }}
        />

        {/* Scale Modal */}
        <ScaleModal
          open={state.scaleModalVisible}
          workload={state.scaleWorkload}
          replicas={state.scaleReplicas}
          workloadType="Deployment"
          onReplicasChange={state.setScaleReplicas}
          onOk={state.handleScale}
          onCancel={() => state.setScaleModalVisible(false)}
          t={state.t}
        />

        {/* Column Settings Drawer */}
        <WorkloadColumnSettingsDrawer
          open={state.columnSettingsVisible}
          visibleColumns={state.visibleColumns}
          setVisibleColumns={state.setVisibleColumns}
          onClose={() => state.setColumnSettingsVisible(false)}
          onSave={state.handleColumnSettingsSave}
          t={state.t}
        />

        {/* Create Modal */}
        <WorkloadCreateModal
          open={state.createModalVisible}
          workloadType="Deployment"
          clusterId={clusterId}
          onClose={() => state.setCreateModalVisible(false)}
          onSuccess={() => {
            state.setCreateModalVisible(false);
            state.loadWorkloads();
          }}
        />
      </div>
    </App>
  );
};

export default DeploymentTab;

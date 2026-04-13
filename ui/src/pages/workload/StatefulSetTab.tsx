import React, { useMemo } from 'react';
import { Table, Button, Space, App } from 'antd';
import { PlusOutlined, ReloadOutlined, SettingOutlined } from '@ant-design/icons';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import type { WorkloadInfo } from '../../services/workloadService';
import EmptyState from '@/components/EmptyState';
import { MultiSearchBar } from '@/components/MultiSearchBar';
import { useWorkloadTab } from './hooks/useWorkloadTab';
import { createWorkloadColumns } from './columns';
import { ScaleModal, WorkloadColumnSettingsDrawer } from './components';
import WorkloadCreateModal from '../../components/workload/WorkloadCreateModal';
import { usePermission } from '@/hooks/usePermission';

interface StatefulSetTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const StatefulSetTab: React.FC<StatefulSetTabProps> = ({ clusterId, onCountChange }) => {
  const { hasFeature, canWrite, canDelete } = usePermission();
  const state = useWorkloadTab({
    clusterId,
    workloadType: 'StatefulSet',
    onCountChange,
  });

  const allColumns = useMemo(() => createWorkloadColumns({
    t: state.t,
    workloadType: 'StatefulSet',
    sortField: state.sortField,
    sortOrder: state.sortOrder,
    navigateToDetail: state.navigateToDetail,
    handleMonitor: state.handleMonitor,
    handleEdit: state.handleEdit,
    openScaleModal: state.openScaleModal,
    handleRestart: state.handleRestart,
    handleDelete: state.handleDelete,
    canDelete: canDelete(),
    showActions: canWrite(),
  }), [
    state.t, state.sortField, state.sortOrder,
    state.navigateToDetail, state.handleMonitor, state.handleEdit,
    state.openScaleModal, state.handleRestart, state.handleDelete, canWrite, canDelete
  ]);

  const columns = useMemo(() => allColumns.filter(col => {
    if (col.key === 'actions') return true;
    return state.visibleColumns.includes(col.key as string);
  }), [allColumns, state.visibleColumns]);

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
            {hasFeature('export') && (
              <Button disabled={state.selectedRowKeys.length === 0} onClick={state.handleExport}>
                {state.selectedRowKeys.length > 1
                  ? `${state.t('actions.batchExport')} (${state.selectedRowKeys.length})`
                  : state.t('actions.export')}
              </Button>
            )}
          </Space>
          {canWrite() && (
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => state.setCreateModalVisible(true)}
            >
              {state.t('actions.create', { type: 'StatefulSet' })}
            </Button>
          )}
        </div>

        <MultiSearchBar
          fieldOptions={[
            { value: 'name', label: state.t('search.workloadName') },
            { value: 'namespace', label: state.t('search.namespace') },
            { value: 'image', label: state.t('search.image') },
            { value: 'status', label: state.t('search.status') },
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
          fieldSelectWidth={140}
          extra={
            <>
              <Button icon={<ReloadOutlined />} onClick={state.loadWorkloads} />
              <Button icon={<SettingOutlined />} onClick={() => state.setColumnSettingsVisible(true)} />
            </>
          }
        />

        <Table
          columns={columns}
          dataSource={state.workloads}
          locale={{ emptyText: <EmptyState description={state.t('common:messages.noData')} /> }}
          rowKey={(record) => `${record.namespace}-${record.name}-${record.type}`}
          rowSelection={state.rowSelection}
          loading={state.loading}
          virtual
          scroll={{ x: 1400, y: 600 }}
          size="small"
          onChange={handleTableChange}
          pagination={{
            current: state.currentPage,
            pageSize: state.pageSize,
            total: state.total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => state.t('messages.totalItems', { count: total, type: 'StatefulSet' }),
            onChange: (page, size) => {
              state.setCurrentPage(page);
              state.setPageSize(size || 20);
            },
            pageSizeOptions: ['10', '20', '50', '100'],
          }}
        />

        <ScaleModal
          open={state.scaleModalVisible}
          workload={state.scaleWorkload}
          replicas={state.scaleReplicas}
          workloadType="StatefulSet"
          onReplicasChange={state.setScaleReplicas}
          onOk={state.handleScale}
          onCancel={() => state.setScaleModalVisible(false)}
          t={state.t}
        />

        <WorkloadColumnSettingsDrawer
          open={state.columnSettingsVisible}
          visibleColumns={state.visibleColumns}
          setVisibleColumns={state.setVisibleColumns}
          onClose={() => state.setColumnSettingsVisible(false)}
          onSave={state.handleColumnSettingsSave}
          t={state.t}
        />

        <WorkloadCreateModal
          open={state.createModalVisible}
          workloadType="StatefulSet"
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

export default StatefulSetTab;

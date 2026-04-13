import React, { useMemo } from 'react';
import { Table, Button, Divider, App, theme } from 'antd';
import { PlusOutlined, ReloadOutlined, SettingOutlined, CloseOutlined, ExportOutlined } from '@ant-design/icons';
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

interface CronJobTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const CronJobTab: React.FC<CronJobTabProps> = ({ clusterId, onCountChange }) => {
  const { hasFeature } = usePermission();
  const { token } = theme.useToken();
  const state = useWorkloadTab({
    clusterId,
    workloadType: 'CronJob',
    onCountChange,
  });

  const allColumns = useMemo(() => createWorkloadColumns({
    t: state.t,
    workloadType: 'CronJob',
    sortField: state.sortField,
    sortOrder: state.sortOrder,
    navigateToDetail: state.navigateToDetail,
    handleMonitor: state.handleMonitor,
    handleEdit: state.handleEdit,
    openScaleModal: state.openScaleModal,
    handleRestart: state.handleRestart,
    handleDelete: state.handleDelete,
    canDelete: hasFeature('workload:delete'),
    showActions: hasFeature('workload:write') || hasFeature('workload:delete'),
  }), [
    state.t, state.sortField, state.sortOrder,
    state.navigateToDetail, state.handleMonitor, state.handleEdit,
    state.openScaleModal, state.handleRestart, state.handleDelete, hasFeature
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
              {state.t('actions.selectedCount', { count: state.selectedRowKeys.length })}
            </span>
            <Divider type="vertical" style={{ margin: 0, height: 14 }} />
            <Button size="small" type="text" icon={<ReloadOutlined />} onClick={state.handleBatchRedeploy}>
              {state.t('actions.batchRedeploy')}
            </Button>
            {hasFeature('export') && (
              <Button size="small" type="text" icon={<ExportOutlined />} onClick={state.handleExport}>
                {state.t('actions.export')}
              </Button>
            )}
            <Button
              size="small" type="text"
              icon={<CloseOutlined />}
              onClick={() => state.setSelectedRowKeys([])}
              style={{ marginLeft: 'auto', color: token.colorTextTertiary }}
            />
          </div>

          {hasFeature('workload:write') && (
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => state.setCreateModalVisible(true)}
            >
              {state.t('actions.create', { type: 'CronJob' })}
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
            showTotal: (total) => state.t('messages.totalItems', { count: total, type: 'CronJob' }),
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
          workloadType="CronJob"
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
          workloadType="CronJob"
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

export default CronJobTab;

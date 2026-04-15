import React, { useMemo, useState, useEffect, useCallback } from 'react';
import { Table, Button, Divider, App, Spin, theme } from 'antd';
import { PlusOutlined, ReloadOutlined, SettingOutlined, CloseOutlined, ExportOutlined } from '@ant-design/icons';
import type { TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import type { WorkloadInfo } from '../../services/workloadService';
import { WorkloadService } from '../../services/workloadService';
import EmptyState from '@/components/EmptyState';
import NotInstalledCard from '@/components/NotInstalledCard';
import { MultiSearchBar } from '@/components/MultiSearchBar';
import { useWorkloadTab } from './hooks/useWorkloadTab';
import { createWorkloadColumns } from './columns';
import { ScaleModal, WorkloadColumnSettingsDrawer } from './components';
import WorkloadCreateModal from '../../components/workload/WorkloadCreateModal';
import { usePermission } from '@/hooks/usePermission';
import { useTranslation } from 'react-i18next';

interface RolloutTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const RolloutTab: React.FC<RolloutTabProps> = ({ clusterId, onCountChange }) => {
  const { hasFeature } = usePermission();
  const { token } = theme.useToken();
  const { t } = useTranslation(['workload', 'common']);
  const [crdInstalled, setCrdInstalled] = useState<boolean | null>(null);
  const [checkLoading, setCheckLoading] = useState(true);

  const checkCRD = useCallback(async () => {
    setCheckLoading(true);
    try {
      const res = await WorkloadService.checkRolloutCRD(clusterId);
      setCrdInstalled(res.enabled ?? false);
    } catch {
      setCrdInstalled(false);
    } finally {
      setCheckLoading(false);
    }
  }, [clusterId]);

  useEffect(() => { checkCRD(); }, [checkCRD]);

  const state = useWorkloadTab({
    clusterId,
    workloadType: 'ArgoRollout',
    onCountChange,
  });

  const allColumns = useMemo(() => createWorkloadColumns({
    t: state.t,
    workloadType: 'ArgoRollout',
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

  if (checkLoading) {
    return <Spin style={{ display: 'block', marginTop: 60 }} />;
  }

  if (!crdInstalled) {
    return (
      <NotInstalledCard
        title={t('workload:rollout.notInstalled')}
        description={t('workload:rollout.installHint')}
        command="kubectl create namespace argo-rollouts && kubectl apply -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml"
        docsUrl="https://argoproj.github.io/argo-rollouts/installation/"
        onRecheck={checkCRD}
        recheckLoading={checkLoading}
      />
    );
  }

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
              {state.t('actions.create', { type: 'ArgoRollout' })}
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
            showTotal: (total) => state.t('messages.totalItems', { count: total, type: 'ArgoRollout' }),
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
          workloadType="ArgoRollout"
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
          workloadType="ArgoRollout"
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

export default RolloutTab;

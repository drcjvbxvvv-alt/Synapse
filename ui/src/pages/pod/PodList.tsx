import React, { useMemo } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Checkbox,
  Drawer,
  Tabs,
  Spin,
} from 'antd';
import EmptyState from '@/components/EmptyState';
import {
  ReloadOutlined,
  SettingOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { MultiSearchBar } from '@/components/MultiSearchBar';
import { usePodList } from './hooks/usePodList';
import { createPodColumns } from './columns';
import { usePermission } from '../../hooks/usePermission';


const PodList: React.FC = () => {
  const { hasFeature, canWrite } = usePermission();
  const {
    clusterId,
    pods,
    total,
    loading,
    currentPage,
    pageSize,
    setCurrentPage,
    setPageSize,
    selectedRowKeys,
    rowSelection,
    searchConditions,
    currentSearchField,
    setCurrentSearchField,
    currentSearchValue,
    setCurrentSearchValue,
    addSearchCondition,
    removeSearchCondition,
    clearAllConditions,
    getFieldLabel,
    columnSettingsVisible,
    setColumnSettingsVisible,
    visibleColumns,
    setVisibleColumns,
    handleColumnSettingsSave,
    sortField,
    sortOrder,
    handleTableChange,
    loadPods,
    confirmDelete,
    handleBatchDelete,
    handleExport,
    handleLogs,
    handleTerminal,
    handleViewDetail,
    handleViewEvents,
    t,
    tc,
  } = usePodList();

  const allColumns = useMemo(
    () =>
      createPodColumns({
        t, tc, sortField, sortOrder, clusterId,
        handleViewDetail, handleLogs, handleTerminal, handleViewEvents, confirmDelete,
        canTerminalPod: canWrite() && hasFeature('terminal:pod'),
        showActions: canWrite(),
      }),
    [t, tc, sortField, sortOrder, clusterId, handleViewDetail, handleLogs, handleTerminal, handleViewEvents, confirmDelete, hasFeature, canWrite]
  );

  const columns = useMemo(
    () => allColumns.filter(col => col.key === 'actions' || visibleColumns.includes(col.key as string)),
    [allColumns, visibleColumns]
  );

  const columnOptions = useMemo(() => [
    { key: 'name', label: t('columns.name') },
    { key: 'status', label: tc('table.status') },
    { key: 'namespace', label: tc('table.namespace') },
    { key: 'podIP', label: t('columns.podIP') },
    { key: 'nodeName', label: t('columns.nodeName') },
    { key: 'restartCount', label: t('columns.restarts') },
    { key: 'cpuRequest', label: 'CPU Request' },
    { key: 'cpuLimit', label: 'CPU Limit' },
    { key: 'memoryRequest', label: 'MEM Request' },
    { key: 'memoryLimit', label: 'MEM Limit' },
    { key: 'createdAt', label: tc('table.createdAt') },
    { key: 'age', label: t('columns.age') },
  ], [t, tc]);

  return (
    <div style={{ padding: '24px' }}>
      <Card variant="borderless">
        <Spin spinning={loading && pods.length === 0}>
          <Tabs
            activeKey="pod"
            items={[{
              key: 'pod',
              label: t('list.tabTitle'),
              children: (
                <div>
                  {/* 操作按鈕欄 */}
                  <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <Space>
                      {canWrite() && (
                        <Button
                          danger
                          disabled={selectedRowKeys.length === 0}
                          onClick={handleBatchDelete}
                          icon={<DeleteOutlined />}
                        >
                          {selectedRowKeys.length > 1
                            ? `${t('actions.batchDelete')} (${selectedRowKeys.length})`
                            : tc('actions.delete')}
                        </Button>
                      )}
                      {hasFeature('export') && (
                        <Button disabled={selectedRowKeys.length === 0} onClick={handleExport}>
                          {selectedRowKeys.length > 1
                            ? `${tc('actions.batchExport')} (${selectedRowKeys.length})`
                            : tc('actions.export')}
                        </Button>
                      )}
                    </Space>
                  </div>

                  <MultiSearchBar
                    fieldOptions={[
                      { value: 'name', label: t('columns.name') },
                      { value: 'namespace', label: tc('table.namespace') },
                      { value: 'status', label: tc('table.status') },
                      { value: 'podIP', label: t('columns.podIP') },
                      { value: 'nodeName', label: t('columns.nodeName') },
                      { value: 'cpuRequest', label: 'CPU Request' },
                      { value: 'cpuLimit', label: 'CPU Limit' },
                      { value: 'memoryRequest', label: 'MEM Request' },
                      { value: 'memoryLimit', label: 'MEM Limit' },
                    ]}
                    conditions={searchConditions}
                    currentField={currentSearchField}
                    currentValue={currentSearchValue}
                    onFieldChange={setCurrentSearchField}
                    onValueChange={setCurrentSearchValue}
                    onAdd={addSearchCondition}
                    onRemove={removeSearchCondition}
                    onClear={clearAllConditions}
                    getFieldLabel={getFieldLabel}
                    fieldSelectWidth={130}
                    extra={
                      <>
                        <Button icon={<ReloadOutlined />} onClick={loadPods} />
                        <Button icon={<SettingOutlined />} onClick={() => setColumnSettingsVisible(true)} />
                      </>
                    }
                  />

                  <Table
                    columns={columns}
                    dataSource={pods}
                    locale={{ emptyText: <EmptyState description={tc('noData')} /> }}
                    rowKey={(record) => `${record.namespace}/${record.name}`}
                    rowSelection={rowSelection}
                    loading={loading}
                    virtual
                    scroll={{ x: 1400, y: 600 }}
                    size="middle"
                    onChange={handleTableChange}
                    pagination={{
                      current: currentPage,
                      pageSize,
                      total,
                      showSizeChanger: true,
                      showQuickJumper: true,
                      showTotal: (total) => `${tc('table.total')} ${total} Pod`,
                      onChange: (page, size) => { setCurrentPage(page); setPageSize(size || 20); },
                      pageSizeOptions: ['10', '20', '50', '100'],
                    }}
                  />

                  <Drawer
                    title={t('list.columnSettings')}
                    placement="right"
                    width={400}
                    open={columnSettingsVisible}
                    onClose={() => setColumnSettingsVisible(false)}
                    footer={
                      <div style={{ textAlign: 'right' }}>
                        <Space>
                          <Button onClick={() => setColumnSettingsVisible(false)}>{tc('actions.cancel')}</Button>
                          <Button type="primary" onClick={handleColumnSettingsSave}>{tc('actions.confirm')}</Button>
                        </Space>
                      </div>
                    }
                  >
                    <div style={{ marginBottom: 16 }}>
                      <p style={{ marginBottom: 8, color: '#666' }}>{t('list.selectColumns')}:</p>
                      <Space direction="vertical" style={{ width: '100%' }}>
                        {columnOptions.map(option => (
                          <Checkbox
                            key={option.key}
                            checked={visibleColumns.includes(option.key)}
                            onChange={(e) => {
                              if (e.target.checked) {
                                setVisibleColumns([...visibleColumns, option.key]);
                              } else {
                                setVisibleColumns(visibleColumns.filter(c => c !== option.key));
                              }
                            }}
                          >
                            {option.label}
                          </Checkbox>
                        ))}
                      </Space>
                    </div>
                  </Drawer>
                </div>
              ),
            }]}
          />
        </Spin>
      </Card>
    </div>
  );
};

export default PodList;

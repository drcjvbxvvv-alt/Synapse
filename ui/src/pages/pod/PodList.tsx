import React, { useMemo } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Input,
  Select,
  Tag,
  Checkbox,
  Drawer,
  Tabs,
  Spin,
} from 'antd';
import {
  ReloadOutlined,
  SettingOutlined,
  SearchOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { usePodList } from './hooks/usePodList';
import { createPodColumns } from './columns';

const { Option } = Select;

const PodList: React.FC = () => {
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
      }),
    [t, tc, sortField, sortOrder, clusterId, handleViewDetail, handleLogs, handleTerminal, handleViewEvents, confirmDelete]
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
      <Card bordered={false}>
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
                      <Button disabled={selectedRowKeys.length === 0} onClick={handleExport}>
                        {selectedRowKeys.length > 1
                          ? `${tc('actions.batchExport')} (${selectedRowKeys.length})`
                          : tc('actions.export')}
                      </Button>
                    </Space>
                  </div>

                  {/* 多條件搜尋欄 */}
                  <div style={{ marginBottom: 16 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
                      <Input
                        prefix={<SearchOutlined />}
                        placeholder={t('list.searchPlaceholder')}
                        style={{ flex: 1 }}
                        value={currentSearchValue}
                        onChange={(e) => setCurrentSearchValue(e.target.value)}
                        onPressEnter={addSearchCondition}
                        allowClear
                        addonBefore={
                          <Select value={currentSearchField} onChange={setCurrentSearchField} style={{ width: 130 }}>
                            <Option value="name">{t('columns.name')}</Option>
                            <Option value="namespace">{tc('table.namespace')}</Option>
                            <Option value="status">{tc('table.status')}</Option>
                            <Option value="podIP">{t('columns.podIP')}</Option>
                            <Option value="nodeName">{t('columns.nodeName')}</Option>
                            <Option value="cpuRequest">CPU Request</Option>
                            <Option value="cpuLimit">CPU Limit</Option>
                            <Option value="memoryRequest">MEM Request</Option>
                            <Option value="memoryLimit">MEM Limit</Option>
                          </Select>
                        }
                      />
                      <Button icon={<ReloadOutlined />} onClick={loadPods} />
                      <Button icon={<SettingOutlined />} onClick={() => setColumnSettingsVisible(true)} />
                    </div>
                    {searchConditions.length > 0 && (
                      <Space size="small" wrap>
                        {searchConditions.map((condition, index) => (
                          <Tag key={index} closable onClose={() => removeSearchCondition(index)} color="blue">
                            {getFieldLabel(condition.field)}: {condition.value}
                          </Tag>
                        ))}
                        <Button size="small" type="link" onClick={clearAllConditions} style={{ padding: 0 }}>
                          {tc('actions.clearAll')}
                        </Button>
                      </Space>
                    )}
                  </div>

                  <Table
                    columns={columns}
                    dataSource={pods}
                    locale={{ emptyText: tc('noData') }}
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

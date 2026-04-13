import React from 'react';
import {
  Table,
  Button,
  Divider,
  Space,
  Drawer,
  Checkbox,
  theme,
} from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  DeleteOutlined,
  SettingOutlined,
  CloseOutlined,
  ExportOutlined,
} from '@ant-design/icons';
import { MultiSearchBar } from '@/components/MultiSearchBar';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import EmptyState from '@/components/EmptyState';
import ConfigMapForm from './ConfigMapForm';
import { useConfigMapList } from './hooks/useConfigMapList';
import { usePermission } from '@/hooks/usePermission';
import { getConfigMapColumns } from './configMapColumns';


interface ConfigMapListProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const COLUMN_KEYS = ['name', 'namespace', 'labels', 'dataCount', 'creationTimestamp', 'age'] as const;

const ConfigMapList: React.FC<ConfigMapListProps> = ({ clusterId, onCountChange }) => {
  const { hasFeature } = usePermission();
  const navigate = useNavigate();
  const { token } = theme.useToken();
  const { t } = useTranslation(['config', 'common']);

  const hook = useConfigMapList(clusterId, onCountChange);

  const allColumns = getConfigMapColumns({
    t,
    clusterId,
    sortField: hook.sortField,
    sortOrder: hook.sortOrder,
    colorTextTertiary: token.colorTextTertiary,
    colorTextSecondary: token.colorTextSecondary,
    navigate,
    handleDelete: hook.handleDelete,
    canDelete: hasFeature('config:delete'),
    showActions: hasFeature('config:write') || hasFeature('config:delete'),
  });

  const columns = allColumns.filter(col =>
    col.key === 'actions' || hook.visibleColumns.includes(col.key as string)
  );

  const rowSelection = {
    selectedRowKeys: hook.selectedRowKeys,
    columnWidth: 48,
    onChange: (keys: React.Key[]) => hook.setSelectedRowKeys(keys as string[]),
  };

  return (
    <div>
      {/* Action toolbar */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        {/* 批次操作列 */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: token.marginSM,
          height: 36,
          padding: `0 ${token.paddingSM}px`,
          borderRadius: token.borderRadius,
          background: hook.selectedRowKeys.length > 0 ? token.colorFillAlter : 'transparent',
          opacity: hook.selectedRowKeys.length > 0 ? 1 : 0,
          pointerEvents: hook.selectedRowKeys.length > 0 ? 'auto' : 'none',
          transition: 'opacity 0.2s ease, background 0.2s ease',
        }}>
          <span style={{ color: token.colorTextSecondary, fontSize: token.fontSizeSM }}>
            {t('common:table.selectedCount', { count: hook.selectedRowKeys.length })}
          </span>
          <Divider type="vertical" style={{ margin: 0, height: 14 }} />
          {hasFeature('config:delete') && (
            <Button size="small" danger type="text" icon={<DeleteOutlined />} onClick={hook.handleBatchDelete}>
              {t('common:actions.batchDelete')}
            </Button>
          )}
          {hasFeature('export') && (
            <Button size="small" type="text" icon={<ExportOutlined />} onClick={hook.handleExport}>
              {t('common:actions.export')}
            </Button>
          )}
          <Button
            size="small" type="text"
            icon={<CloseOutlined />}
            onClick={() => hook.setSelectedRowKeys([])}
            style={{ marginLeft: 'auto', color: token.colorTextTertiary }}
          />
        </div>

        {hasFeature('config:write') && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => hook.setCreateModalOpen(true)}
          >
            {t('config:list.createConfigMap')}
          </Button>
        )}
      </div>

      <MultiSearchBar
        fieldOptions={[
          { value: 'name', label: t('config:list.searchFields.name') },
          { value: 'namespace', label: t('config:list.searchFields.namespace') },
          { value: 'label', label: t('config:list.searchFields.label') },
        ]}
        conditions={hook.searchConditions}
        currentField={hook.currentSearchField}
        currentValue={hook.currentSearchValue}
        onFieldChange={hook.setCurrentSearchField}
        onValueChange={hook.setCurrentSearchValue}
        onAdd={hook.addSearchCondition}
        onRemove={hook.removeSearchCondition}
        onClear={hook.clearAllConditions}
        getFieldLabel={hook.getFieldLabel}
        extra={
          <>
            <Button icon={<ReloadOutlined />} onClick={hook.loadConfigMaps} />
            <Button icon={<SettingOutlined />} onClick={() => hook.setColumnSettingsVisible(true)} />
          </>
        }
      />

      <Table
        columns={columns}
        dataSource={hook.configMaps}
        rowKey={(record) => `${record.namespace}/${record.name}`}
        rowSelection={rowSelection}
        loading={hook.loading}
        virtual
        scroll={{ x: 'max-content', y: 600 }}
        size="middle"
        onChange={hook.handleTableChange}
        pagination={{
          current: hook.currentPage,
          pageSize: hook.pageSize,
          total: hook.total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t('config:list.pagination.totalConfigMap', { total }),
          onChange: (page, size) => {
            hook.setCurrentPage(page);
            hook.setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
        locale={{ emptyText: <EmptyState description={t('common:messages.noData')} /> }}
      />

      <ConfigMapForm
        open={hook.createModalOpen}
        clusterId={clusterId}
        onClose={() => hook.setCreateModalOpen(false)}
        onSuccess={() => { hook.setCreateModalOpen(false); hook.loadConfigMaps(); }}
      />

      {/* Column settings drawer */}
      <Drawer
        title={t('common:search.columnSettings')}
        placement="right"
        width={400}
        open={hook.columnSettingsVisible}
        onClose={() => hook.setColumnSettingsVisible(false)}
        footer={
          <div style={{ textAlign: 'right' }}>
            <Space>
              <Button onClick={() => hook.setColumnSettingsVisible(false)}>{t('common:actions.cancel')}</Button>
              <Button type="primary" onClick={hook.handleColumnSettingsSave}>{t('common:actions.confirm')}</Button>
            </Space>
          </div>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8, color: '#666' }}>{t('common:search.selectColumns')}</p>
          <Space direction="vertical" style={{ width: '100%' }}>
            {COLUMN_KEYS.map(key => (
              <Checkbox
                key={key}
                checked={hook.visibleColumns.includes(key)}
                onChange={(e) => hook.toggleColumn(key, e.target.checked)}
              >
                {t(`config:list.columnSettings.${key === 'creationTimestamp' ? 'createdAt' : key}`)}
              </Checkbox>
            ))}
          </Space>
        </div>
      </Drawer>
    </div>
  );
};

export default ConfigMapList;

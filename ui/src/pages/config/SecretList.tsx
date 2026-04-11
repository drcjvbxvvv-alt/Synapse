import React from 'react';
import {
  Table,
  Button,
  Input,
  Space,
  Tag,
  Select,
  Drawer,
  Checkbox,
  theme,
} from 'antd';
import {
  PlusOutlined,
  SearchOutlined,
  ReloadOutlined,
  DeleteOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import SecretForm from './SecretForm';
import { useSecretList } from './hooks/useSecretList';
import { getSecretColumns } from './secretColumns';

const { Option } = Select;

interface SecretListProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const COLUMN_KEYS = ['name', 'namespace', 'type', 'labels', 'dataCount', 'creationTimestamp', 'age'] as const;

const SecretList: React.FC<SecretListProps> = ({ clusterId, onCountChange }) => {
  const navigate = useNavigate();
  const { token } = theme.useToken();
  const { t } = useTranslation(['config', 'common']);

  const hook = useSecretList(clusterId, onCountChange);

  const allColumns = getSecretColumns({
    t,
    clusterId,
    sortField: hook.sortField,
    sortOrder: hook.sortOrder,
    colorTextTertiary: token.colorTextTertiary,
    colorTextSecondary: token.colorTextSecondary,
    navigate,
    handleDelete: hook.handleDelete,
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
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Space>
          <Button
            disabled={hook.selectedRowKeys.length === 0}
            danger
            icon={<DeleteOutlined />}
            onClick={hook.handleBatchDelete}
          >
            {hook.selectedRowKeys.length > 1
              ? `${t('common:actions.batchDelete')} (${hook.selectedRowKeys.length})`
              : t('common:actions.delete')}
          </Button>
          <Button disabled={hook.selectedRowKeys.length === 0} onClick={hook.handleExport}>
            {hook.selectedRowKeys.length > 1
              ? `${t('common:actions.batchExport')} (${hook.selectedRowKeys.length})`
              : t('common:actions.export')}
          </Button>
        </Space>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => hook.setCreateModalOpen(true)}
        >
          {t('config:list.createSecret')}
        </Button>
      </div>

      {/* Search bar */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
          <Input
            prefix={<SearchOutlined />}
            placeholder={t('common:search.placeholder')}
            style={{ flex: 1 }}
            value={hook.currentSearchValue}
            onChange={(e) => hook.setCurrentSearchValue(e.target.value)}
            onPressEnter={hook.addSearchCondition}
            allowClear
            addonBefore={
              <Select value={hook.currentSearchField} onChange={hook.setCurrentSearchField} style={{ width: 120 }}>
                <Option value="name">{t('config:list.searchFields.name')}</Option>
                <Option value="namespace">{t('config:list.searchFields.namespace')}</Option>
                <Option value="type">{t('config:list.searchFields.type')}</Option>
                <Option value="label">{t('config:list.searchFields.label')}</Option>
              </Select>
            }
          />
          <Button icon={<ReloadOutlined />} onClick={hook.loadSecrets} />
          <Button icon={<SettingOutlined />} onClick={() => hook.setColumnSettingsVisible(true)} />
        </div>

        {hook.searchConditions.length > 0 && (
          <Space size="small" wrap>
            {hook.searchConditions.map((condition, index) => (
              <Tag key={index} closable onClose={() => hook.removeSearchCondition(index)} color="blue">
                {hook.getFieldLabel(condition.field)}: {condition.value}
              </Tag>
            ))}
            <Button size="small" type="link" onClick={hook.clearAllConditions} style={{ padding: 0 }}>
              {t('common:actions.clearAll')}
            </Button>
          </Space>
        )}
      </div>

      <Table
        columns={columns}
        dataSource={hook.secrets}
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
          showTotal: (total) => t('config:list.pagination.totalSecret', { total }),
          onChange: (page, size) => {
            hook.setCurrentPage(page);
            hook.setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />

      <SecretForm
        open={hook.createModalOpen}
        clusterId={clusterId}
        onClose={() => hook.setCreateModalOpen(false)}
        onSuccess={() => { hook.setCreateModalOpen(false); hook.loadSecrets(); }}
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

export default SecretList;

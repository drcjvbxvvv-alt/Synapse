import React from 'react';
import { Drawer, Checkbox, Space, Button } from 'antd';
import type { TFunction } from 'i18next';

interface ColumnSettingsDrawerProps {
  open: boolean;
  visibleColumns: string[];
  setVisibleColumns: (columns: string[]) => void;
  onClose: () => void;
  onSave: () => void;
  t: TFunction;
  tc: TFunction;
}

const COLUMN_OPTIONS = [
  { key: 'status', labelKey: 'columns.status' },
  { key: 'name', labelKey: 'columns.name' },
  { key: 'roles', labelKey: 'columns.roles' },
  { key: 'version', labelKey: 'columns.version' },
  { key: 'readyStatus', labelKey: 'columns.readyStatus' },
  { key: 'cpuUsage', labelKey: 'columns.cpu' },
  { key: 'memoryUsage', labelKey: 'columns.memory' },
  { key: 'podCount', labelKey: 'columns.pods' },
  { key: 'taints', labelKey: 'columns.taints' },
  { key: 'createdAt', labelKey: null, commonKey: 'table.createdAt' },
];

export const ColumnSettingsDrawer: React.FC<ColumnSettingsDrawerProps> = ({
  open,
  visibleColumns,
  setVisibleColumns,
  onClose,
  onSave,
  t,
  tc,
}) => {
  const handleColumnToggle = (columnKey: string, checked: boolean) => {
    if (checked) {
      setVisibleColumns([...visibleColumns, columnKey]);
    } else {
      setVisibleColumns(visibleColumns.filter(c => c !== columnKey));
    }
  };

  return (
    <Drawer
      title={t('list.columnSettings')}
      placement="right"
      width={400}
      open={open}
      onClose={onClose}
      footer={
        <div style={{ textAlign: 'right' }}>
          <Space>
            <Button onClick={onClose}>{tc('actions.cancel')}</Button>
            <Button type="primary" onClick={onSave}>{tc('actions.confirm')}</Button>
          </Space>
        </div>
      }
    >
      <div style={{ marginBottom: 16 }}>
        <p style={{ marginBottom: 8, color: '#666' }}>{t('list.selectColumns')}:</p>
        <Space direction="vertical" style={{ width: '100%' }}>
          {COLUMN_OPTIONS.map((option) => (
            <Checkbox
              key={option.key}
              checked={visibleColumns.includes(option.key)}
              onChange={(e) => handleColumnToggle(option.key, e.target.checked)}
            >
              {option.labelKey ? t(option.labelKey) : tc(option.commonKey!)}
            </Checkbox>
          ))}
        </Space>
      </div>
    </Drawer>
  );
};

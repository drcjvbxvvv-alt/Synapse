import React from 'react';
import { Drawer, Checkbox, Space, Button } from 'antd';
import type { TFunction } from 'i18next';

interface WorkloadColumnSettingsDrawerProps {
  open: boolean;
  visibleColumns: string[];
  setVisibleColumns: (columns: string[]) => void;
  onClose: () => void;
  onSave: () => void;
  t: TFunction;
}

const COLUMN_OPTIONS = [
  { key: 'name', labelKey: 'columns.name' },
  { key: 'namespace', labelKey: 'columns.namespace' },
  { key: 'status', labelKey: 'columns.status' },
  { key: 'replicas', labelKey: 'columns.replicas' },
  { key: 'cpuLimit', labelKey: 'columns.cpuLimit' },
  { key: 'cpuRequest', labelKey: 'columns.cpuRequest' },
  { key: 'memoryLimit', labelKey: 'columns.memoryLimit' },
  { key: 'memoryRequest', labelKey: 'columns.memoryRequest' },
  { key: 'images', labelKey: 'columns.images' },
  { key: 'createdAt', labelKey: 'columns.createdAt' },
];

export const WorkloadColumnSettingsDrawer: React.FC<WorkloadColumnSettingsDrawerProps> = ({
  open,
  visibleColumns,
  setVisibleColumns,
  onClose,
  onSave,
  t,
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
      title={t('columnSettings.title')}
      placement="right"
      width={400}
      open={open}
      onClose={onClose}
      footer={
        <div style={{ textAlign: 'right' }}>
          <Space>
            <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
            <Button type="primary" onClick={onSave}>{t('common:actions.confirm')}</Button>
          </Space>
        </div>
      }
    >
      <div style={{ marginBottom: 16 }}>
        <p style={{ marginBottom: 8, color: '#666' }}>{t('columnSettings.selectColumns')}</p>
        <Space direction="vertical" style={{ width: '100%' }}>
          {COLUMN_OPTIONS.map((option) => (
            <Checkbox
              key={option.key}
              checked={visibleColumns.includes(option.key)}
              onChange={(e) => handleColumnToggle(option.key, e.target.checked)}
            >
              {t(option.labelKey)}
            </Checkbox>
          ))}
        </Space>
      </div>
    </Drawer>
  );
};

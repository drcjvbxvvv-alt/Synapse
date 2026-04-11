import React from 'react';
import { Drawer, Button, Space, Checkbox } from 'antd';
import type { TFunction } from 'i18next';

interface ColumnSettingsDrawerProps {
  open: boolean;
  visibleColumns: string[];
  setVisibleColumns: (cols: string[]) => void;
  onClose: () => void;
  onSave: () => void;
  t: TFunction;
}

const COLUMN_KEYS = ['name', 'status', 'labels', 'creationTimestamp'] as const;

const ColumnSettingsDrawer: React.FC<ColumnSettingsDrawerProps> = ({
  open,
  visibleColumns,
  setVisibleColumns,
  onClose,
  onSave,
  t,
}) => (
  <Drawer
    title={t('common:table.columnSettings')}
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
      <p style={{ marginBottom: 8, color: '#666' }}>{t('common:table.selectColumns')}</p>
      <Space direction="vertical" style={{ width: '100%' }}>
        {COLUMN_KEYS.map(col => (
          <Checkbox
            key={col}
            checked={visibleColumns.includes(col)}
            onChange={(e) => {
              if (e.target.checked) {
                setVisibleColumns([...visibleColumns, col]);
              } else {
                setVisibleColumns(visibleColumns.filter(c => c !== col));
              }
            }}
          >
            {t(`columns.${col === 'creationTimestamp' ? 'createdAt' : col}`)}
          </Checkbox>
        ))}
      </Space>
    </div>
  </Drawer>
);

export default ColumnSettingsDrawer;

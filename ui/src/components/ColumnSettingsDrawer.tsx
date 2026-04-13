import React from 'react';
import { Drawer, Button, Space, Checkbox } from 'antd';
import { useTranslation } from 'react-i18next';

export interface ColumnOption {
  key: string;
  label: string;
}

interface ColumnSettingsDrawerProps {
  open: boolean;
  onClose: () => void;
  onSave: () => void;
  visibleColumns: string[];
  onChange: (columns: string[]) => void;
  columnOptions: ColumnOption[];
  title?: string;
  saveLabel?: string;
}

const ColumnSettingsDrawer: React.FC<ColumnSettingsDrawerProps> = ({
  open,
  onClose,
  onSave,
  visibleColumns,
  onChange,
  columnOptions,
  title,
  saveLabel,
}) => {
  const { t } = useTranslation(['storage', 'common']);

  return (
    <Drawer
      title={title ?? t('storage:columnSettings.title')}
      placement="right"
      width={400}
      open={open}
      onClose={onClose}
      footer={
        <div style={{ textAlign: 'right' }}>
          <Space>
            <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
            <Button type="primary" onClick={onSave}>
              {saveLabel ?? t('storage:columnSettings.confirm')}
            </Button>
          </Space>
        </div>
      }
    >
      <div style={{ marginBottom: 16 }}>
        <p style={{ marginBottom: 8, color: '#666' }}>{t('storage:columnSettings.selectColumns')}</p>
        <Space direction="vertical" style={{ width: '100%' }}>
          {columnOptions.map(item => (
            <Checkbox
              key={item.key}
              checked={visibleColumns.includes(item.key)}
              onChange={(e) => {
                if (e.target.checked) {
                  onChange([...visibleColumns, item.key]);
                } else {
                  onChange(visibleColumns.filter(c => c !== item.key));
                }
              }}
            >
              {item.label}
            </Checkbox>
          ))}
        </Space>
      </div>
    </Drawer>
  );
};

export default ColumnSettingsDrawer;

import React from 'react';
import { Drawer, Button, Space, Checkbox } from 'antd';
import { useTranslation } from 'react-i18next';

interface ColumnOption {
  key: string;
  label: string;
}

interface StorageClassColumnSettingsDrawerProps {
  open: boolean;
  visibleColumns: string[];
  onVisibleColumnsChange: (columns: string[]) => void;
  onClose: () => void;
  onSave: () => void;
}

const StorageClassColumnSettingsDrawer: React.FC<StorageClassColumnSettingsDrawerProps> = ({
  open,
  visibleColumns,
  onVisibleColumnsChange,
  onClose,
  onSave,
}) => {
  const { t } = useTranslation(['storage', 'common']);

  const columnOptions: ColumnOption[] = [
    { key: 'provisioner', label: t('storage:columns.provisioner') },
    { key: 'reclaimPolicy', label: t('storage:columns.reclaimPolicy') },
    { key: 'volumeBindingMode', label: t('storage:columns.volumeBindingMode') },
    { key: 'allowVolumeExpansion', label: t('storage:columns.allowVolumeExpansion') },
    { key: 'isDefault', label: t('storage:columns.isDefault') },
    { key: 'createdAt', label: t('common:table.createdAt') },
  ];

  const handleCheckboxChange = (key: string, checked: boolean) => {
    if (checked) {
      onVisibleColumnsChange([...visibleColumns, key]);
    } else {
      onVisibleColumnsChange(visibleColumns.filter(c => c !== key));
    }
  };

  return (
    <Drawer
      title={t('storage:columnSettings.title')}
      placement="right"
      width={400}
      open={open}
      onClose={onClose}
      footer={
        <div style={{ textAlign: 'right' }}>
          <Space>
            <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
            <Button type="primary" onClick={onSave}>
              {t('storage:columnSettings.confirm')}
            </Button>
          </Space>
        </div>
      }
    >
      <div style={{ marginBottom: 16 }}>
        <p style={{ marginBottom: 8, color: '#666' }}>
          {t('storage:columnSettings.selectColumns')}
        </p>
        <Space direction="vertical" style={{ width: '100%' }}>
          {columnOptions.map(item => (
            <Checkbox
              key={item.key}
              checked={visibleColumns.includes(item.key)}
              onChange={(e) => handleCheckboxChange(item.key, e.target.checked)}
            >
              {item.label}
            </Checkbox>
          ))}
        </Space>
      </div>
    </Drawer>
  );
};

export default StorageClassColumnSettingsDrawer;

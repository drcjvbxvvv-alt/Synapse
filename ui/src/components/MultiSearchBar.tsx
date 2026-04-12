import React from 'react';
import { Input, Select, Button, Tag, Space } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { MultiSearchCondition } from '../hooks/useMultiSearch';

export interface FieldOption {
  label: string;
  value: string;
}

interface MultiSearchBarProps {
  /** Field selector options shown in the addonBefore Select. */
  fieldOptions: FieldOption[];
  conditions: MultiSearchCondition[];
  currentField: string;
  currentValue: string;
  onFieldChange: (field: string) => void;
  onValueChange: (value: string) => void;
  /** Called when the user presses Enter or clicks Add. */
  onAdd: () => void;
  onRemove: (index: number) => void;
  onClear: () => void;
  /** Maps field key → human-readable label for the tag pills. */
  getFieldLabel: (field: string) => string;
  /** Extra buttons rendered to the right of the search input (e.g. Reload, Settings). */
  extra?: React.ReactNode;
  /** Width of the field selector Select. Defaults to 120. */
  fieldSelectWidth?: number;
}

/**
 * Reusable multi-condition search bar.
 * Renders a field-selector + value input row, followed by removable tag pills.
 */
export function MultiSearchBar({
  fieldOptions,
  conditions,
  currentField,
  currentValue,
  onFieldChange,
  onValueChange,
  onAdd,
  onRemove,
  onClear,
  getFieldLabel,
  extra,
  fieldSelectWidth = 120,
}: MultiSearchBarProps) {
  const { t } = useTranslation('common');

  return (
    <div style={{ marginBottom: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: conditions.length > 0 ? 8 : 0 }}>
        <Input
          prefix={<SearchOutlined />}
          placeholder={t('common:search.placeholder')}
          style={{ flex: 1 }}
          value={currentValue}
          onChange={(e) => onValueChange(e.target.value)}
          onPressEnter={onAdd}
          allowClear
          addonBefore={
            <Select
              value={currentField}
              onChange={onFieldChange}
              style={{ width: fieldSelectWidth }}
              options={fieldOptions}
            />
          }
        />
        {extra}
      </div>

      {conditions.length > 0 && (
        <Space size="small" wrap>
          {conditions.map((condition, index) => (
            <Tag
              key={index}
              closable
              onClose={() => onRemove(index)}
              color="blue"
            >
              {getFieldLabel(condition.field)}: {condition.value}
            </Tag>
          ))}
          <Button size="small" type="link" onClick={onClear} style={{ padding: 0 }}>
            {t('common:actions.clearAll')}
          </Button>
        </Space>
      )}
    </div>
  );
}

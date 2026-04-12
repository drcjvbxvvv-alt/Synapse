import React, { useCallback } from 'react';
import { Input, Button, Flex, theme } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface KeyValueEditorProps {
  /** Controlled value — Record<string, string> */
  value?: Record<string, string>;
  /** Called on every change */
  onChange?: (value: Record<string, string>) => void;
  /** Placeholder for the key column (default: "Key") */
  keyPlaceholder?: string;
  /** Placeholder for the value column (default: "Value") */
  valuePlaceholder?: string;
  /** Disables all inputs and buttons */
  disabled?: boolean;
  /** Label for the add row button (default: common:actions.create) */
  addLabel?: string;
}

// Internal row type to allow editing keys without collision
interface KVRow {
  id: string;
  key: string;
  value: string;
}

// ─── Helpers ───────────────────────────────────────────────────────────────

function recordToRows(record: Record<string, string>): KVRow[] {
  return Object.entries(record).map(([key, value]) => ({
    id: `${key}-${Math.random()}`,
    key,
    value,
  }));
}

function rowsToRecord(rows: KVRow[]): Record<string, string> {
  const result: Record<string, string> = {};
  for (const row of rows) {
    if (row.key !== '') {
      result[row.key] = row.value;
    }
  }
  return result;
}

// ─── Component ─────────────────────────────────────────────────────────────

/**
 * KeyValueEditor — inline editor for key-value pairs (labels, annotations, env vars).
 *
 * Designed to be used as a Form.Item child or standalone:
 *
 *   <Form.Item name="labels" label={t('form.labels')}>
 *     <KeyValueEditor />
 *   </Form.Item>
 *
 * Or standalone (controlled):
 *   const [labels, setLabels] = useState<Record<string, string>>({});
 *   <KeyValueEditor value={labels} onChange={setLabels} />
 */
export function KeyValueEditor({
  value = {},
  onChange,
  keyPlaceholder = 'Key',
  valuePlaceholder = 'Value',
  disabled = false,
  addLabel,
}: KeyValueEditorProps) {
  const { t } = useTranslation('common');
  const { token } = theme.useToken();

  const [rows, setRows] = React.useState<KVRow[]>(() => recordToRows(value));

  // Sync when value prop changes externally (e.g. form.setFieldsValue)
  React.useEffect(() => {
    setRows(recordToRows(value));
  }, [JSON.stringify(value)]); // eslint-disable-line react-hooks/exhaustive-deps

  const emit = useCallback(
    (next: KVRow[]) => {
      setRows(next);
      onChange?.(rowsToRecord(next));
    },
    [onChange],
  );

  const handleAdd = () => {
    emit([...rows, { id: String(Date.now()), key: '', value: '' }]);
  };

  const handleRemove = (id: string) => {
    emit(rows.filter((r) => r.id !== id));
  };

  const handleKeyChange = (id: string, newKey: string) => {
    emit(rows.map((r) => (r.id === id ? { ...r, key: newKey } : r)));
  };

  const handleValueChange = (id: string, newVal: string) => {
    emit(rows.map((r) => (r.id === id ? { ...r, value: newVal } : r)));
  };

  return (
    <div>
      {rows.map((row) => (
        <Flex key={row.id} gap={token.marginXS} style={{ marginBottom: token.marginXS }}>
          <Input
            placeholder={keyPlaceholder}
            value={row.key}
            disabled={disabled}
            style={{ flex: 1 }}
            onChange={(e) => handleKeyChange(row.id, e.target.value)}
          />
          <Input
            placeholder={valuePlaceholder}
            value={row.value}
            disabled={disabled}
            style={{ flex: 2 }}
            onChange={(e) => handleValueChange(row.id, e.target.value)}
          />
          <Button
            type="text"
            danger
            disabled={disabled}
            icon={<DeleteOutlined />}
            onClick={() => handleRemove(row.id)}
          />
        </Flex>
      ))}

      <Button
        type="dashed"
        icon={<PlusOutlined />}
        disabled={disabled}
        onClick={handleAdd}
        style={{ width: '100%', marginTop: rows.length > 0 ? token.marginXS : 0 }}
      >
        {addLabel ?? t('actions.create', '新增')}
      </Button>
    </div>
  );
}

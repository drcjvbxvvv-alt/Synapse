/**
 * KeyValueEditor — Reusable key/value pair list editor
 * Used for Labels and Annotations sections in Secret/ConfigMap forms.
 */
import React from 'react';
import { Button, Card, Input, Row, Col, Space, Tooltip } from 'antd';
import { PlusOutlined, DeleteOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

interface KVItem {
  key: string;
  value: string;
}

interface KeyValueEditorProps {
  title: string;
  tooltip: string;
  addLabel: string;
  emptyText: string;
  items: KVItem[];
  onAdd: () => void;
  onRemove: (index: number) => void;
  onChange: (index: number, field: 'key' | 'value', value: string) => void;
}

const KeyValueEditor: React.FC<KeyValueEditorProps> = ({
  title, tooltip, addLabel, emptyText, items, onAdd, onRemove, onChange,
}) => {
  const { t } = useTranslation('config');

  return (
    <Card
      title={
        <Space>
          <span>{title}</span>
          <Tooltip title={tooltip}>
            <QuestionCircleOutlined style={{ color: '#999' }} />
          </Tooltip>
        </Space>
      }
      extra={
        <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={onAdd}>
          {addLabel}
        </Button>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }} size="small">
        {items.map((item, index) => (
          <Row key={index} gutter={8} align="middle">
            <Col span={10}>
              <Input
                placeholder={t('create.keyPlaceholder')}
                value={item.key}
                onChange={(e) => onChange(index, 'key', e.target.value)}
              />
            </Col>
            <Col span={10}>
              <Input
                placeholder={t('create.valuePlaceholder')}
                value={item.value}
                onChange={(e) => onChange(index, 'value', e.target.value)}
              />
            </Col>
            <Col span={4}>
              <Button
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={() => onRemove(index)}
              >
                {t('common:actions.delete', { defaultValue: '刪除' })}
              </Button>
            </Col>
          </Row>
        ))}
        {items.length === 0 && (
          <div style={{ textAlign: 'center', color: '#999', padding: '20px' }}>
            {emptyText}
          </div>
        )}
      </Space>
    </Card>
  );
};

export default KeyValueEditor;

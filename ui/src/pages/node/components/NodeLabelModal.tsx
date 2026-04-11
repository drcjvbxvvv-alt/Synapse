import React from 'react';
import { Modal, Form, Input, Space, Button } from 'antd';
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';
import type { FormInstance } from 'antd';
import type { TFunction } from 'i18next';

interface NodeLabelModalProps {
  open: boolean;
  isBatch: boolean;
  selectedCount: number;
  form: FormInstance<{ entries: { key: string; value: string }[] }>;
  submitting: boolean;
  onCancel: () => void;
  onSubmit: () => void;
  tc: TFunction;
}

export const NodeLabelModal: React.FC<NodeLabelModalProps> = ({
  open,
  isBatch,
  selectedCount,
  form,
  submitting,
  onCancel,
  onSubmit,
  tc,
}) => {
  return (
    <Modal
      title={isBatch ? `批次新增標籤（${selectedCount} 個節點）` : '新增標籤'}
      open={open}
      onCancel={onCancel}
      onOk={onSubmit}
      okText="套用"
      cancelText={tc('actions.cancel')}
      confirmLoading={submitting}
    >
      <Form form={form} layout="vertical">
        <Form.List name="entries" initialValue={[{ key: '', value: '' }]}>
          {(fields, { add, remove }) => (
            <>
              {fields.map(field => (
                <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                  <Form.Item
                    {...field}
                    name={[field.name, 'key']}
                    rules={[{ required: true, message: '請輸入鍵名' }]}
                    noStyle
                  >
                    <Input placeholder="key" style={{ width: 160 }} />
                  </Form.Item>
                  <span>=</span>
                  <Form.Item
                    {...field}
                    name={[field.name, 'value']}
                    noStyle
                  >
                    <Input placeholder="value" style={{ width: 160 }} />
                  </Form.Item>
                  {fields.length > 1 && (
                    <MinusCircleOutlined onClick={() => remove(field.name)} />
                  )}
                </Space>
              ))}
              <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />} size="small">
                新增標籤
              </Button>
            </>
          )}
        </Form.List>
      </Form>
    </Modal>
  );
};

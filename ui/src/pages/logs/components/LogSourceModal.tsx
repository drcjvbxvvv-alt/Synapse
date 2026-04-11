import React from 'react';
import { Modal, Form, Input, Select } from 'antd';
import type { FormInstance } from 'antd';
import type { LogSource } from '../../../services/logService';

interface LogSourceModalProps {
  visible: boolean;
  editingSource: LogSource | null;
  form: FormInstance;
  onOk: () => void;
  onCancel: () => void;
}

export const LogSourceModal: React.FC<LogSourceModalProps> = ({
  visible,
  editingSource,
  form,
  onOk,
  onCancel,
}) => {
  return (
    <Modal
      title={editingSource ? '編輯日誌源' : '新增日誌源'}
      open={visible}
      onOk={onOk}
      onCancel={onCancel}
      okText="儲存"
      cancelText="取消"
    >
      <Form form={form} layout="vertical">
        <Form.Item name="type" label="型別" rules={[{ required: true }]}>
          <Select
            options={[
              { label: 'Loki', value: 'loki' },
              { label: 'Elasticsearch', value: 'elasticsearch' },
            ]}
            disabled={!!editingSource}
          />
        </Form.Item>
        <Form.Item name="name" label="名稱" rules={[{ required: true }]}>
          <Input placeholder="如：prod-loki" />
        </Form.Item>
        <Form.Item name="url" label="URL" rules={[{ required: true }]}>
          <Input placeholder="如：http://loki.monitoring:3100" />
        </Form.Item>
        <Form.Item name="username" label="使用者名稱（可選）">
          <Input placeholder="HTTP Basic Auth 使用者名稱" />
        </Form.Item>
        <Form.Item name="password" label="密碼（可選）">
          <Input.Password placeholder="HTTP Basic Auth 密碼" />
        </Form.Item>
        <Form.Item name="apiKey" label="API Key（可選）">
          <Input.Password placeholder="Loki：X-Scope-OrgID；ES：ApiKey" />
        </Form.Item>
      </Form>
    </Modal>
  );
};

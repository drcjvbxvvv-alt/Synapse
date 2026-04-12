import React from 'react';
import { Modal, Form, Input, Select } from 'antd';
import { useTranslation } from 'react-i18next';
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
  const { t } = useTranslation(['logs', 'common']);

  return (
    <Modal
      title={editingSource ? t('logs:form.editLogSource') : t('logs:form.addLogSource')}
      open={visible}
      onOk={onOk}
      onCancel={onCancel}
      okText={t('logs:form.save')}
      cancelText={t('logs:form.cancel')}
    >
      <Form form={form} layout="vertical">
        <Form.Item name="type" label={t('logs:form.type')} rules={[{ required: true }]}>
          <Select
            options={[
              { label: 'Loki', value: 'loki' },
              { label: 'Elasticsearch', value: 'elasticsearch' },
            ]}
            disabled={!!editingSource}
          />
        </Form.Item>
        <Form.Item name="name" label={t('logs:form.name')} rules={[{ required: true }]}>
          <Input placeholder="e.g.: prod-loki" />
        </Form.Item>
        <Form.Item name="url" label={t('logs:form.url')} rules={[{ required: true }]}>
          <Input placeholder="e.g.: http://loki.monitoring:3100" />
        </Form.Item>
        <Form.Item name="username" label={t('logs:form.username')}>
          <Input placeholder="HTTP Basic Auth username" />
        </Form.Item>
        <Form.Item name="password" label={t('logs:form.password')}>
          <Input.Password placeholder="HTTP Basic Auth password" />
        </Form.Item>
        <Form.Item name="apiKey" label={t('logs:form.apiKey')}>
          <Input.Password placeholder="Loki: X-Scope-OrgID; ES: ApiKey" />
        </Form.Item>
      </Form>
    </Modal>
  );
};

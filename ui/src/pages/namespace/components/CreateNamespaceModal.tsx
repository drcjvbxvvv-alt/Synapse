import React from 'react';
import { Modal, Form, Input } from 'antd';
import type { FormInstance } from 'antd';
import type { TFunction } from 'i18next';
import type { CreateNamespaceRequest } from '../../../services/namespaceService';

interface CreateNamespaceModalProps {
  open: boolean;
  form: FormInstance;
  onCancel: () => void;
  onFinish: (values: CreateNamespaceRequest) => void;
  t: TFunction;
}

const CreateNamespaceModal: React.FC<CreateNamespaceModalProps> = ({
  open,
  form,
  onCancel,
  onFinish,
  t,
}) => (
  <Modal
    title={t('list.createNamespace')}
    open={open}
    onCancel={onCancel}
    onOk={() => form.submit()}
    okText={t('common:actions.confirm')}
    cancelText={t('common:actions.cancel')}
    width={600}
  >
    <Form form={form} layout="vertical" onFinish={onFinish} autoComplete="off">
      <Form.Item
        name="name"
        label={t('create.nameLabel')}
        rules={[
          { required: true, message: t('create.nameRequired') },
          {
            pattern: /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/,
            message: t('create.namePattern'),
          },
        ]}
      >
        <Input placeholder={t('namespace:create.namePlaceholder')} />
      </Form.Item>

      <Form.Item name={['annotations', 'description']} label={t('create.descriptionLabel')}>
        <Input.TextArea rows={3} placeholder={t('create.descriptionPlaceholder')} />
      </Form.Item>
    </Form>
  </Modal>
);

export default CreateNamespaceModal;

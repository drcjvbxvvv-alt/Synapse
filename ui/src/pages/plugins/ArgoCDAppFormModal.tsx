import React from 'react';
import {
  Modal,
  Form,
  Input,
  Switch,
  Row,
  Col,
} from 'antd';
import type { FormInstance } from 'antd';
import { useTranslation } from 'react-i18next';

export interface ArgoCDAppFormModalProps {
  open: boolean;
  form: FormInstance;
  creating: boolean;
  onOk: () => void;
  onCancel: () => void;
}

const ArgoCDAppFormModal: React.FC<ArgoCDAppFormModalProps> = ({
  open,
  form,
  creating,
  onOk,
  onCancel,
}) => {
  const { t } = useTranslation(['plugins', 'common']);

  return (
    <Modal
      title={t('plugins:argocd.createAppTitle')}
      open={open}
      onOk={onOk}
      onCancel={onCancel}
      confirmLoading={creating}
      width={600}
      okText={t('plugins:argocd.createBtn')}
      cancelText={t('common:actions.cancel')}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="name"
          label={t('plugins:argocd.appName')}
          rules={[
            { required: true, message: t('plugins:argocd.appNameRequired') },
            { pattern: /^[a-z0-9-]+$/, message: t('plugins:argocd.appNamePattern') }
          ]}
        >
          <Input placeholder="my-app" />
        </Form.Item>

        <Form.Item
          name="path"
          label={t('plugins:argocd.gitPath')}
          rules={[{ required: true, message: t('plugins:argocd.gitPathRequired') }]}
          extra={t('plugins:argocd.gitPathExtra')}
        >
          <Input placeholder="apps/my-app 或 environments/prod/my-app" />
        </Form.Item>

        <Form.Item
          name="target_revision"
          label={t('plugins:argocd.targetRevision')}
          initialValue="HEAD"
          extra={t('plugins:argocd.targetRevisionExtra')}
        >
          <Input placeholder="HEAD, main, v1.0.0, commit SHA" />
        </Form.Item>

        <Form.Item
          name="dest_namespace"
          label={t('plugins:argocd.destNamespace')}
          rules={[{ required: true, message: t('plugins:argocd.destNamespaceRequired') }]}
          extra={t('plugins:argocd.destNamespaceExtra')}
        >
          <Input placeholder="production" />
        </Form.Item>

        <Form.Item
          name="helm_values"
          label={t('plugins:argocd.helmValues')}
          extra={t('plugins:argocd.helmValuesExtra')}
        >
          <Input.TextArea rows={4} placeholder="replicaCount: 3&#10;image:&#10;  tag: latest" />
        </Form.Item>

        <Row gutter={16}>
          <Col span={8}>
            <Form.Item name="auto_sync" label={t('plugins:argocd.autoSync')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) => prevValues.auto_sync !== currentValues.auto_sync}
          >
            {({ getFieldValue }) =>
              getFieldValue('auto_sync') && (
                <>
                  <Col span={8}>
                    <Form.Item
                      name="self_heal"
                      label={t('plugins:argocd.selfHeal')}
                      valuePropName="checked"
                      tooltip={t('plugins:argocd.selfHealTooltip')}
                    >
                      <Switch />
                    </Form.Item>
                  </Col>
                  <Col span={8}>
                    <Form.Item
                      name="prune"
                      label={t('plugins:argocd.autoPrune')}
                      valuePropName="checked"
                      tooltip={t('plugins:argocd.autoPruneTooltip')}
                    >
                      <Switch />
                    </Form.Item>
                  </Col>
                </>
              )
            }
          </Form.Item>
        </Row>
      </Form>
    </Modal>
  );
};

export default ArgoCDAppFormModal;

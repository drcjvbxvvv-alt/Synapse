import React, { useEffect } from 'react';
import {
  Modal,
  Form,
  Input,
  Select,
  InputNumber,
  Switch,
  Button,
  App,
  Row,
  Col,
} from 'antd';
import { useTranslation } from 'react-i18next';
import { useMutation } from '@tanstack/react-query';
import { sloService, type SLO, type CreateSLOPayload, type SLIType, type SLOWindow } from '../../services/sloService';

interface Props {
  open: boolean;
  clusterId: number;
  slo: SLO | null;
  onClose: () => void;
  onSuccess: () => void;
}

const SLOFormModal: React.FC<Props> = ({ open, clusterId, slo, onClose, onSuccess }) => {
  const { message } = App.useApp();
  const { t } = useTranslation(['slo', 'common']);
  const [form] = Form.useForm();
  const isEdit = !!slo;

  // Build options dynamically with i18n
  const windowOptions: { label: string; value: SLOWindow }[] = [
    { label: t('slo:windows.7d'), value: '7d' },
    { label: t('slo:windows.28d'), value: '28d' },
    { label: t('slo:windows.30d'), value: '30d' },
  ];

  const sliTypeOptions: { label: string; value: SLIType }[] = [
    { label: t('slo:sliTypes.availability'), value: 'availability' },
    { label: t('slo:sliTypes.latency'), value: 'latency' },
    { label: t('slo:sliTypes.error_rate'), value: 'error_rate' },
    { label: t('slo:sliTypes.custom'), value: 'custom' },
  ];

  // Reset form when modal opens
  useEffect(() => {
    if (open) {
      if (slo) {
        form.setFieldsValue({
          ...slo,
          target: slo.target * 100, // store as %, display as %
        });
      } else {
        form.resetFields();
        form.setFieldsValue({
          window: '30d',
          sli_type: 'availability',
          burn_rate_warning: 2,
          burn_rate_critical: 10,
          enabled: true,
        });
      }
    }
  }, [open, slo, form]);

  const mutation = useMutation({
    mutationFn: (payload: CreateSLOPayload) =>
      isEdit
        ? sloService.update(clusterId, slo!.id, payload)
        : sloService.create(clusterId, payload),
    onSuccess: () => {
      message.success(isEdit ? t('slo:form.success') : t('slo:form.successCreate'));
      onSuccess();
    },
    onError: (err: Error) => message.error(`${t('slo:form.errorPrefix')}${err.message}`),
  });

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      const payload: CreateSLOPayload = {
        name:               values.name,
        description:        values.description ?? '',
        namespace:          values.namespace ?? '',
        sli_type:           values.sli_type,
        prom_query:         values.prom_query,
        total_query:        values.total_query ?? '',
        target:             values.target / 100, // convert % → ratio
        window:             values.window,
        burn_rate_warning:  values.burn_rate_warning,
        burn_rate_critical: values.burn_rate_critical,
        enabled:            values.enabled ?? true,
      };
      mutation.mutate(payload);
    } catch {
      // validation error — form displays inline messages
    }
  };

  return (
    <Modal
      title={isEdit ? t('slo:form.editTitle') : t('slo:form.newTitle')}
      open={open}
      onCancel={onClose}
      width={680}
      footer={[
        <Button key="cancel" onClick={onClose}>
          {t('common:actions.cancel')}
        </Button>,
        <Button key="submit" type="primary" loading={mutation.isPending} onClick={handleSubmit}>
          {isEdit ? t('common:actions.save') : t('common:actions.create')}
        </Button>,
      ]}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" disabled={mutation.isPending}>
        <Row gutter={16}>
          <Col span={16}>
            <Form.Item
              name="name"
              label={t('slo:form.sloName')}
              rules={[{ required: true, message: t('slo:form.required') }, { max: 255 }]}
            >
              <Input placeholder={t('slo:form.sloNamePlaceholder')} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="namespace" label={t('slo:form.namespace')} tooltip={t('slo:form.namespaceTooltip')}>
              <Input placeholder={t('slo:form.namespacePlaceholder')} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item name="description" label={t('slo:form.description')}>
          <Input.TextArea rows={2} placeholder={t('slo:form.descriptionPlaceholder')} />
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="sli_type"
              label={t('slo:form.sliType')}
              rules={[{ required: true, message: t('slo:form.required') }]}
            >
              <Select options={sliTypeOptions} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="window"
              label={t('slo:form.window')}
              rules={[{ required: true, message: t('slo:form.required') }]}
            >
              <Select options={windowOptions} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item
          name="prom_query"
          label={t('slo:form.promQuery')}
          tooltip={t('slo:form.promQueryTooltip')}
          rules={[{ required: true, message: t('slo:form.required') }]}
        >
          <Input.TextArea
            rows={2}
            placeholder={t('slo:form.promQueryPlaceholder')}
            style={{ fontFamily: 'monospace', fontSize: 12 }}
          />
        </Form.Item>

        <Form.Item
          name="total_query"
          label={t('slo:form.totalQuery')}
          tooltip={t('slo:form.totalQueryTooltip')}
        >
          <Input.TextArea
            rows={2}
            placeholder={t('slo:form.totalQueryPlaceholder')}
            style={{ fontFamily: 'monospace', fontSize: 12 }}
          />
        </Form.Item>

        <Row gutter={16}>
          <Col span={8}>
            <Form.Item
              name="target"
              label={t('slo:form.target')}
              tooltip={t('slo:form.targetTooltip')}
              rules={[
                { required: true, message: t('slo:form.required') },
                { type: 'number', min: 0.001, max: 99.99, message: t('slo:form.targetValidation') },
              ]}
            >
              <InputNumber
                style={{ width: '100%' }}
                min={0.001}
                max={99.99}
                precision={3}
                addonAfter="%"
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="burn_rate_warning"
              label={t('slo:form.burnRateWarning')}
              rules={[{ required: true, message: t('slo:form.required') }]}
            >
              <InputNumber style={{ width: '100%' }} min={1} precision={1} addonAfter="x" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="burn_rate_critical"
              label={t('slo:form.burnRateCritical')}
              rules={[{ required: true, message: t('slo:form.required') }]}
            >
              <InputNumber style={{ width: '100%' }} min={1} precision={1} addonAfter="x" />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item name="enabled" label={t('slo:form.enabled')} valuePropName="checked">
          <Switch />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default SLOFormModal;

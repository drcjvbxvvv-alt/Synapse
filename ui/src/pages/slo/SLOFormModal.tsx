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
import { useMutation } from '@tanstack/react-query';
import { sloService, type SLO, type CreateSLOPayload, type SLIType, type SLOWindow } from '../../services/sloService';

interface Props {
  open: boolean;
  clusterId: number;
  slo: SLO | null;
  onClose: () => void;
  onSuccess: () => void;
}

const SLO_WINDOW_OPTIONS: { label: string; value: SLOWindow }[] = [
  { label: '7 天', value: '7d' },
  { label: '28 天', value: '28d' },
  { label: '30 天', value: '30d' },
];

const SLI_TYPE_OPTIONS: { label: string; value: SLIType }[] = [
  { label: '可用性（Availability）', value: 'availability' },
  { label: '延遲（Latency）', value: 'latency' },
  { label: '錯誤率（Error Rate）', value: 'error_rate' },
  { label: '自定義（Custom）', value: 'custom' },
];

const SLOFormModal: React.FC<Props> = ({ open, clusterId, slo, onClose, onSuccess }) => {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const isEdit = !!slo;

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
      message.success(isEdit ? 'SLO 已更新' : 'SLO 已建立');
      onSuccess();
    },
    onError: (err: Error) => message.error(`操作失敗: ${err.message}`),
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
      title={isEdit ? '編輯 SLO' : '新增 SLO'}
      open={open}
      onCancel={onClose}
      width={680}
      footer={[
        <Button key="cancel" onClick={onClose}>
          取消
        </Button>,
        <Button key="submit" type="primary" loading={mutation.isPending} onClick={handleSubmit}>
          {isEdit ? '儲存' : '建立'}
        </Button>,
      ]}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" disabled={mutation.isPending}>
        <Row gutter={16}>
          <Col span={16}>
            <Form.Item
              name="name"
              label="SLO 名稱"
              rules={[{ required: true, message: '此欄位為必填' }, { max: 255 }]}
            >
              <Input placeholder="例如：api-availability-99.9" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="namespace" label="命名空間" tooltip="空白表示叢集層級">
              <Input placeholder="留空 = 叢集層級" />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item name="description" label="描述">
          <Input.TextArea rows={2} placeholder="可選說明" />
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="sli_type"
              label="SLI 類型"
              rules={[{ required: true, message: '此欄位為必填' }]}
            >
              <Select options={SLI_TYPE_OPTIONS} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="window"
              label="計算視窗"
              rules={[{ required: true, message: '此欄位為必填' }]}
            >
              <Select options={SLO_WINDOW_OPTIONS} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item
          name="prom_query"
          label="PromQL 好事件（或直接比率）"
          tooltip="若 TotalQuery 留空，此式須直接回傳 0.0-1.0 的比率。可用 $window 佔位符。"
          rules={[{ required: true, message: '此欄位為必填' }]}
        >
          <Input.TextArea
            rows={2}
            placeholder="例：sum(rate(http_requests_total{status!~'5..'}[$window])) 或 avg_over_time(up[$window])"
            style={{ fontFamily: 'monospace', fontSize: 12 }}
          />
        </Form.Item>

        <Form.Item
          name="total_query"
          label="TotalQuery（選填）"
          tooltip="填入後：SLI = PromQuery / TotalQuery。可用 $window 佔位符。"
        >
          <Input.TextArea
            rows={2}
            placeholder="例：sum(rate(http_requests_total[$window]))"
            style={{ fontFamily: 'monospace', fontSize: 12 }}
          />
        </Form.Item>

        <Row gutter={16}>
          <Col span={8}>
            <Form.Item
              name="target"
              label="SLO 目標 (%)"
              tooltip="例如 99.9 表示 99.9%"
              rules={[
                { required: true, message: '此欄位為必填' },
                { type: 'number', min: 0.001, max: 99.99, message: '請輸入 0.001 – 99.99' },
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
              label="燃燒率警告倍數"
              rules={[{ required: true, message: '此欄位為必填' }]}
            >
              <InputNumber style={{ width: '100%' }} min={1} precision={1} addonAfter="x" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="burn_rate_critical"
              label="燃燒率嚴重倍數"
              rules={[{ required: true, message: '此欄位為必填' }]}
            >
              <InputNumber style={{ width: '100%' }} min={1} precision={1} addonAfter="x" />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item name="enabled" label="啟用" valuePropName="checked">
          <Switch />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default SLOFormModal;

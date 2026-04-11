import React, { useCallback } from 'react';
import {
  Modal,
  Form,
  Input,
  Select,
  InputNumber,
  Button,
  Divider,
  theme,
  App,
} from 'antd';
import { useTranslation } from 'react-i18next';
import { useMutation } from '@tanstack/react-query';
import {
  chaosService,
  type ChaosKind,
  type CreateScheduleRequest,
} from '../../services/chaosService';

interface Props {
  open: boolean;
  clusterId: string;
  onClose: () => void;
  onSuccess: () => void;
}

type FormValues = {
  kind: ChaosKind;
  name: string;
  namespace: string;
  cron_expr: string;
  duration?: string;
  // PodChaos
  pod_action?: string;
  pod_selector_ns?: string;
  // NetworkChaos
  net_action?: string;
  net_latency?: string;
  net_loss?: string;
  net_duration?: string;
  net_selector_ns?: string;
  // StressChaos
  stress_cpu_workers?: number;
  stress_cpu_load?: number;
  stress_mem_workers?: number;
  stress_mem_size?: string;
  stress_selector_ns?: string;
};

// Common cron presets
const CRON_PRESETS = [
  { label: '每小時',   value: '@every 1h' },
  { label: '每 6 小時', value: '@every 6h' },
  { label: '每天午夜',  value: '0 0 * * *' },
  { label: '每週一',    value: '0 9 * * 1' },
  { label: '每 30 分鐘', value: '@every 30m' },
];

const ScheduleFormModal: React.FC<Props> = ({ open, clusterId, onClose, onSuccess }) => {
  const { t } = useTranslation(['common']);
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const [form] = Form.useForm<FormValues>();
  const kind = Form.useWatch('kind', form) as ChaosKind | undefined;

  const mutation = useMutation({
    mutationFn: (payload: CreateScheduleRequest) =>
      chaosService.createSchedule(clusterId, payload),
    onSuccess: () => {
      message.success(t('common:messages.success'));
      onSuccess();
    },
    onError: () => message.error(t('common:messages.failed')),
  });

  const handleSubmit = useCallback(async () => {
    const values = await form.validateFields();
    const payload: CreateScheduleRequest = {
      kind: values.kind,
      name: values.name,
      namespace: values.namespace,
      cron_expr: values.cron_expr,
      duration: values.duration,
      target: { namespace: values.namespace },
    };

    if (values.kind === 'PodChaos') {
      payload.pod_chaos = {
        action: (values.pod_action ?? 'pod-kill') as 'pod-kill' | 'pod-failure' | 'container-kill',
        mode: 'one',
        selector: { namespace: values.pod_selector_ns ?? values.namespace },
        container_names: [],
      };
      if (values.pod_selector_ns) payload.target.namespace = values.pod_selector_ns;
    } else if (values.kind === 'NetworkChaos') {
      payload.network_chaos = {
        action: (values.net_action ?? 'delay') as 'delay' | 'loss' | 'duplicate' | 'corrupt' | 'bandwidth' | 'partition',
        mode: 'one',
        selector: { namespace: values.net_selector_ns ?? values.namespace },
        ...(values.net_latency ? { delay: { latency: values.net_latency } } : {}),
        ...(values.net_loss    ? { loss:  { loss: values.net_loss } }      : {}),
      };
    } else if (values.kind === 'StressChaos') {
      payload.stress_chaos = {
        mode: 'one',
        selector: { namespace: values.stress_selector_ns ?? values.namespace },
        stressors: {
          ...(values.stress_cpu_workers
            ? { cpu: { workers: values.stress_cpu_workers, load: values.stress_cpu_load } }
            : {}),
          ...(values.stress_mem_workers
            ? { memory: { workers: values.stress_mem_workers, size: values.stress_mem_size } }
            : {}),
        },
      };
    }

    mutation.mutate(payload);
  }, [form, mutation]);

  const handleClose = useCallback(() => {
    form.resetFields();
    onClose();
  }, [form, onClose]);

  return (
    <Modal
      title="建立混沌排程"
      open={open}
      onCancel={handleClose}
      width={640}
      destroyOnHidden
      footer={[
        <Button key="cancel" onClick={handleClose}>
          {t('common:actions.cancel')}
        </Button>,
        <Button
          key="submit"
          type="primary"
          loading={mutation.isPending}
          onClick={handleSubmit}
        >
          {t('common:actions.create')}
        </Button>,
      ]}
    >
      <Form
        form={form}
        layout="vertical"
        disabled={mutation.isPending}
        initialValues={{ kind: 'PodChaos', pod_action: 'pod-kill', net_action: 'delay' }}
      >
        <Form.Item name="namespace" label="Namespace" rules={[{ required: true }]}>
          <Input placeholder="default" />
        </Form.Item>

        <Form.Item name="name" label="排程名稱" rules={[{ required: true }]}>
          <Input placeholder="weekly-pod-kill" />
        </Form.Item>

        <Form.Item
          name="cron_expr"
          label="Cron 表達式"
          rules={[{ required: true, message: t('common:validation.required') }]}
          tooltip="支援標準 cron（0 * * * *）或 @every 縮寫（@every 1h）"
        >
          <Select
            showSearch
            allowClear
            placeholder="選擇預設或自行輸入"
            options={CRON_PRESETS}
            filterOption={false}
            onSearch={(v) => form.setFieldValue('cron_expr', v)}
          />
        </Form.Item>

        <Form.Item name="duration" label="單次實驗持續時間" tooltip="例如 30s、5m；留空表示一直注入到到期">
          <Input placeholder="5m" />
        </Form.Item>

        <Form.Item name="kind" label="實驗類型" rules={[{ required: true }]}>
          <Select
            options={[
              { label: 'PodChaos',     value: 'PodChaos' },
              { label: 'NetworkChaos', value: 'NetworkChaos' },
              { label: 'StressChaos',  value: 'StressChaos' },
            ]}
          />
        </Form.Item>

        {/* PodChaos */}
        {kind === 'PodChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>PodChaos 設定</Divider>
            <Form.Item name="pod_action" label="故障動作" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: 'pod-kill',       value: 'pod-kill' },
                  { label: 'pod-failure',    value: 'pod-failure' },
                  { label: 'container-kill', value: 'container-kill' },
                ]}
              />
            </Form.Item>
            <Form.Item name="pod_selector_ns" label="目標 Namespace（留空同上）">
              <Input />
            </Form.Item>
          </>
        )}

        {/* NetworkChaos */}
        {kind === 'NetworkChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>NetworkChaos 設定</Divider>
            <Form.Item name="net_action" label="故障動作" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: 'delay — 延遲', value: 'delay' },
                  { label: 'loss — 丟包',  value: 'loss' },
                ]}
              />
            </Form.Item>
            <Form.Item name="net_latency" label="延遲">
              <Input placeholder="100ms" />
            </Form.Item>
            <Form.Item name="net_loss" label="丟包率">
              <Input placeholder="50%" />
            </Form.Item>
          </>
        )}

        {/* StressChaos */}
        {kind === 'StressChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>StressChaos 設定</Divider>
            <Form.Item name="stress_cpu_workers" label="CPU Workers">
              <InputNumber min={1} max={256} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="stress_cpu_load" label="CPU 負載 %">
              <InputNumber min={0} max={100} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="stress_mem_workers" label="Memory Workers">
              <InputNumber min={1} max={256} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="stress_mem_size" label="Memory 大小">
              <Input placeholder="256MB" />
            </Form.Item>
          </>
        )}

        {!kind && (
          <span style={{ color: token.colorTextTertiary, fontSize: token.fontSizeSM }}>
            請先選擇實驗類型
          </span>
        )}
      </Form>
    </Modal>
  );
};

export default ScheduleFormModal;

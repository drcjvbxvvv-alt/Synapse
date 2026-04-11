import React, { useState, useCallback } from 'react';
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
  type CreateChaosRequest,
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
  // PodChaos
  pod_action?: string;
  pod_mode?: string;
  pod_value?: string;
  pod_duration?: string;
  pod_selector_ns?: string;
  // NetworkChaos
  net_action?: string;
  net_mode?: string;
  net_duration?: string;
  net_latency?: string;
  net_loss?: string;
  net_selector_ns?: string;
  // StressChaos
  stress_mode?: string;
  stress_duration?: string;
  stress_cpu_workers?: number;
  stress_cpu_load?: number;
  stress_mem_workers?: number;
  stress_mem_size?: string;
  stress_selector_ns?: string;
};

const ChaosFormModal: React.FC<Props> = ({ open, clusterId, onClose, onSuccess }) => {
  const { t } = useTranslation(['common']);
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const [form] = Form.useForm<FormValues>();
  const [kind, setKind] = useState<ChaosKind>('PodChaos');

  const mutation = useMutation({
    mutationFn: (payload: CreateChaosRequest) =>
      chaosService.createExperiment(clusterId, payload),
    onSuccess: () => {
      message.success(t('common:messages.success'));
      onSuccess();
    },
    onError: () => message.error(t('common:messages.failed')),
  });

  const handleSubmit = useCallback(async () => {
    const values = await form.validateFields();
    const payload: CreateChaosRequest = {
      kind: values.kind,
      name: values.name,
      namespace: values.namespace,
    };

    if (values.kind === 'PodChaos') {
      payload.pod_chaos = {
        action: values.pod_action as 'pod-kill' | 'pod-failure' | 'container-kill',
        mode: (values.pod_mode ?? 'one') as 'one' | 'all' | 'fixed' | 'fixed-percent' | 'random-max-percent',
        value: values.pod_value,
        duration: values.pod_duration,
        selector: { namespace: values.pod_selector_ns },
      };
    } else if (values.kind === 'NetworkChaos') {
      payload.network_chaos = {
        action: values.net_action as 'delay' | 'loss' | 'duplicate' | 'corrupt' | 'bandwidth' | 'partition',
        mode: values.net_mode ?? 'one',
        duration: values.net_duration,
        selector: { namespace: values.net_selector_ns },
        ...(values.net_latency ? { delay: { latency: values.net_latency } } : {}),
        ...(values.net_loss    ? { loss: { loss: values.net_loss } }       : {}),
      };
    } else if (values.kind === 'StressChaos') {
      payload.stress_chaos = {
        mode: values.stress_mode ?? 'one',
        duration: values.stress_duration,
        selector: { namespace: values.stress_selector_ns },
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
    setKind('PodChaos');
    onClose();
  }, [form, onClose]);

  return (
    <Modal
      title="建立混沌實驗"
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
        initialValues={{ kind: 'PodChaos', pod_mode: 'one', net_mode: 'one', stress_mode: 'one' }}
      >
        {/* ── Common fields ── */}
        <Form.Item
          name="kind"
          label="實驗類型"
          rules={[{ required: true }]}
        >
          <Select
            options={[
              { label: 'PodChaos — Pod 故障',         value: 'PodChaos' },
              { label: 'NetworkChaos — 網路故障',      value: 'NetworkChaos' },
              { label: 'StressChaos — 資源壓力',       value: 'StressChaos' },
              { label: 'HTTPChaos — HTTP 故障',        value: 'HTTPChaos' },
              { label: 'IOChaos — I/O 故障',           value: 'IOChaos' },
            ]}
            onChange={(v) => setKind(v)}
          />
        </Form.Item>

        <Form.Item
          name="namespace"
          label="Namespace"
          rules={[{ required: true, message: t('common:validation.required') }]}
        >
          <Input placeholder="default" />
        </Form.Item>

        <Form.Item
          name="name"
          label="實驗名稱"
          rules={[{ required: true, message: t('common:validation.required') }]}
        >
          <Input placeholder="my-chaos-experiment" />
        </Form.Item>

        {/* ── PodChaos ── */}
        {kind === 'PodChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>PodChaos 設定</Divider>
            <Form.Item name="pod_action" label="故障動作" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: 'pod-kill — 殺掉 Pod',       value: 'pod-kill' },
                  { label: 'pod-failure — Pod 故障',    value: 'pod-failure' },
                  { label: 'container-kill — 殺掉容器', value: 'container-kill' },
                ]}
              />
            </Form.Item>
            <Form.Item name="pod_selector_ns" label="目標 Namespace">
              <Input placeholder="留空表示與實驗相同" />
            </Form.Item>
            <Form.Item name="pod_mode" label="選取模式">
              <Select
                options={[
                  { label: 'one — 隨機一個', value: 'one' },
                  { label: 'all — 全部',     value: 'all' },
                  { label: 'fixed — 固定數量', value: 'fixed' },
                ]}
              />
            </Form.Item>
            <Form.Item name="pod_duration" label="持續時間" tooltip="例如 30s、5m、1h">
              <Input placeholder="30s" />
            </Form.Item>
          </>
        )}

        {/* ── NetworkChaos ── */}
        {kind === 'NetworkChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>NetworkChaos 設定</Divider>
            <Form.Item name="net_action" label="故障動作" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: 'delay — 延遲',      value: 'delay' },
                  { label: 'loss — 丟包',       value: 'loss' },
                  { label: 'duplicate — 重複包', value: 'duplicate' },
                  { label: 'corrupt — 損壞包',  value: 'corrupt' },
                  { label: 'partition — 分區',  value: 'partition' },
                ]}
              />
            </Form.Item>
            <Form.Item name="net_selector_ns" label="目標 Namespace">
              <Input placeholder="留空表示與實驗相同" />
            </Form.Item>
            <Form.Item name="net_latency" label="延遲（latency）" tooltip="例如 100ms、1s">
              <Input placeholder="100ms" />
            </Form.Item>
            <Form.Item name="net_loss" label="丟包率（loss）" tooltip="例如 50%">
              <Input placeholder="50%" />
            </Form.Item>
            <Form.Item name="net_duration" label="持續時間">
              <Input placeholder="1m" />
            </Form.Item>
          </>
        )}

        {/* ── StressChaos ── */}
        {kind === 'StressChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>StressChaos 設定</Divider>
            <Form.Item name="stress_selector_ns" label="目標 Namespace">
              <Input placeholder="留空表示與實驗相同" />
            </Form.Item>
            <Form.Item name="stress_cpu_workers" label="CPU 壓力 Workers">
              <InputNumber min={1} max={256} style={{ width: '100%' }} placeholder="4" />
            </Form.Item>
            <Form.Item
              name="stress_cpu_load"
              label="CPU 負載 %"
              tooltip="0–100，100 表示 100%"
            >
              <InputNumber min={0} max={100} style={{ width: '100%' }} placeholder="80" />
            </Form.Item>
            <Form.Item name="stress_mem_workers" label="Memory 壓力 Workers">
              <InputNumber min={1} max={256} style={{ width: '100%' }} placeholder="2" />
            </Form.Item>
            <Form.Item name="stress_mem_size" label="Memory 大小" tooltip="例如 256MB、1GB">
              <Input placeholder="256MB" />
            </Form.Item>
            <Form.Item name="stress_duration" label="持續時間">
              <Input placeholder="5m" />
            </Form.Item>
          </>
        )}

        {/* Unsupported kinds hint */}
        {(kind === 'HTTPChaos' || kind === 'IOChaos') && (
          <Form.Item>
            <span style={{ color: token.colorTextSecondary, fontSize: token.fontSizeSM }}>
              {kind} 的進階設定請直接透過 kubectl apply 或 Chaos Mesh Dashboard 建立。
            </span>
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
};

export default ChaosFormModal;

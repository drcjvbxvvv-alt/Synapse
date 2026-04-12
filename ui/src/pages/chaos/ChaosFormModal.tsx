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
  const { t } = useTranslation(['chaos', 'common']);
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
      title={t('chaos:form.createExperiment')}
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
          label={t('chaos:form.kind')}
          rules={[{ required: true }]}
        >
          <Select
            options={[
              { label: t('chaos:kinds.PodChaos'),     value: 'PodChaos' },
              { label: t('chaos:kinds.NetworkChaos'), value: 'NetworkChaos' },
              { label: t('chaos:kinds.StressChaos'),  value: 'StressChaos' },
              { label: t('chaos:kinds.HTTPChaos'),    value: 'HTTPChaos' },
              { label: t('chaos:kinds.IOChaos'),      value: 'IOChaos' },
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
          label={t('chaos:form.name')}
          rules={[{ required: true, message: t('common:validation.required') }]}
        >
          <Input placeholder="my-chaos-experiment" />
        </Form.Item>

        {/* ── PodChaos ── */}
        {kind === 'PodChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>{t('chaos:form.podConfig')}</Divider>
            <Form.Item name="pod_action" label={t('chaos:form.action')} rules={[{ required: true }]}>
              <Select
                options={[
                  { label: t('chaos:podActions.podKill'),       value: 'pod-kill' },
                  { label: t('chaos:podActions.podFailure'),    value: 'pod-failure' },
                  { label: t('chaos:podActions.containerKill'), value: 'container-kill' },
                ]}
              />
            </Form.Item>
            <Form.Item name="pod_selector_ns" label={t('chaos:form.targetNs')}>
              <Input placeholder={t('chaos:form.targetNsSame')} />
            </Form.Item>
            <Form.Item name="pod_mode" label={t('chaos:form.mode')}>
              <Select
                options={[
                  { label: t('chaos:modes.one'),   value: 'one' },
                  { label: t('chaos:modes.all'),   value: 'all' },
                  { label: t('chaos:modes.fixed'), value: 'fixed' },
                ]}
              />
            </Form.Item>
            <Form.Item name="pod_duration" label={t('chaos:form.duration')} tooltip={t('chaos:form.durationTooltip')}>
              <Input placeholder="30s" />
            </Form.Item>
          </>
        )}

        {/* ── NetworkChaos ── */}
        {kind === 'NetworkChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>{t('chaos:form.networkConfig')}</Divider>
            <Form.Item name="net_action" label={t('chaos:form.action')} rules={[{ required: true }]}>
              <Select
                options={[
                  { label: t('chaos:netActions.delay'),     value: 'delay' },
                  { label: t('chaos:netActions.loss'),      value: 'loss' },
                  { label: t('chaos:netActions.duplicate'), value: 'duplicate' },
                  { label: t('chaos:netActions.corrupt'),   value: 'corrupt' },
                  { label: t('chaos:netActions.partition'), value: 'partition' },
                ]}
              />
            </Form.Item>
            <Form.Item name="net_selector_ns" label={t('chaos:form.targetNs')}>
              <Input placeholder={t('chaos:form.targetNsSame')} />
            </Form.Item>
            <Form.Item name="net_latency" label={t('chaos:form.netLatency')} tooltip={t('chaos:form.netLatencyTooltip')}>
              <Input placeholder="100ms" />
            </Form.Item>
            <Form.Item name="net_loss" label={t('chaos:form.netLoss')} tooltip={t('chaos:form.netLossTooltip')}>
              <Input placeholder="50%" />
            </Form.Item>
            <Form.Item name="net_duration" label={t('chaos:form.duration')}>
              <Input placeholder="1m" />
            </Form.Item>
          </>
        )}

        {/* ── StressChaos ── */}
        {kind === 'StressChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>{t('chaos:form.stressConfig')}</Divider>
            <Form.Item name="stress_selector_ns" label={t('chaos:form.targetNs')}>
              <Input placeholder={t('chaos:form.targetNsSame')} />
            </Form.Item>
            <Form.Item name="stress_cpu_workers" label={t('chaos:form.cpuWorkers')}>
              <InputNumber min={1} max={256} style={{ width: '100%' }} placeholder="4" />
            </Form.Item>
            <Form.Item
              name="stress_cpu_load"
              label={t('chaos:form.cpuLoad')}
              tooltip={t('chaos:form.cpuLoadTooltip')}
            >
              <InputNumber min={0} max={100} style={{ width: '100%' }} placeholder="80" />
            </Form.Item>
            <Form.Item name="stress_mem_workers" label={t('chaos:form.memWorkers')}>
              <InputNumber min={1} max={256} style={{ width: '100%' }} placeholder="2" />
            </Form.Item>
            <Form.Item name="stress_mem_size" label={t('chaos:form.memSize')} tooltip={t('chaos:form.memSizeTooltip')}>
              <Input placeholder="256MB" />
            </Form.Item>
            <Form.Item name="stress_duration" label={t('chaos:form.duration')}>
              <Input placeholder="5m" />
            </Form.Item>
          </>
        )}

        {/* Unsupported kinds hint */}
        {(kind === 'HTTPChaos' || kind === 'IOChaos') && (
          <Form.Item>
            <span style={{ color: token.colorTextSecondary, fontSize: token.fontSizeSM }}>
              {t('chaos:form.unsupportedHint', { kind })}
            </span>
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
};

export default ChaosFormModal;

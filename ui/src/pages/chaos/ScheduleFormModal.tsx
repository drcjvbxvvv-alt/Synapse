import React, { useCallback, useMemo } from 'react';
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

const ScheduleFormModal: React.FC<Props> = ({ open, clusterId, onClose, onSuccess }) => {
  const { t } = useTranslation(['chaos', 'common']);
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const [form] = Form.useForm<FormValues>();
  const kind = Form.useWatch('kind', form) as ChaosKind | undefined;

  const cronPresets = useMemo(() => [
    { label: t('chaos:cronPresets.everyHour'),     value: '@every 1h' },
    { label: t('chaos:cronPresets.every6Hours'),   value: '@every 6h' },
    { label: t('chaos:cronPresets.midnightDaily'), value: '0 0 * * *' },
    { label: t('chaos:cronPresets.mondayMorning'), value: '0 9 * * 1' },
    { label: t('chaos:cronPresets.every30Min'),    value: '@every 30m' },
  ], [t]);

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
      title={t('chaos:form.createSchedule')}
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

        <Form.Item name="name" label={t('chaos:form.scheduleName')} rules={[{ required: true }]}>
          <Input placeholder="weekly-pod-kill" />
        </Form.Item>

        <Form.Item
          name="cron_expr"
          label={t('chaos:form.cronExpr')}
          rules={[{ required: true, message: t('common:validation.required') }]}
          tooltip={t('chaos:form.cronTooltip')}
        >
          <Select
            showSearch
            allowClear
            placeholder={t('chaos:form.cronPlaceholder')}
            options={cronPresets}
            filterOption={false}
            onSearch={(v) => form.setFieldValue('cron_expr', v)}
          />
        </Form.Item>

        <Form.Item name="duration" label={t('chaos:form.experimentDuration')} tooltip={t('chaos:form.experimentDurationTooltip')}>
          <Input placeholder="5m" />
        </Form.Item>

        <Form.Item name="kind" label={t('chaos:form.kind')} rules={[{ required: true }]}>
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
            <Form.Item name="pod_selector_ns" label={t('chaos:form.targetNsSameShort')}>
              <Input />
            </Form.Item>
          </>
        )}

        {/* NetworkChaos */}
        {kind === 'NetworkChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>{t('chaos:form.networkConfig')}</Divider>
            <Form.Item name="net_action" label={t('chaos:form.action')} rules={[{ required: true }]}>
              <Select
                options={[
                  { label: t('chaos:netActions.delay'), value: 'delay' },
                  { label: t('chaos:netActions.loss'),  value: 'loss' },
                ]}
              />
            </Form.Item>
            <Form.Item name="net_latency" label={t('chaos:form.netLatency')}>
              <Input placeholder="100ms" />
            </Form.Item>
            <Form.Item name="net_loss" label={t('chaos:form.netLoss')}>
              <Input placeholder="50%" />
            </Form.Item>
          </>
        )}

        {/* StressChaos */}
        {kind === 'StressChaos' && (
          <>
            <Divider style={{ marginTop: 0 }}>{t('chaos:form.stressConfig')}</Divider>
            <Form.Item name="stress_cpu_workers" label={t('chaos:form.cpuWorkers')}>
              <InputNumber min={1} max={256} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="stress_cpu_load" label={t('chaos:form.cpuLoad')}>
              <InputNumber min={0} max={100} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="stress_mem_workers" label={t('chaos:form.memWorkers')}>
              <InputNumber min={1} max={256} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="stress_mem_size" label={t('chaos:form.memSize')}>
              <Input placeholder="256MB" />
            </Form.Item>
          </>
        )}

        {!kind && (
          <span style={{ color: token.colorTextTertiary, fontSize: token.fontSizeSM }}>
            {t('chaos:form.selectKindFirst')}
          </span>
        )}
      </Form>
    </Modal>
  );
};

export default ScheduleFormModal;

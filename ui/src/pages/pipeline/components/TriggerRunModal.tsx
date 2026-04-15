import React from 'react';
import { Modal, Form, Select, Input, App } from 'antd';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { clusterService } from '../../../services/clusterService';
import pipelineService from '../../../services/pipelineService';
import type { Pipeline } from '../../../services/pipelineService';

interface TriggerRunModalProps {
  open: boolean;
  onClose: () => void;
  pipeline: Pipeline;
}

const TriggerRunModal: React.FC<TriggerRunModalProps> = ({ open, onClose, pipeline }) => {
  const { t } = useTranslation(['cicd', 'common']);
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [form] = Form.useForm();

  const { data: clustersData, isLoading: clustersLoading } = useQuery({
    queryKey: ['clusters-list'],
    queryFn: () => clusterService.getClusters({ pageSize: 200 }),
    staleTime: 60_000,
    enabled: open,
  });

  const clusterOptions = (clustersData?.items ?? []).map((c) => ({
    label: c.name,
    value: Number(c.id),
  }));

  const triggerMutation = useMutation({
    mutationFn: (values: { cluster_id: number; namespace: string }) =>
      pipelineService.triggerRun(pipeline.id, values),
    onSuccess: (data) => {
      message.success(t('cicd:run.triggered', { id: data.run_id }));
      queryClient.invalidateQueries({ queryKey: ['pipeline-runs', pipeline.id] });
      onClose();
      form.resetFields();
    },
    onError: () => message.error(t('cicd:run.triggerFailed')),
  });

  const handleOk = async () => {
    const values = await form.validateFields();
    triggerMutation.mutate(values);
  };

  const handleCancel = () => {
    onClose();
    form.resetFields();
  };

  return (
    <Modal
      title={t('cicd:run.triggerTitle', { name: pipeline.name })}
      open={open}
      onCancel={handleCancel}
      onOk={handleOk}
      okText={t('cicd:run.trigger')}
      cancelText={t('common:actions.cancel')}
      confirmLoading={triggerMutation.isPending}
      destroyOnHidden
      width={480}
    >
      <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
        <Form.Item
          name="cluster_id"
          label={t('cicd:run.targetCluster')}
          rules={[{ required: true, message: t('common:validation.required') }]}
        >
          <Select
            options={clusterOptions}
            loading={clustersLoading}
            showSearch
            placeholder={t('cicd:run.selectCluster')}
            filterOption={(input, opt) =>
              String(opt?.label ?? '').toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item
          name="namespace"
          label={t('cicd:run.targetNamespace')}
          rules={[{ required: true, message: t('common:validation.required') }]}
        >
          <Input placeholder="e.g. app-dev" />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default TriggerRunModal;

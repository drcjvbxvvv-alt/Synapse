/**
 * TriggerRunModal — Modal for triggering a CI engine run (M19c).
 *
 * On success it calls onTriggered(runId) so the parent can open RunViewer.
 */
import React from 'react';
import { Modal, Form, Input, App, theme } from 'antd';
import { useTranslation } from 'react-i18next';
import { useMutation } from '@tanstack/react-query';

import ciEngineService, {
  type TriggerRunRequest,
} from '../../../services/ciEngineService';

// ─── Props ───────────────────────────────────────────────────────────────────

export interface TriggerRunModalProps {
  open: boolean;
  engineId: number;
  engineName: string;
  onClose: () => void;
  onTriggered: (runId: string) => void;
}

// ─── Component ───────────────────────────────────────────────────────────────

const TriggerRunModal: React.FC<TriggerRunModalProps> = ({
  open,
  engineId,
  engineName,
  onClose,
  onTriggered,
}) => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['cicd', 'common']);
  const [form] = Form.useForm();

  const triggerMutation = useMutation({
    mutationFn: (req: TriggerRunRequest) => ciEngineService.triggerRun(engineId, req),
    onSuccess: (result) => {
      message.success(t('cicd:ciEngine.triggerRun.triggerSuccess', { runId: result.run_id }));
      form.resetFields();
      onTriggered(result.run_id);
    },
    onError: () => message.error(t('cicd:ciEngine.triggerRun.triggerFailed')),
  });

  const handleOk = async () => {
    const values = await form.validateFields();
    const req: TriggerRunRequest = {
      ref: values.ref || undefined,
    };

    if (values.variables) {
      const lines = (values.variables as string).split('\n').filter(Boolean);
      const vars: Record<string, string> = {};
      for (const line of lines) {
        const idx = line.indexOf('=');
        if (idx < 1) {
          message.error(t('cicd:ciEngine.triggerRun.variablesParseError'));
          return;
        }
        vars[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
      }
      if (Object.keys(vars).length > 0) {
        req.variables = vars;
      }
    }

    triggerMutation.mutate(req);
  };

  const handleCancel = () => {
    form.resetFields();
    onClose();
  };

  return (
    <Modal
      title={t('cicd:ciEngine.triggerRun.title', { name: engineName })}
      open={open}
      onCancel={handleCancel}
      onOk={handleOk}
      okText={t('cicd:ciEngine.triggerRun.trigger')}
      cancelText={t('common:actions.cancel')}
      confirmLoading={triggerMutation.isPending}
      destroyOnHidden
      width={480}
    >
      <Form
        form={form}
        layout="vertical"
        style={{ marginTop: token.marginMD }}
      >
        <Form.Item
          name="ref"
          label={t('cicd:ciEngine.triggerRun.ref')}
        >
          <Input placeholder={t('cicd:ciEngine.triggerRun.refPlaceholder')} />
        </Form.Item>
        <Form.Item
          name="variables"
          label={t('cicd:ciEngine.triggerRun.variables')}
          tooltip={t('cicd:ciEngine.triggerRun.variablesPlaceholder')}
        >
          <Input.TextArea
            rows={4}
            placeholder={t('cicd:ciEngine.triggerRun.variablesPlaceholder')}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default TriggerRunModal;

import React, { useEffect } from 'react';
import { Modal, Button, Form } from 'antd';
import type { FormInstance } from 'antd';
import { useTranslation } from 'react-i18next';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface FormModalProps {
  /** Controls visibility */
  open: boolean;
  /** Called when the user cancels or closes the modal */
  onClose: () => void;
  /**
   * Called when the submit button is clicked.
   * Should call form.validateFields() and perform the mutation.
   * The modal stays open until this resolves/rejects.
   */
  onSubmit: () => Promise<void>;
  /** Ant Design form instance (from Form.useForm()) */
  form: FormInstance;
  /** Determines title and submit button label (create vs save) */
  isEdit?: boolean;
  /** Custom modal title — overrides the auto create/edit title */
  title?: string;
  /** Title used when isEdit=false (defaults to common:actions.create) */
  createTitle?: string;
  /** Title used when isEdit=true (defaults to common:actions.edit) */
  editTitle?: string;
  /** Modal width (default: 640) */
  width?: number;
  /** External loading state (merged with internal submit loading) */
  loading?: boolean;
  children: React.ReactNode;
}

// ─── Component ─────────────────────────────────────────────────────────────

/**
 * FormModal — standardised create/edit modal wrapper.
 *
 * Enforces:
 *   - layout="vertical" Form
 *   - destroyOnHide — form fields are reset on close
 *   - Custom footer with cancel + submit
 *   - Correct button labels for create vs edit mode
 *
 * Usage:
 *   const [form] = Form.useForm();
 *
 *   <FormModal
 *     open={visible}
 *     onClose={() => setVisible(false)}
 *     onSubmit={async () => {
 *       const values = await form.validateFields();
 *       await createMutation.mutateAsync(values);
 *     }}
 *     form={form}
 *     isEdit={!!editingItem}
 *   >
 *     <Form.Item name="name" label={t('form.name')} rules={[...]}>
 *       <Input />
 *     </Form.Item>
 *   </FormModal>
 */
export function FormModal({
  open,
  onClose,
  onSubmit,
  form,
  isEdit = false,
  title,
  createTitle,
  editTitle,
  width = 640,
  loading = false,
  children,
}: FormModalProps) {
  const { t } = useTranslation('common');
  const [submitting, setSubmitting] = React.useState(false);

  // Reset form state when modal opens/closes
  useEffect(() => {
    if (!open) {
      // Small delay to avoid flash of reset state while close animation plays
      const id = setTimeout(() => form.resetFields(), 300);
      return () => clearTimeout(id);
    }
  }, [open, form]);

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      await onSubmit();
    } finally {
      setSubmitting(false);
    }
  };

  const resolvedTitle =
    title ??
    (isEdit
      ? (editTitle ?? t('actions.edit', '編輯'))
      : (createTitle ?? t('actions.create', '新增')));

  const isLoading = submitting || loading;

  return (
    <Modal
      title={resolvedTitle}
      open={open}
      onCancel={onClose}
      width={width}
      destroyOnHidden
      footer={[
        <Button key="cancel" onClick={onClose} disabled={isLoading}>
          {t('actions.cancel', '取消')}
        </Button>,
        <Button
          key="submit"
          type="primary"
          loading={isLoading}
          onClick={handleSubmit}
        >
          {isEdit ? t('actions.save', '儲存') : t('actions.create', '新增')}
        </Button>,
      ]}
    >
      <Form form={form} layout="vertical" disabled={isLoading}>
        {children}
      </Form>
    </Modal>
  );
}

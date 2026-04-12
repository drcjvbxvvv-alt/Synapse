import React from 'react';
import { Modal, Space, Typography } from 'antd';
import { DiffOutlined } from '@ant-design/icons';
import { DiffEditor } from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export interface YAMLDiffModalProps {
  open: boolean;
  /** Original (left-side) YAML content. */
  original: string;
  /** Modified (right-side) YAML content. */
  modified: string;
  onConfirm: () => void;
  onCancel: () => void;
  confirmLoading?: boolean;
  /** Override the modal title. */
  title?: string;
}

/**
 * Read-only side-by-side YAML diff modal.
 * Shows original vs modified YAML and asks the user to confirm the change.
 */
export function YAMLDiffModal({
  open,
  original,
  modified,
  onConfirm,
  onCancel,
  confirmLoading,
  title,
}: YAMLDiffModalProps) {
  const { t } = useTranslation('components');

  return (
    <Modal
      title={
        <Space>
          <DiffOutlined />
          <span>{title ?? t('resourceYAMLEditor.diffTitle')}</span>
        </Space>
      }
      open={open}
      onCancel={onCancel}
      onOk={onConfirm}
      confirmLoading={confirmLoading}
      okText={t('resourceYAMLEditor.confirmUpdate')}
      cancelText={t('resourceYAMLEditor.cancel')}
      width="90%"
      style={{ top: 20 }}
      destroyOnHidden
    >
      <div style={{ marginBottom: 12 }}>
        <Text type="secondary">{t('resourceYAMLEditor.diffHint')}</Text>
      </div>
      <div style={{ border: '1px solid #d9d9d9', borderRadius: 4 }}>
        <DiffEditor
          height="60vh"
          language="yaml"
          original={original}
          modified={modified}
          options={{
            readOnly: true,
            minimap: { enabled: false },
            fontSize: 13,
            lineNumbers: 'on',
            wordWrap: 'on',
            automaticLayout: true,
            scrollBeyondLastLine: false,
            renderSideBySide: true,
            diffWordWrap: 'on',
          }}
        />
      </div>
    </Modal>
  );
}

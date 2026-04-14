import React from 'react';
import { Modal, Space, Typography } from 'antd';
import { DiffOutlined } from '@ant-design/icons';
import { DiffEditor } from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

interface DeploymentDiffModalProps {
  open: boolean;
  originalYaml: string;
  modifiedYaml: string;
  onCancel: () => void;
  onConfirm: () => void;
}

const DeploymentDiffModal: React.FC<DeploymentDiffModalProps> = ({
  open,
  originalYaml,
  modifiedYaml,
  onCancel,
  onConfirm,
}) => {
  const { t } = useTranslation(['workload', 'common']);

  return (
    <Modal
      title={
        <Space>
          <DiffOutlined />
          <span>{t('workload:create.diffTitle')}</span>
        </Space>
      }
      open={open}
      onCancel={onCancel}
      onOk={onConfirm}
      width="90%"
      style={{ top: 20 }}
      okText={t('workload:create.confirmUpdate')}
      cancelText={t('workload:create.cancel')}
      destroyOnHidden
    >
      <div style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('workload:create.diffDesc')}</Text>
      </div>
      <div style={{ border: '1px solid #d9d9d9', borderRadius: 4 }}>
        <DiffEditor
          height="60vh"
          language="yaml"
          original={originalYaml}
          modified={modifiedYaml}
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
};

export default DeploymentDiffModal;

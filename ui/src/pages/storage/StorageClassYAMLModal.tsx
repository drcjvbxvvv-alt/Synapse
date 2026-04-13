import React from 'react';
import { Modal } from 'antd';
import { useTranslation } from 'react-i18next';

interface StorageClassYAMLModalProps {
  open: boolean;
  loading: boolean;
  yaml: string;
  onClose: () => void;
}

const StorageClassYAMLModal: React.FC<StorageClassYAMLModalProps> = ({
  open,
  loading,
  yaml,
  onClose,
}) => {
  const { t } = useTranslation(['common']);

  return (
    <Modal
      title="StorageClass YAML"
      open={open}
      onCancel={onClose}
      footer={null}
      width={800}
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <span>{t('common:messages.loading')}</span>
        </div>
      ) : (
        <pre style={{ maxHeight: 600, overflow: 'auto', background: '#f5f5f5', padding: 16 }}>
          {yaml}
        </pre>
      )}
    </Modal>
  );
};

export default StorageClassYAMLModal;

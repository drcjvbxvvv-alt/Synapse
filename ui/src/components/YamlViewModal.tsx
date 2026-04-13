import React from 'react';
import { Modal } from 'antd';
import { useTranslation } from 'react-i18next';

interface YamlViewModalProps {
  title: string;
  open: boolean;
  onCancel: () => void;
  yaml: string;
  loading: boolean;
}

const YamlViewModal: React.FC<YamlViewModalProps> = ({ title, open, onCancel, yaml, loading }) => {
  const { t } = useTranslation('common');

  return (
    <Modal
      title={title}
      open={open}
      onCancel={onCancel}
      footer={null}
      width={800}
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <span>{t('messages.loading')}</span>
        </div>
      ) : (
        <pre style={{ maxHeight: 600, overflow: 'auto', background: '#f5f5f5', padding: 16 }}>
          {yaml}
        </pre>
      )}
    </Modal>
  );
};

export default YamlViewModal;

import React from 'react';
import { Modal, Button, Alert, Space, Typography } from 'antd';
import { DiffOutlined } from '@ant-design/icons';
import { DiffEditor } from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export interface YAMLDiffViewProps {
  // Preview modal
  previewVisible: boolean;
  previewResult: Record<string, unknown> | null;
  onPreviewClose: () => void;
  onPreviewApply: () => void;

  // Diff modal
  diffModalVisible: boolean;
  originalYaml: string;
  pendingYaml: string;
  applying: boolean;
  onDiffClose: () => void;
  onDiffConfirm: () => void;
}

const YAMLDiffView: React.FC<YAMLDiffViewProps> = ({
  previewVisible,
  previewResult,
  onPreviewClose,
  onPreviewApply,
  diffModalVisible,
  originalYaml,
  pendingYaml,
  applying,
  onDiffClose,
  onDiffConfirm,
}) => {
  const { t } = useTranslation(['yaml', 'common']);

  return (
    <>
      {/* 預覽模態框 */}
      <Modal
        title={t('preview.title')}
        open={previewVisible}
        onCancel={onPreviewClose}
        footer={[
          <Button key="cancel" onClick={onPreviewClose}>
            {t('editor.close')}
          </Button>,
          <Button
            key="apply"
            type="primary"
            onClick={() => {
              onPreviewClose();
              onPreviewApply();
            }}
          >
            {t('editor.confirmApply')}
          </Button>,
        ]}
        width={800}
      >
        {previewResult && (
          <div>
            <Alert
              message={t('preview.validationSuccess')}
              description={t('preview.validationSuccessDesc')}
              type="success"
              showIcon
              style={{ marginBottom: 16 }}
            />
            <pre
              style={{
                background: '#f5f5f5',
                padding: '16px',
                borderRadius: '4px',
                overflow: 'auto',
                maxHeight: '400px',
              }}
            >
              {JSON.stringify(previewResult, null, 2)}
            </pre>
          </div>
        )}
      </Modal>

      {/* YAML Diff 對比 Modal */}
      <Modal
        title={
          <Space>
            <DiffOutlined />
            <span>{t('diff.title')}</span>
          </Space>
        }
        open={diffModalVisible}
        onCancel={onDiffClose}
        width={1200}
        footer={[
          <Button key="cancel" onClick={onDiffClose}>
            {t('editor.cancel')}
          </Button>,
          <Button
            key="submit"
            type="primary"
            loading={applying}
            onClick={onDiffConfirm}
          >
            {t('editor.confirmUpdate')}
          </Button>,
        ]}
      >
        <Alert
          message={t('diff.reviewChanges')}
          description={t('diff.reviewChangesDesc')}
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <div style={{ display: 'flex', gap: 16, marginBottom: 8 }}>
          <div style={{ flex: 1 }}>
            <Text strong style={{ color: '#cf1322' }}>
              {t('diff.originalConfig')}
            </Text>
          </div>
          <div style={{ flex: 1 }}>
            <Text strong style={{ color: '#389e0d' }}>
              {t('diff.modifiedConfig')}
            </Text>
          </div>
        </div>
        <div style={{ border: '1px solid #d9d9d9', borderRadius: '4px' }}>
          <DiffEditor
            height="500px"
            language="yaml"
            original={originalYaml}
            modified={pendingYaml}
            options={{
              readOnly: true,
              minimap: { enabled: false },
              fontSize: 13,
              lineNumbers: 'on',
              scrollBeyondLastLine: false,
              automaticLayout: true,
              renderSideBySide: true,
              enableSplitViewResizing: true,
            }}
            theme="vs-light"
          />
        </div>
      </Modal>
    </>
  );
};

export default YAMLDiffView;

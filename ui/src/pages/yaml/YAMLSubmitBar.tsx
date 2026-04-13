import React from 'react';
import { Button, Space, Typography, Alert, Switch } from 'antd';
import {
  ArrowLeftOutlined,
  SaveOutlined,
  EyeOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Title, Text } = Typography;

export interface DryRunResult {
  success: boolean;
  message: string;
}

export interface YAMLSubmitBarProps {
  workloadRef: string | null;
  workloadType: string | null;
  hasUnsavedChanges: boolean;
  applying: boolean;
  dryRun: boolean;
  error: string | null;
  dryRunResult: DryRunResult | null;
  onBack: () => void;
  onSave: () => void;
  onPreview: () => void;
  onReset: () => void;
  onDryRunChange: (value: boolean) => void;
  onRetry: () => void;
  onCloseDryRunResult: () => void;
}

const YAMLSubmitBar: React.FC<YAMLSubmitBarProps> = ({
  workloadRef,
  workloadType,
  hasUnsavedChanges,
  applying,
  dryRun,
  error,
  dryRunResult,
  onBack,
  onSave,
  onPreview,
  onReset,
  onDryRunChange,
  onRetry,
  onCloseDryRunResult,
}) => {
  const { t } = useTranslation(['yaml', 'common']);

  return (
    <>
      {/* 頁面頭部 */}
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={onBack}>
            {t('editor.back')}
          </Button>
          <Title level={3} style={{ margin: 0 }}>
            {t('editor.title')}
          </Title>
          {workloadRef && (
            <Text type="secondary">
              {workloadType}: {workloadRef}
            </Text>
          )}
          {hasUnsavedChanges && (
            <Text type="warning">{t('alert.hasUnsavedChanges')}</Text>
          )}
        </Space>

        <div style={{ marginTop: 16 }}>
          <Space>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={onSave}
              loading={applying}
              disabled={!hasUnsavedChanges}
            >
              {t('editor.apply')}
            </Button>

            <Button icon={<EyeOutlined />} onClick={onPreview} loading={applying}>
              {t('editor.preview')}
            </Button>

            <Button
              icon={<ReloadOutlined />}
              onClick={onReset}
              disabled={!hasUnsavedChanges}
            >
              {t('editor.reset')}
            </Button>

            <div style={{ marginLeft: 16 }}>
              <Space>
                <Text>{t('editor.dryRunMode')}:</Text>
                <Switch
                  checked={dryRun}
                  onChange={onDryRunChange}
                  checkedChildren={t('editor.on')}
                  unCheckedChildren={t('editor.off')}
                />
              </Space>
            </div>
          </Space>
        </div>
      </div>

      {/* 提示資訊 */}
      {error && (
        <Alert
          message={t('alert.loadFailed')}
          description={error}
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          action={
            <Button size="small" onClick={onRetry}>
              {t('alert.retry')}
            </Button>
          }
        />
      )}

      {/* 預檢結果提示 */}
      {dryRunResult && (
        <Alert
          message={
            dryRunResult.success
              ? t('messages.dryRunCheckPassed')
              : t('messages.dryRunCheckFailed')
          }
          description={dryRunResult.message}
          type={dryRunResult.success ? 'success' : 'error'}
          showIcon
          icon={
            dryRunResult.success ? (
              <CheckCircleOutlined />
            ) : (
              <ExclamationCircleOutlined />
            )
          }
          closable
          onClose={onCloseDryRunResult}
          style={{ marginBottom: 16 }}
        />
      )}

      {hasUnsavedChanges && !error && !dryRunResult && (
        <Alert
          message={t('alert.unsavedChanges')}
          description={t('alert.unsavedChangesDesc')}
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}
    </>
  );
};

export default YAMLSubmitBar;

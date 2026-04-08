import React from 'react';
import { Button, Space, Typography, App } from 'antd';
import { WarningFilled, CopyOutlined, LinkOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

interface NotInstalledCardProps {
  title: string;
  description?: string;
  /** Shell command to display and copy */
  command?: string;
  /** Label above the command block, defaults to "安裝指令：" */
  commandLabel?: string;
  /** Official docs URL */
  docsUrl?: string;
  /** Called when the user clicks "重新偵測" */
  onRecheck?: () => void;
  recheckLoading?: boolean;
}

const NotInstalledCard: React.FC<NotInstalledCardProps> = ({
  title,
  description,
  command,
  commandLabel,
  docsUrl,
  onRecheck,
  recheckLoading,
}) => {
  const { t } = useTranslation('common');
  const { message } = App.useApp();

  const handleCopy = async () => {
    if (!command) return;
    try {
      await navigator.clipboard.writeText(command);
      message.success(t('copySuccess'));
    } catch {
      message.error(t('copyError'));
    }
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', width: '100%', paddingTop: 48 }}>
      <div
        style={{
          background: '#fffbe6',
          border: '1px solid #ffe58f',
          borderRadius: 8,
          padding: '20px 24px',
          width: '100%',
          maxWidth: 600,
        }}
      >
      <Space align="start" size={12} style={{ width: '100%' }}>
        <WarningFilled style={{ fontSize: 20, color: '#fa8c16', marginTop: 2 }} />
        <Space direction="vertical" size={6} style={{ flex: 1, minWidth: 0 }}>
          <Text strong style={{ fontSize: 15 }}>{title}</Text>
          {description && <Text type="secondary">{description}</Text>}

          {command && (
            <Space direction="vertical" size={4} style={{ width: '100%', marginTop: 4 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {commandLabel ?? t('installCommand')}
              </Text>
              <div
                style={{
                  background: '#1f1f1f',
                  borderRadius: 6,
                  padding: '10px 14px',
                  fontFamily: 'monospace',
                  fontSize: 13,
                  color: '#e6e6e6',
                  overflowX: 'auto',
                  whiteSpace: 'pre',
                }}
              >
                {command}
              </div>
            </Space>
          )}

          <Space style={{ marginTop: 8 }} wrap>
            {command && (
              <Button size="small" icon={<CopyOutlined />} onClick={handleCopy}>
                {t('copyCommand')}
              </Button>
            )}
            {docsUrl && (
              <Button
                size="small"
                icon={<LinkOutlined />}
                onClick={() => window.open(docsUrl, '_blank', 'noopener')}
              >
                {t('viewDocs')}
              </Button>
            )}
            {onRecheck && (
              <Button
                size="small"
                icon={<ReloadOutlined />}
                loading={recheckLoading}
                onClick={onRecheck}
              >
                {t('recheck')}
              </Button>
            )}
          </Space>
        </Space>
      </Space>
      </div>
    </div>
  );
};

export default NotInstalledCard;

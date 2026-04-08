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
          boxSizing: 'border-box',
          overflow: 'hidden',
        }}
      >
        {/* Header row: icon + text, 原生 flexbox 確保 minWidth:0 正確約束 */}
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
          <WarningFilled style={{ fontSize: 20, color: '#fa8c16', marginTop: 2, flexShrink: 0 }} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <Text strong style={{ fontSize: 15, display: 'block' }}>{title}</Text>
            {description && (
              <Text type="secondary" style={{ display: 'block', marginTop: 4 }}>{description}</Text>
            )}

            {command && (
              <div style={{ marginTop: 10 }}>
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
                    marginTop: 4,
                    maxWidth: '100%',
                  }}
                >
                  {command}
                </div>
              </div>
            )}

            <Space style={{ marginTop: 12 }} wrap>
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
          </div>
        </div>
      </div>
    </div>
  );
};

export default NotInstalledCard;

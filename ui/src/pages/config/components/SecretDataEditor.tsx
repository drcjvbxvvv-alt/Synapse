/**
 * SecretDataEditor — Secret data key/value editor with visibility toggle and Monaco
 */
import React from 'react';
import { Button, Card, Input, Space, Tag, Tooltip, Switch } from 'antd';
import { PlusOutlined, DeleteOutlined, QuestionCircleOutlined, EyeOutlined, EyeInvisibleOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import MonacoEditor from '@monaco-editor/react';
import type { DataItem } from '../hooks/useSecretCreate';

interface SecretDataEditorProps {
  items: DataItem[];
  onAdd: () => void;
  onRemove: (index: number) => void;
  onKeyChange: (index: number, value: string) => void;
  onValueChange: (index: number, value: string | undefined) => void;
  onToggleVisibility: (index: number) => void;
}

const SecretDataEditor: React.FC<SecretDataEditorProps> = ({
  items, onAdd, onRemove, onKeyChange, onValueChange, onToggleVisibility,
}) => {
  const { t } = useTranslation(['config', 'common']);

  return (
    <Card
      title={
        <Space>
          <span>{t('config:create.dataContent')}</span>
          <Tag color="orange">{t('config:create.sensitiveData')}</Tag>
          <Tooltip title={t('config:create.sensitiveDataTooltip')}>
            <QuestionCircleOutlined style={{ color: '#999' }} />
          </Tooltip>
        </Space>
      }
      extra={
        <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={onAdd}>
          {t('config:create.addDataItem')}
        </Button>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        {items.map((item, index) => (
          <Card
            key={index}
            size="small"
            title={
              <Space>
                <Input
                  placeholder={t('config:create.secretDataKeyPlaceholder')}
                  value={item.key}
                  onChange={(e) => onKeyChange(index, e.target.value)}
                  style={{ width: '400px' }}
                />
                <Tooltip title={item.visible ? t('config:create.hideContent') : t('config:create.showContent')}>
                  <Switch
                    checkedChildren={<EyeOutlined />}
                    unCheckedChildren={<EyeInvisibleOutlined />}
                    checked={item.visible}
                    onChange={() => onToggleVisibility(index)}
                  />
                </Tooltip>
              </Space>
            }
            extra={
              <Button
                type="text"
                danger
                size="small"
                icon={<DeleteOutlined />}
                onClick={() => onRemove(index)}
              >
                {t('common:actions.delete')}
              </Button>
            }
          >
            {item.visible ? (
              <div style={{ border: '1px solid #d9d9d9', borderRadius: '4px' }}>
                <MonacoEditor
                  height="200px"
                  language="plaintext"
                  value={item.value}
                  onChange={(value) => onValueChange(index, value)}
                  options={{
                    minimap: { enabled: false },
                    lineNumbers: 'on',
                    scrollBeyondLastLine: false,
                    automaticLayout: true,
                    tabSize: 2,
                  }}
                  theme="vs-light"
                />
              </div>
            ) : (
              <div style={{
                padding: '20px',
                textAlign: 'center',
                background: '#fafafa',
                border: '1px dashed #d9d9d9',
                borderRadius: '4px',
              }}>
                <EyeInvisibleOutlined style={{ fontSize: '24px', color: '#999', marginBottom: '8px' }} />
                <div style={{ color: '#999' }}>
                  {t('config:create.sensitiveDataHidden')}
                </div>
              </div>
            )}
          </Card>
        ))}
        {items.length === 0 && (
          <div style={{ textAlign: 'center', color: '#999', padding: '20px' }}>
            {t('config:create.noDataItems')}
          </div>
        )}
      </Space>
    </Card>
  );
};

export default SecretDataEditor;

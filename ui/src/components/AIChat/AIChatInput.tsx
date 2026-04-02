import React, { useState, useRef, useEffect } from 'react';
import { Button, Tag } from 'antd';
import { SendOutlined, StopOutlined, CodeOutlined, SearchOutlined } from '@ant-design/icons';

interface AIChatInputProps {
  onSend: (message: string) => void;
  onStop: () => void;
  loading: boolean;
  disabled?: boolean;
}

const COMMAND_HINTS: Record<string, { icon: React.ReactNode; label: string; description: string }> = {
  '/yaml': {
    icon: <CodeOutlined />,
    label: '/yaml',
    description: '生成 K8s YAML 設定檔',
  },
  '/query': {
    icon: <SearchOutlined />,
    label: '/query',
    description: '自然語言查詢叢集資源',
  },
};

function detectCommand(value: string): string | null {
  const trimmed = value.trim();
  for (const cmd of Object.keys(COMMAND_HINTS)) {
    if (trimmed.startsWith(cmd)) return cmd;
  }
  return null;
}

const AIChatInput: React.FC<AIChatInputProps> = ({
  onSend,
  onStop,
  loading,
  disabled = false,
}) => {
  const [value, setValue] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (!loading && textareaRef.current) {
      textareaRef.current.focus();
    }
  }, [loading]);

  const handleSubmit = () => {
    const trimmed = value.trim();
    if (!trimmed || loading || disabled) return;
    onSend(trimmed);
    setValue('');
    if (textareaRef.current) {
      textareaRef.current.style.height = '40px';
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const handleInput = () => {
    const textarea = textareaRef.current;
    if (textarea) {
      textarea.style.height = '40px';
      const scrollHeight = textarea.scrollHeight;
      textarea.style.height = Math.min(scrollHeight, 120) + 'px';
    }
  };

  const activeCommand = detectCommand(value);
  const hint = activeCommand ? COMMAND_HINTS[activeCommand] : null;

  const insertCommand = (cmd: string) => {
    setValue(cmd + ' ');
    setTimeout(() => textareaRef.current?.focus(), 0);
  };

  return (
    <div
      style={{
        borderTop: '1px solid #f0f0f0',
        background: '#fff',
      }}
    >
      {/* Command hint bar */}
      {hint && (
        <div
          style={{
            padding: '4px 16px 0',
            display: 'flex',
            alignItems: 'center',
            gap: 6,
          }}
        >
          <Tag icon={hint.icon} color="blue" style={{ fontSize: 11, margin: 0 }}>
            {hint.label}
          </Tag>
          <span style={{ fontSize: 11, color: '#999' }}>{hint.description}</span>
        </div>
      )}

      {/* Quick command shortcuts */}
      {!value && !loading && (
        <div style={{ padding: '6px 16px 0', display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          {Object.entries(COMMAND_HINTS).map(([cmd, info]) => (
            <Tag
              key={cmd}
              icon={info.icon}
              color="default"
              style={{ fontSize: 11, cursor: 'pointer', userSelect: 'none' }}
              onClick={() => !disabled && insertCommand(cmd)}
            >
              {info.label}
            </Tag>
          ))}
        </div>
      )}

      <div
        style={{
          display: 'flex',
          alignItems: 'flex-end',
          gap: 8,
          padding: '8px 16px 12px',
        }}
      >
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => {
            setValue(e.target.value);
            handleInput();
          }}
          onKeyDown={handleKeyDown}
          placeholder="輸入訊息，Shift+Enter 換行… 試試 /yaml 或 /query"
          disabled={loading || disabled}
          rows={1}
          style={{
            flex: 1,
            resize: 'none',
            border: `1px solid ${activeCommand ? '#1677ff' : '#d9d9d9'}`,
            borderRadius: 8,
            padding: '8px 12px',
            fontSize: 14,
            lineHeight: '22px',
            outline: 'none',
            height: 40,
            maxHeight: 120,
            fontFamily: 'inherit',
            transition: 'border-color 0.2s',
          }}
          onFocus={(e) => {
            e.currentTarget.style.borderColor = activeCommand ? '#1677ff' : '#667eea';
          }}
          onBlur={(e) => {
            e.currentTarget.style.borderColor = activeCommand ? '#1677ff' : '#d9d9d9';
          }}
        />
        {loading ? (
          <Button
            type="default"
            danger
            icon={<StopOutlined />}
            onClick={onStop}
            style={{ height: 40, width: 40, borderRadius: 8 }}
          />
        ) : (
          <Button
            type="primary"
            icon={<SendOutlined />}
            onClick={handleSubmit}
            disabled={!value.trim() || disabled}
            style={{
              height: 40,
              width: 40,
              borderRadius: 8,
              background: value.trim() && !disabled ? '#667eea' : undefined,
              borderColor: value.trim() && !disabled ? '#667eea' : undefined,
            }}
          />
        )}
      </div>
    </div>
  );
};

export default AIChatInput;

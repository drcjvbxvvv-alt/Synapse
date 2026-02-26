import React, { useState, useRef, useEffect } from 'react';
import { Button } from 'antd';
import { SendOutlined, StopOutlined } from '@ant-design/icons';

interface AIChatInputProps {
  onSend: (message: string) => void;
  onStop: () => void;
  loading: boolean;
  disabled?: boolean;
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

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'flex-end',
        gap: 8,
        padding: '12px 16px',
        borderTop: '1px solid #f0f0f0',
        background: '#fff',
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
        placeholder="输入消息，Shift+Enter 换行..."
        disabled={loading || disabled}
        rows={1}
        style={{
          flex: 1,
          resize: 'none',
          border: '1px solid #d9d9d9',
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
          e.currentTarget.style.borderColor = '#667eea';
        }}
        onBlur={(e) => {
          e.currentTarget.style.borderColor = '#d9d9d9';
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
  );
};

export default AIChatInput;

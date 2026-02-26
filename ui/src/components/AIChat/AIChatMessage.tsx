import React from 'react';
import { Avatar, Spin, Tag } from 'antd';
import { UserOutlined, RobotOutlined, ToolOutlined } from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { DisplayMessage } from '../../types/ai';

interface AIChatMessageProps {
  message: DisplayMessage;
}

const markdownStyles: React.CSSProperties = {
  lineHeight: 1.7,
  fontSize: 14,
};

const AIChatMessage: React.FC<AIChatMessageProps> = ({ message }) => {
  const isUser = message.role === 'user';

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: isUser ? 'row-reverse' : 'row',
        gap: 8,
        marginBottom: 16,
        alignItems: 'flex-start',
      }}
    >
      <Avatar
        size={32}
        icon={isUser ? <UserOutlined /> : <RobotOutlined />}
        style={{
          backgroundColor: isUser ? '#667eea' : '#52c41a',
          flexShrink: 0,
        }}
      />

      <div
        style={{
          maxWidth: '85%',
          minWidth: 60,
        }}
      >
        <div
          style={{
            padding: '8px 14px',
            borderRadius: isUser ? '16px 16px 4px 16px' : '16px 16px 16px 4px',
            backgroundColor: isUser ? '#667eea' : '#f5f5f5',
            color: isUser ? '#fff' : '#333',
            wordBreak: 'break-word',
          }}
        >
          {message.loading && !message.content ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Spin size="small" />
              <span style={{ color: '#999', fontSize: 13 }}>思考中...</span>
            </div>
          ) : (
            <div style={markdownStyles} className="ai-chat-markdown">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  pre: ({ children }) => (
                    <pre
                      style={{
                        background: isUser ? 'rgba(255,255,255,0.1)' : '#282c34',
                        color: isUser ? '#fff' : '#abb2bf',
                        padding: '10px 14px',
                        borderRadius: 8,
                        overflow: 'auto',
                        fontSize: 13,
                        margin: '8px 0',
                      }}
                    >
                      {children}
                    </pre>
                  ),
                  code: ({ children, className }) => {
                    const isInline = !className;
                    if (isInline) {
                      return (
                        <code
                          style={{
                            background: isUser ? 'rgba(255,255,255,0.15)' : '#e8e8e8',
                            padding: '2px 6px',
                            borderRadius: 4,
                            fontSize: 13,
                          }}
                        >
                          {children}
                        </code>
                      );
                    }
                    return <code className={className}>{children}</code>;
                  },
                  table: ({ children }) => (
                    <div style={{ overflowX: 'auto', margin: '8px 0' }}>
                      <table
                        style={{
                          borderCollapse: 'collapse',
                          width: '100%',
                          fontSize: 13,
                        }}
                      >
                        {children}
                      </table>
                    </div>
                  ),
                  th: ({ children }) => (
                    <th
                      style={{
                        border: `1px solid ${isUser ? 'rgba(255,255,255,0.3)' : '#d9d9d9'}`,
                        padding: '6px 10px',
                        textAlign: 'left',
                        background: isUser ? 'rgba(255,255,255,0.1)' : '#fafafa',
                      }}
                    >
                      {children}
                    </th>
                  ),
                  td: ({ children }) => (
                    <td
                      style={{
                        border: `1px solid ${isUser ? 'rgba(255,255,255,0.3)' : '#d9d9d9'}`,
                        padding: '6px 10px',
                      }}
                    >
                      {children}
                    </td>
                  ),
                  p: ({ children }) => (
                    <p style={{ margin: '4px 0' }}>{children}</p>
                  ),
                  ul: ({ children }) => (
                    <ul style={{ margin: '4px 0', paddingLeft: 20 }}>{children}</ul>
                  ),
                  ol: ({ children }) => (
                    <ol style={{ margin: '4px 0', paddingLeft: 20 }}>{children}</ol>
                  ),
                }}
              >
                {message.content}
              </ReactMarkdown>
            </div>
          )}
        </div>

        {message.toolCalls && message.toolCalls.length > 0 && (
          <div style={{ marginTop: 4, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
            {message.toolCalls.map((tc) => (
              <Tag
                key={tc.id}
                icon={<ToolOutlined />}
                color="processing"
                style={{ fontSize: 11 }}
              >
                {tc.function.name}
              </Tag>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default AIChatMessage;

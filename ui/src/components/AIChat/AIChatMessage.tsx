import React, { useState } from 'react';
import { Avatar, Spin, Tag, Collapse, Button, message as antMessage, Tooltip } from 'antd';
import {
  UserOutlined,
  RobotOutlined,
  ToolOutlined,
  BookOutlined,
  CopyOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { DisplayMessage } from '../../types/ai';

interface AIChatMessageProps {
  message: DisplayMessage;
  onApplyYAML?: (yaml: string) => void;
}

const markdownStyles: React.CSSProperties = {
  lineHeight: 1.7,
  fontSize: 14,
};

// Extract YAML code blocks from markdown content
function extractYAMLBlocks(content: string): string[] {
  const yamlBlockRegex = /```yaml\n([\s\S]*?)```/g;
  const blocks: string[] = [];
  let match;
  while ((match = yamlBlockRegex.exec(content)) !== null) {
    blocks.push(match[1].trim());
  }
  return blocks;
}

const AIChatMessage: React.FC<AIChatMessageProps> = ({ message, onApplyYAML }) => {
  const isUser = message.role === 'user';
  const [copiedBlock, setCopiedBlock] = useState<number | null>(null);

  const yamlBlocks = !isUser && !message.loading ? extractYAMLBlocks(message.content) : [];

  const handleCopyYAML = async (yaml: string, idx: number) => {
    try {
      await navigator.clipboard.writeText(yaml);
      setCopiedBlock(idx);
      antMessage.success('已複製 YAML');
      setTimeout(() => setCopiedBlock(null), 2000);
    } catch {
      antMessage.error('複製失敗');
    }
  };

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

      <div style={{ maxWidth: '85%', minWidth: 60 }}>
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
                      <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 13 }}>
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
                  p: ({ children }) => <p style={{ margin: '4px 0' }}>{children}</p>,
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

        {/* Tool call tags */}
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

        {/* YAML block actions — copy + apply */}
        {yamlBlocks.length > 0 && (
          <div style={{ marginTop: 6, display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {yamlBlocks.map((yaml, idx) => (
              <React.Fragment key={idx}>
                <Tooltip title="複製 YAML">
                  <Button
                    size="small"
                    icon={<CopyOutlined />}
                    onClick={() => handleCopyYAML(yaml, idx)}
                    type={copiedBlock === idx ? 'primary' : 'default'}
                    style={{ fontSize: 12 }}
                  >
                    {copiedBlock === idx ? '已複製' : '複製 YAML'}
                  </Button>
                </Tooltip>
                {onApplyYAML && (
                  <Tooltip title="套用至叢集">
                    <Button
                      size="small"
                      icon={<CodeOutlined />}
                      onClick={() => onApplyYAML(yaml)}
                      style={{ fontSize: 12 }}
                    >
                      套用至叢集
                    </Button>
                  </Tooltip>
                )}
              </React.Fragment>
            ))}
          </div>
        )}

        {/* Runbook recommendations */}
        {message.runbooks && message.runbooks.length > 0 && (
          <div style={{ marginTop: 8 }}>
            <Collapse
              size="small"
              ghost
              items={[
                {
                  key: 'runbooks',
                  label: (
                    <span style={{ fontSize: 12, color: '#1677ff' }}>
                      <BookOutlined style={{ marginRight: 4 }} />
                      相關 Runbook（{message.runbooks.length} 筆）
                    </span>
                  ),
                  children: message.runbooks.map((rb) => (
                    <div
                      key={rb.id}
                      style={{
                        marginBottom: 12,
                        padding: '8px 10px',
                        background: '#fafafa',
                        borderRadius: 6,
                        border: '1px solid #f0f0f0',
                      }}
                    >
                      <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
                        {rb.title}
                      </div>
                      <div style={{ fontSize: 12, color: '#666', marginBottom: 6 }}>
                        {rb.summary}
                      </div>
                      <ol style={{ margin: 0, paddingLeft: 16 }}>
                        {rb.steps.map((step, i) => (
                          <li key={i} style={{ fontSize: 12, color: '#444', marginBottom: 2 }}>
                            {step.replace(/^\d+\.\s*/, '')}
                          </li>
                        ))}
                      </ol>
                    </div>
                  )),
                },
              ]}
            />
          </div>
        )}
      </div>
    </div>
  );
};

export default AIChatMessage;

import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Drawer, Button, Empty, App } from 'antd';
import { RobotOutlined, DeleteOutlined, CloseOutlined } from '@ant-design/icons';
import { useLocation } from 'react-router-dom';
import { aiService } from '../../services/aiService';
import type { ChatMessage, DisplayMessage, ToolCall } from '../../types/ai';
import AIChatMessage from './AIChatMessage';
import AIChatInput from './AIChatInput';

let messageIdCounter = 0;
const genId = () => `msg-${++messageIdCounter}-${Date.now()}`;

const AIChatPanel: React.FC = () => {
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [chatHistory, setChatHistory] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const location = useLocation();
  const { message: antMessage } = App.useApp();

  const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
  const clusterId = clusterMatch ? clusterMatch[1] : null;

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    if (open) {
      setTimeout(scrollToBottom, 100);
    }
  }, [messages, open, scrollToBottom]);

  const handleSend = useCallback(
    (content: string) => {
      if (!clusterId) {
        antMessage.warning('请先进入集群详情页面');
        return;
      }

      const userMsg: DisplayMessage = {
        id: genId(),
        role: 'user',
        content,
        timestamp: Date.now(),
      };

      const assistantMsg: DisplayMessage = {
        id: genId(),
        role: 'assistant',
        content: '',
        loading: true,
        timestamp: Date.now(),
      };

      setMessages((prev) => [...prev, userMsg, assistantMsg]);

      const newHistory: ChatMessage[] = [
        ...chatHistory,
        { role: 'user', content },
      ];
      setChatHistory(newHistory);
      setLoading(true);

      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      let accumulatedContent = '';
      const accumulatedToolCalls: ToolCall[] = [];

      aiService.chatStream(
        clusterId,
        newHistory,
        (eventType, data) => {
          switch (eventType) {
            case 'content': {
              const evt = data as { content: string };
              accumulatedContent += evt.content;
              setMessages((prev) => {
                const updated = [...prev];
                const lastMsg = updated[updated.length - 1];
                if (lastMsg && lastMsg.role === 'assistant') {
                  updated[updated.length - 1] = {
                    ...lastMsg,
                    content: accumulatedContent,
                    loading: true,
                  };
                }
                return updated;
              });
              break;
            }
            case 'tool_call': {
              const tc = data as { id: string; name: string; arguments: string };
              accumulatedToolCalls.push({
                id: tc.id,
                type: 'function',
                function: { name: tc.name, arguments: tc.arguments },
              });
              setMessages((prev) => {
                const updated = [...prev];
                const lastMsg = updated[updated.length - 1];
                if (lastMsg && lastMsg.role === 'assistant') {
                  updated[updated.length - 1] = {
                    ...lastMsg,
                    toolCalls: [...accumulatedToolCalls],
                  };
                }
                return updated;
              });
              break;
            }
            case 'tool_result': {
              break;
            }
            case 'error': {
              const errEvt = data as { error: string };
              setMessages((prev) => {
                const updated = [...prev];
                const lastMsg = updated[updated.length - 1];
                if (lastMsg && lastMsg.role === 'assistant') {
                  updated[updated.length - 1] = {
                    ...lastMsg,
                    content: lastMsg.content || `错误: ${errEvt.error}`,
                    loading: false,
                  };
                }
                return updated;
              });
              break;
            }
          }
        },
        () => {
          setMessages((prev) => {
            const updated = [...prev];
            const lastMsg = updated[updated.length - 1];
            if (lastMsg && lastMsg.role === 'assistant') {
              updated[updated.length - 1] = {
                ...lastMsg,
                loading: false,
              };

              setChatHistory((prevHistory) => [
                ...prevHistory,
                {
                  role: 'assistant' as const,
                  content: lastMsg.content || accumulatedContent,
                },
              ]);
            }
            return updated;
          });
          setLoading(false);
          abortControllerRef.current = null;
        },
        (error) => {
          setMessages((prev) => {
            const updated = [...prev];
            const lastMsg = updated[updated.length - 1];
            if (lastMsg && lastMsg.role === 'assistant') {
              updated[updated.length - 1] = {
                ...lastMsg,
                content: `连接失败: ${error}`,
                loading: false,
              };
            }
            return updated;
          });
          setLoading(false);
          abortControllerRef.current = null;
        },
        abortController.signal,
      );
    },
    [clusterId, chatHistory, antMessage],
  );

  const handleStop = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
    setMessages((prev) => {
      const updated = [...prev];
      const lastMsg = updated[updated.length - 1];
      if (lastMsg && lastMsg.loading) {
        updated[updated.length - 1] = {
          ...lastMsg,
          content: lastMsg.content || '(已停止)',
          loading: false,
        };
      }
      return updated;
    });
    setLoading(false);
  }, []);

  const handleClear = useCallback(() => {
    setMessages([]);
    setChatHistory([]);
  }, []);

  return (
    <>
      {clusterId && (
        <Button
          type="primary"
          shape="circle"
          size="large"
          icon={<RobotOutlined style={{ fontSize: 22 }} />}
          onClick={() => setOpen(true)}
          style={{
            position: 'fixed',
            right: 24,
            bottom: 24,
            zIndex: 1000,
            width: 52,
            height: 52,
            background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
            border: 'none',
            boxShadow: '0 4px 16px rgba(102, 126, 234, 0.4)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        />
      )}

      <Drawer
        title={
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <RobotOutlined style={{ fontSize: 18, color: '#667eea' }} />
              <span style={{ fontSize: 15, fontWeight: 600 }}>AI 助手</span>
              {clusterId && (
                <span style={{ fontSize: 12, color: '#999', fontWeight: 400 }}>
                  集群 #{clusterId}
                </span>
              )}
            </div>
            <div style={{ display: 'flex', gap: 4 }}>
              <Button
                type="text"
                size="small"
                icon={<DeleteOutlined />}
                onClick={handleClear}
                disabled={loading || messages.length === 0}
                title="清空对话"
              />
              <Button
                type="text"
                size="small"
                icon={<CloseOutlined />}
                onClick={() => setOpen(false)}
              />
            </div>
          </div>
        }
        placement="right"
        open={open}
        onClose={() => setOpen(false)}
        width={480}
        closable={false}
        styles={{
          body: {
            padding: 0,
            display: 'flex',
            flexDirection: 'column',
            height: '100%',
          },
          header: {
            padding: '12px 16px',
            borderBottom: '1px solid #f0f0f0',
          },
        }}
      >
        <div
          style={{
            flex: 1,
            overflow: 'auto',
            padding: '16px',
          }}
        >
          {messages.length === 0 ? (
            <div style={{ marginTop: 80 }}>
              <Empty
                image={<RobotOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />}
                description={
                  <div>
                    <div style={{ fontSize: 15, color: '#666', marginBottom: 4 }}>
                      KubePolaris AI 助手
                    </div>
                    <div style={{ fontSize: 13, color: '#999' }}>
                      我可以帮你查看集群资源、分析问题、执行操作
                    </div>
                    <div style={{ fontSize: 12, color: '#bbb', marginTop: 12 }}>
                      试试问: "哪些 Pod 异常？" 或 "列出所有 Deployment"
                    </div>
                  </div>
                }
              />
            </div>
          ) : (
            messages.map((msg) => <AIChatMessage key={msg.id} message={msg} />)
          )}
          <div ref={messagesEndRef} />
        </div>

        <AIChatInput
          onSend={handleSend}
          onStop={handleStop}
          loading={loading}
          disabled={!clusterId}
        />
      </Drawer>
    </>
  );
};

export default AIChatPanel;

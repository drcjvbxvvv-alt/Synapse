import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Drawer, Button, Empty, App, Modal, Spin, Alert } from 'antd';
import {
  RobotOutlined,
  DeleteOutlined,
  CloseOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';
import { aiService } from '../../services/aiService';
import type { ChatMessage, DisplayMessage, ToolCall, Runbook } from '../../types/ai';
import AIChatMessage from './AIChatMessage';
import AIChatInput from './AIChatInput';

let messageIdCounter = 0;
const genId = () => `msg-${++messageIdCounter}-${Date.now()}`;

// Keywords in AI response content that suggest runbooks should be shown
const RUNBOOK_KEYWORDS = [
  'OOMKilled', 'CrashLoopBackOff', 'ImagePullBackOff', 'ErrImagePull',
  'Evicted', 'NodeNotReady', 'PVCPending', 'Throttl', 'FailedScheduling',
  'DiskPressure', 'NetworkPolicy',
];

function detectRunbookReasons(content: string): string[] {
  const found: string[] = [];
  for (const kw of RUNBOOK_KEYWORDS) {
    if (content.includes(kw) && !found.includes(kw)) {
      found.push(kw);
    }
  }
  return found;
}

const AIChatPanel: React.FC = () => {
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [chatHistory, setChatHistory] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(false);

  // NL Query modal state
  const [nlModalOpen, setNlModalOpen] = useState(false);
  const [nlQuestion, setNlQuestion] = useState('');
  const [nlLoading, setNlLoading] = useState(false);
  const [nlError, setNlError] = useState('');

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const location = useLocation();
  const navigate = useNavigate();
  const { message: antMessage } = App.useApp();

  const clusterMatch = location.pathname.match(/\/clusters\/([^/]+)/);
  const clusterId = clusterMatch ? clusterMatch[1] : null;

  // Listen for ai:diagnose custom events dispatched from detail pages
  const handleSendRef = useRef<(content: string) => void>(() => {});

  useEffect(() => {
    const listener = (e: Event) => {
      const detail = (e as CustomEvent<{ message: string }>).detail;
      if (!detail?.message) return;
      setOpen(true);
      setTimeout(() => {
        handleSendRef.current(detail.message);
      }, 300);
    };
    window.addEventListener('ai:diagnose', listener);
    return () => window.removeEventListener('ai:diagnose', listener);
  }, []);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    if (open) {
      setTimeout(scrollToBottom, 100);
    }
  }, [messages, open, scrollToBottom]);

  // Fetch runbooks for keywords found in an AI response
  const fetchRunbooksForMessage = useCallback(async (msgId: string, content: string) => {
    const reasons = detectRunbookReasons(content);
    if (reasons.length === 0) return;

    try {
      const allRunbooks: Runbook[] = [];
      const seen = new Set<string>();
      for (const reason of reasons.slice(0, 2)) {
        const results = await aiService.getRunbooks(reason);
        for (const rb of (results as Runbook[])) {
          if (!seen.has(rb.id)) {
            seen.add(rb.id);
            allRunbooks.push(rb);
          }
        }
      }
      if (allRunbooks.length === 0) return;

      setMessages((prev) =>
        prev.map((m) => (m.id === msgId ? { ...m, runbooks: allRunbooks } : m))
      );
    } catch {
      // Silently ignore runbook fetch failures
    }
  }, []);

  const handleSend = useCallback(
    (content: string) => {
      if (!clusterId) {
        antMessage.warning('請先進入叢集詳情頁面');
        return;
      }

      // /query command → open NL Query modal
      if (content.trim().startsWith('/query')) {
        const question = content.trim().replace(/^\/query\s*/, '');
        setNlQuestion(question);
        setNlModalOpen(true);
        return;
      }

      const userMsg: DisplayMessage = {
        id: genId(),
        role: 'user',
        content,
        timestamp: Date.now(),
      };

      const assistantMsgId = genId();
      const assistantMsg: DisplayMessage = {
        id: assistantMsgId,
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
                    content: lastMsg.content || `錯誤：${errEvt.error}`,
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
              // After response complete, check if runbooks should be shown
              const finalContent = lastMsg.content || accumulatedContent;
              fetchRunbooksForMessage(lastMsg.id, finalContent);
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
                content: `連線失敗：${error}`,
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
    [clusterId, chatHistory, antMessage, fetchRunbooksForMessage],
  );

  // keep ref in sync so the ai:diagnose event listener can invoke the latest handleSend
  useEffect(() => {
    handleSendRef.current = handleSend;
  }, [handleSend]);

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
          content: lastMsg.content || '（已停止）',
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

  // Handle YAML apply — navigate to YAML editor with prefilled content
  const handleApplyYAML = useCallback(
    (yaml: string) => {
      if (!clusterId) return;
      // Store YAML in sessionStorage and navigate to YAML apply page
      sessionStorage.setItem('ai_yaml_content', yaml);
      navigate(`/clusters/${clusterId}/yaml/apply`);
      setOpen(false);
    },
    [clusterId, navigate],
  );

  // NL Query submission
  const handleNLQuery = useCallback(async () => {
    if (!clusterId || !nlQuestion.trim()) return;
    setNlLoading(true);
    setNlError('');
    try {
      const result = await aiService.nlQuery(clusterId, nlQuestion.trim());
      // Display result as an assistant message in the chat
      const summary = (result as { summary: string }).summary || '查詢完成';
      const toolUsed = (result as { tool_used?: string }).tool_used || '';
      const queryResult = (result as { result: unknown }).result;

      let content = summary;
      if (queryResult && Array.isArray(queryResult) && queryResult.length > 0) {
        content += `\n\n共找到 **${queryResult.length}** 筆資源。`;
      } else if (toolUsed) {
        content += `\n\n_使用工具：${toolUsed}_`;
      }

      const userMsg: DisplayMessage = {
        id: genId(),
        role: 'user',
        content: `/query ${nlQuestion}`,
        timestamp: Date.now(),
      };
      const assistantMsg: DisplayMessage = {
        id: genId(),
        role: 'assistant',
        content,
        timestamp: Date.now(),
      };
      setMessages((prev) => [...prev, userMsg, assistantMsg]);
      setNlModalOpen(false);
      setNlQuestion('');
      setOpen(true);
    } catch (err) {
      setNlError((err as Error).message || '查詢失敗');
    } finally {
      setNlLoading(false);
    }
  }, [clusterId, nlQuestion]);

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
                  叢集 #{clusterId}
                </span>
              )}
            </div>
            <div style={{ display: 'flex', gap: 4 }}>
              <Button
                type="text"
                size="small"
                icon={<SearchOutlined />}
                onClick={() => setNlModalOpen(true)}
                disabled={!clusterId}
                title="自然語言查詢"
              />
              <Button
                type="text"
                size="small"
                icon={<DeleteOutlined />}
                onClick={handleClear}
                disabled={loading || messages.length === 0}
                title="清空對話"
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
        <div style={{ flex: 1, overflow: 'auto', padding: '16px' }}>
          {messages.length === 0 ? (
            <div style={{ marginTop: 80 }}>
              <Empty
                image={<RobotOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />}
                description={
                  <div>
                    <div style={{ fontSize: 15, color: '#666', marginBottom: 4 }}>
                      Synapse AI 助手
                    </div>
                    <div style={{ fontSize: 13, color: '#999' }}>
                      可檢視叢集資源、診斷問題、生成 YAML
                    </div>
                    <div style={{ fontSize: 12, color: '#bbb', marginTop: 12 }}>
                      試試：「哪些 Pod 異常？」或 /yaml 生成 Deployment
                    </div>
                  </div>
                }
              />
            </div>
          ) : (
            messages.map((msg) => (
              <AIChatMessage
                key={msg.id}
                message={msg}
                onApplyYAML={handleApplyYAML}
              />
            ))
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

      {/* NL Query Modal */}
      <Modal
        title={
          <span>
            <SearchOutlined style={{ marginRight: 8, color: '#667eea' }} />
            自然語言叢集查詢
          </span>
        }
        open={nlModalOpen}
        onOk={handleNLQuery}
        onCancel={() => {
          setNlModalOpen(false);
          setNlError('');
        }}
        confirmLoading={nlLoading}
        okText="查詢"
        cancelText="取消"
        width={480}
      >
        <div style={{ marginBottom: 8, color: '#666', fontSize: 13 }}>
          用自然語言描述你想查詢的資源，例如：
        </div>
        <ul style={{ fontSize: 12, color: '#999', marginBottom: 16, paddingLeft: 18 }}>
          <li>列出所有重啟超過 5 次的 Pod</li>
          <li>哪些 Deployment 副本數不健康？</li>
          <li>顯示 production 命名空間的所有 Service</li>
        </ul>
        <textarea
          value={nlQuestion}
          onChange={(e) => setNlQuestion(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              handleNLQuery();
            }
          }}
          placeholder="輸入查詢問題…"
          rows={3}
          style={{
            width: '100%',
            resize: 'vertical',
            border: '1px solid #d9d9d9',
            borderRadius: 6,
            padding: '8px 12px',
            fontSize: 14,
            fontFamily: 'inherit',
            outline: 'none',
          }}
          onFocus={(e) => (e.target.style.borderColor = '#667eea')}
          onBlur={(e) => (e.target.style.borderColor = '#d9d9d9')}
        />
        {nlLoading && (
          <div style={{ textAlign: 'center', padding: '12px 0' }}>
            <Spin size="small" />
            <span style={{ marginLeft: 8, fontSize: 13, color: '#999' }}>
              AI 正在解析查詢…
            </span>
          </div>
        )}
        {nlError && (
          <Alert type="error" message={nlError} style={{ marginTop: 8 }} showIcon />
        )}
      </Modal>
    </>
  );
};

export default AIChatPanel;

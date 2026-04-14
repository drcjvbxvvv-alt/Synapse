import React, { useEffect, useRef, useState, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import {
  Card,
  Button,
  Space,
  message,
  Typography,
  Alert,
} from 'antd';
import {
  PlayCircleOutlined,
  StopOutlined,
  ClearOutlined,
  FullscreenOutlined,
} from '@ant-design/icons';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebLinksAddon } from 'xterm-addon-web-links';
import { ClipboardAddon } from '@xterm/addon-clipboard';
import 'xterm/css/xterm.css';
import { buildWebSocketUrl } from '../../utils/wsUrl';
import { tokenManager } from '../../services/authService';
import { useTranslation } from 'react-i18next';

const { Title, Text } = Typography;

const KubectlTerminalPage: React.FC = () => {
const { t } = useTranslation(["terminal", "common"]);
const { id: clusterId } = useParams<{ id: string }>();
  
  const terminalRef = useRef<HTMLDivElement>(null);
  const terminal = useRef<Terminal | null>(null);
  const fitAddon = useRef<FitAddon | null>(null);
  const websocket = useRef<WebSocket | null>(null);
  
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  
  const connectedRef = useRef(false);
  const isManualDisconnectRef = useRef(false);
  const retryDelayRef = useRef(1000);
  const isMountedRef = useRef(true);

  // Track component lifecycle for reconnect guard
  useEffect(() => {
    isMountedRef.current = true;
    return () => { isMountedRef.current = false; };
  }, []);

  // 處理終端輸入 - 直接傳送所有輸入到服務端（Pod Terminal 模式）
  const handleTerminalInput = useCallback((data: string) => {
    if (!connectedRef.current || !websocket.current) return;

    if (websocket.current.readyState !== WebSocket.OPEN) {
      terminal.current?.write('\r\nConnection lost. Please reconnect.\r\n');
      return;
    }

    // 直接傳送所有輸入到服務端，由服務端處理並回顯
    websocket.current.send(JSON.stringify({
      type: 'input',
      data: data,
    }));
  }, []);

  // 貼上剪貼簿內容
  const pasteFromClipboard = useCallback(() => {
    if (!connectedRef.current) {
      message.error(t('messages.connectFirst'));
      return;
    }
    
    navigator.clipboard.readText()
      .then((text) => {
        if (text && websocket.current && websocket.current.readyState === WebSocket.OPEN) {
          // 直接作為輸入傳送
          websocket.current.send(JSON.stringify({
            type: 'input',
            data: text
          }));
        }
      })
      .catch((err) => {
        console.error('貼上失敗:', err);
        message.error(t('messages.pasteFailed'));
      });
  }, [t]);

  // 顯示歡迎資訊
  const showWelcomeMessage = useCallback(() => {
    if (!terminal.current) return;
    
    terminal.current.clear();
    terminal.current.writeln('\x1b[32m╭─────────────────────────────────────────────────────────────╮\x1b[0m');
    terminal.current.writeln('\x1b[32m│                  Synapse Kubectl Terminal               │\x1b[0m');
    terminal.current.writeln('\x1b[32m╰─────────────────────────────────────────────────────────────╯\x1b[0m');
    terminal.current.writeln('');
    terminal.current.writeln(`\x1b[36mCluster:\x1b[0m ${clusterId}`);
    terminal.current.writeln('');
    terminal.current.writeln('\x1b[33m' + t('kubectl.welcomeMessage') + '\x1b[0m');
    terminal.current.writeln('');
  }, [clusterId, t]);

  // 初始化終端
  useEffect(() => {
    const initTerminal = () => {
      if (terminalRef.current && !terminal.current) {
        try {
          terminal.current = new Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: 'Monaco, Menlo, "Ubuntu Mono", Consolas, monospace',
            theme: {
              background: '#1e1e1e',
              foreground: '#d4d4d4',
              cursor: '#ffffff',
              selectionBackground: '#264f78',
            },
            cols: 120,
            rows: 30,
            allowTransparency: true,
            rightClickSelectsWord: true,
          });

          // 新增外掛
          fitAddon.current = new FitAddon();
          terminal.current.loadAddon(fitAddon.current);
          terminal.current.loadAddon(new WebLinksAddon());
          
          // 新增剪貼簿支援
          try {
            const clipboardAddon = new ClipboardAddon();
            terminal.current.loadAddon(clipboardAddon);
          } catch (e) {
            console.warn('Clipboard addon not available:', e);
          }

          terminal.current.open(terminalRef.current);
          
          // 等待 DOM 完全渲染後再 fit
          const fitTerminal = () => {
            if (fitAddon.current && terminal.current && terminalRef.current) {
              try {
                const rect = terminalRef.current.getBoundingClientRect();
                if (rect.width > 0 && rect.height > 0) {
                  fitAddon.current.fit();
                } else {
                  setTimeout(fitTerminal, 100);
                }
              } catch (e) {
                console.warn('Fit addon error:', e);
              }
            }
          };

          // 延遲執行 fit 和顯示歡迎資訊
          setTimeout(() => {
            fitTerminal();
            setTimeout(() => {
              showWelcomeMessage();
            }, 200);
          }, 100);

          // 設定終端輸入處理
          terminal.current.onData((data) => {
            handleTerminalInput(data);
          });

          // 新增鍵盤快捷鍵支援
          terminal.current.attachCustomKeyEventHandler((event) => {
            if (event.type === 'keydown') {
              // Ctrl+C 複製
              if (event.ctrlKey && event.key === 'c' && terminal.current?.hasSelection()) {
                const selection = terminal.current.getSelection();
                if (selection) {
                  navigator.clipboard.writeText(selection);
                }
                return false;
              }
              
              // Ctrl+V 貼上
              if (event.ctrlKey && event.key === 'v') {
                pasteFromClipboard();
                return false;
              }
            }
            return true;
          });

        } catch (error) {
          console.error('初始化終端失敗:', error);
          message.error(t('messages.initFailed'));
        }
      }
    };

    const timer = setTimeout(initTerminal, 100);

    return () => {
      clearTimeout(timer);
      if (websocket.current) {
        websocket.current.close();
      }
      if (terminal.current) {
        terminal.current.dispose();
        terminal.current = null;
      }
    };
  }, [showWelcomeMessage, handleTerminalInput, pasteFromClipboard, t]);

  // 處理 WebSocket 訊息
  interface WebSocketMessage {
    type: string;
    data: string;
  }

  const handleWebSocketMessage = (msg: WebSocketMessage) => {
    if (!terminal.current) return;

    switch (msg.type) {
      case 'data':
        // Pod Terminal 模式：直接寫入終端輸出
        terminal.current.write(msg.data);
        break;
      case 'output':
        // 舊模式相容
        terminal.current.write(msg.data);
        break;
      case 'kubectl_prep':
        // 由 onmessage 統一單行重新整理，此處兜底
        terminal.current.write(`\r\x1b[2K\x1b[33m${msg.data}\x1b[0m`);
        break;
      case 'connected':
        // Pod 連線成功（主流程在 onmessage 裡處理 UI 狀態）
        console.log('Pod terminal connected:', msg.data);
        break;
      case 'disconnected':
        terminal.current.writeln(`\r\n\x1b[33m${msg.data}\x1b[0m`);
        break;
      case 'error':
        setConnecting(false);
        terminal.current.writeln(`\r\n\x1b[31m${msg.data}\x1b[0m`);
        break;
      case 'command_result':
        // 舊模式相容
        break;
      case 'clear':
        terminal.current.clear();
        break;
      default:
        break;
    }
  };

  // 連線終端
  const connectTerminal = () => {
    if (!clusterId) {
      message.error(t('messages.missingClusterId'));
      return;
    }
    
    // 獲取認證 token
    const token = tokenManager.getToken();
    if (!token) {
      message.error(t('messages.notLoggedIn'));
      return;
    }
    
    isManualDisconnectRef.current = false;
    retryDelayRef.current = 1000;
    setConnecting(true);

    if (terminal.current) {
      terminal.current.clear();
      // 不換行，便於後續 kubectl_prep 用 \r 重新整理同一行進度
      terminal.current.write('\x1b[33m' + t('kubectl.connecting') + '\x1b[0m');
    }

    const wsUrl = buildWebSocketUrl(
      `/ws/clusters/${clusterId}/kubectl?token=${encodeURIComponent(token)}`
    );
    
    try {
      const ws = new WebSocket(wsUrl);
      websocket.current = ws;
      
      ws.onopen = () => {
        // WebSocket 已建立；kubectl Pod 可能仍在建立/拉取映像，等服務端 type=connected 再視為可互動
      };
      
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WebSocketMessage;
          if (msg.type === 'connected') {
            setConnected(true);
            setConnecting(false);
            connectedRef.current = true;
            message.success(t('messages.connectSuccess'));
            if (terminal.current) {
              terminal.current.clear();
            }
            if (fitAddon.current && terminal.current) {
              const dimensions = fitAddon.current.proposeDimensions();
              if (dimensions) {
                ws.send(JSON.stringify({
                  type: 'resize',
                  cols: dimensions.cols,
                  rows: dimensions.rows
                }));
              }
            }
            return;
          }
          if (msg.type === 'kubectl_prep' && terminal.current) {
            terminal.current.write(`\r\x1b[2K\x1b[33m${msg.data}\x1b[0m`);
            return;
          }
          handleWebSocketMessage(msg);
        } catch {
          terminal.current?.write(event.data);
        }
      };
      
      ws.onerror = (_error) => {
        console.error('WebSocket錯誤:', _error);
        message.error(t('messages.connectError'));
        setConnected(false);
        setConnecting(false);
        connectedRef.current = false;
        
        if (terminal.current) {
          terminal.current.writeln('\x1b[31m' + t('messages.connectionError') + '\x1b[0m');
        }
      };
      
      ws.onclose = () => {
        setConnected(false);
        setConnecting(false);
        connectedRef.current = false;

        if (isManualDisconnectRef.current || !isMountedRef.current) {
          message.info(t('messages.connectionLost'));
          if (terminal.current) {
            terminal.current.writeln('\x1b[31m\r\n' + t('messages.connectionClosed') + '\x1b[0m');
          }
          return;
        }

        // Unexpected disconnect — reconnect with exponential backoff
        const delay = retryDelayRef.current;
        retryDelayRef.current = Math.min(retryDelayRef.current * 2, 30_000);
        if (terminal.current) {
          terminal.current.writeln(`\x1b[33m\r\n[Connection lost. Reconnecting in ${delay / 1000}s...]\x1b[0m`);
        }
        setTimeout(() => {
          if (isMountedRef.current && !isManualDisconnectRef.current) {
            connectTerminal();
          }
        }, delay);
      };
      
    } catch (error) {
      console.error('建立WebSocket連線失敗:', error);
      message.error(t('messages.createFailed'));
      setConnecting(false);
      
      if (terminal.current) {
        terminal.current.writeln('\x1b[31m' + t('messages.createConnectionFailed') + '\x1b[0m');
      }
    }
  };

  // 斷開終端連線
  const disconnectTerminal = () => {
    isManualDisconnectRef.current = true;
    if (websocket.current) {
      websocket.current.close();
      websocket.current = null;
    }
    setConnected(false);
    connectedRef.current = false;

    if (terminal.current) {
      terminal.current.writeln('\x1b[33m\r\n' + t('messages.disconnected') + '\x1b[0m');
    }
  };

  // 清空終端
  const clearTerminal = () => {
    if (terminal.current) {
      terminal.current.clear();
    }
  };

  // 全屏模式
  const toggleFullscreen = () => {
    if (terminalRef.current) {
      if (document.fullscreenElement) {
        document.exitFullscreen();
      } else {
        terminalRef.current.requestFullscreen();
      }
    }
  };

  // 視窗大小變化時重新調整終端大小
  useEffect(() => {
    const handleResize = () => {
      if (fitAddon.current && terminal.current) {
        setTimeout(() => {
          try {
            fitAddon.current?.fit();
            // 傳送新的終端尺寸到服務端
            if (websocket.current && websocket.current.readyState === WebSocket.OPEN) {
              const dimensions = fitAddon.current?.proposeDimensions();
              if (dimensions) {
                websocket.current.send(JSON.stringify({
                  type: 'resize',
                  cols: dimensions.cols,
                  rows: dimensions.rows
                }));
              }
            }
          } catch (e) {
            console.warn('Resize error:', e);
          }
        }, 100);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  if (!clusterId) {
    return <div>{t('messages.clusterNotFound')}</div>;
  }

  return (
    <div style={{ padding: '24px', height: '100vh', display: 'flex', flexDirection: 'column' }}>
      {/* 頁面頭部 */}
      <div style={{ marginBottom: 16, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16 }}>
        <Space size={8}>
          <Title level={4} style={{ margin: 0 }}>
            {t('kubectl.title')}
          </Title>
          <Text type="secondary" style={{ fontSize: 13 }}>
            {t('kubectl.cluster')}: {clusterId}
          </Text>
        </Space>

        <Space wrap>
          {!connected ? (
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={connectTerminal}
              loading={connecting}
            >
              {t('kubectl.connect')}
            </Button>
          ) : (
            <Button
              danger
              icon={<StopOutlined />}
              onClick={disconnectTerminal}
            >
              {t('kubectl.disconnect')}
            </Button>
          )}
          <Button icon={<ClearOutlined />} onClick={clearTerminal}>
            {t('kubectl.clear')}
          </Button>
          <Button icon={<FullscreenOutlined />} onClick={toggleFullscreen}>
            {t('kubectl.fullscreen')}
          </Button>
        </Space>
      </div>

      {/* 連線狀態提示（緊湊 banner，不佔整行高度） */}
      {(connecting || connected) && (
        <Alert
          message={connecting ? t('kubectl.prepHint') : t('kubectl.connectedTo', { clusterId })}
          type={connecting ? 'info' : 'success'}
          showIcon
          banner
          style={{ marginBottom: 8, flexShrink: 0 }}
        />
      )}

      {/* 終端介面 */}
      <Card 
        style={{ 
          flex: 1, 
          display: 'flex', 
          flexDirection: 'column',
          padding: 0,
        }}
        styles={{ 
          body: {
            flex: 1, 
            padding: 0,
            display: 'flex',
            flexDirection: 'column',
          }
        }}
      >
        <div
          ref={terminalRef}
          style={{
            flex: 1,
            minHeight: '400px',
            width: '100%',
          }}
        />
      </Card>
    </div>
  );
};

export default KubectlTerminalPage;

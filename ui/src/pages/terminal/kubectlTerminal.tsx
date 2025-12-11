import React, { useEffect, useRef, useState } from 'react';
import { useParams } from 'react-router-dom';
import {
  Card,
  Button,
  Space,
  Select,
  message,
  Typography,
  Alert,
  Row,
  Col,
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

const { Title, Text } = Typography;
const { Option } = Select;

const KubectlTerminalPage: React.FC = () => {
  const { id: clusterId } = useParams<{ id: string }>();
  
  const terminalRef = useRef<HTMLDivElement>(null);
  const terminal = useRef<Terminal | null>(null);
  const fitAddon = useRef<FitAddon | null>(null);
  const websocket = useRef<WebSocket | null>(null);
  
  const [selectedNamespace, setSelectedNamespace] = useState<string>('default');
  const [namespaces, setNamespaces] = useState<string[]>(['default', 'kube-system', 'kube-public']);
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  
  const connectedRef = useRef(false);
  const currentLineRef = useRef('');

  // 初始化终端
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

          // 添加插件
          fitAddon.current = new FitAddon();
          terminal.current.loadAddon(fitAddon.current);
          terminal.current.loadAddon(new WebLinksAddon());
          
          // 添加剪贴板支持
          try {
            const clipboardAddon = new ClipboardAddon();
            terminal.current.loadAddon(clipboardAddon);
          } catch (e) {
            console.warn('Clipboard addon not available:', e);
          }

          terminal.current.open(terminalRef.current);
          
          // 等待 DOM 完全渲染后再 fit
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

          // 延迟执行 fit 和显示欢迎信息
          setTimeout(() => {
            fitTerminal();
            setTimeout(() => {
              showWelcomeMessage();
            }, 200);
          }, 100);

          // 设置终端输入处理
          terminal.current.onData((data) => {
            handleTerminalInput(data);
          });

          // 添加键盘快捷键支持
          terminal.current.attachCustomKeyEventHandler((event) => {
            if (event.type === 'keydown') {
              // Ctrl+C 复制
              if (event.ctrlKey && event.key === 'c' && terminal.current?.hasSelection()) {
                const selection = terminal.current.getSelection();
                if (selection) {
                  navigator.clipboard.writeText(selection);
                }
                return false;
              }
              
              // Ctrl+V 粘贴
              if (event.ctrlKey && event.key === 'v') {
                pasteFromClipboard();
                return false;
              }
            }
            return true;
          });

        } catch (error) {
          console.error('初始化终端失败:', error);
          message.error('初始化终端失败');
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
  }, []);

  // 显示欢迎信息
  const showWelcomeMessage = () => {
    if (!terminal.current) return;
    
    terminal.current.clear();
    terminal.current.writeln('\x1b[32m╭─────────────────────────────────────────────────────────────╮\x1b[0m');
    terminal.current.writeln('\x1b[32m│                  KubePolaris Kubectl Terminal               │\x1b[0m');
    terminal.current.writeln('\x1b[32m╰─────────────────────────────────────────────────────────────╯\x1b[0m');
    terminal.current.writeln('');
    terminal.current.writeln(`\x1b[36mCluster:\x1b[0m ${clusterId}`);
    terminal.current.writeln(`\x1b[36mNamespace:\x1b[0m ${selectedNamespace}`);
    terminal.current.writeln('');
    terminal.current.writeln('\x1b[33m请选择命名空间并点击"连接终端"开始...\x1b[0m');
    terminal.current.writeln('');
  };

  // 处理终端输入
  const handleTerminalInput = (data: string) => {
    if (!connectedRef.current || !websocket.current) return;

    if (websocket.current.readyState !== WebSocket.OPEN) {
      terminal.current?.write('\r\nConnection lost. Please reconnect.\r\n');
      return;
    }

    const code = data.charCodeAt(0);
    
    // 处理回车键
    if (code === 13) {
      terminal.current?.write('\r\n');
      websocket.current.send(JSON.stringify({
        type: 'command',
        data: currentLineRef.current.trim(),
      }));
      currentLineRef.current = '';
      return;
    }
    
    // 处理退格键
    if (code === 127) {
      if (currentLineRef.current.length > 0) {
        currentLineRef.current = currentLineRef.current.slice(0, -1);
        websocket.current.send(JSON.stringify({
          type: 'input',
          data: '\u007f',
        }));
      }
      return;
    }
    
    // 处理 Ctrl+C (中断)
    if (code === 3) {
      if (websocket.current && websocket.current.readyState === WebSocket.OPEN) {
        websocket.current.send(JSON.stringify({
          type: 'interrupt',
          data: '',
        }));
        terminal.current?.write('^C\r\n');
        currentLineRef.current = '';
      }
      return;
    }
    
    // 处理 ESC 序列
    if (code === 27) {
      return;
    }
    
    // 处理普通字符
    if (code >= 32 && code <= 126) {
      currentLineRef.current += data;
      websocket.current.send(JSON.stringify({
        type: 'input',
        data: data,
      }));
    }
  };

  // 粘贴剪贴板内容
  const pasteFromClipboard = () => {
    if (!connectedRef.current) {
      message.error('请先连接终端');
      return;
    }
    
    navigator.clipboard.readText()
      .then((text) => {
        if (text && websocket.current && websocket.current.readyState === WebSocket.OPEN) {
          websocket.current.send(JSON.stringify({
            type: 'quick_command',
            data: text
          }));
          currentLineRef.current = '';
        }
      })
      .catch((err) => {
        console.error('粘贴失败:', err);
        message.error('粘贴失败，请检查浏览器权限');
      });
  };

  // 处理 WebSocket 消息
  const handleWebSocketMessage = (msg: any) => {
    if (!terminal.current) return;

    switch (msg.type) {
      case 'output':
        const outputText = msg.data;
        if (outputText.includes('\n')) {
          const lines = outputText.split('\n');
          for (let i = 0; i < lines.length; i++) {
            if (i < lines.length - 1 || outputText.endsWith('\n')) {
              terminal.current.writeln(lines[i]);
            } else {
              terminal.current.write(lines[i]);
            }
          }
        } else {
          terminal.current.write(outputText);
        }
        break;
      case 'error':
        terminal.current.writeln(`\r\n\x1b[31m${msg.data}\x1b[0m`);
        break;
      case 'command_result':
        terminal.current.write('$ ');
        break;
      case 'clear':
        terminal.current.clear();
        break;
      case 'namespace_changed':
        setSelectedNamespace(msg.data);
        terminal.current.writeln(`\r\nNamespace changed to: ${msg.data}\r\n`);
        break;
      default:
        break;
    }
  };

  // 连接终端
  const connectTerminal = () => {
    if (!clusterId) {
      message.error('缺少集群ID');
      return;
    }
    
    setConnecting(true);
    
    if (terminal.current) {
      terminal.current.clear();
      terminal.current.writeln('\x1b[33m正在连接终端...\x1b[0m');
    }
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.hostname}:8080/ws/clusters/${clusterId}/terminal?namespace=${selectedNamespace}`;
    
    try {
      const ws = new WebSocket(wsUrl);
      websocket.current = ws;
      
      ws.onopen = () => {
        setConnected(true);
        setConnecting(false);
        connectedRef.current = true;
        message.success('终端连接成功');
        
        if (terminal.current) {
          terminal.current.clear();
          terminal.current.writeln(`\x1b[32m✓ 已连接到集群: ${clusterId}\x1b[0m`);
          terminal.current.writeln(`\x1b[32m✓ 命名空间: ${selectedNamespace}\x1b[0m`);
          terminal.current.writeln('');
          terminal.current.writeln('\x1b[36m提示: 输入 help 或 ? 查看帮助信息\x1b[0m');
          terminal.current.writeln('');
        }
      };
      
      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          handleWebSocketMessage(msg);
        } catch {
          terminal.current?.write(event.data);
        }
      };
      
      ws.onerror = (error) => {
        console.error('WebSocket错误:', error);
        message.error('终端连接出错');
        setConnected(false);
        setConnecting(false);
        connectedRef.current = false;
        
        if (terminal.current) {
          terminal.current.writeln('\x1b[31m连接出错\x1b[0m');
        }
      };
      
      ws.onclose = () => {
        setConnected(false);
        setConnecting(false);
        connectedRef.current = false;
        message.info('终端连接已断开');
        
        if (terminal.current) {
          terminal.current.writeln('\x1b[31m\r\n连接已断开\x1b[0m');
        }
      };
      
    } catch (error) {
      console.error('创建WebSocket连接失败:', error);
      message.error('创建终端连接失败');
      setConnecting(false);
      
      if (terminal.current) {
        terminal.current.writeln('\x1b[31m创建连接失败\x1b[0m');
      }
    }
  };

  // 断开终端连接
  const disconnectTerminal = () => {
    if (websocket.current) {
      websocket.current.close();
      websocket.current = null;
    }
    setConnected(false);
    connectedRef.current = false;
    currentLineRef.current = '';
    
    if (terminal.current) {
      terminal.current.writeln('\x1b[33m\r\n手动断开连接\x1b[0m');
    }
  };

  // 清空终端
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

  // 命名空间变更
  const handleNamespaceChange = (value: string) => {
    setSelectedNamespace(value);
    
    if (connected && websocket.current && websocket.current.readyState === WebSocket.OPEN) {
      websocket.current.send(JSON.stringify({
        type: 'change_namespace',
        data: value
      }));
    }
  };

  // 窗口大小变化时重新调整终端大小
  useEffect(() => {
    const handleResize = () => {
      if (fitAddon.current && terminal.current) {
        setTimeout(() => {
          try {
            fitAddon.current?.fit();
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
    return <div>集群ID不存在</div>;
  }

  return (
    <div style={{ padding: '24px', height: '100vh', display: 'flex', flexDirection: 'column' }}>
      {/* 页面头部 */}
      <div style={{ marginBottom: 16, flexShrink: 0 }}>
        <Space>
          <Title level={3} style={{ margin: 0 }}>
            Kubectl 终端
          </Title>
          <Text type="secondary">
            集群: {clusterId}
          </Text>
        </Space>
        
        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col span={4}>
            <Select
              placeholder="选择命名空间"
              value={selectedNamespace}
              onChange={handleNamespaceChange}
              style={{ width: '100%' }}
              disabled={connected}
            >
              {namespaces.map(ns => (
                <Option key={ns} value={ns}>
                  {ns}
                </Option>
              ))}
            </Select>
          </Col>
          
          <Col span={20}>
            <Space>
              {!connected ? (
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={connectTerminal}
                  loading={connecting}
                >
                  连接终端
                </Button>
              ) : (
                <Button
                  danger
                  icon={<StopOutlined />}
                  onClick={disconnectTerminal}
                >
                  断开连接
                </Button>
              )}
              
              <Button
                icon={<ClearOutlined />}
                onClick={clearTerminal}
              >
                清空
              </Button>
              
              <Button
                icon={<FullscreenOutlined />}
                onClick={toggleFullscreen}
              >
                全屏
              </Button>
            </Space>
          </Col>
        </Row>
      </div>

      {/* 连接状态提示 */}
      {connected && (
        <Alert
          message={`已连接到集群 ${clusterId} - 命名空间: ${selectedNamespace}`}
          type="success"
          showIcon
          style={{ marginBottom: 16, flexShrink: 0 }}
        />
      )}

      {/* 终端界面 */}
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

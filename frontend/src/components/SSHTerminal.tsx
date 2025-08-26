import React, { useEffect, useRef, useState } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebLinksAddon } from 'xterm-addon-web-links';
import { Card, Button, Space, Input, Modal, Form, message, Select, Spin } from 'antd';
import { 
  PlayCircleOutlined, 
  StopOutlined, 
  ReloadOutlined, 
  SettingOutlined,
  DisconnectOutlined,
  LinkOutlined
} from '@ant-design/icons';
import 'xterm/css/xterm.css';

interface SSHTerminalProps {
  nodeIP?: string;
  nodeName?: string;
  clusterId?: string;
}

interface SSHConnection {
  host: string;
  port: number;
  username: string;
  password?: string;
  privateKey?: string;
  authType: 'password' | 'key';
}

const SSHTerminal: React.FC<SSHTerminalProps> = ({ nodeIP, nodeName, clusterId }) => {
  const terminalRef = useRef<HTMLDivElement>(null);
  const terminal = useRef<Terminal | null>(null);
  const fitAddon = useRef<FitAddon | null>(null);
  const websocket = useRef<WebSocket | null>(null);
  
  const [isConnected, setIsConnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [connectionModalVisible, setConnectionModalVisible] = useState(false);
  const [form] = Form.useForm();

  // 初始化终端
  const initTerminal = () => {
    if (!terminalRef.current) return;

    // 创建终端实例
    terminal.current = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
      theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
        cursor: '#ffffff',
        selectionBackground: '#264f78',
        black: '#000000',
        red: '#cd3131',
        green: '#0dbc79',
        yellow: '#e5e510',
        blue: '#2472c8',
        magenta: '#bc3fbc',
        cyan: '#11a8cd',
        white: '#e5e5e5',
        brightBlack: '#666666',
        brightRed: '#f14c4c',
        brightGreen: '#23d18b',
        brightYellow: '#f5f543',
        brightBlue: '#3b8eea',
        brightMagenta: '#d670d6',
        brightCyan: '#29b8db',
        brightWhite: '#e5e5e5'
      },
      cols: 80,
      rows: 24,
    });

    // 添加插件
    fitAddon.current = new FitAddon();
    terminal.current.loadAddon(fitAddon.current);
    terminal.current.loadAddon(new WebLinksAddon());

    // 打开终端
    terminal.current.open(terminalRef.current);
    fitAddon.current.fit();

    // 显示欢迎信息
    terminal.current.writeln('\x1b[1;32m欢迎使用SSH终端\x1b[0m');
    terminal.current.writeln('\x1b[1;33m请点击"连接"按钮配置SSH连接\x1b[0m');
    terminal.current.writeln('');

    // 监听终端输入
    terminal.current.onData((data) => {
      if (websocket.current && websocket.current.readyState === WebSocket.OPEN) {
        websocket.current.send(JSON.stringify({
          type: 'input',
          data: data
        }));
      }
    });

    // 监听窗口大小变化
    const handleResize = () => {
      if (fitAddon.current) {
        fitAddon.current.fit();
        if (websocket.current && websocket.current.readyState === WebSocket.OPEN) {
          const { cols, rows } = terminal.current!;
          websocket.current.send(JSON.stringify({
            type: 'resize',
            cols,
            rows
          }));
        }
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  };

  // 连接SSH
  const connectSSH = async (connection: SSHConnection) => {
    if (!terminal.current) return;

    setIsConnecting(true);
    terminal.current.clear();
    terminal.current.writeln('\x1b[1;33m正在连接SSH服务器...\x1b[0m');

    try {
      // 创建WebSocket连接到后端SSH代理
      const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${wsProtocol}//${window.location.hostname}:8080/api/ssh/connect`;
      
      websocket.current = new WebSocket(wsUrl);

      websocket.current.onopen = () => {
        // 发送连接配置
        websocket.current!.send(JSON.stringify({
          type: 'connect',
          config: connection
        }));
      };

      websocket.current.onmessage = (event) => {
        const message = JSON.parse(event.data);
        
        switch (message.type) {
          case 'connected':
            setIsConnected(true);
            setIsConnecting(false);
            terminal.current!.clear();
            terminal.current!.writeln('\x1b[1;32m✓ SSH连接成功\x1b[0m');
            terminal.current!.writeln(`\x1b[1;36m连接到: ${connection.username}@${connection.host}:${connection.port}\x1b[0m`);
            terminal.current!.writeln('');
            message.success('SSH连接成功');
            break;
            
          case 'data':
            terminal.current!.write(message.data);
            break;
            
          case 'error':
            setIsConnecting(false);
            setIsConnected(false);
            terminal.current!.writeln(`\x1b[1;31m✗ 连接失败: ${message.error}\x1b[0m`);
            message.error(`SSH连接失败: ${message.error}`);
            break;
            
          case 'disconnected':
            setIsConnected(false);
            terminal.current!.writeln('\x1b[1;33m连接已断开\x1b[0m');
            break;
        }
      };

      websocket.current.onerror = (error) => {
        setIsConnecting(false);
        setIsConnected(false);
        terminal.current!.writeln('\x1b[1;31m✗ WebSocket连接失败\x1b[0m');
        message.error('WebSocket连接失败');
      };

      websocket.current.onclose = () => {
        setIsConnected(false);
        setIsConnecting(false);
        terminal.current!.writeln('\x1b[1;33m连接已关闭\x1b[0m');
      };

    } catch (error) {
      setIsConnecting(false);
      terminal.current.writeln(`\x1b[1;31m✗ 连接失败: ${error}\x1b[0m`);
      message.error('SSH连接失败');
    }
  };

  // 断开连接
  const disconnect = () => {
    if (websocket.current) {
      websocket.current.close();
      websocket.current = null;
    }
    setIsConnected(false);
    if (terminal.current) {
      terminal.current.writeln('\x1b[1;33m连接已断开\x1b[0m');
    }
  };

  // 清空终端
  const clearTerminal = () => {
    if (terminal.current) {
      terminal.current.clear();
    }
  };

  // 处理连接表单提交
  const handleConnect = async (values: any) => {
    const connection: SSHConnection = {
      host: values.host || nodeIP || '',
      port: values.port || 22,
      username: values.username,
      authType: values.authType,
      password: values.authType === 'password' ? values.password : undefined,
      privateKey: values.authType === 'key' ? values.privateKey : undefined,
    };

    await connectSSH(connection);
    setConnectionModalVisible(false);
    form.resetFields();
  };

  // 组件挂载时初始化终端
  useEffect(() => {
    const cleanup = initTerminal();
    return () => {
      if (cleanup) cleanup();
      if (websocket.current) {
        websocket.current.close();
      }
      if (terminal.current) {
        terminal.current.dispose();
      }
    };
  }, []);

  // 设置默认连接信息
  useEffect(() => {
    if (nodeIP) {
      form.setFieldsValue({
        host: nodeIP,
        port: 22,
        username: 'root',
        authType: 'password'
      });
    }
  }, [nodeIP, form]);

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* 工具栏 */}
      <Card size="small" style={{ marginBottom: 16 }}>
        <Space>
          <Button
            type="primary"
            icon={<LinkOutlined />}
            onClick={() => setConnectionModalVisible(true)}
            disabled={isConnecting}
          >
            {isConnected ? '重新连接' : '连接'}
          </Button>
          
          <Button
            icon={<DisconnectOutlined />}
            onClick={disconnect}
            disabled={!isConnected}
            danger
          >
            断开连接
          </Button>
          
          <Button
            icon={<ReloadOutlined />}
            onClick={clearTerminal}
          >
            清空
          </Button>

          {isConnecting && (
            <Space>
              <Spin size="small" />
              <span>连接中...</span>
            </Space>
          )}

          {isConnected && (
            <span style={{ color: '#52c41a' }}>
              ✓ 已连接到 {nodeIP}
            </span>
          )}
        </Space>
      </Card>

      {/* 终端容器 */}
      <Card 
        style={{ 
          flex: 1, 
          padding: 0,
          backgroundColor: '#1e1e1e'
        }}
        bodyStyle={{ 
          padding: 8, 
          height: '100%',
          backgroundColor: '#1e1e1e'
        }}
      >
        <div
          ref={terminalRef}
          style={{ 
            height: '500px',
            width: '100%'
          }}
        />
      </Card>

      {/* SSH连接配置模态框 */}
      <Modal
        title="SSH连接配置"
        open={connectionModalVisible}
        onCancel={() => setConnectionModalVisible(false)}
        footer={null}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleConnect}
          initialValues={{
            host: nodeIP,
            port: 22,
            username: 'root',
            authType: 'password'
          }}
        >
          <Form.Item
            label="主机地址"
            name="host"
            rules={[{ required: true, message: '请输入主机地址' }]}
          >
            <Input placeholder="请输入SSH服务器地址" />
          </Form.Item>

          <Form.Item
            label="端口"
            name="port"
            rules={[{ required: true, message: '请输入端口号' }]}
          >
            <Input type="number" placeholder="22" />
          </Form.Item>

          <Form.Item
            label="用户名"
            name="username"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input placeholder="请输入用户名" />
          </Form.Item>

          <Form.Item
            label="认证方式"
            name="authType"
            rules={[{ required: true, message: '请选择认证方式' }]}
          >
            <Select>
              <Select.Option value="password">密码认证</Select.Option>
              <Select.Option value="key">私钥认证</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) =>
              prevValues.authType !== currentValues.authType
            }
          >
            {({ getFieldValue }) => {
              const authType = getFieldValue('authType');
              
              if (authType === 'password') {
                return (
                  <Form.Item
                    label="密码"
                    name="password"
                    rules={[{ required: true, message: '请输入密码' }]}
                  >
                    <Input.Password placeholder="请输入密码" />
                  </Form.Item>
                );
              }
              
              if (authType === 'key') {
                return (
                  <Form.Item
                    label="私钥"
                    name="privateKey"
                    rules={[{ required: true, message: '请输入私钥内容' }]}
                  >
                    <Input.TextArea
                      rows={6}
                      placeholder="请粘贴私钥内容（PEM格式）"
                    />
                  </Form.Item>
                );
              }
              
              return null;
            }}
          </Form.Item>

          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setConnectionModalVisible(false)}>
                取消
              </Button>
              <Button type="primary" htmlType="submit" loading={isConnecting}>
                连接
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default SSHTerminal;
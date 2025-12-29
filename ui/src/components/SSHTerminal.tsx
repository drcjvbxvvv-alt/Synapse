import React, { useEffect, useRef, useState } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebLinksAddon } from 'xterm-addon-web-links';
import { Card, Button, Space, Input, Modal, Form, message, Select, Spin } from 'antd';
import { 
  ReloadOutlined, 
  DisconnectOutlined,
  LinkOutlined
} from '@ant-design/icons';
import { systemSettingService } from '../services/authService';
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

const SSHTerminal: React.FC<SSHTerminalProps> = ({ nodeIP }) => {
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
    terminal.current.writeln('\x1b[1;32m欢迎使用 SSH 终端\x1b[0m');
    terminal.current.writeln('\x1b[1;33m请点击"连接"按钮连接到服务器\x1b[0m');
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

    // 获取认证 token
    const token = localStorage.getItem('token');
    if (!token) {
      message.error('未登录，请先登录');
      return;
    }

    setIsConnecting(true);
    terminal.current.clear();
    terminal.current.writeln('\x1b[1;33m正在连接SSH服务器...\x1b[0m');

    try {
      // 创建WebSocket连接到后端SSH代理
      const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      // 在 URL 中添加 token 参数用于 WebSocket 认证
      const wsUrl = `${wsProtocol}//${window.location.hostname}:8080/ws/ssh/terminal?token=${encodeURIComponent(token)}`;
      
      websocket.current = new WebSocket(wsUrl);

      websocket.current.onopen = () => {
        // 发送连接配置
        websocket.current!.send(JSON.stringify({
          type: 'connect',
          config: connection
        }));
      };

      websocket.current.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        
        switch (msg.type) {
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
            terminal.current!.write(msg.data);
            break;
            
          case 'error':
            setIsConnecting(false);
            setIsConnected(false);
            terminal.current!.writeln(`\x1b[1;31m✗ 连接失败: ${msg.error}\x1b[0m`);
            message.error(`SSH连接失败: ${msg.error}`);
            break;
            
          case 'disconnected':
            setIsConnected(false);
            terminal.current!.writeln('\x1b[1;33m连接已断开\x1b[0m');
            break;
        }
      };

      websocket.current.onerror = () => {
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

  // 点击连接按钮 - 优先使用全局配置，否则弹出表单
  const handleConnectClick = async () => {
    if (!nodeIP) {
      message.error('节点 IP 未知');
      return;
    }

    setIsConnecting(true);
    terminal.current?.clear();
    terminal.current?.writeln('\x1b[1;36m正在检查全局 SSH 配置...\x1b[0m');

    try {
      // 获取全局 SSH 凭据
      const response = await systemSettingService.getSSHCredentials();
      
      if (response.code === 200 && response.data?.enabled) {
        const sshConfig = response.data;
        
        terminal.current?.writeln('\x1b[1;32m✓ 已启用全局 SSH 配置，正在连接...\x1b[0m');
        
        // 使用全局配置连接
        const connection: SSHConnection = {
          host: nodeIP,
          port: sshConfig.port || 22,
          username: sshConfig.username || 'root',
          authType: sshConfig.auth_type as 'password' | 'key',
          password: sshConfig.auth_type === 'password' ? sshConfig.password : undefined,
          privateKey: sshConfig.auth_type === 'key' ? sshConfig.private_key : undefined,
        };
        
        // 连接
        await connectSSH(connection);
      } else {
        // 全局配置未启用，弹出表单让用户手动配置
        setIsConnecting(false);
        terminal.current?.writeln('\x1b[1;33m全局 SSH 配置未启用，请手动配置连接信息\x1b[0m');
        terminal.current?.writeln('');
        setConnectionModalVisible(true);
      }
    } catch (error) {
      console.error('获取全局SSH配置失败:', error);
      setIsConnecting(false);
      terminal.current?.writeln('\x1b[1;33m获取全局 SSH 配置失败，请手动配置连接信息\x1b[0m');
      terminal.current?.writeln('');
      setConnectionModalVisible(true);
    }
  };

  // 处理手动连接表单提交
  const handleManualConnect = async (values: Record<string, unknown>) => {
    const connection: SSHConnection = {
      host: (values.host as string) || nodeIP || '',
      port: (values.port as number) || 22,
      username: values.username as string,
      authType: values.authType as 'password' | 'key',
      password: values.authType === 'password' ? (values.password as string) : undefined,
      privateKey: values.authType === 'key' ? (values.privateKey as string) : undefined,
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
            onClick={handleConnectClick}
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
        styles={{
          body: { 
            padding: 8, 
            height: '100%',
            backgroundColor: '#1e1e1e'
          }
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

      {/* SSH连接配置模态框（仅在全局配置未启用时使用） */}
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
          onFinish={handleManualConnect}
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

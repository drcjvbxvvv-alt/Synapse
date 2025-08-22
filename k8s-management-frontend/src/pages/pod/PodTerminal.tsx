import React, { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
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
  ArrowLeftOutlined,
  ReloadOutlined,
  DisconnectOutlined,
  FullscreenOutlined,
} from '@ant-design/icons';
import { PodService } from '../../services/podService';
import type { PodInfo } from '../../services/podService';

const { Title, Text } = Typography;
const { Option } = Select;

interface PodTerminalProps {}

const PodTerminal: React.FC<PodTerminalProps> = () => {
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();
  const navigate = useNavigate();
  
  const [pod, setPod] = useState<PodInfo | null>(null);
  const [selectedContainer, setSelectedContainer] = useState<string>('');
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  
  const terminalRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  // 获取Pod详情
  const fetchPodDetail = async () => {
    if (!clusterId || !namespace || !name) return;
    
    try {
      const response = await PodService.getPodDetail(clusterId, namespace, name);
      
      if (response.code === 200) {
        setPod(response.data.pod);
        // 如果没有选择容器且有容器列表，默认选择第一个
        if (!selectedContainer && response.data.pod.containers.length > 0) {
          setSelectedContainer(response.data.pod.containers[0].name);
        }
      } else {
        message.error(response.message || '获取Pod详情失败');
      }
    } catch (error) {
      console.error('获取Pod详情失败:', error);
      message.error('获取Pod详情失败');
    }
  };

  // 连接终端
  const connectTerminal = () => {
    if (!clusterId || !namespace || !name || !selectedContainer) {
      message.error('缺少必要参数');
      return;
    }
    
    if (pod?.status !== 'Running') {
      message.error('Pod必须处于Running状态才能连接终端');
      return;
    }
    
    setConnecting(true);
    
    // 构建WebSocket URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/clusters/${clusterId}/pods/${namespace}/${name}/terminal?container=${selectedContainer}`;
    
    try {
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;
      
      ws.onopen = () => {
        setConnected(true);
        setConnecting(false);
        message.success('终端连接成功');
        
        // 初始化终端显示
        if (terminalRef.current) {
          terminalRef.current.innerHTML = `
            <div style="color: #00ff00; margin-bottom: 10px;">
              ✓ 已连接到 Pod: ${namespace}/${name}
              ✓ 容器: ${selectedContainer}
              ✓ 集群: ${clusterId}
            </div>
            <div style="color: #ffff00; margin-bottom: 10px;">
              提示: 您现在可以在此容器中执行命令
            </div>
            <div style="color: #ffffff;">
              root@${name}:/#&nbsp;
            </div>
          `;
        }
      };
      
      ws.onmessage = (event) => {
        if (terminalRef.current) {
          const data = event.data;
          // 简单的终端输出显示
          terminalRef.current.innerHTML += data;
          // 自动滚动到底部
          terminalRef.current.scrollTop = terminalRef.current.scrollHeight;
        }
      };
      
      ws.onerror = (error) => {
        console.error('WebSocket错误:', error);
        message.error('终端连接出错');
        setConnected(false);
        setConnecting(false);
      };
      
      ws.onclose = () => {
        setConnected(false);
        setConnecting(false);
        message.info('终端连接已断开');
        
        if (terminalRef.current) {
          terminalRef.current.innerHTML += `
            <div style="color: #ff0000; margin-top: 10px;">
              连接已断开
            </div>
          `;
        }
      };
      
    } catch (error) {
      console.error('创建WebSocket连接失败:', error);
      message.error('创建终端连接失败');
      setConnecting(false);
    }
  };

  // 断开终端连接
  const disconnectTerminal = () => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setConnected(false);
  };

  // 发送命令到终端
  const sendCommand = (command: string) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(command + '\n');
    }
  };

  // 处理键盘输入
  const handleKeyPress = (event: React.KeyboardEvent) => {
    if (!connected) return;
    
    if (event.key === 'Enter') {
      const input = (event.target as HTMLInputElement);
      const command = input.value;
      if (command.trim()) {
        sendCommand(command);
        input.value = '';
      }
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

  useEffect(() => {
    fetchPodDetail();
  }, [clusterId, namespace, name]);

  // 组件卸载时断开连接
  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  if (!pod) {
    return <div>加载中...</div>;
  }

  return (
    <div style={{ padding: '24px', height: 'calc(100vh - 64px)' }}>
      {/* 页面头部 */}
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => navigate(`/clusters/${clusterId}/pods/${namespace}/${name}`)}
          >
            返回
          </Button>
          <Title level={3} style={{ margin: 0 }}>
            Pod 终端
          </Title>
          <Text type="secondary">
            {namespace}/{name}
          </Text>
        </Space>
        
        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col span={4}>
            <Select
              placeholder="选择容器"
              value={selectedContainer}
              onChange={setSelectedContainer}
              style={{ width: '100%' }}
              disabled={connected}
            >
              {pod.containers.map(container => (
                <Option key={container.name} value={container.name}>
                  {container.name}
                </Option>
              ))}
            </Select>
          </Col>
          
          <Col span={20}>
            <Space>
              {!connected ? (
                <Button
                  type="primary"
                  icon={<ReloadOutlined />}
                  onClick={connectTerminal}
                  loading={connecting}
                  disabled={!selectedContainer || pod.status !== 'Running'}
                >
                  连接终端
                </Button>
              ) : (
                <Button
                  danger
                  icon={<DisconnectOutlined />}
                  onClick={disconnectTerminal}
                >
                  断开连接
                </Button>
              )}
              
              <Button
                icon={<FullscreenOutlined />}
                onClick={toggleFullscreen}
                disabled={!connected}
              >
                全屏
              </Button>
            </Space>
          </Col>
        </Row>
      </div>

      {/* 状态提示 */}
      {pod.status !== 'Running' && (
        <Alert
          message="Pod状态异常"
          description={`Pod当前状态为 ${pod.status}，只有Running状态的Pod才能连接终端。`}
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {connected && (
        <Alert
          message="终端已连接"
          description="您现在可以在下方终端中执行命令。注意：这是一个简化的终端实现，完整功能需要后端WebSocket支持。"
          type="success"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {/* 终端界面 */}
      <Card style={{ height: 'calc(100% - 140px)' }}>
        <div
          ref={terminalRef}
          style={{
            height: 'calc(100% - 60px)',
            backgroundColor: '#1e1e1e',
            color: '#d4d4d4',
            padding: '16px',
            fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
            fontSize: '14px',
            lineHeight: '1.4',
            overflow: 'auto',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}
        >
          {!connected && (
            <div style={{ color: '#888' }}>
              请选择容器并点击"连接终端"按钮开始...
            </div>
          )}
        </div>
        
        {/* 命令输入框 */}
        {connected && (
          <div style={{ marginTop: '10px', display: 'flex', alignItems: 'center' }}>
            <Text style={{ color: '#00ff00', marginRight: '8px' }}>$</Text>
            <input
              type="text"
              placeholder="输入命令并按回车执行..."
              onKeyPress={handleKeyPress}
              style={{
                flex: 1,
                backgroundColor: '#2d2d2d',
                color: '#d4d4d4',
                border: '1px solid #555',
                padding: '8px 12px',
                borderRadius: '4px',
                fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
                fontSize: '14px',
              }}
            />
          </div>
        )}
      </Card>
    </div>
  );
};

export default PodTerminal;
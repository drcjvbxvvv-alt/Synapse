import React, { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Button,
  Space,
  Select,
  Switch,
  InputNumber,
  message,
  Typography,
  Alert,
  Spin,
  Row,
  Col,
} from 'antd';
import {
  ArrowLeftOutlined,
  ReloadOutlined,
  DownloadOutlined,
  ClearOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
} from '@ant-design/icons';
import { PodService } from '../../services/podService';
import type { PodInfo } from '../../services/podService';

const { Title, Text } = Typography;
const { Option } = Select;

interface PodLogsProps {}

const PodLogs: React.FC<PodLogsProps> = () => {
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();
  const navigate = useNavigate();
  
  const [pod, setPod] = useState<PodInfo | null>(null);
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(false);
  const [following, setFollowing] = useState(false);
  
  // 日志选项
  const [selectedContainer, setSelectedContainer] = useState<string>('');
  const [previous, setPrevious] = useState(false);
  const [tailLines, setTailLines] = useState<number>(100);
  const [sinceSeconds, setSinceSeconds] = useState<number | undefined>(undefined);
  
  const logsRef = useRef<HTMLPreElement>(null);
  const followIntervalRef = useRef<NodeJS.Timeout | null>(null);

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

  // 获取日志
  const fetchLogs = async (isFollow = false) => {
    if (!clusterId || !namespace || !name) return;
    
    setLoading(true);
    try {
      const response = await PodService.getPodLogs(
        clusterId,
        namespace,
        name,
        selectedContainer || undefined,
        isFollow,
        previous,
        tailLines,
        sinceSeconds
      );
      
      if (response.code === 200) {
        if (isFollow) {
          // 追加日志内容
          setLogs(prev => prev + response.data.logs);
        } else {
          // 替换日志内容
          setLogs(response.data.logs);
        }
        
        // 自动滚动到底部
        setTimeout(() => {
          if (logsRef.current) {
            logsRef.current.scrollTop = logsRef.current.scrollHeight;
          }
        }, 100);
      } else {
        message.error(response.message || '获取日志失败');
      }
    } catch (error) {
      console.error('获取日志失败:', error);
      message.error('获取日志失败');
    } finally {
      setLoading(false);
    }
  };

  // 开始/停止跟踪日志
  const toggleFollow = () => {
    if (following) {
      // 停止跟踪
      if (followIntervalRef.current) {
        clearInterval(followIntervalRef.current);
        followIntervalRef.current = null;
      }
      setFollowing(false);
    } else {
      // 开始跟踪
      setFollowing(true);
      fetchLogs(false); // 先获取一次完整日志
      
      // 每5秒获取新日志
      followIntervalRef.current = setInterval(() => {
        fetchLogs(true);
      }, 5000);
    }
  };

  // 清空日志
  const clearLogs = () => {
    setLogs('');
  };

  // 下载日志
  const downloadLogs = () => {
    if (!logs) {
      message.warning('没有日志内容可下载');
      return;
    }
    
    const blob = new Blob([logs], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${namespace}-${name}-${selectedContainer || 'all'}-logs.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    
    message.success('日志下载成功');
  };

  // 刷新日志
  const refreshLogs = () => {
    fetchLogs(false);
  };

  useEffect(() => {
    fetchPodDetail();
  }, [clusterId, namespace, name]);

  useEffect(() => {
    if (selectedContainer) {
      fetchLogs(false);
    }
  }, [selectedContainer, previous, tailLines, sinceSeconds]);

  // 组件卸载时清理定时器
  useEffect(() => {
    return () => {
      if (followIntervalRef.current) {
        clearInterval(followIntervalRef.current);
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
            Pod 日志
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
            >
              {pod.containers.map(container => (
                <Option key={container.name} value={container.name}>
                  {container.name}
                </Option>
              ))}
            </Select>
          </Col>
          
          <Col span={3}>
            <Space>
              <Text>显示行数:</Text>
              <InputNumber
                min={10}
                max={10000}
                value={tailLines}
                onChange={(value) => setTailLines(value || 100)}
                style={{ width: 80 }}
              />
            </Space>
          </Col>
          
          <Col span={3}>
            <Space>
              <Text>时间范围(秒):</Text>
              <InputNumber
                min={1}
                placeholder="全部"
                value={sinceSeconds}
                onChange={setSinceSeconds}
                style={{ width: 100 }}
              />
            </Space>
          </Col>
          
          <Col span={3}>
            <Space>
              <Text>前一个容器:</Text>
              <Switch
                checked={previous}
                onChange={setPrevious}
                size="small"
              />
            </Space>
          </Col>
          
          <Col span={11}>
            <Space>
              <Button
                type="primary"
                icon={<ReloadOutlined />}
                onClick={refreshLogs}
                loading={loading}
              >
                刷新
              </Button>
              
              <Button
                icon={following ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
                onClick={toggleFollow}
                type={following ? 'default' : 'primary'}
              >
                {following ? '停止跟踪' : '跟踪日志'}
              </Button>
              
              <Button
                icon={<DownloadOutlined />}
                onClick={downloadLogs}
                disabled={!logs}
              >
                下载
              </Button>
              
              <Button
                icon={<ClearOutlined />}
                onClick={clearLogs}
                disabled={!logs}
              >
                清空
              </Button>
            </Space>
          </Col>
        </Row>
      </div>

      {/* 状态提示 */}
      {following && (
        <Alert
          message="正在跟踪日志"
          description="日志将每5秒自动更新一次，点击"停止跟踪"按钮可停止自动更新。"
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {/* 日志内容 */}
      <Card style={{ height: 'calc(100% - 140px)' }}>
        <Spin spinning={loading} tip="加载日志中...">
          <pre
            ref={logsRef}
            style={{
              height: '100%',
              overflow: 'auto',
              backgroundColor: '#1e1e1e',
              color: '#d4d4d4',
              padding: '16px',
              margin: 0,
              fontSize: '13px',
              fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace',
              lineHeight: '1.4',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {logs || '暂无日志内容'}
          </pre>
        </Spin>
      </Card>
    </div>
  );
};

export default PodLogs;
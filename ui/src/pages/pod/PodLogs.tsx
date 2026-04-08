import React, { useState, useEffect, useRef, useCallback } from 'react';
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
import { useTranslation } from 'react-i18next';

const { Title, Text } = Typography;
const { Option } = Select;

// WebSocket訊息型別
interface LogMessage {
  type: 'connected' | 'start' | 'log' | 'end' | 'error' | 'closed';
  data?: string;
  message?: string;
}

type PodLogsProps = Record<string, never>;

const PodLogs: React.FC<PodLogsProps> = () => {
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();
  const navigate = useNavigate();
  
const { t } = useTranslation(['pod', 'common']);
const [pod, setPod] = useState<PodInfo | null>(null);
  const [logs, setLogs] = useState('');
  const [loading, setLoading] = useState(false);
  const [following, setFollowing] = useState(false);
  const [connected, setConnected] = useState(false);
  
  // 日誌選項
  const [selectedContainer, setSelectedContainer] = useState<string>('');
  const [previous, setPrevious] = useState(false);
  const [tailLines, setTailLines] = useState<number>(100);
  const [sinceSeconds, setSinceSeconds] = useState<number | undefined>(undefined);
  
  const logsRef = useRef<HTMLPreElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const followingRef = useRef(false); // stable ref to avoid stale closure in onclose

  // 獲取Pod詳情
  const fetchPodDetail = useCallback(async () => {
    if (!clusterId || !namespace || !name) return;
    
    try {
      const response = await PodService.getPodDetail(clusterId, namespace, name);
      
      setPod(response.pod);
      if (!selectedContainer && response.pod.containers.length > 0) {
        setSelectedContainer(response.pod.containers[0].name);
      }
    } catch (error) {
      console.error('獲取Pod詳情失敗:', error);
      message.error(t('pod:logs.fetchPodError'));
    }
  }, [clusterId, namespace, name, selectedContainer]);

  // 獲取日誌
  const fetchLogs = useCallback(async (isFollow = false) => {
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
      
      if (isFollow) {
        setLogs(prev => prev + response.logs);
      } else {
        setLogs(response.logs);
      }
      
      setTimeout(() => {
        if (logsRef.current) {
          logsRef.current.scrollTop = logsRef.current.scrollHeight;
        }
      }, 100);
    } catch (error) {
      console.error('獲取日誌失敗:', error);
      message.error(t('pod:logs.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, name, selectedContainer, previous, tailLines, sinceSeconds]);

  // 建立 WebSocket 連線（支援重連呼叫）
  const connectWebSocket = useCallback(() => {
    if (!clusterId || !namespace || !name) return;
    try {
      const ws = PodService.createLogStream(clusterId, namespace, name, {
        container: selectedContainer || undefined,
        previous,
        tailLines,
        sinceSeconds,
      });
      wsRef.current = ws;

      ws.onopen = () => {
        reconnectAttemptsRef.current = 0;
        setConnected(true);
        setLoading(false);
      };

      ws.onmessage = (event) => {
        try {
          const msg: LogMessage = JSON.parse(event.data);
          switch (msg.type) {
            case 'connected':
              message.success(t('pod:logs.connectedToStream'));
              break;
            case 'log':
              if (msg.data) {
                setLogs((prev) => prev + msg.data);
                setTimeout(() => {
                  if (logsRef.current) {
                    logsRef.current.scrollTop = logsRef.current.scrollHeight;
                  }
                }, 0);
              }
              break;
            case 'end':
              message.info(t('pod:logs.streamEnded'));
              followingRef.current = false;
              setFollowing(false);
              setConnected(false);
              break;
            case 'error':
              message.error(msg.message || t('pod:logs.streamError'));
              followingRef.current = false;
              setFollowing(false);
              setConnected(false);
              break;
            case 'closed':
              followingRef.current = false;
              setFollowing(false);
              setConnected(false);
              break;
            default:
              break;
          }
        } catch {
          // ignore parse errors
        }
      };

      ws.onerror = () => {
        setConnected(false);
        setLoading(false);
      };

      ws.onclose = () => {
        setConnected(false);
        setLoading(false);
        // 指數退避重連（只在用戶仍想跟蹤時）
        if (followingRef.current) {
          const attempts = reconnectAttemptsRef.current;
          const delay = Math.min(1000 * Math.pow(2, attempts), 30000); // 1s → 2s → 4s … 最多 30s
          reconnectAttemptsRef.current += 1;
          reconnectTimerRef.current = setTimeout(() => {
            if (followingRef.current) {
              setLoading(true);
              connectWebSocket();
            }
          }, delay);
        }
      };
    } catch {
      message.error(t('pod:logs.createConnectionFailed'));
      followingRef.current = false;
      setFollowing(false);
      setLoading(false);
    }
  }, [clusterId, namespace, name, selectedContainer, previous, tailLines, sinceSeconds, t]);

  // 開始/停止跟蹤日誌
  const toggleFollow = () => {
    if (following) {
      // 停止跟蹤
      followingRef.current = false;
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      reconnectAttemptsRef.current = 0;
      setFollowing(false);
      setConnected(false);
    } else {
      if (!clusterId || !namespace || !name) {
        message.error(t('pod:logs.missingParams'));
        return;
      }
      followingRef.current = true;
      reconnectAttemptsRef.current = 0;
      setFollowing(true);
      setLoading(true);
      connectWebSocket();
    }
  };

  // 清空日誌
  const clearLogs = () => {
    setLogs('');
  };

  // 下載日誌
  const downloadLogs = () => {
    if (!logs) {
      message.warning(t('pod:logs.noContentToDownload'));
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
    
    message.success(t('pod:logs.downloadSuccess'));
  };

  // 重新整理日誌
  const refreshLogs = () => {
    fetchLogs(false);
  };

  useEffect(() => {
    fetchPodDetail();
  }, [fetchPodDetail]);

  useEffect(() => {
    if (selectedContainer) {
      fetchLogs(false);
    }
  }, [selectedContainer, previous, tailLines, sinceSeconds, fetchLogs]);

  // 元件解除安裝時清理 WebSocket 連線與重連計時器
  useEffect(() => {
    return () => {
      followingRef.current = false;
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, []);

  if (!pod) {
    return <div>{t('pod:logs.loading')}</div>;
  }

  return (
    <div style={{ padding: '24px', height: 'calc(100vh - 64px)' }}>
      {/* 頁面頭部 */}
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => navigate(`/clusters/${clusterId}/pods/${namespace}/${name}`)}
          >
            {t('pod:logs.back')}
          </Button>
          <Title level={3} style={{ margin: 0 }}>
            {t('pod:logs.title')}
          </Title>
          <Text type="secondary">
            {namespace}/{name}
          </Text>
        </Space>
        
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center', marginTop: 16 }}>
          <Select
            placeholder={t('pod:logs.selectContainer')}
            value={selectedContainer}
            onChange={setSelectedContainer}
            style={{ width: 160 }}
          >
            {pod.containers.map(container => (
              <Option key={container.name} value={container.name}>
                {container.name}
              </Option>
            ))}
          </Select>

          <Space size={4}>
            <Text>{t('pod:logs.tailLines')}:</Text>
            <InputNumber
              min={10}
              max={10000}
              value={tailLines}
              onChange={(value) => setTailLines(value || 100)}
              style={{ width: 80 }}
            />
          </Space>

          <Space size={4}>
            <Text>{t('pod:logs.sinceSeconds')}:</Text>
            <InputNumber
              min={1}
              placeholder={t('pod:logs.allTime')}
              value={sinceSeconds}
              onChange={(value) => setSinceSeconds(value ?? undefined)}
              style={{ width: 100 }}
            />
          </Space>

          <Space size={4}>
            <Text>{t('pod:logs.previousContainer')}:</Text>
            <Switch
              checked={previous}
              onChange={setPrevious}
              size="small"
            />
          </Space>

          <Space size={4}>
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              onClick={refreshLogs}
              loading={loading}
            >
              {t('pod:logs.refresh')}
            </Button>

            <Button
              icon={following ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
              onClick={toggleFollow}
              type={following ? 'default' : 'primary'}
            >
              {following ? t('pod:logs.stopFollow') : t('pod:logs.startFollow')}
            </Button>

            <Button
              icon={<DownloadOutlined />}
              onClick={downloadLogs}
              disabled={!logs}
            >
              {t('pod:logs.downloadBtn')}
            </Button>

            <Button
              icon={<ClearOutlined />}
              onClick={clearLogs}
              disabled={!logs}
            >
              {t('pod:logs.clearBtn')}
            </Button>
          </Space>
        </div>
      </div>

      {/* 狀態提示 */}
      {following && connected && (
        <Alert
          message={t('pod:logs.followingAlert')}
          description={t('pod:logs.followingAlertDesc')}
          type="success"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}
      
      {following && !connected && (
        <Alert
          message={t('pod:logs.connectingAlert')}
          description={t('pod:logs.connectingAlertDesc')}
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {/* 日誌內容 */}
      <Card style={{ height: 'calc(100% - 140px)' }}>
        <Spin spinning={loading} tip={t('pod:logs.loadingLogs')}>
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
            {logs || t('pod:logs.noLogContent')}
          </pre>
        </Spin>
      </Card>
    </div>
  );
};

export default PodLogs;
import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import {
  Card,
  Tabs,
  Row,
  Col,
  Statistic,
  Space,
  Tag,
  Button,
  Select,
  Switch,
  message,
  Empty,
  Badge,
  Tooltip,
  Input,
  DatePicker,
  Table,
  Typography,
  Modal,
  Checkbox,
  Spin,
  Alert,
} from 'antd';
import {
  FileTextOutlined,
  ThunderboltOutlined,
  SearchOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  DownloadOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ClearOutlined,
  PlusOutlined,
  DatabaseOutlined,
  EditOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { Form } from 'antd';
import { useParams } from 'react-router-dom';
import { List as VirtualList } from 'react-window';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import { logService, logSourceService } from '../../services/logService';
import { useTranslation } from 'react-i18next';
import type {
  LogEntry,
  EventLogEntry,
  LogStats,
  LogStreamTarget,
  LogPodInfo,
  LogSearchParams,
  LogSource,
} from '../../services/logService';

const { TabPane } = Tabs;
const { RangePicker } = DatePicker;
const { Text } = Typography;

// 日誌級別顏色
const levelColors: Record<string, string> = {
  error: '#ff4d4f',
  warn: '#faad14',
  info: '#1890ff',
  debug: '#8c8c8c',
};

// 日誌級別Tag顏色
const levelTagColors: Record<string, string> = {
  error: 'red',
  warn: 'orange',
  info: 'blue',
  debug: 'default',
};

const LogCenter: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
const { t } = useTranslation(['logs', 'common']);
const [activeTab, setActiveTab] = useState('stream');
  const [stats, setStats] = useState<LogStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);

  // ===== {t('logs:center.realTimeLogs')}流狀態 =====
  const [streaming, setStreaming] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [targets, setTargets] = useState<LogStreamTarget[]>([]);
  const [maxLines] = useState(1000);
  const [showTimestamp, setShowTimestamp] = useState(true);
  const [showSource, setShowSource] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const [levelFilter, setLevelFilter] = useState<string[]>([]);
  const [logSearchKeyword, setLogSearchKeyword] = useState(''); // 實時{t('logs:center.logSearch')}關鍵字
  const wsRef = useRef<WebSocket | null>(null);
  const logsEndRef = useRef<HTMLDivElement>(null);

  // ===== 外部日誌源狀態 =====
  const [logSources, setLogSources] = useState<LogSource[]>([]);
  const [logSourcesLoading, setLogSourcesLoading] = useState(false);
  const [srcModalOpen, setSrcModalOpen] = useState(false);
  const [editingSrc, setEditingSrc] = useState<LogSource | null>(null);
  const [srcForm] = Form.useForm();
  const [selectedSrcId, setSelectedSrcId] = useState<number | null>(null);
  const [extQuery, setExtQuery] = useState('');
  const [extIndex, setExtIndex] = useState('');
  const [extDateRange, setExtDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);
  const [extResults, setExtResults] = useState<LogEntry[]>([]);
  const [extSearchLoading, setExtSearchLoading] = useState(false);

  // Pod選擇器狀態
  const [podSelectorVisible, setPodSelectorVisible] = useState(false);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [selectedNamespace, setSelectedNamespace] = useState<string>('');
  const [pods, setPods] = useState<LogPodInfo[]>([]);
  const [podsLoading, setPodsLoading] = useState(false);
  const [selectedPods, setSelectedPods] = useState<LogStreamTarget[]>([]);
  const [podSearchKeyword, setPodSearchKeyword] = useState(''); // Pod搜尋關鍵字

  // ===== 效能最佳化：使用 useMemo =====
  // 已選 Pod 的 Set，用於 O(1) 查詢
  const selectedPodsSet = useMemo(() => {
    return new Set(selectedPods.map((p) => `${p.namespace}/${p.pod}`));
  }, [selectedPods]);

  // 過濾後的實時日誌（日誌級別 + 關鍵字搜尋）
  const filteredLogs = useMemo(() => {
    let result = logs;
    
    // 1. 日誌級別過濾
    if (levelFilter.length > 0) {
      result = result.filter((log) => levelFilter.includes(log.level));
    }
    
    // 2. 關鍵字搜尋過濾
    if (logSearchKeyword.trim()) {
      const keyword = logSearchKeyword.toLowerCase();
      result = result.filter(
        (log) =>
          log.message.toLowerCase().includes(keyword) ||
          log.pod_name?.toLowerCase().includes(keyword) ||
          log.namespace?.toLowerCase().includes(keyword) ||
          log.container?.toLowerCase().includes(keyword)
      );
    }
    
    return result;
  }, [logs, levelFilter, logSearchKeyword]);

  // 高亮關鍵字的函式
  const highlightKeyword = (text: string, keyword: string) => {
    if (!keyword.trim() || !text) return text;
    const regex = new RegExp(`(${keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
    const parts = text.split(regex);
    return parts.map((part, i) =>
      regex.test(part) ? (
        <span key={i} style={{ backgroundColor: '#ffe58f', color: '#000', padding: '0 2px', borderRadius: 2 }}>
          {part}
        </span>
      ) : (
        part
      )
    );
  };

  // 過濾後的 Pod 列表
  const filteredPods = useMemo(() => {
    if (!podSearchKeyword.trim()) return pods;
    const keyword = podSearchKeyword.toLowerCase();
    return pods.filter(
      (pod) =>
        pod.name.toLowerCase().includes(keyword) ||
        pod.containers.some((c) => c.toLowerCase().includes(keyword))
    );
  }, [pods, podSearchKeyword]);

  // ===== 事件日誌狀態 =====
  const [events, setEvents] = useState<EventLogEntry[]>([]);
  const [eventsLoading, setEventsLoading] = useState(false);
  const [eventNamespace, setEventNamespace] = useState<string>('');
  const [eventType, setEventType] = useState<'Normal' | 'Warning' | undefined>();

  // ===== 日誌搜尋狀態 =====
  const [searchResults, setSearchResults] = useState<LogEntry[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searchNamespaces, setSearchNamespaces] = useState<string[]>([]);
  const [searchLevels, setSearchLevels] = useState<string[]>([]);
  const [searchDateRange, setSearchDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);

  // 獲取統計資料
  const fetchStats = useCallback(async () => {
    if (!clusterId) return;
    setStatsLoading(true);
    try {
      const res = await logService.getLogStats(clusterId, { timeRange: '1h' });
      setStats(res);
    } catch (error) {
      console.error('獲取日誌統計失敗', error);
    } finally {
      setStatsLoading(false);
    }
  }, [clusterId]);

  // 獲取命名空間列表
  const fetchNamespaces = useCallback(async () => {
    if (!clusterId) return;
    try {
      const res = await logService.getNamespaces(clusterId);
      setNamespaces(res || []);
    } catch (error) {
      console.error('獲取命名空間失敗', error);
    }
  }, [clusterId]);

  // 獲取Pod列表
  const fetchPods = useCallback(async (namespace?: string) => {
    if (!clusterId) return;
    setPodsLoading(true);
    try {
      const res = await logService.getPods(clusterId, namespace);
      setPods(res || []);
    } catch (error) {
      console.error('獲取Pod列表失敗', error);
    } finally {
      setPodsLoading(false);
    }
  }, [clusterId]);

  // 獲取事件日誌
  const fetchEvents = useCallback(async () => {
    if (!clusterId) return;
    setEventsLoading(true);
    try {
      const res = await logService.getEventLogs(clusterId, {
        namespace: eventNamespace || undefined,
        type: eventType,
        limit: 200,
      });
      setEvents(res?.items || []);
    } catch (error) {
      console.error('獲取事件日誌失敗', error);
      message.error(t('logs:center.fetchEventsFailed'));
    } finally {
      setEventsLoading(false);
    }
  }, [clusterId, eventNamespace, eventType]);

  // 日誌搜尋
  const handleSearch = useCallback(async () => {
    if (!clusterId) return;
    setSearchLoading(true);
    try {
      const params: LogSearchParams = {
        keyword: searchKeyword || undefined,
        namespaces: searchNamespaces.length > 0 ? searchNamespaces : undefined,
        levels: searchLevels.length > 0 ? searchLevels : undefined,
        limit: 500,
      };
      if (searchDateRange) {
        params.startTime = searchDateRange[0].toISOString();
        params.endTime = searchDateRange[1].toISOString();
      }

      const res = await logService.searchLogs(clusterId, params);
      setSearchResults(res?.items || []);
    } catch (error) {
      console.error('日誌搜尋失敗', error);
      message.error(t('logs:center.searchFailed'));
    } finally {
      setSearchLoading(false);
    }
  }, [clusterId, searchKeyword, searchNamespaces, searchLevels, searchDateRange]);

  useEffect(() => {
    fetchStats();
    fetchNamespaces();
  }, [fetchStats, fetchNamespaces]);

  useEffect(() => {
    if (activeTab === 'events') {
      fetchEvents();
    }
  }, [activeTab, fetchEvents]);

  // 自動滾動
  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoScroll]);

  // 開始/停止日誌流
  const toggleStream = useCallback(() => {
    if (!clusterId) return;

    if (streaming) {
      wsRef.current?.close();
      wsRef.current = null;
      setStreaming(false);
    } else {
      if (targets.length === 0) {
        message.warning(t('logs:center.selectPodForMonitor'));
        return;
      }

      const streamConfig = {
        targets,
        tail_lines: 100,
        show_timestamp: showTimestamp,
        show_source: showSource,
      };
      
      const { ws, config } = logService.createAggregateLogStream(clusterId, streamConfig);

      ws.onopen = () => {
        // 連線成功後傳送配置
        ws.send(JSON.stringify(config));
        setStreaming(true);
        message.success(t('logs:center.connectedToSources', { count: targets.length }));
      };

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          if (msg.type === 'log') {
            setLogs((prev) => {
              const newLogs = [...prev, msg as LogEntry];
              if (newLogs.length > maxLines) {
                return newLogs.slice(-maxLines);
              }
              return newLogs;
            });
          } else if (msg.type === 'error') {
            message.error(msg.message);
          }
        } catch (e) {
          console.error('解析訊息失敗', e);
        }
      };

      ws.onerror = () => {
        message.error(t('logs:center.connectionError'));
        setStreaming(false);
      };

      ws.onclose = () => {
        setStreaming(false);
      };

      wsRef.current = ws;
    }
  }, [streaming, targets, clusterId, maxLines, showTimestamp, showSource]);

  // 清空日誌
  const clearLogs = () => setLogs([]);

  // 下載日誌
  const downloadLogs = () => {
    const content = logs
      .map((log) => {
        const parts = [];
        if (showTimestamp) parts.push(log.timestamp);
        if (showSource) parts.push(`[${log.namespace}/${log.pod_name}]`);
        parts.push(log.message);
        return parts.join(' ');
      })
      .join('\n');

    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `logs-${new Date().toISOString()}.txt`;
    a.click();
    URL.revokeObjectURL(url);
    message.success(t('logs:center.downloadSuccess'));
  };

  // 移除目標
  const removeTarget = (index: number) => {
    setTargets(targets.filter((_, i) => i !== index));
  };

  // 開啟Pod選擇器
  const openPodSelector = () => {
    setPodSelectorVisible(true);
    setSelectedPods([]);
  };

  // 確認選擇Pod
  const confirmPodSelection = () => {
    setTargets([...targets, ...selectedPods]);
    setPodSelectorVisible(false);
    setSelectedPods([]);
  };

  // 事件表格列
  const eventColumns: ColumnsType<EventLogEntry> = [
    {
      title: t('logs:center.time'),
      dataIndex: 'last_timestamp',
      width: 170,
      render: (time: string) => (
        <Text type="secondary">
          {dayjs(time).format('YYYY-MM-DD HH:mm:ss')}
        </Text>
      ),
    },
    {
      title: t('common:table.type'),
      dataIndex: 'type',
      width: 80,
      render: (type: string) => (
        <Tag color={type === 'Warning' ? 'orange' : 'green'}>{type}</Tag>
      ),
    },
    {
      title: t('logs:center.reason'),
      dataIndex: 'reason',
      width: 120,
    },
    {
      title: t('logs:center.resource'),
      key: 'resource',
      width: 200,
      render: (_, record) => (
        <Space>
          <Tag color="cyan">{record.involved_kind}</Tag>
          <Text ellipsis style={{ maxWidth: 120 }}>
            {record.involved_name}
          </Text>
        </Space>
      ),
    },
    {
      title: t('logs:center.message'),
      dataIndex: 'message',
      ellipsis: true,
    },
    {
      title: t('logs:center.count'),
      dataIndex: 'count',
      width: 60,
      align: 'center',
    },
  ];

  // 搜尋結果表格列
  const searchColumns: ColumnsType<LogEntry> = [
    {
      title: t('logs:center.time'),
      dataIndex: 'timestamp',
      width: 180,
      render: (time: string) => (
        <Text type="secondary">
          {dayjs(time).format('YYYY-MM-DD HH:mm:ss.SSS')}
        </Text>
      ),
    },
    {
      title: t('logs:center.level'),
      dataIndex: 'level',
      width: 80,
      render: (level: string) => (
        <Tag color={levelTagColors[level] || 'default'}>
          {level.toUpperCase()}
        </Tag>
      ),
    },
    {
      title: t('logs:center.source'),
      key: 'source',
      width: 250,
      render: (_, record) => (
        <Tooltip title={`${record.namespace}/${record.pod_name}:${record.container}`}>
          <Text ellipsis style={{ maxWidth: 230 }}>
            <Tag color="cyan">{record.namespace}</Tag>
            {record.pod_name}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('logs:center.logContent'),
      dataIndex: 'message',
      render: (message: string) => (
        <Text
          style={{
            fontFamily: 'monospace',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}
        >
          {message}
        </Text>
      ),
    },
  ];

  // ===== 外部日誌源 handlers =====
  const loadLogSources = useCallback(async () => {
    if (!clusterId) return;
    setLogSourcesLoading(true);
    try {
      const data = await logSourceService.list(clusterId);
      setLogSources(data || []);
    } catch {
      // ignore
    } finally {
      setLogSourcesLoading(false);
    }
  }, [clusterId]);

  useEffect(() => {
    if (activeTab === 'external') {
      loadLogSources();
    }
  }, [activeTab, loadLogSources]);

  const handleSaveLogSource = async () => {
    try {
      const values = await srcForm.validateFields();
      if (editingSrc) {
        await logSourceService.update(clusterId!, editingSrc.id, values);
      } else {
        await logSourceService.create(clusterId!, { ...values, enabled: true });
      }
      message.success(editingSrc ? '更新成功' : '建立成功');
      setSrcModalOpen(false);
      srcForm.resetFields();
      setEditingSrc(null);
      loadLogSources();
    } catch (e: unknown) {
      if ((e as { errorFields?: unknown }).errorFields) return;
      message.error('操作失敗');
    }
  };

  const handleDeleteLogSource = async (src: LogSource) => {
    try {
      await logSourceService.delete(clusterId!, src.id);
      message.success('刪除成功');
      if (selectedSrcId === src.id) setSelectedSrcId(null);
      loadLogSources();
    } catch {
      message.error('刪除失敗');
    }
  };

  const handleExtSearch = async () => {
    if (!selectedSrcId) {
      message.warning('請先選擇一個日誌源');
      return;
    }
    setExtSearchLoading(true);
    try {
      const params: { query: string; index?: string; startTime?: string; endTime?: string; limit: number } = {
        query: extQuery,
        limit: 500,
      };
      if (extIndex) params.index = extIndex;
      if (extDateRange) {
        params.startTime = extDateRange[0].toISOString();
        params.endTime = extDateRange[1].toISOString();
      }
      const result = await logSourceService.search(clusterId!, selectedSrcId, params);
      setExtResults(result.items || []);
    } catch (err: unknown) {
      message.error('查詢失敗: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      setExtSearchLoading(false);
    }
  };

  return (
    <div style={{ padding: 24, background: '#f0f2f5', minHeight: '100vh' }}>
      {/* 統計概覽 */}
      <Spin spinning={statsLoading}>
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={4}>
            <Card size="small" bordered={false}>
              <Statistic
                title={t('logs:center.totalCount1h')}
                value={stats?.total_count || 0}
                prefix={<FileTextOutlined style={{ color: '#1890ff' }} />}
              />
            </Card>
          </Col>
          <Col span={4}>
            <Card size="small" bordered={false}>
              <Statistic
                title={t('logs:center.errorEvents')}
                value={stats?.error_count || 0}
                valueStyle={{ color: '#ff4d4f' }}
                prefix={<CloseCircleOutlined />}
              />
            </Card>
          </Col>
          <Col span={4}>
            <Card size="small" bordered={false}>
              <Statistic
                title={t('logs:center.warningEvents')}
                value={stats?.warn_count || 0}
                valueStyle={{ color: '#faad14' }}
                prefix={<WarningOutlined />}
              />
            </Card>
          </Col>
          <Col span={12}>
            <Card size="small" bordered={false}>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                }}
              >
                <span style={{ fontWeight: 500 }}>{t('logs:center.namespaceDistribution')}</span>
                <Space wrap size="small">
                  {stats?.namespace_stats?.slice(0, 5).map((ns) => (
                    <Tag key={ns.namespace} color="blue">
                      {ns.namespace}: {ns.count}
                    </Tag>
                  ))}
                </Space>
              </div>
            </Card>
          </Col>
        </Row>
      </Spin>

      {/* 主內容區 */}
      <Card bordered={false}>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          tabBarExtraContent={
            <Space>
              <Button icon={<SyncOutlined />} onClick={fetchStats}>
                {t('logs:center.refreshStats')}
              </Button>
            </Space>
          }
        >
          {/* 實時日誌流 Tab */}
          <TabPane
            tab={
              <span>
                <ThunderboltOutlined />
                實時日誌
              </span>
            }
            key="stream"
          >
            {/* 工具欄 */}
            <div
              style={{
                marginBottom: 16,
                display: 'flex',
                justifyContent: 'space-between',
              }}
            >
              <Space>
                <Button
                  type={streaming ? 'default' : 'primary'}
                  icon={streaming ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
                  onClick={toggleStream}
                  danger={streaming}
                >
                  {streaming ? t('logs:center.stop') : t('logs:center.startMonitor')}
                </Button>
                <Button icon={<ClearOutlined />} onClick={clearLogs}>
                  {t('logs:center.clear')}
                </Button>
                <Button
                  icon={<DownloadOutlined />}
                  onClick={downloadLogs}
                  disabled={logs.length === 0}
                >
                  {t('logs:center.download')}
                </Button>
              </Space>

              <Space>
                <Select
                  mode="multiple"
                  placeholder={t('logs:center.logLevelFilter')}
                  style={{ width: 200 }}
                  value={levelFilter}
                  onChange={setLevelFilter}
                  options={[
                    { label: t('logs:center.error'), value: 'error' },
                    { label: t('logs:center.warning'), value: 'warn' },
                    { label: t('logs:center.info'), value: 'info' },
                    { label: t('logs:center.debug'), value: 'debug' },
                  ]}
                />
                <Tooltip title={t('logs:center.showTimestamp')}>
                  <Switch
                    checked={showTimestamp}
                    onChange={setShowTimestamp}
                    checkedChildren={t('logs:center.timestamp')}
                    unCheckedChildren={t('logs:center.timestamp')}
                  />
                </Tooltip>
                <Tooltip title={t('logs:center.showSource')}>
                  <Switch
                    checked={showSource}
                    onChange={setShowSource}
                    checkedChildren={t('logs:center.source')}
                    unCheckedChildren={t('logs:center.source')}
                  />
                </Tooltip>
                <Tooltip title={t('logs:center.autoScroll')}>
                  <Switch
                    checked={autoScroll}
                    onChange={setAutoScroll}
                    checkedChildren={t('logs:center.scroll')}
                    unCheckedChildren={t('logs:center.scroll')}
                  />
                </Tooltip>
              </Space>
            </div>

            {/* Pod選擇器 */}
            <Card size="small" style={{ marginBottom: 16 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontWeight: 500 }}>{t('logs:center.monitorTarget')}</span>
                {targets.map((t, i) => (
                  <Tag
                    key={i}
                    closable
                    onClose={() => removeTarget(i)}
                    color="blue"
                  >
                    {t.namespace}/{t.pod}
                    {t.container && `:${t.container}`}
                  </Tag>
                ))}
                <Button
                  type="dashed"
                  size="small"
                  icon={<PlusOutlined />}
                  onClick={openPodSelector}
                >
                  {t('logs:center.addPod')}
                </Button>
                {streaming && (
                  <Badge
                    status="processing"
                    text={t('logs:center.monitoring')}
                    style={{ marginLeft: 'auto' }}
                  />
                )}
              </div>
            </Card>

            {/* 日誌搜尋框 */}
            <div style={{ marginBottom: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
              <Input
                placeholder={t('logs:center.searchLogPlaceholder')}
                prefix={<SearchOutlined />}
                allowClear
                value={logSearchKeyword}
                onChange={(e) => setLogSearchKeyword(e.target.value)}
                style={{ width: 350 }}
              />
              {logSearchKeyword && (
                <Text type="secondary">
                  {t('logs:center.matchCount', { filtered: filteredLogs.length, total: logs.length })}
                </Text>
              )}
            </div>

            {/* 日誌顯示區 */}
            <div
              style={{
                height: 'calc(100vh - 540px)',
                minHeight: 400,
                backgroundColor: '#1e1e1e',
                borderRadius: 8,
                overflow: 'auto',
                fontFamily: "'Fira Code', 'Monaco', 'Menlo', monospace",
                fontSize: 13,
                lineHeight: 1.6,
              }}
            >
              {filteredLogs.length === 0 ? (
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    height: '100%',
                    color: '#666',
                  }}
                >
                  <Empty
                    description={
                      streaming ? t('logs:center.waitingLogs') : t('logs:center.selectPodFirst')
                    }
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                  />
                </div>
              ) : (
                <div style={{ padding: 16 }}>
                  {filteredLogs.map((log, index) => (
                    <div
                      key={log.id || index}
                      style={{
                        display: 'flex',
                        gap: 8,
                        marginBottom: 2,
                        color: '#d4d4d4',
                      }}
                    >
                      {showTimestamp && (
                        <span style={{ color: '#6a9955', whiteSpace: 'nowrap' }}>
                          {dayjs(log.timestamp).format('HH:mm:ss.SSS')}
                        </span>
                      )}
                      {showSource && (
                        <span style={{ color: '#569cd6', whiteSpace: 'nowrap' }}>
                          [{logSearchKeyword ? highlightKeyword(`${log.namespace}/${log.pod_name}`, logSearchKeyword) : `${log.namespace}/${log.pod_name}`}]
                        </span>
                      )}
                      <span
                        style={{
                          color: levelColors[log.level] || '#d4d4d4',
                          fontWeight: log.level === 'error' ? 600 : 400,
                          wordBreak: 'break-all',
                        }}
                      >
                        {logSearchKeyword ? highlightKeyword(log.message, logSearchKeyword) : log.message}
                      </span>
                    </div>
                  ))}
                  <div ref={logsEndRef} />
                </div>
              )}
            </div>

            {/* 狀態列 */}
            <div
              style={{
                marginTop: 8,
                display: 'flex',
                justifyContent: 'space-between',
                color: '#8c8c8c',
                fontSize: 12,
              }}
            >
              <span>{t('logs:center.totalLogs', { count: filteredLogs.length })}</span>
              <span>{t('logs:center.maxRetain', { max: maxLines })}</span>
            </div>
          </TabPane>

          {/* {t('logs:center.k8sEvents')} Tab */}
          <TabPane
            tab={
              <span>
                <WarningOutlined />
                K8s事件
              </span>
            }
            key="events"
          >
            {/* 篩選 */}
            <Space wrap style={{ marginBottom: 16 }}>
              <Select
                placeholder={t('common:table.namespace')}
                allowClear
                style={{ width: 180 }}
                value={eventNamespace || undefined}
                onChange={(v) => setEventNamespace(v || '')}
                showSearch
                options={namespaces.map((ns) => ({ label: ns, value: ns }))}
              />
              <Select
                placeholder={t('logs:events.eventType')}
                allowClear
                style={{ width: 120 }}
                value={eventType}
                onChange={setEventType}
                options={[
                  { label: 'Normal', value: 'Normal' },
                  { label: 'Warning', value: 'Warning' },
                ]}
              />
              <Button
                type="primary"
                icon={<SearchOutlined />}
                onClick={fetchEvents}
                loading={eventsLoading}
              >
                {t('logs:center.query')}
              </Button>
            </Space>

            <Table
              columns={eventColumns}
              dataSource={events}
              rowKey="id"
              loading={eventsLoading}
              pagination={{
                pageSize: 20,
                showSizeChanger: true,
                showTotal: (total) => t('logs:center.totalCount', { total }),
              }}
              size="small"
              scroll={{ y: 'calc(100vh - 500px)' }}
            />
          </TabPane>

          {/* 日誌搜尋 Tab */}
          <TabPane
            tab={
              <span>
                <SearchOutlined />
                日誌搜尋
              </span>
            }
            key="search"
          >
            {/* 搜尋欄 */}
            <Card size="small" style={{ marginBottom: 16 }}>
              <Space wrap style={{ width: '100%' }}>
                <Input.Search
                  placeholder={t('logs:center.searchKeywordPlaceholder')}
                  style={{ width: 300 }}
                  value={searchKeyword}
                  onChange={(e) => setSearchKeyword(e.target.value)}
                  onSearch={handleSearch}
                  enterButton={<SearchOutlined />}
                />

                <Select
                  mode="multiple"
                  placeholder={t('common:table.namespace')}
                  style={{ width: 200 }}
                  value={searchNamespaces}
                  onChange={setSearchNamespaces}
                  options={namespaces.map((ns) => ({ label: ns, value: ns }))}
                />

                <Select
                  mode="multiple"
                  placeholder={t('logs:center.logLevel')}
                  style={{ width: 150 }}
                  value={searchLevels}
                  onChange={setSearchLevels}
                  options={[
                    { label: 'ERROR', value: 'error' },
                    { label: 'WARN', value: 'warn' },
                    { label: 'INFO', value: 'info' },
                    { label: 'DEBUG', value: 'debug' },
                  ]}
                />

                <RangePicker
                  showTime
                  value={searchDateRange}
                  onChange={(dates) =>
                    setSearchDateRange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)
                  }
                  placeholder={[t('logs:center.startTime'), t('logs:center.endTime')]}
                />

                <Button
                  type="primary"
                  icon={<SearchOutlined />}
                  onClick={handleSearch}
                  loading={searchLoading}
                >
                  {t('logs:center.searchBtn')}
                </Button>
              </Space>
            </Card>

            {/* 搜尋結果 */}
            <Card
              size="small"
              title={t('logs:center.searchResults', { count: searchResults.length })}
            >
              <Table
                columns={searchColumns}
                dataSource={searchResults}
                rowKey="id"
                loading={searchLoading}
                pagination={{
                  pageSize: 50,
                  showSizeChanger: true,
                  showTotal: (total) => t('common:table.totalCount', { count: total }),
                }}
                size="small"
                scroll={{ y: 'calc(100vh - 550px)' }}
              />
            </Card>
          </TabPane>

          {/* 外部日誌源 Tab */}
          <TabPane
            tab={<span><DatabaseOutlined />外部日誌</span>}
            key="external"
          >
            {/* 日誌源管理 */}
            <Card
              size="small"
              title="日誌源管理"
              style={{ marginBottom: 16 }}
              extra={
                <Button
                  type="primary"
                  size="small"
                  icon={<PlusOutlined />}
                  onClick={() => { setEditingSrc(null); srcForm.resetFields(); setSrcModalOpen(true); }}
                >
                  新增日誌源
                </Button>
              }
            >
              <Table<LogSource>
                scroll={{ x: 'max-content' }}
                loading={logSourcesLoading}
                dataSource={logSources}
                rowKey="id"
                size="small"
                pagination={false}
                locale={{ emptyText: '暫無日誌源，點選「新增日誌源」配置 Loki 或 Elasticsearch' }}
                rowSelection={{
                  type: 'radio',
                  selectedRowKeys: selectedSrcId ? [selectedSrcId] : [],
                  onChange: (keys) => setSelectedSrcId(keys[0] as number),
                }}
                columns={[
                  { title: '名稱', dataIndex: 'name' },
                  { title: '型別', dataIndex: 'type', width: 130, render: (t: string) => <Tag color={t === 'loki' ? 'blue' : 'orange'}>{t.toUpperCase()}</Tag> },
                  { title: 'URL', dataIndex: 'url', ellipsis: true },
                  { title: '狀態', dataIndex: 'enabled', width: 80, render: (v: boolean) => <Tag color={v ? 'green' : 'red'}>{v ? '啟用' : '停用'}</Tag> },
                  {
                    title: '操作', width: 110,
                    render: (_: unknown, record: LogSource) => (
                      <Space>
                        <Button size="small" icon={<EditOutlined />} type="link" onClick={() => {
                          setEditingSrc(record);
                          srcForm.setFieldsValue({ type: record.type, name: record.name, url: record.url, username: record.username, enabled: record.enabled });
                          setSrcModalOpen(true);
                        }} />
                        <Tooltip title="刪除">
                          <Button size="small" icon={<DeleteOutlined />} type="link" danger onClick={() => handleDeleteLogSource(record)} />
                        </Tooltip>
                      </Space>
                    ),
                  },
                ]}
              />
            </Card>

            {/* 查詢介面 */}
            <Card size="small" title="查詢日誌" style={{ marginBottom: 16 }}>
              <Space wrap style={{ width: '100%' }}>
                <Input
                  placeholder={selectedSrcId ? (logSources.find(s => s.id === selectedSrcId)?.type === 'loki' ? 'LogQL 查詢，如 {namespace="default"}' : 'Lucene 查詢，如 error AND namespace:default') : '請先在上方選擇日誌源'}
                  style={{ width: 420 }}
                  value={extQuery}
                  onChange={(e) => setExtQuery(e.target.value)}
                  onPressEnter={handleExtSearch}
                />
                {selectedSrcId && logSources.find(s => s.id === selectedSrcId)?.type === 'elasticsearch' && (
                  <Input
                    placeholder="ES Index（如 k8s-logs-*）"
                    style={{ width: 180 }}
                    value={extIndex}
                    onChange={(e) => setExtIndex(e.target.value)}
                  />
                )}
                <RangePicker
                  showTime
                  value={extDateRange}
                  onChange={(dates) => setExtDateRange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)}
                  placeholder={['開始時間', '結束時間']}
                />
                <Button type="primary" icon={<SearchOutlined />} loading={extSearchLoading} onClick={handleExtSearch}>
                  查詢
                </Button>
              </Space>
            </Card>

            {/* 查詢結果 */}
            <Card size="small" title={`查詢結果（${extResults.length} 筆）`}>
              <Table<LogEntry>
                dataSource={extResults}
                rowKey={(r, i) => r.id || String(i)}
                size="small"
                loading={extSearchLoading}
                pagination={{ pageSize: 50, showSizeChanger: true }}
                scroll={{ y: 'calc(100vh - 600px)' }}
                columns={[
                  {
                    title: '時間', dataIndex: 'timestamp', width: 180,
                    render: (v: string) => new Date(v).toLocaleString('zh-TW'),
                  },
                  { title: '級別', dataIndex: 'level', width: 80, render: (v: string) => <Tag color={levelTagColors[v] || 'default'}>{v?.toUpperCase()}</Tag> },
                  { title: '命名空間', dataIndex: 'namespace', width: 130 },
                  { title: 'Pod', dataIndex: 'pod_name', width: 150, ellipsis: true },
                  { title: '訊息', dataIndex: 'message', ellipsis: true },
                ]}
              />
            </Card>
          </TabPane>
        </Tabs>
      </Card>

      {/* 外部日誌源 Modal */}
      <Modal
        title={editingSrc ? '編輯日誌源' : '新增日誌源'}
        open={srcModalOpen}
        onOk={handleSaveLogSource}
        onCancel={() => { setSrcModalOpen(false); setEditingSrc(null); srcForm.resetFields(); }}
        okText="儲存"
        cancelText="取消"
      >
        <Form form={srcForm} layout="vertical">
          <Form.Item name="type" label="型別" rules={[{ required: true }]}>
            <Select options={[{ label: 'Loki', value: 'loki' }, { label: 'Elasticsearch', value: 'elasticsearch' }]} disabled={!!editingSrc} />
          </Form.Item>
          <Form.Item name="name" label="名稱" rules={[{ required: true }]}>
            <Input placeholder="如：prod-loki" />
          </Form.Item>
          <Form.Item name="url" label="URL" rules={[{ required: true }]}>
            <Input placeholder="如：http://loki.monitoring:3100" />
          </Form.Item>
          <Form.Item name="username" label="使用者名稱（可選）">
            <Input placeholder="HTTP Basic Auth 使用者名稱" />
          </Form.Item>
          <Form.Item name="password" label="密碼（可選）">
            <Input.Password placeholder="HTTP Basic Auth 密碼" />
          </Form.Item>
          <Form.Item name="apiKey" label="API Key（可選）">
            <Input.Password placeholder="Loki：X-Scope-OrgID；ES：ApiKey" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Pod選擇器彈窗 */}
      <Modal
        title={t('logs:center.selectPod')}
        open={podSelectorVisible}
        onOk={confirmPodSelection}
        onCancel={() => {
          setPodSelectorVisible(false);
          setPodSearchKeyword(''); // 關閉時清空搜尋
        }}
        width={700}
        okText={t('logs:center.confirmAdd')}
        cancelText={t('common:actions.cancel')}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Select
            placeholder={t('logs:center.selectNamespace')}
            style={{ width: '100%' }}
            value={selectedNamespace || undefined}
            onChange={(v) => {
              setSelectedNamespace(v);
              setPodSearchKeyword(''); // 切換命名空間時清空搜尋
              fetchPods(v);
            }}
            showSearch
            options={namespaces.map((ns) => ({ label: ns, value: ns }))}
          />

          {/* Pod 搜尋框 */}
          {pods.length > 0 && (
            <Input
              placeholder={t('logs:center.searchPodPlaceholder')}
              prefix={<SearchOutlined />}
              allowClear
              value={podSearchKeyword}
              onChange={(e) => setPodSearchKeyword(e.target.value)}
              style={{ marginBottom: 8 }}
            />
          )}

          <Spin spinning={podsLoading}>
            {pods.length === 0 ? (
              <Empty description={t('logs:center.selectNamespaceFirst')} />
            ) : filteredPods.length === 0 ? (
              <Empty description={t('logs:center.noMatchingPods')} />
            ) : (
              <>
                {/* 顯示過濾結果統計和全選按鈕 */}
                <div style={{ marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ color: '#888' }}>
                    {t('logs:center.totalPods', { total: pods.length })}
                    {podSearchKeyword && `, ${t('logs:center.matchingPods', { filtered: filteredPods.length })}`}
                    {t('logs:center.selectedPods', { count: selectedPods.length })}
                  </span>
                  <Checkbox
                    indeterminate={
                      filteredPods.some((p) => selectedPodsSet.has(`${p.namespace}/${p.name}`)) &&
                      !filteredPods.every((p) => selectedPodsSet.has(`${p.namespace}/${p.name}`))
                    }
                    checked={
                      filteredPods.length > 0 &&
                      filteredPods.every((p) => selectedPodsSet.has(`${p.namespace}/${p.name}`))
                    }
                    onChange={(e) => {
                      if (e.target.checked) {
                        // 全選：新增所有過濾後的 Pod（去重）
                        const newTargets = filteredPods
                          .filter((p) => !selectedPodsSet.has(`${p.namespace}/${p.name}`))
                          .map((p) => ({
                            namespace: p.namespace,
                            pod: p.name,
                            container: p.containers[0],
                          }));
                        setSelectedPods([...selectedPods, ...newTargets]);
                      } else {
                        // 取消全選：移除所有過濾後的 Pod
                        const filteredSet = new Set(filteredPods.map((p) => `${p.namespace}/${p.name}`));
                        setSelectedPods(selectedPods.filter((p) => !filteredSet.has(`${p.namespace}/${p.pod}`)));
                      }
                    }}
                  >
                    {podSearchKeyword ? t('logs:center.selectAllMatching') : t('logs:center.selectAll')}
                  </Checkbox>
                </div>
                
                {/* 虛擬滾動列表 - 使用 react-window */}
                <div
                  style={{
                    border: '1px solid #d9d9d9',
                    borderRadius: 8,
                    overflow: 'hidden',
                  }}
                >
                  <VirtualList<{ pods: LogPodInfo[]; selectedPodsSet: Set<string>; onToggle: (pod: LogPodInfo) => void }>
                    style={{ height: 360 }}
                    rowCount={filteredPods.length}
                    rowHeight={60}
                    rowProps={{
                      pods: filteredPods,
                      selectedPodsSet,
                      onToggle: (pod: LogPodInfo) => {
                        const isSelected = selectedPodsSet.has(`${pod.namespace}/${pod.name}`);
                        if (isSelected) {
                          setSelectedPods(
                            selectedPods.filter(
                              (p) => !(p.namespace === pod.namespace && p.pod === pod.name)
                            )
                          );
                        } else {
                          setSelectedPods([
                            ...selectedPods,
                            {
                              namespace: pod.namespace,
                              pod: pod.name,
                              container: pod.containers[0],
                            },
                          ]);
                        }
                      },
                    }}
                    rowComponent={({ index, style, pods, selectedPodsSet: selSet, onToggle }) => {
                      const pod = pods[index];
                      if (!pod) return <div style={style} />;
                      const isSelected = selSet.has(`${pod.namespace}/${pod.name}`);
                      return (
                        <div
                          style={{
                            ...style,
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'center',
                            padding: '8px 12px',
                            borderBottom: '1px solid #f0f0f0',
                            cursor: 'pointer',
                            backgroundColor: isSelected ? '#e6f7ff' : '#fff',
                            boxSizing: 'border-box',
                          }}
                          onClick={() => onToggle(pod)}
                        >
                          <div style={{ flex: 1, minWidth: 0, overflow: 'hidden' }}>
                            <Text strong style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                              {pod.name}
                            </Text>
                            <Text type="secondary" style={{ fontSize: 12, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                              {t('logs:center.container')}: {pod.containers.join(', ')}
                            </Text>
                          </div>
                          <Space style={{ flexShrink: 0 }}>
                            <Tag color={pod.status === 'Running' ? 'green' : 'orange'}>
                              {pod.status}
                            </Tag>
                            <Checkbox checked={isSelected} />
                          </Space>
                        </div>
                      );
                    }}
                  />
                </div>
              </>
            )}
          </Spin>

          {selectedPods.length > 0 && (
            <Alert
              message={t('logs:center.selectedPodsCount', { count: selectedPods.length })}
              type="info"
              showIcon
            />
          )}
        </Space>
      </Modal>
    </div>
  );
};

export default LogCenter;


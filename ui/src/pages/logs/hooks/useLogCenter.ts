import { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import { Form, message } from 'antd';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { logService, logSourceService } from '../../../services/logService';
import type {
  LogEntry,
  EventLogEntry,
  LogStats,
  LogStreamTarget,
  LogPodInfo,
  LogSearchParams,
  LogSource,
} from '../../../services/logService';

export function useLogCenter(clusterId: string | undefined) {
  const { t } = useTranslation(['logs', 'common']);

  const [activeTab, setActiveTab] = useState('stream');
  const [stats, setStats] = useState<LogStats | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);

  // Real-time streaming state
  const [streaming, setStreaming] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [targets, setTargets] = useState<LogStreamTarget[]>([]);
  const [maxLines] = useState(1000);
  const [showTimestamp, setShowTimestamp] = useState(true);
  const [showSource, setShowSource] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const [levelFilter, setLevelFilter] = useState<string[]>([]);
  const [logSearchKeyword, setLogSearchKeyword] = useState('');
  const wsRef = useRef<WebSocket | null>(null);
  const logsEndRef = useRef<HTMLDivElement>(null);

  // External log sources state
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

  // Pod selector state
  const [podSelectorVisible, setPodSelectorVisible] = useState(false);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [selectedNamespace, setSelectedNamespace] = useState<string>('');
  const [pods, setPods] = useState<LogPodInfo[]>([]);
  const [podsLoading, setPodsLoading] = useState(false);
  const [selectedPods, setSelectedPods] = useState<LogStreamTarget[]>([]);
  const [podSearchKeyword, setPodSearchKeyword] = useState('');

  // Events state
  const [events, setEvents] = useState<EventLogEntry[]>([]);
  const [eventsLoading, setEventsLoading] = useState(false);
  const [eventNamespace, setEventNamespace] = useState<string>('');
  const [eventType, setEventType] = useState<'Normal' | 'Warning' | undefined>();

  // Search state
  const [searchResults, setSearchResults] = useState<LogEntry[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searchNamespaces, setSearchNamespaces] = useState<string[]>([]);
  const [searchLevels, setSearchLevels] = useState<string[]>([]);
  const [searchDateRange, setSearchDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);

  // Computed values
  const selectedPodsSet = useMemo(() => {
    return new Set(selectedPods.map((p) => `${p.namespace}/${p.pod}`));
  }, [selectedPods]);

  const filteredLogs = useMemo(() => {
    let result = logs;
    if (levelFilter.length > 0) {
      result = result.filter((log) => levelFilter.includes(log.level));
    }
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

  const filteredPods = useMemo(() => {
    if (!podSearchKeyword.trim()) return pods;
    const keyword = podSearchKeyword.toLowerCase();
    return pods.filter(
      (pod) =>
        pod.name.toLowerCase().includes(keyword) ||
        pod.containers.some((c) => c.toLowerCase().includes(keyword))
    );
  }, [pods, podSearchKeyword]);

  // Data fetchers
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

  const fetchNamespaces = useCallback(async () => {
    if (!clusterId) return;
    try {
      const res = await logService.getNamespaces(clusterId);
      setNamespaces(res || []);
    } catch (error) {
      console.error('獲取命名空間失敗', error);
    }
  }, [clusterId]);

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
  }, [clusterId, eventNamespace, eventType, t]);

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
  }, [clusterId, searchKeyword, searchNamespaces, searchLevels, searchDateRange, t]);

  // Streaming controls
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
      setLogs([]);
      const streamConfig = {
        targets,
        tail_lines: 100,
        show_timestamp: showTimestamp,
        show_source: showSource,
      };
      const { ws, config } = logService.createAggregateLogStream(clusterId, streamConfig);
      ws.onopen = () => {
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
  }, [streaming, targets, clusterId, maxLines, showTimestamp, showSource, t]);

  const clearLogs = () => setLogs([]);

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

  const removeTarget = (index: number) => {
    setTargets(targets.filter((_, i) => i !== index));
  };

  const openPodSelector = () => {
    setPodSelectorVisible(true);
    setSelectedPods([]);
  };

  const confirmPodSelection = () => {
    setTargets([...targets, ...selectedPods]);
    setPodSelectorVisible(false);
    setSelectedPods([]);
  };

  // External log source handlers
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

  const handleSaveLogSource = async () => {
    if (!clusterId) return;
    try {
      const values = await srcForm.validateFields();
      if (editingSrc) {
        await logSourceService.update(clusterId, editingSrc.id, values);
      } else {
        await logSourceService.create(clusterId, { ...values, enabled: true });
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
    if (!clusterId) return;
    try {
      await logSourceService.delete(clusterId, src.id);
      message.success('刪除成功');
      if (selectedSrcId === src.id) setSelectedSrcId(null);
      loadLogSources();
    } catch {
      message.error('刪除失敗');
    }
  };

  const handleExtSearch = async () => {
    if (!clusterId) return;
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
      const result = await logSourceService.search(clusterId, selectedSrcId, params);
      setExtResults(result.items || []);
    } catch (err: unknown) {
      message.error('查詢失敗: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      setExtSearchLoading(false);
    }
  };

  // Effects
  useEffect(() => {
    fetchStats();
    fetchNamespaces();
  }, [fetchStats, fetchNamespaces]);

  useEffect(() => {
    if (activeTab === 'events') {
      fetchEvents();
    }
  }, [activeTab, fetchEvents]);

  useEffect(() => {
    if (activeTab === 'external') {
      loadLogSources();
    }
  }, [activeTab, loadLogSources]);

  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoScroll]);

  return {
    // Tab state
    activeTab,
    setActiveTab,

    // Stats
    stats,
    statsLoading,
    fetchStats,

    // Streaming
    streaming,
    logs,
    targets,
    setTargets,
    showTimestamp,
    setShowTimestamp,
    showSource,
    setShowSource,
    autoScroll,
    setAutoScroll,
    levelFilter,
    setLevelFilter,
    logSearchKeyword,
    setLogSearchKeyword,
    filteredLogs,
    logsEndRef,
    toggleStream,
    clearLogs,
    downloadLogs,
    removeTarget,

    // Pod selector
    podSelectorVisible,
    setPodSelectorVisible,
    namespaces,
    selectedNamespace,
    setSelectedNamespace,
    pods,
    podsLoading,
    selectedPods,
    setSelectedPods,
    podSearchKeyword,
    setPodSearchKeyword,
    filteredPods,
    selectedPodsSet,
    fetchPods,
    openPodSelector,
    confirmPodSelection,

    // Events
    events,
    eventsLoading,
    eventNamespace,
    setEventNamespace,
    eventType,
    setEventType,
    fetchEvents,

    // Search
    searchResults,
    searchLoading,
    searchKeyword,
    setSearchKeyword,
    searchNamespaces,
    setSearchNamespaces,
    searchLevels,
    setSearchLevels,
    searchDateRange,
    setSearchDateRange,
    handleSearch,

    // External log sources
    logSources,
    logSourcesLoading,
    srcModalOpen,
    setSrcModalOpen,
    editingSrc,
    setEditingSrc,
    srcForm,
    selectedSrcId,
    setSelectedSrcId,
    extQuery,
    setExtQuery,
    extIndex,
    setExtIndex,
    extDateRange,
    setExtDateRange,
    extResults,
    extSearchLoading,
    loadLogSources,
    handleSaveLogSource,
    handleDeleteLogSource,
    handleExtSearch,
  };
}

export type LogCenterState = ReturnType<typeof useLogCenter>;

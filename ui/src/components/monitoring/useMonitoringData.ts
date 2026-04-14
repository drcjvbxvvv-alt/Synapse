import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import api from '../../utils/api';
import type { ClusterMetricsData, MonitoringChartsProps } from './types';

const CACHE_DURATION = 30000; // 30 seconds

export function useMonitoringData({
  clusterId,
  clusterName,
  nodeName,
  namespace,
  podName,
  workloadName,
  type,
  lazyLoad = false,
}: MonitoringChartsProps) {
  const [metrics, setMetrics] = useState<ClusterMetricsData | null>(null);
  const [loading, setLoading] = useState(false);
  const [timeRange, setTimeRange] = useState('1h');
  const [step, setStep] = useState('15s');
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [hasLoaded, setHasLoaded] = useState(false);
  const metricsCacheRef = useRef<{ key: string; data: ClusterMetricsData; timestamp: number } | null>(null);
  const intervalRef = useRef<NodeJS.Timeout | null>(null);

  const cacheKey = useMemo(() => {
    return `${clusterId}-${type}-${timeRange}-${step}-${clusterName || ''}-${nodeName || ''}-${namespace || ''}-${podName || ''}-${workloadName || ''}`;
  }, [clusterId, type, timeRange, step, clusterName, nodeName, namespace, podName, workloadName]);

  const getCachedData = useCallback(() => {
    if (metricsCacheRef.current && metricsCacheRef.current.key === cacheKey) {
      const now = Date.now();
      if (now - metricsCacheRef.current.timestamp < CACHE_DURATION) {
        return metricsCacheRef.current.data;
      }
    }
    return null;
  }, [cacheKey]);

  const fetchMetrics = useCallback(async (forceRefresh = false) => {
    if (!forceRefresh) {
      const cachedData = getCachedData();
      if (cachedData) {
        setMetrics(cachedData);
        setLoading(false);
        return;
      }
    }

    try {
      setLoading(true);
      let url = '';
      const params = new URLSearchParams({
        range: timeRange,
        step: step,
      });

      if (clusterName) {
        params.append('clusterName', clusterName);
      }

      switch (type) {
        case 'cluster':
          url = `/clusters/${clusterId}/monitoring/metrics`;
          break;
        case 'node':
          url = `/clusters/${clusterId}/nodes/${nodeName}/metrics`;
          break;
        case 'pod':
          url = `/clusters/${clusterId}/pods/${namespace}/${podName}/metrics`;
          break;
        case 'workload':
          url = `/clusters/${clusterId}/workloads/${namespace}/${workloadName}/metrics`;
          break;
      }

      const response = await api.get(`${url}?${params.toString()}`);
      const data = response.data;
      setMetrics(data);

      metricsCacheRef.current = {
        key: cacheKey,
        data: data,
        timestamp: Date.now(),
      };

      setHasLoaded(true);
    } catch (error) {
      console.error('獲取監控資料失敗:', error);
    } finally {
      setLoading(false);
    }
  }, [clusterId, timeRange, step, clusterName, nodeName, namespace, podName, workloadName, type, cacheKey, getCachedData]);

  useEffect(() => {
    if (lazyLoad && !hasLoaded) {
      const timer = setTimeout(() => {
        const cachedData = getCachedData();
        if (cachedData) {
          setMetrics(cachedData);
          setHasLoaded(true);
          return;
        }
        fetchMetrics();
      }, 100);
      return () => clearTimeout(timer);
    }

    if (!lazyLoad || hasLoaded) {
      const cachedData = getCachedData();
      if (cachedData) {
        setMetrics(cachedData);
        setHasLoaded(true);
        return;
      }
      fetchMetrics();
    }

    if (autoRefresh) {
      intervalRef.current = setInterval(() => {
        if (!document.hidden) fetchMetrics(true);
      }, 30000);
    }

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [clusterId, timeRange, step, clusterName, nodeName, namespace, podName, fetchMetrics, autoRefresh, lazyLoad, hasLoaded, getCachedData]);

  return {
    metrics,
    loading,
    timeRange,
    setTimeRange,
    step,
    setStep,
    autoRefresh,
    setAutoRefresh,
    fetchMetrics,
  };
}

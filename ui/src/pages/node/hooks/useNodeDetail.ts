import { useState, useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { message } from 'antd';
import { nodeService } from '../../../services/nodeService';
import { PodService } from '../../../services/podService';
import type { Node, NodeTaint, Pod } from '../../../types';
import type { TFunction } from 'i18next';

interface DrainOptions {
  ignoreDaemonSets: boolean;
  deleteLocalData: boolean;
  force: boolean;
  gracePeriodSeconds: number;
}

interface UseNodeDetailOptions {
  clusterId: string | undefined;
  nodeName: string | undefined;
  t: TFunction;
  tc: TFunction;
}

export function useNodeDetail({ clusterId, nodeName, t, tc }: UseNodeDetailOptions) {
  const navigate = useNavigate();

  // Basic state
  const [loading, setLoading] = useState(false);
  const [node, setNode] = useState<Node | null>(null);
  const [pods, setPods] = useState<Pod[]>([]);
  const [loadingPods, setLoadingPods] = useState(false);

  // Modal visibility state
  const [labelModalVisible, setLabelModalVisible] = useState(false);
  const [taintModalVisible, setTaintModalVisible] = useState(false);
  const [drainModalVisible, setDrainModalVisible] = useState(false);

  // Label form state
  const [newLabelKey, setNewLabelKey] = useState('');
  const [newLabelValue, setNewLabelValue] = useState('');

  // Taint form state
  const [newTaintKey, setNewTaintKey] = useState('');
  const [newTaintValue, setNewTaintValue] = useState('');
  const [newTaintEffect, setNewTaintEffect] = useState<'NoSchedule' | 'PreferNoSchedule' | 'NoExecute'>('NoSchedule');

  // Drain options
  const [drainOptions, setDrainOptions] = useState<DrainOptions>({
    ignoreDaemonSets: true,
    deleteLocalData: false,
    force: false,
    gracePeriodSeconds: 30,
  });

  // Fetch node details
  const fetchNodeDetail = useCallback(async () => {
    if (!clusterId || !nodeName) return;

    setLoading(true);
    try {
      const response = await nodeService.getNode(clusterId, nodeName);
      setNode(response);
    } catch (error) {
      console.error('Failed to fetch node details:', error);
      message.error(t('messages.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, nodeName, t]);

  // Fetch pods on node
  const fetchNodePods = useCallback(async () => {
    if (!clusterId || !nodeName) return;

    setLoadingPods(true);
    try {
      const response = await PodService.getPods(
        clusterId,
        undefined,
        nodeName,
        undefined,
        undefined,
        undefined,
        1,
        1000
      );

      if (response?.items) {
        const convertedPods: Pod[] = response.items.map((podInfo) => {
          let totalCpuLimit = 0;
          let totalMemoryLimit = 0;

          podInfo.containers.forEach((c) => {
            if (c.resources?.limits) {
              const cpuStr = c.resources.limits.cpu || c.resources.limits.CPU || '';
              if (cpuStr) {
                if (cpuStr.endsWith('m')) {
                  totalCpuLimit += parseInt(cpuStr.replace('m', ''), 10) || 0;
                } else {
                  totalCpuLimit += (parseFloat(cpuStr) || 0) * 1000;
                }
              }

              const memStr = c.resources.limits.memory || c.resources.limits.Memory || '';
              if (memStr) {
                if (memStr.endsWith('Gi')) {
                  totalMemoryLimit += (parseFloat(memStr.replace('Gi', '')) || 0) * 1024;
                } else if (memStr.endsWith('Mi')) {
                  totalMemoryLimit += parseFloat(memStr.replace('Mi', '')) || 0;
                } else if (memStr.endsWith('Ki')) {
                  totalMemoryLimit += (parseFloat(memStr.replace('Ki', '')) || 0) / 1024;
                } else if (memStr.endsWith('G')) {
                  totalMemoryLimit += (parseFloat(memStr.replace('G', '')) || 0) * 1024;
                } else if (memStr.endsWith('M')) {
                  totalMemoryLimit += parseFloat(memStr.replace('M', '')) || 0;
                } else if (memStr.endsWith('K')) {
                  totalMemoryLimit += (parseFloat(memStr.replace('K', '')) || 0) / 1024;
                } else {
                  totalMemoryLimit += (parseFloat(memStr) || 0) / (1024 * 1024);
                }
              }
            }
          });

          return {
            id: podInfo.name,
            name: podInfo.name,
            namespace: podInfo.namespace,
            clusterId: clusterId || '',
            nodeName: podInfo.nodeName,
            status: podInfo.status as Pod['status'],
            phase: podInfo.phase,
            restartCount: podInfo.restartCount,
            cpuUsage: totalCpuLimit,
            memoryUsage: totalMemoryLimit,
            containers: podInfo.containers.map((c) => ({
              name: c.name,
              image: c.image,
              ready: c.ready,
              restartCount: c.restartCount,
              state: {
                running: c.state.state === 'Running' ? { startedAt: c.state.startedAt || '' } : undefined,
                waiting: c.state.state === 'Waiting' ? { reason: c.state.reason || '', message: c.state.message } : undefined,
                terminated: c.state.state === 'Terminated' ? {
                  exitCode: 0,
                  reason: c.state.reason || '',
                  message: c.state.message,
                  startedAt: c.state.startedAt || '',
                  finishedAt: ''
                } : undefined,
              },
            })),
            labels: podInfo.labels || {},
            createdAt: podInfo.createdAt,
          };
        });
        setPods(convertedPods);
      } else {
        setPods([]);
      }
    } catch (error) {
      console.error('Failed to fetch node pods:', error);
      setPods([]);
    } finally {
      setLoadingPods(false);
    }
  }, [clusterId, nodeName]);

  // Refresh all data
  const refreshAllData = useCallback(() => {
    fetchNodeDetail();
    fetchNodePods();
  }, [fetchNodeDetail, fetchNodePods]);

  // Export pods to CSV
  const handleExportPods = useCallback(() => {
    if (pods.length === 0) {
      message.warning(tc('messages.noData'));
      return;
    }

    const headers = [tc('table.name'), tc('table.namespace'), tc('table.status'), t('columns.restarts'), t('resources.cpu'), t('resources.memory'), tc('table.createdAt')];
    const csvData = pods.map((pod) => [
      pod.name,
      pod.namespace,
      pod.status,
      pod.restartCount.toString(),
      pod.cpuUsage > 0 ? `${Math.round(pod.cpuUsage)}m` : '-',
      pod.memoryUsage > 0 ? `${Math.round(pod.memoryUsage)}Mi` : '-',
      new Date(pod.createdAt).toLocaleString(),
    ]);

    csvData.unshift(headers);
    const csvContent = csvData.map((row) => row.map((cell) => `"${cell}"`).join(',')).join('\n');
    const BOM = '\uFEFF';
    const blob = new Blob([BOM + csvContent], { type: 'text/csv;charset=utf-8;' });

    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);
    link.setAttribute('href', url);
    link.setAttribute('download', `${nodeName}_pods_${new Date().toISOString().split('T')[0]}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    message.success(tc('messages.exportSuccess'));
  }, [pods, nodeName, t, tc]);

  // Node operations
  const handleCordon = useCallback(async () => {
    try {
      await nodeService.cordonNode(clusterId || '', nodeName || '');
      message.success(t('messages.cordonSuccess'));
      fetchNodeDetail();
    } catch (error) {
      console.error('Failed to cordon node:', error);
      message.error(t('messages.cordonError'));
    }
  }, [clusterId, nodeName, t, fetchNodeDetail]);

  const handleUncordon = useCallback(async () => {
    try {
      await nodeService.uncordonNode(clusterId || '', nodeName || '');
      message.success(t('messages.uncordonSuccess'));
      fetchNodeDetail();
    } catch (error) {
      console.error('Failed to uncordon node:', error);
      message.error(t('messages.uncordonError'));
    }
  }, [clusterId, nodeName, t, fetchNodeDetail]);

  const handleDrain = useCallback(async () => {
    try {
      await nodeService.drainNode(clusterId || '', nodeName || '', drainOptions);
      message.success(t('messages.drainSuccess'));
      setDrainModalVisible(false);
      fetchNodeDetail();
    } catch (error) {
      console.error('Failed to drain node:', error);
      message.error(t('messages.drainError'));
    }
  }, [clusterId, nodeName, t, drainOptions, fetchNodeDetail]);

  // Label operations
  const handleAddLabel = useCallback(() => {
    if (!newLabelKey || !newLabelValue) {
      message.warning(t('messages.labelKeyValueRequired'));
      return;
    }
    message.success(tc('messages.success'));
    setNewLabelKey('');
    setNewLabelValue('');
    setLabelModalVisible(false);
  }, [newLabelKey, newLabelValue, t, tc]);

  const handleRemoveLabel = useCallback((key: string) => {
    message.success(tc('messages.success'));
    console.log('Remove label:', key);
  }, [tc]);

  // Taint operations
  const handleAddTaint = useCallback(async () => {
    if (!newTaintKey) {
      message.warning(t('messages.taintKeyRequired'));
      return;
    }

    const current = node?.taints ?? [];
    const newTaint = {
      key: newTaintKey,
      value: newTaintValue,
      effect: newTaintEffect,
    };
    const updated = [...current.filter(taint => taint.key !== newTaintKey), newTaint];

    try {
      await nodeService.patchNodeTaints(clusterId!, nodeName!, updated);
      message.success(tc('messages.success'));
      setNewTaintKey('');
      setNewTaintValue('');
      setNewTaintEffect('NoSchedule');
      setTaintModalVisible(false);
      fetchNodeDetail();
    } catch {
      message.error(t('messages.fetchError'));
    }
  }, [clusterId, nodeName, node, newTaintKey, newTaintValue, newTaintEffect, t, tc, fetchNodeDetail]);

  const handleRemoveTaint = useCallback(async (taint: NodeTaint) => {
    const updated = (node?.taints ?? []).filter(t => t.key !== taint.key);
    try {
      await nodeService.patchNodeTaints(clusterId!, nodeName!, updated);
      message.success(tc('messages.success'));
      fetchNodeDetail();
    } catch {
      message.error(t('messages.fetchError'));
    }
  }, [clusterId, nodeName, node, t, tc, fetchNodeDetail]);

  // Initial load
  useEffect(() => {
    if (clusterId && nodeName) {
      fetchNodeDetail();
      fetchNodePods();
    }
  }, [clusterId, nodeName, fetchNodeDetail, fetchNodePods]);

  return {
    // State
    loading,
    node,
    pods,
    loadingPods,
    labelModalVisible,
    taintModalVisible,
    drainModalVisible,
    newLabelKey,
    newLabelValue,
    newTaintKey,
    newTaintValue,
    newTaintEffect,
    drainOptions,

    // Setters
    setLabelModalVisible,
    setTaintModalVisible,
    setDrainModalVisible,
    setNewLabelKey,
    setNewLabelValue,
    setNewTaintKey,
    setNewTaintValue,
    setNewTaintEffect,
    setDrainOptions,

    // Handlers
    refreshAllData,
    handleExportPods,
    handleCordon,
    handleUncordon,
    handleDrain,
    handleAddLabel,
    handleRemoveLabel,
    handleAddTaint,
    handleRemoveTaint,

    // Navigation
    navigate,
  };
}

export type UseNodeDetailReturn = ReturnType<typeof useNodeDetail>;

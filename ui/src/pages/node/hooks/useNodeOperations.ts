import { useState, useEffect } from 'react';
import { message, App } from 'antd';
import { nodeService } from '../../../services/nodeService';
import type { Node } from '../../../types';
import { useTranslation } from 'react-i18next';

export interface NodeOperationStatusItem {
  nodeName: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'skipped' | 'waiting';
  message?: string;
  description?: string;
  progress?: number;
}

export interface OperationResults {
  success: number;
  failed: number;
  skipped: number;
  details: Array<{
    nodeName: string;
    status: string;
    message: string;
  }>;
  startTime: string;
  endTime: string;
  duration: string;
}

export interface DrainOptions {
  ignoreDaemonSets: boolean;
  deleteLocalData: boolean;
  force: boolean;
  gracePeriodSeconds: number;
  timeoutSeconds: number;
}

export interface ConfirmChecks {
  serviceInterruption: boolean;
  replicaConfirmed: boolean;
  teamNotified: boolean;
}

interface UseNodeOperationsProps {
  clusterId: string;
  selectedNodes: Node[];
  onClose: () => void;
  onSuccess: () => void;
}

export function useNodeOperations({
  clusterId,
  selectedNodes,
  onClose,
  onSuccess,
}: UseNodeOperationsProps) {
  const { t } = useTranslation(['nodeOps', 'common']);
  const { modal } = App.useApp();

  const [currentStep, setCurrentStep] = useState(0);
  const [operationType, setOperationType] = useState<'cordon' | 'uncordon' | 'drain'>('cordon');
  const [operationReason, setOperationReason] = useState('');
  const [drainOptions, setDrainOptions] = useState<DrainOptions>({
    ignoreDaemonSets: true,
    deleteLocalData: false,
    force: false,
    gracePeriodSeconds: 30,
    timeoutSeconds: 300,
  });
  const [confirmChecks, setConfirmChecks] = useState<ConfirmChecks>({
    serviceInterruption: false,
    replicaConfirmed: false,
    teamNotified: false,
  });
  const [operationProgress, setOperationProgress] = useState(0);
  const [nodeOperationStatus, setNodeOperationStatus] = useState<NodeOperationStatusItem[]>([]);
  const [operationResults, setOperationResults] = useState<OperationResults>({
    success: 0,
    failed: 0,
    skipped: 0,
    details: [],
    startTime: '',
    endTime: '',
    duration: '',
  });
  const [executionStrategy, setExecutionStrategy] = useState('serial');
  const [failureHandling, setFailureHandling] = useState('stop');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (selectedNodes.length > 0) {
      const initialStatus: NodeOperationStatusItem[] = selectedNodes.map(node => ({
        nodeName: node.name,
        status: 'pending' as const,
        description: t('nodeOps:execution.waitingOperation'),
        progress: 0,
      }));
      setNodeOperationStatus(initialStatus);
    }
  }, [selectedNodes, t]);

  const handleOperationTypeChange = (value: 'cordon' | 'uncordon' | 'drain') => {
    setOperationType(value);
  };

  const handleNext = () => {
    if (currentStep === 0) {
      if (!operationType) {
        message.error(t('nodeOps:operationType.selectRequired'));
        return;
      }
      setCurrentStep(currentStep + 1);
    } else if (currentStep === 1) {
      if (operationType === 'drain') {
        if (!confirmChecks.serviceInterruption || !confirmChecks.replicaConfirmed || !confirmChecks.teamNotified) {
          message.error(t('nodeOps:drain.confirmAllRisks'));
          return;
        }
      }
      setCurrentStep(currentStep + 1);
    } else if (currentStep === 2) {
      executeOperation();
    }
  };

  const handlePrevious = () => {
    setCurrentStep(currentStep - 1);
  };

  const handleCancel = () => {
    modal.confirm({
      title: t('nodeOps:cancel.title'),
      content: t('nodeOps:cancel.content'),
      onOk: () => {
        onClose();
      },
    });
  };

  const handleConfirmChecksChange = (checkedValues: string[]) => {
    setConfirmChecks({
      serviceInterruption: checkedValues.includes('service-interruption'),
      replicaConfirmed: checkedValues.includes('replica-confirmed'),
      teamNotified: checkedValues.includes('team-notified'),
    });
  };

  const handleDrainOptionsChange = (checkedValues: string[]) => {
    setDrainOptions(prev => ({
      ...prev,
      ignoreDaemonSets: checkedValues.includes('ignore-daemonsets'),
      deleteLocalData: checkedValues.includes('delete-emptydir-data'),
      force: checkedValues.includes('force'),
    }));
  };

  const updateNodeStatus = (index: number, status: NodeOperationStatusItem['status'], description: string, progress: number) => {
    setNodeOperationStatus(prev => {
      const updated = [...prev];
      updated[index] = { ...updated[index], status, description, progress };
      return updated;
    });
  };

  const executeNodeOperation = async (nodeName: string, index: number) => {
    switch (operationType) {
      case 'cordon':
        updateNodeStatus(index, 'running', t('nodeOps:execution.cordoning'), 30);
        await nodeService.cordonNode(clusterId, nodeName);
        updateNodeStatus(index, 'running', t('nodeOps:execution.cordonSuccess'), 90);
        break;
      case 'uncordon':
        updateNodeStatus(index, 'running', t('nodeOps:execution.uncordoning'), 30);
        await nodeService.uncordonNode(clusterId, nodeName);
        updateNodeStatus(index, 'running', t('nodeOps:execution.uncordonSuccess'), 90);
        break;
      case 'drain':
        updateNodeStatus(index, 'running', t('nodeOps:execution.draining'), 30);
        await nodeService.drainNode(clusterId, nodeName, {
          ignoreDaemonSets: drainOptions.ignoreDaemonSets,
          deleteLocalData: drainOptions.deleteLocalData,
          force: drainOptions.force,
          gracePeriodSeconds: drainOptions.gracePeriodSeconds,
        });
        updateNodeStatus(index, 'running', t('nodeOps:execution.drainSuccess'), 90);
        break;
      default:
        throw new Error(t('nodeOps:execution.unsupportedType'));
    }
    await new Promise(resolve => setTimeout(resolve, 500));
  };

  const executeParallel = async () => {
    const operations = selectedNodes.map(async (node, index) => {
      try {
        updateNodeStatus(index, 'running', t('nodeOps:execution.executingOperation'), 10);
        await executeNodeOperation(node.name, index);
        updateNodeStatus(index, 'success', t('nodeOps:execution.operationSuccess'), 100);
        setOperationResults(prev => ({
          ...prev,
          success: prev.success + 1,
          details: [...prev.details, {
            nodeName: node.name,
            status: 'success',
            message: `${operationType}操作成功`,
          }],
        }));
      } catch (error) {
        updateNodeStatus(index, 'failed', `操作失敗: ${error}`, 100);
        setOperationResults(prev => ({
          ...prev,
          failed: prev.failed + 1,
          details: [...prev.details, {
            nodeName: node.name,
            status: 'failed',
            message: `${operationType}操作失敗: ${error}`,
          }],
        }));
        if (failureHandling === 'stop') {
          throw error;
        }
      }
    });
    await Promise.all(operations);
    setOperationProgress(100);
  };

  const executeSerial = async () => {
    for (let i = 0; i < selectedNodes.length; i++) {
      const node = selectedNodes[i];
      try {
        updateNodeStatus(i, 'running', t('nodeOps:execution.executingOperation'), 10);
        await executeNodeOperation(node.name, i);
        updateNodeStatus(i, 'success', t('nodeOps:execution.operationSuccess'), 100);
        setOperationResults(prev => ({
          ...prev,
          success: prev.success + 1,
          details: [...prev.details, {
            nodeName: node.name,
            status: 'success',
            message: `${operationType}操作成功`,
          }],
        }));
        setOperationProgress(Math.round(((i + 1) / selectedNodes.length) * 100));
        if (i < selectedNodes.length - 1) {
          await new Promise(resolve => setTimeout(resolve, 1000));
        }
      } catch (error) {
        updateNodeStatus(i, 'failed', `操作失敗: ${error}`, 100);
        setOperationResults(prev => ({
          ...prev,
          failed: prev.failed + 1,
          details: [...prev.details, {
            nodeName: node.name,
            status: 'failed',
            message: `${operationType}操作失敗: ${error}`,
          }],
        }));
        if (failureHandling === 'stop') {
          break;
        }
      }
    }
  };

  const executeOperation = async () => {
    setLoading(true);
    const startTime = new Date();
    setOperationResults(prev => ({
      ...prev,
      startTime: startTime.toLocaleString(),
    }));

    try {
      const updatedStatus: NodeOperationStatusItem[] = nodeOperationStatus.map(node => ({
        ...node,
        status: 'waiting' as const,
        description: t('nodeOps:execution.waitingOperation'),
      }));
      setNodeOperationStatus(updatedStatus);

      if (executionStrategy === 'parallel') {
        await executeParallel();
      } else {
        await executeSerial();
      }

      const endTime = new Date();
      const duration = Math.round((endTime.getTime() - startTime.getTime()) / 1000);
      const minutes = Math.floor(duration / 60);
      const seconds = duration % 60;

      setOperationResults(prev => ({
        ...prev,
        endTime: endTime.toLocaleString(),
        duration: `${minutes}分${seconds}秒`,
      }));

      setCurrentStep(s => s + 1);
    } catch (error) {
      console.error('執行節點操作失敗:', error);
      message.error(t('nodeOps:execution.executeFailed'));
    } finally {
      setLoading(false);
    }
  };

  const handleFinish = () => {
    message.success(t('nodeOps:result.nodeOperationComplete'));
    onSuccess();
    onClose();
  };

  const getOperationTitle = () => {
    switch (operationType) {
      case 'cordon':   return t('nodeOps:cordon.titleShort');
      case 'uncordon': return t('nodeOps:uncordon.titleShort');
      case 'drain':    return t('nodeOps:drain.titleShort');
      default:         return t('nodeOps:operationType.nodeOperation');
    }
  };

  const getOperationDescription = () => {
    switch (operationType) {
      case 'cordon':   return t('nodeOps:cordon.description');
      case 'uncordon': return t('nodeOps:uncordon.description');
      case 'drain':    return t('nodeOps:drain.description');
      default:         return '';
    }
  };

  return {
    // state
    currentStep,
    operationType,
    operationReason,
    drainOptions,
    confirmChecks,
    operationProgress,
    nodeOperationStatus,
    operationResults,
    executionStrategy,
    failureHandling,
    loading,
    // handlers
    handleOperationTypeChange,
    handleNext,
    handlePrevious,
    handleCancel,
    handleFinish,
    handleConfirmChecksChange,
    handleDrainOptionsChange,
    setOperationReason,
    setDrainOptions,
    setExecutionStrategy,
    setFailureHandling,
    // helpers
    getOperationTitle,
    getOperationDescription,
  };
}

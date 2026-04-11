import { useState, useEffect, useCallback } from 'react';
import { message } from 'antd';
import { useTranslation } from 'react-i18next';
import { WorkloadService } from '../../../../services/workloadService';
import type { ContainerTabProps, DeploymentSpec } from '../containerTypes';

interface UseContainerTabReturn {
  loading: boolean;
  spec: DeploymentSpec | null;
  selectedContainer: string;
  setSelectedContainer: (name: string) => void;
  selectedSection: string;
  setSelectedSection: (section: string) => void;
}

export function useContainerTab({
  clusterId,
  namespace,
  deploymentName,
  rolloutName,
  statefulSetName,
  daemonSetName,
  jobName,
  cronJobName,
}: ContainerTabProps): UseContainerTabReturn {
  const { t } = useTranslation(['workload', 'common']);
  const [loading, setLoading] = useState(false);
  const [spec, setSpec] = useState<DeploymentSpec | null>(null);
  const [selectedContainer, setSelectedContainer] = useState<string>('');
  const [selectedSection, setSelectedSection] = useState<string>('basic');

  const workloadName = deploymentName || rolloutName || statefulSetName || daemonSetName || jobName || cronJobName;
  const workloadType = deploymentName ? 'Deployment'
    : rolloutName ? 'Rollout'
    : statefulSetName ? 'StatefulSet'
    : daemonSetName ? 'DaemonSet'
    : jobName ? 'Job'
    : cronJobName ? 'CronJob'
    : '';

  const loadSpec = useCallback(async () => {
    if (!clusterId || !namespace || !workloadName || !workloadType) return;

    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloadDetail(
        clusterId,
        workloadType,
        namespace,
        workloadName
      );

      const data = response as unknown as {
        raw?: Record<string, unknown> & { spec?: DeploymentSpec };
        workload?: Record<string, unknown> & { spec?: DeploymentSpec };
      };
      const deployment = data.raw || data.workload;
      setSpec(deployment?.spec || null);

      const specData = deployment?.spec;
      if (
        specData?.template?.spec?.containers &&
        Array.isArray(specData.template.spec.containers) &&
        specData.template.spec.containers.length > 0
      ) {
        setSelectedContainer(specData.template.spec.containers[0].name);
      }
    } catch (error) {
      console.error('獲取容器資訊失敗:', error);
      message.error(t('messages.fetchContainerError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, workloadName, workloadType, t]);

  useEffect(() => {
    loadSpec();
  }, [loadSpec]);

  return {
    loading,
    spec,
    selectedContainer,
    setSelectedContainer,
    selectedSection,
    setSelectedSection,
  };
}

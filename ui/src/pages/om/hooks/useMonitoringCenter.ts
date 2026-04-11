import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import {
  omService,
  type HealthDiagnosisResponse,
  type ResourceTopResponse,
  type ControlPlaneStatusResponse,
} from '../../../services/omService';

export function useMonitoringCenter() {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { message } = App.useApp();
  const { t } = useTranslation(['om', 'common']);

  // Health diagnosis state
  const [healthDiagnosis, setHealthDiagnosis] = useState<HealthDiagnosisResponse | null>(null);
  const [healthLoading, setHealthLoading] = useState(true);

  // Resource Top N state
  const [resourceTop, setResourceTop] = useState<ResourceTopResponse | null>(null);
  const [resourceLoading, setResourceLoading] = useState(false);
  const [resourceType, setResourceType] = useState<'cpu' | 'memory' | 'disk' | 'network'>('cpu');
  const [resourceLevel, setResourceLevel] = useState<'namespace' | 'workload' | 'pod'>('namespace');

  // Control plane state
  const [controlPlaneStatus, setControlPlaneStatus] = useState<ControlPlaneStatusResponse | null>(null);
  const [controlPlaneLoading, setControlPlaneLoading] = useState(true);

  const loadHealthDiagnosis = useCallback(async () => {
    if (!clusterId) return;
    setHealthLoading(true);
    try {
      const response = await omService.getHealthDiagnosis(clusterId);
      setHealthDiagnosis(response);
    } catch (error) {
      console.error('載入健康診斷失敗:', error);
      message.error(t('common:messages.fetchError'));
    } finally {
      setHealthLoading(false);
    }
  }, [clusterId, message, t]);

  const loadResourceTop = useCallback(async () => {
    if (!clusterId) return;
    setResourceLoading(true);
    try {
      const response = await omService.getResourceTop(clusterId, {
        type: resourceType,
        level: resourceLevel,
        limit: 10,
      });
      setResourceTop(response);
    } catch (error) {
      console.error('載入資源 Top N 失敗:', error);
      message.error(t('common:messages.fetchError'));
    } finally {
      setResourceLoading(false);
    }
  }, [clusterId, resourceType, resourceLevel, message, t]);

  const loadControlPlaneStatus = useCallback(async () => {
    if (!clusterId) return;
    setControlPlaneLoading(true);
    try {
      const response = await omService.getControlPlaneStatus(clusterId);
      setControlPlaneStatus(response);
    } catch (error) {
      console.error('載入控制面狀態失敗:', error);
      message.error(t('common:messages.fetchError'));
    } finally {
      setControlPlaneLoading(false);
    }
  }, [clusterId, message, t]);

  // Initial load
  useEffect(() => {
    loadHealthDiagnosis();
    loadControlPlaneStatus();
  }, [loadHealthDiagnosis, loadControlPlaneStatus]);

  // Reload when resource type or level changes
  useEffect(() => {
    loadResourceTop();
  }, [loadResourceTop]);

  const handleRefreshAll = () => {
    loadHealthDiagnosis();
    loadResourceTop();
    loadControlPlaneStatus();
  };

  return {
    t,
    // health
    healthDiagnosis,
    healthLoading,
    loadHealthDiagnosis,
    // resource top
    resourceTop,
    resourceLoading,
    resourceType,
    setResourceType,
    resourceLevel,
    setResourceLevel,
    loadResourceTop,
    // control plane
    controlPlaneStatus,
    controlPlaneLoading,
    loadControlPlaneStatus,
    // actions
    handleRefreshAll,
  };
}

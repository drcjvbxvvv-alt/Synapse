import { useState, useEffect, useCallback } from 'react';
import { Form, App } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { alertService } from '../../../services/alertService';
import type {
  Alert,
  Silence,
  AlertStats,
  CreateSilenceRequest,
  Matcher,
} from '../../../services/alertService';

export function useAlertCenter(clusterId: string | undefined) {
  const { t } = useTranslation(['alert', 'common']);
  const navigate = useNavigate();
  const { message } = App.useApp();

  const [loading, setLoading] = useState(false);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [silences, setSilences] = useState<Silence[]>([]);
  const [stats, setStats] = useState<AlertStats | null>(null);
  const [searchText, setSearchText] = useState('');
  const [severityFilter, setSeverityFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [silenceModalVisible, setSilenceModalVisible] = useState(false);
  const [, setSelectedAlert] = useState<Alert | null>(null);
  const [silenceForm] = Form.useForm();
  const [configEnabled, setConfigEnabled] = useState(false);
  const [configLoading, setConfigLoading] = useState(true);

  const loadConfig = useCallback(async () => {
    if (!clusterId) return;
    try {
      setConfigLoading(true);
      const response = await alertService.getConfig(clusterId);
      setConfigEnabled(response?.enabled || false);
    } catch (error) {
      console.error('載入配置失敗:', error);
      setConfigEnabled(false);
    } finally {
      setConfigLoading(false);
    }
  }, [clusterId]);

  const loadAlerts = useCallback(async () => {
    if (!clusterId || !configEnabled) return;
    try {
      setLoading(true);
      const [alertsRes, statsRes] = await Promise.all([
        alertService.getAlerts(clusterId, {
          severity: severityFilter || undefined,
        }),
        alertService.getAlertStats(clusterId),
      ]);
      setAlerts(alertsRes || []);
      setStats(statsRes);
    } catch (error) {
      console.error('載入告警失敗:', error);
      message.error(t('alert:center.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, configEnabled, severityFilter, message, t]);

  const loadSilences = useCallback(async () => {
    if (!clusterId || !configEnabled) return;
    try {
      const response = await alertService.getSilences(clusterId);
      setSilences(response || []);
    } catch (error) {
      console.error('載入靜默規則失敗:', error);
    }
  }, [clusterId, configEnabled]);

  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  useEffect(() => {
    if (configEnabled) {
      loadAlerts();
      loadSilences();
    }
  }, [configEnabled, loadAlerts, loadSilences]);

  const handleRefresh = () => {
    loadAlerts();
    loadSilences();
  };

  const handleOpenSilenceModal = (alert?: Alert) => {
    setSelectedAlert(alert || null);
    if (alert) {
      const matchers: Matcher[] = Object.entries(alert.labels).map(([name, value]) => ({
        name,
        value,
        isRegex: false,
        isEqual: true,
      }));
      silenceForm.setFieldsValue({
        matchers,
        timeRange: [dayjs(), dayjs().add(2, 'hour')],
        comment: `${t('alert:center.createSilence')}: ${alert.labels.alertname || t('alert:center.unknownAlert')}`,
      });
    } else {
      silenceForm.resetFields();
      silenceForm.setFieldsValue({
        timeRange: [dayjs(), dayjs().add(2, 'hour')],
      });
    }
    setSilenceModalVisible(true);
  };

  const handleCreateSilence = async () => {
    try {
      const values = await silenceForm.validateFields();
      const [startsAt, endsAt] = values.timeRange;

      const silenceReq: CreateSilenceRequest = {
        matchers: values.matchers || [],
        startsAt: startsAt.toISOString(),
        endsAt: endsAt.toISOString(),
        createdBy: 'Synapse',
        comment: values.comment || '',
      };

      await alertService.createSilence(clusterId!, silenceReq);
      message.success(t('alert:center.silenceCreateSuccess'));
      setSilenceModalVisible(false);
      loadSilences();
      loadAlerts();
    } catch (error: unknown) {
      console.error('建立靜默規則失敗:', error);
      let errorMsg = t('alert:center.silenceCreateFailed');
      if (error && typeof error === 'object' && 'response' in error) {
        const axiosError = error as { response?: { data?: { message?: string } } };
        errorMsg = axiosError.response?.data?.message || errorMsg;
      }
      message.error(errorMsg);
    }
  };

  const handleDeleteSilence = async (silenceId: string) => {
    try {
      await alertService.deleteSilence(clusterId!, silenceId);
      message.success(t('alert:center.silenceDeleteSuccess'));
      loadSilences();
      loadAlerts();
    } catch (error: unknown) {
      console.error('刪除靜默規則失敗:', error);
      message.error(t('alert:center.silenceDeleteFailed'));
    }
  };

  const filteredAlerts = alerts.filter((alert) => {
    const matchSearch =
      !searchText ||
      alert.labels.alertname?.toLowerCase().includes(searchText.toLowerCase()) ||
      alert.annotations?.description?.toLowerCase().includes(searchText.toLowerCase()) ||
      alert.annotations?.summary?.toLowerCase().includes(searchText.toLowerCase());

    const matchStatus = !statusFilter || alert.status.state === statusFilter;

    return matchSearch && matchStatus;
  });

  const goToConfig = () =>
    navigate(`/clusters/${clusterId}/config-center?tab=alertmanager`);

  return {
    t,
    loading,
    alerts,
    silences,
    stats,
    searchText,
    setSearchText,
    severityFilter,
    setSeverityFilter,
    statusFilter,
    setStatusFilter,
    silenceModalVisible,
    setSilenceModalVisible,
    silenceForm,
    configEnabled,
    configLoading,
    filteredAlerts,
    loadSilences,
    handleRefresh,
    handleOpenSilenceModal,
    handleCreateSilence,
    handleDeleteSilence,
    goToConfig,
  };
}

export type AlertCenterState = ReturnType<typeof useAlertCenter>;

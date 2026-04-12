import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { 
  App,
  Card,
  Popconfirm,
  Tabs, 
  Spin, 
  message, 
  Button, 
  Space,
  Tag,
  Descriptions,
  Typography,
  Row,
  Col
} from 'antd';
import {
  ArrowLeftOutlined,
  SyncOutlined,
  LineChartOutlined,
  FileTextOutlined,
  CaretRightOutlined,
  FastForwardOutlined,
  StopOutlined,
} from '@ant-design/icons';
import { WorkloadService } from '../../services/workloadService';
import { useTranslation } from 'react-i18next';
import { clusterService } from '../../services/clusterService';
import InstancesTab from './tabs/InstancesTab';
import AccessTab from './tabs/AccessTab';
import ContainerTab from './tabs/ContainerTab';
import ScalingTab from './tabs/ScalingTab';
import SchedulingTab from './tabs/SchedulingTab';
import HistoryTab from './tabs/HistoryTab';
import EventsTab from './tabs/EventsTab';
import MonitoringTab from './tabs/MonitoringTab';

const { Title, Text } = Typography;

interface RolloutDetailData {
  name: string;
  namespace: string;
  status: string;
  replicas?: number;
  readyReplicas?: number;
  availableReplicas?: number;
  updatedReplicas?: number;
  strategy?: string;
  createdAt: string;
  labels: Record<string, string>;
  annotations: Record<string, string>;
  selector: Record<string, string>;
  images: string[];
  conditions?: Array<{
    type: string;
    status: string;
    lastUpdateTime: string;
    lastTransitionTime: string;
    reason: string;
    message: string;
  }>;
}

const RolloutDetail: React.FC = () => {
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { message: messageApi } = App.useApp();

const { t } = useTranslation(["workload", "common"]);
const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [rollout, setRollout] = useState<RolloutDetailData | null>(null);
  // 從 URL 參數獲取預設 Tab，支援透過 ?tab=monitoring 直接跳轉到監控頁
  const [activeTab, setActiveTab] = useState(searchParams.get('tab') || 'instances');
  const [clusterName, setClusterName] = useState<string>('');

  const loadRolloutDetail = useCallback(async () => {
    if (!clusterId || !namespace || !name) return;

    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloadDetail(
        clusterId,
        'Rollout',
        namespace,
        name
      );

      setRollout(response.workload);
    } catch (error) {
      console.error('獲取Rollout詳情失敗:', error);
      message.error(t('messages.fetchDetailError', { type: 'Rollout' }));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, name, t]);

  useEffect(() => {
    loadRolloutDetail();
  }, [loadRolloutDetail]);

  // 載入叢集資訊獲取叢集名稱（用於 Grafana 資料來源）
  useEffect(() => {
    const loadClusterInfo = async () => {
      if (!clusterId) return;
      try {
        const response = await clusterService.getCluster(clusterId);
        if (response) {
          setClusterName(response.name);
        }
      } catch (error) {
        console.error('獲取叢集資訊失敗:', error);
      }
    };
    loadClusterInfo();
  }, [clusterId]);

  // 返回列表
  const handleBack = () => {
    navigate(`/clusters/${clusterId}/workloads?tab=rollout`);
  };

  // 重新整理
  const handleRefresh = () => {
    loadRolloutDetail();
  };

  const handlePromote = async () => {
    if (!clusterId || !namespace || !name) return;
    setActionLoading('promote');
    try {
      await WorkloadService.promoteRollout(clusterId, namespace, name);
      messageApi.success('Promote 成功');
      loadRolloutDetail();
    } catch (e) {
      messageApi.error('Promote 失敗: ' + String(e));
    } finally {
      setActionLoading(null);
    }
  };

  const handlePromoteFull = async () => {
    if (!clusterId || !namespace || !name) return;
    setActionLoading('promote-full');
    try {
      await WorkloadService.promoteFullRollout(clusterId, namespace, name);
      messageApi.success('Promote Full 成功');
      loadRolloutDetail();
    } catch (e) {
      messageApi.error('Promote Full 失敗: ' + String(e));
    } finally {
      setActionLoading(null);
    }
  };

  const handleAbort = async () => {
    if (!clusterId || !namespace || !name) return;
    setActionLoading('abort');
    try {
      await WorkloadService.abortRollout(clusterId, namespace, name);
      messageApi.success('Abort 成功');
      loadRolloutDetail();
    } catch (e) {
      messageApi.error('Abort 失敗: ' + String(e));
    } finally {
      setActionLoading(null);
    }
  };

  // 渲染狀態標籤
  const renderStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      'Running': { color: 'success', text: t('detailPage.statusMap.running') },
      'Stopped': { color: 'default', text: t('detailPage.statusMap.stopped') },
      'Degraded': { color: 'warning', text: t('detailPage.statusMap.degraded') },
      'Failed': { color: 'error', text: t('detailPage.statusMap.failed') },
      'Healthy': { color: 'success', text: t('detailPage.statusMap.healthy') },
      'Progressing': { color: 'processing', text: t('detailPage.statusMap.progressing') },
      'Paused': { color: 'warning', text: t('detailPage.statusMap.paused') },
    };
    
    const statusInfo = statusMap[status] || { color: 'default', text: status };
    return <Tag color={statusInfo.color}>{statusInfo.text}</Tag>;
  };

  // 格式化時間
  const formatTime = (timeStr: string) => {
    if (!timeStr) return '-';
    const date = new Date(timeStr);
    return date.toLocaleString('zh-TW', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false
    }).replace(/\//g, '-');
  };

  if (loading && !rollout) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Spin size="large" tip={t("common:messages.loading")} />
      </div>
    );
  }

  if (!rollout) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Text type="secondary">{t("messages.notFound", { type: "Rollout" })}</Text>
      </div>
    );
  }

  const tabItems = [
    {
      key: 'instances',
      label: t('detailTabs.instances'),
      children: (
        <InstancesTab 
          clusterId={clusterId!}
          namespace={rollout.namespace}
          rolloutName={rollout.name}
        />
      ),
    },
    {
      key: 'access',
      label: t('detailTabs.access'),
      children: (
        <AccessTab 
          clusterId={clusterId!}
          namespace={rollout.namespace}
          rolloutName={rollout.name}
        />
      ),
    },
    {
      key: 'container',
      label: t('detailTabs.container'),
      children: (
        <ContainerTab 
          clusterId={clusterId!}
          namespace={rollout.namespace}
          rolloutName={rollout.name}
        />
      ),
    },
    {
      key: 'scaling',
      label: t('detailTabs.scaling'),
      children: (
        <ScalingTab 
          clusterId={clusterId!}
          namespace={rollout.namespace}
          rolloutName={rollout.name}
        />
      ),
    },
    {
      key: 'scheduling',
      label: t('detailTabs.scheduling'),
      children: (
        <SchedulingTab 
          clusterId={clusterId!}
          namespace={rollout.namespace}
          rolloutName={rollout.name}
        />
      ),
    },
    {
      key: 'history',
      label: t('detailTabs.history'),
      children: (
        <HistoryTab 
          clusterId={clusterId!}
          namespace={rollout.namespace}
          rolloutName={rollout.name}
        />
      ),
    },
    {
      key: 'events',
      label: t('detailTabs.events'),
      children: (
        <EventsTab 
          clusterId={clusterId!}
          namespace={rollout.namespace}
          rolloutName={rollout.name}
        />
      ),
    },
    {
      key: 'monitoring',
      label: (
        <span>
          <LineChartOutlined style={{ marginRight: 4 }} />
          {t('detailTabs.monitoring')}
        </span>
      ),
      children: (
        <MonitoringTab
          clusterId={clusterId!}
          clusterName={clusterName}
          namespace={rollout.namespace}
          workloadName={rollout.name}
          workloadType="Rollout"
        />
      ),
    },
  ];

  return (
    <div style={{ padding: '16px 24px', background: '#f0f2f5', minHeight: '100vh' }}>
      {/* 頂部導航區域 */}
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Button 
            icon={<ArrowLeftOutlined />} 
            onClick={handleBack}
            type="text"
          >
            {t('detailPage.backToList')}
          </Button>
        </Space>
      </div>

      {/* 標題和操作按鈕 */}
      <div style={{ 
        background: '#fff', 
        padding: '16px 24px', 
        marginBottom: 16,
        borderRadius: '8px',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}>
        <div>
          <Space size="large">
            <Title level={4} style={{ margin: 0 }}>
              {rollout.name}
            </Title>
            {renderStatusTag(rollout.status)}
          </Space>
        </div>
        <Space>
          <Button
            icon={<CaretRightOutlined />}
            onClick={handlePromote}
            loading={actionLoading === 'promote'}
            disabled={actionLoading !== null && actionLoading !== 'promote'}
          >
            Promote
          </Button>
          <Popconfirm
            title="全量 Promote 將跳過所有 Pause 和 Analysis，確定執行？"
            onConfirm={handlePromoteFull}
            okText="確定"
            cancelText="取消"
          >
            <Button
              icon={<FastForwardOutlined />}
              loading={actionLoading === 'promote-full'}
              disabled={actionLoading !== null && actionLoading !== 'promote-full'}
            >
              Promote Full
            </Button>
          </Popconfirm>
          <Popconfirm
            title="確定中止此 Rollout？"
            onConfirm={handleAbort}
            okText="中止"
            okButtonProps={{ danger: true }}
            cancelText="取消"
          >
            <Button
              danger
              icon={<StopOutlined />}
              loading={actionLoading === 'abort'}
              disabled={actionLoading !== null && actionLoading !== 'abort'}
            >
              Abort
            </Button>
          </Popconfirm>
          <Button
            icon={<LineChartOutlined />}
            onClick={() => setActiveTab('monitoring')}
            type={activeTab === 'monitoring' ? 'primary' : 'default'}
          >
            {t('detailPage.monitoring')}
          </Button>
          <Button icon={<FileTextOutlined />} onClick={() => navigate(`/clusters/${clusterId}/logs`)}>{t('detailPage.logs')}</Button>
          <Button icon={<SyncOutlined />} onClick={handleRefresh}>
            {t('detailPage.refresh')}
          </Button>
        </Space>
      </div>

      {/* 基礎資訊卡片 */}
      <Card 
        title={t('detailPage.basicInfo')} 
        style={{ marginBottom: 16 }}
        variant="borderless"
      >
        <Row gutter={[48, 16]}>
          <Col span={12}>
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('detailPage.loadName')}>
                {rollout.name}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.status')}>
                {renderStatusTag(rollout.status)}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.instanceCount')}>
                <Text strong>
                  {rollout.readyReplicas || 0}/{rollout.replicas || 0}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.containerRuntime')}>
                {t('detailPage.normalRuntime')}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.description')}>
                {rollout.annotations?.['description'] || '-'}
              </Descriptions.Item>
            </Descriptions>
          </Col>
          <Col span={12}>
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('detailPage.namespace')}>
                {rollout.namespace}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.createdAt')}>
                {formatTime(rollout.createdAt)}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.releaseStrategy')}>
                {rollout.strategy || 'Canary'}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.availableInstances')}>
                {rollout.availableReplicas || 0}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.updatedInstances')}>
                {rollout.updatedReplicas || 0}
              </Descriptions.Item>
            </Descriptions>
          </Col>
        </Row>
      </Card>

      {/* Tab頁內容 */}
      <Card variant="borderless">
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={tabItems}
        />
      </Card>
    </div>
  );
};

export default RolloutDetail;


import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { 
  Card, 
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
  FileTextOutlined
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

interface DeploymentDetailData {
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

const DeploymentDetail: React.FC = () => {
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  
const { t } = useTranslation(["workload", "common"]);
const [loading, setLoading] = useState(false);
  const [deployment, setDeployment] = useState<DeploymentDetailData | null>(null);
  // 從 URL 參數獲取預設 Tab，支援透過 ?tab=monitoring 直接跳轉到監控頁
  const [activeTab, setActiveTab] = useState(searchParams.get('tab') || 'instances');
  const [clusterName, setClusterName] = useState<string>('');

  const loadDeploymentDetail = useCallback(async () => {
    if (!clusterId || !namespace || !name) return;

    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloadDetail(
        clusterId,
        'Deployment',
        namespace,
        name
      );

      setDeployment(response.workload);
    } catch (error) {
      console.error('獲取Deployment詳情失敗:', error);
      message.error(t('messages.fetchDetailError', { type: 'Deployment' }));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, name, t]);

  useEffect(() => {
    loadDeploymentDetail();
  }, [loadDeploymentDetail]);

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
    navigate(`/clusters/${clusterId}/workloads?tab=deployment`);
  };

  // 重新整理
  const handleRefresh = () => {
    loadDeploymentDetail();
  };

  // 渲染狀態標籤
  const renderStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      'Running': { color: 'success', text: t('detailPage.statusMap.running') },
      'Stopped': { color: 'default', text: t('detailPage.statusMap.stopped') },
      'Degraded': { color: 'warning', text: t('detailPage.statusMap.degraded') },
      'Failed': { color: 'error', text: t('detailPage.statusMap.failed') },
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

  if (loading && !deployment) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Spin size="large" tip={t("common:messages.loading")} />
      </div>
    );
  }

  if (!deployment) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Text type="secondary">{t("messages.notFound", { type: "Deployment" })}</Text>
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
          namespace={deployment.namespace}
          deploymentName={deployment.name}
        />
      ),
    },
    {
      key: 'access',
      label: t('detailTabs.access'),
      children: (
        <AccessTab 
          clusterId={clusterId!}
          namespace={deployment.namespace}
          deploymentName={deployment.name}
        />
      ),
    },
    {
      key: 'container',
      label: t('detailTabs.container'),
      children: (
        <ContainerTab 
          clusterId={clusterId!}
          namespace={deployment.namespace}
          deploymentName={deployment.name}
        />
      ),
    },
    {
      key: 'scaling',
      label: t('detailTabs.scaling'),
      children: (
        <ScalingTab 
          clusterId={clusterId!}
          namespace={deployment.namespace}
          deploymentName={deployment.name}
        />
      ),
    },
    {
      key: 'scheduling',
      label: t('detailTabs.scheduling'),
      children: (
        <SchedulingTab 
          clusterId={clusterId!}
          namespace={deployment.namespace}
          deploymentName={deployment.name}
        />
      ),
    },
    {
      key: 'history',
      label: t('detailTabs.history'),
      children: (
        <HistoryTab 
          clusterId={clusterId!}
          namespace={deployment.namespace}
          deploymentName={deployment.name}
        />
      ),
    },
    {
      key: 'events',
      label: t('detailTabs.events'),
      children: (
        <EventsTab 
          clusterId={clusterId!}
          namespace={deployment.namespace}
          deploymentName={deployment.name}
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
          clusterName={clusterName || ''}
          namespace={deployment.namespace}
          workloadName={deployment.name}
          workloadType="Deployment"
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
              {deployment.name}
            </Title>
            {renderStatusTag(deployment.status)}
          </Space>
        </div>
        <Space>
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
        bordered={false}
      >
        <Row gutter={[48, 16]}>
          <Col span={12}>
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('detailPage.loadName')}>
                {deployment.name}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.status')}>
                {renderStatusTag(deployment.status)}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.instanceCount')}>
                <Text strong>
                  {deployment.readyReplicas || 0}/{deployment.replicas || 0}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.containerRuntime')}>
                {t('detailPage.normalRuntime')}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.description')}>
                {deployment.annotations?.['description'] || '-'}
              </Descriptions.Item>
            </Descriptions>
          </Col>
          <Col span={12}>
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('detailPage.namespace')}>
                {deployment.namespace}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.createdAt')}>
                {formatTime(deployment.createdAt)}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.upgradeStrategy')}>
                {deployment.strategy || t('detailPage.rollingUpgrade')}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.availableInstances')}>
                {deployment.availableReplicas || 0}
              </Descriptions.Item>
              <Descriptions.Item label={t('detailPage.updatedInstances')}>
                {deployment.updatedReplicas || 0}
              </Descriptions.Item>
            </Descriptions>
          </Col>
        </Row>
      </Card>

      {/* Tab頁內容 */}
      <Card bordered={false}>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={tabItems}
        />
      </Card>
    </div>
  );
};

export default DeploymentDetail;


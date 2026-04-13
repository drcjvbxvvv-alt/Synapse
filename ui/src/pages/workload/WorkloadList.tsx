import React, { useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import {
  Card,
  Tabs,
  Spin,
} from 'antd';
import DeploymentTab from './DeploymentTab';
import ArgoRolloutTab from './ArgoRolloutTab';
import StatefulSetTab from './StatefulSetTab';
import DaemonSetTab from './DaemonSetTab';
import JobTab from './JobTab';
import CronJobTab from './CronJobTab';
import { useTranslation } from 'react-i18next';
const WorkloadList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const loading = false;
const { t } = useTranslation(['workload']);
// 從URL讀取當前Tab
  const activeTab = searchParams.get('tab') || 'deployment';

  // 統計資訊狀態（保留用於回撥，但不顯示）
  const [_deploymentCount, setDeploymentCount] = useState(0);
  const [_rolloutCount, setRolloutCount] = useState(0);
  const [_statefulSetCount, setStatefulSetCount] = useState(0);
  const [_daemonSetCount, setDaemonSetCount] = useState(0);
  const [_jobCount, setJobCount] = useState(0);
  const [_cronJobCount, setCronJobCount] = useState(0);

  // Tab切換處理
  const handleTabChange = (key: string) => {
    setSearchParams({ tab: key });
  };

  // Tab項配置
const tabItems = [
    {
      key: 'deployment',
      label: t('tabs.deployment'),
      children: (
        <DeploymentTab
          clusterId={clusterId || ''}
          onCountChange={setDeploymentCount}
        />
      ),
    },
    {
      key: 'rollout',
      label: t('tabs.rollout'),
      children: (
        <ArgoRolloutTab
          clusterId={clusterId || ''}
          onCountChange={setRolloutCount}
        />
      ),
    },
    {
      key: 'statefulset',
      label: t('tabs.statefulset'),
      children: (
        <StatefulSetTab
          clusterId={clusterId || ''}
          onCountChange={setStatefulSetCount}
        />
      ),
    },
    {
      key: 'daemonset',
      label: t('tabs.daemonset'),
      children: (
        <DaemonSetTab
          clusterId={clusterId || ''}
          onCountChange={setDaemonSetCount}
        />
      ),
    },
    {
      key: 'job',
      label: t('tabs.job'),
      children: (
        <JobTab
          clusterId={clusterId || ''}
          onCountChange={setJobCount}
        />
      ),
    },
    {
      key: 'cronjob',
      label: t('tabs.cronjob'),
      children: (
        <CronJobTab
          clusterId={clusterId || ''}
          onCountChange={setCronJobCount}
        />
      ),
    },
  ];
return (
    <div style={{ padding: '24px' }}>
      <Card variant="borderless">
        <Spin spinning={loading}>
          <Tabs
            activeKey={activeTab}
            onChange={handleTabChange}
            items={tabItems}
          />
        </Spin>
      </Card>
    </div>
  );
};

export default WorkloadList;

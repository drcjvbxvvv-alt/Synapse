import React, { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Button, Space, message, Spin } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { WorkloadService } from '../../../services/workloadService';
import { useTranslation } from 'react-i18next';

interface ReplicaSetInfo {
  name: string;
  namespace: string;
  replicas: number;
  readyReplicas: number;
  availableReplicas: number;
  revision: string;
  images: string[];
  createdAt: string;
}

interface HistoryTabProps {
  clusterId: string;
  namespace: string;
  deploymentName?: string;
  rolloutName?: string;
  statefulSetName?: string;
  daemonSetName?: string;
  jobName?: string;
  cronJobName?: string;
}

const HistoryTab: React.FC<HistoryTabProps> = ({ 
  clusterId, 
  namespace, 
  deploymentName,
  rolloutName,
  statefulSetName,
  daemonSetName,
  jobName,
  cronJobName
}) => {
const { t } = useTranslation(['workload', 'common']);
const [loading, setLoading] = useState(false);
  const [replicaSets, setReplicaSets] = useState<ReplicaSetInfo[]>([]);

  // 獲取工作負載名稱和型別
  const workloadName = deploymentName || rolloutName || statefulSetName || daemonSetName || jobName || cronJobName;
  const workloadType = deploymentName ? 'Deployment' 
    : rolloutName ? 'Rollout'
    : statefulSetName ? 'StatefulSet'
    : daemonSetName ? 'DaemonSet'
    : jobName ? 'Job'
    : cronJobName ? 'CronJob'
    : '';

  // 載入ReplicaSet列表
  const loadReplicaSets = useCallback(async () => {
    if (!clusterId || !namespace || !workloadName || !workloadType) return;
    
    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloadReplicaSets(
        clusterId,
        namespace,
        workloadType,
        workloadName
      );
      
      setReplicaSets(((response as unknown as { items?: unknown[] }).items || []) as ReplicaSetInfo[]);
    } catch (error) {
      console.error('獲取版本記錄失敗:', error);
      message.error(t('messages.fetchHistoryError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, workloadName, workloadType]);

  useEffect(() => {
    loadReplicaSets();
  }, [loadReplicaSets]);

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

  const columns: ColumnsType<ReplicaSetInfo> = [
    {
      title: t('history.rsName'),
      dataIndex: 'name',
      key: 'name',
      width: 300,
    },
    {
      title: t('history.revision'),
      dataIndex: 'revision',
      key: 'revision',
      width: 100,
      render: (revision: string) => <Tag color="blue">Revision {revision}</Tag>,
    },
    {
      title: t('history.replicas'),
      key: 'replicas',
      width: 120,
      render: (_, record: ReplicaSetInfo) => (
        <span>
          {record.readyReplicas}/{record.replicas}
        </span>
      ),
    },
    {
      title: t('history.status'),
      key: 'status',
      width: 100,
      render: (_, record: ReplicaSetInfo) => {
        if (record.replicas === 0) {
          return <Tag color="default">{t('history.historicalVersion')}</Tag>;
        }
        if (record.readyReplicas === record.replicas) {
          return <Tag color="success">{t('history.currentVersion')}</Tag>;
        }
        return <Tag color="processing">{t('history.updating')}</Tag>;
      },
    },
    {
      title: t('history.image'),
      dataIndex: 'images',
      key: 'images',
      width: 300,
      render: (images: string[]) => {
        if (!images || images.length === 0) return '-';
        return images.map((img, index) => {
          // 只顯示 name:version 部分
          const parts = img.split('/');
          const nameVersion = parts[parts.length - 1];
          return <div key={index}>{nameVersion}</div>;
        });
      },
    },
    {
      title: t('history.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      render: (time: string) => formatTime(time),
    },
  ];

  return (
    <Spin spinning={loading}>
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Button onClick={loadReplicaSets}>{t('history.refresh')}</Button>
        </Space>
      </div>
      <Table
        scroll={{ x: 'max-content' }}
        columns={columns}
        dataSource={replicaSets}
        rowKey="name"
        pagination={{
          total: replicaSets.length,
          pageSize: 10,
          showSizeChanger: true,
          showTotal: (total) => t('history.total', { count: total }),
        }}
      />
    </Spin>
  );
};

export default HistoryTab;


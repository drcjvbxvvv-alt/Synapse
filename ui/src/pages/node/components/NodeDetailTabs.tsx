import EmptyState from '@/components/EmptyState';
import React from 'react';
import {
  Card,
  Typography,
  Button,
  Space,
  Tag,
  Table,
  Divider,
  Badge,
  Statistic,
  Descriptions,
} from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  DesktopOutlined,
  CodeOutlined,
  EditOutlined,
  SettingOutlined,
  PauseCircleOutlined,
  DeleteOutlined,
  PlusOutlined,
  BarChartOutlined,
  AppstoreOutlined,
  DownloadOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { Node, NodeCondition, NodeTaint, Pod } from '../../../types';
import type { TFunction } from 'i18next';
import SSHTerminal from '../../../components/SSHTerminal';
import MonitoringCharts from '../../../components/MonitoringCharts';

const { Text } = Typography;

interface NodeDetailTabsProps {
  clusterId: string;
  nodeName: string;
  node: Node | null;
  pods: Pod[];
  loadingPods: boolean;
  t: TFunction;
  tc: TFunction;
  navigate: (path: string) => void;
  handleExportPods: () => void;
  handleRemoveLabel: (key: string) => void;
  handleRemoveTaint: (taint: NodeTaint) => void | Promise<void>;
  setLabelModalVisible: (visible: boolean) => void;
  setTaintModalVisible: (visible: boolean) => void;
}

// Helper to get condition status badge
const getConditionStatus = (condition: NodeCondition, tc: TFunction, t: TFunction) => {
  if (condition.status === 'True') {
    return <Badge status="success" text={tc('status.healthy')} />;
  } else if (condition.status === 'False') {
    if (['DiskPressure', 'MemoryPressure', 'PIDPressure', 'NetworkUnavailable'].includes(condition.type)) {
      return <Badge status="success" text={tc('status.healthy')} />;
    }
    return <Badge status="error" text={tc('status.unhealthy')} />;
  }
  return <Badge status="default" text={t('status.unknown')} />;
};

// Pod table columns builder
const buildPodColumns = (
  clusterId: string,
  navigate: (path: string) => void,
  t: TFunction,
  tc: TFunction
): ColumnsType<Pod> => [
  {
    title: tc('table.status'),
    key: 'status',
    width: 60,
    render: (_, record) => {
      if (record.status === 'Running') return <Badge status="success" />;
      if (record.status === 'Pending') return <Badge status="processing" />;
      if (record.status === 'Succeeded') return <Badge status="default" />;
      return <Badge status="error" />;
    },
  },
  {
    title: tc('table.name'),
    dataIndex: 'name',
    key: 'name',
    render: (text, record) => (
      <a onClick={() => navigate(`/clusters/${clusterId}/namespaces/${record.namespace}/pods/${text}`)}>
        {text}
      </a>
    ),
  },
  {
    title: tc('table.namespace'),
    dataIndex: 'namespace',
    key: 'namespace',
    render: (namespace) => <Tag color="blue">{namespace}</Tag>,
  },
  {
    title: tc('table.status'),
    dataIndex: 'status',
    key: 'podStatus',
    render: (status) => {
      if (status === 'Running') return <Tag color="success">{tc('status.running')}</Tag>;
      if (status === 'Pending') return <Tag color="processing">{tc('status.pending')}</Tag>;
      if (status === 'Succeeded') return <Tag color="default">{tc('status.succeeded')}</Tag>;
      return <Tag color="error">{tc('status.failed')}</Tag>;
    },
  },
  {
    title: t('columns.restarts'),
    dataIndex: 'restartCount',
    key: 'restartCount',
  },
  {
    title: t('resources.cpu'),
    key: 'cpuLimit',
    render: (_, record) => record.cpuUsage > 0 ? `${Math.round(record.cpuUsage)}m` : '-',
  },
  {
    title: t('resources.memory'),
    key: 'memoryLimit',
    render: (_, record) => record.memoryUsage > 0 ? `${Math.round(record.memoryUsage)}Mi` : '-',
  },
  {
    title: tc('table.createdAt'),
    dataIndex: 'createdAt',
    key: 'createdAt',
    render: (time) => new Date(time).toLocaleString(),
  },
];

export function createNodeDetailTabItems(props: NodeDetailTabsProps) {
  const {
    clusterId,
    nodeName,
    node,
    pods,
    loadingPods,
    t,
    tc,
    navigate,
    handleExportPods,
    handleRemoveLabel,
    handleRemoveTaint,
    setLabelModalVisible,
    setTaintModalVisible,
  } = props;

  const podColumns = buildPodColumns(clusterId, navigate, t, tc);

  return [
    {
      key: 'monitoring',
      label: (
        <span>
          <BarChartOutlined />
          {tc('menu.monitoring')}
        </span>
      ),
      children: (
        <MonitoringCharts
          clusterId={clusterId}
          nodeName={nodeName}
          type="node"
        />
      ),
    },
    {
      key: 'overview',
      label: (
        <span>
          <DesktopOutlined />
          {t('detail.nodeStatus')}
        </span>
      ),
      children: (
        <Card title={t('detail.nodeStatus')}>
          <Statistic
            title={t('columns.status')}
            value={node?.status || 'Unknown'}
            valueStyle={{ color: node?.status === 'Ready' ? '#3f8600' : '#cf1322' }}
            prefix={node?.status === 'Ready' ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
          />
          <Divider />
          <div>
            <Text strong>{t('detail.schedulingStatus')}: </Text>
            {node?.unschedulable || node?.taints?.some(taint => taint.effect === 'NoSchedule') ? (
              <Tag icon={<PauseCircleOutlined />} color="warning">{t('status.unschedulable')}</Tag>
            ) : (
              <Tag icon={<CheckCircleOutlined />} color="success">{t('status.schedulable')}</Tag>
            )}
          </div>
          <div style={{ marginTop: 8 }}>
            <Text strong>{t('detail.conditions')}: </Text>
            <div style={{ marginTop: 8 }}>
              {node?.conditions?.map((condition, index) => (
                <div key={index} style={{ marginBottom: 4 }}>
                  <Space>
                    {getConditionStatus(condition, tc, t)}
                    <Text>{condition.type}</Text>
                  </Space>
                </div>
              ))}
            </div>
          </div>
        </Card>
      ),
    },
    {
      key: 'pods',
      label: (
        <span>
          <AppstoreOutlined />
          Pod ({pods.length})
        </span>
      ),
      children: (
        <div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 16 }}>
            <Button
              icon={<DownloadOutlined />}
              onClick={handleExportPods}
              disabled={pods.length === 0}
            >
              {tc('actions.export')}
            </Button>
          </div>
          <Table
            scroll={{ x: 'max-content' }}
            columns={podColumns}
            dataSource={pods}
            rowKey="id"
            pagination={{
              pageSize: 10,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => `${tc('table.total')} ${total} Pod`,
            }}
            loading={loadingPods}
            locale={{ emptyText: tc('messages.noData') }}
          />
        </div>
      ),
    },
    {
      key: 'labels',
      label: (
        <span>
          <EditOutlined />
          {t('detail.labels')}
        </span>
      ),
      children: (
        <div>
          <Card title={t('detail.systemLabels')} style={{ marginBottom: 16 }}>
            <Space wrap>
              {node?.labels && Array.isArray(node.labels) && node.labels
                .filter(label => label.key.startsWith('kubernetes.io/') || label.key.startsWith('node.kubernetes.io/') || label.key.startsWith('topology.kubernetes.io/'))
                .map((label: { key: string; value: string }, index: number) => (
                  <Tag key={index} color="blue">
                    {label.key}={label.value}
                  </Tag>
                ))}
            </Space>
          </Card>

          <Card title={t('detail.customLabels')}>
            <Space wrap style={{ marginBottom: 16 }}>
              {node?.labels && Array.isArray(node.labels) && node.labels
                .filter(label => !label.key.startsWith('kubernetes.io/') && !label.key.startsWith('node.kubernetes.io/') && !label.key.startsWith('topology.kubernetes.io/'))
                .map((label: { key: string; value: string }, index: number) => (
                  <Tag
                    key={index}
                    closable
                    onClose={() => handleRemoveLabel(label.key)}
                  >
                    {label.key}={label.value}
                  </Tag>
                ))}
            </Space>

            <Button
              type="dashed"
              icon={<PlusOutlined />}
              onClick={() => setLabelModalVisible(true)}
            >
              {t('detail.addLabel')}
            </Button>
          </Card>
        </div>
      ),
    },
    {
      key: 'taints',
      label: (
        <span>
          <SettingOutlined />
          {t('detail.taints')}
        </span>
      ),
      children: (
        <Card title={t('detail.currentTaints')}>
          {node?.taints && node.taints.length > 0 ? (
            node.taints.map((taint, index) => (
              <Card
                key={index}
                type="inner"
                style={{ marginBottom: 16 }}
                title={`${taint.key}${taint.value ? `=${taint.value}` : ''}:${taint.effect}`}
                extra={
                  <Button
                    type="text"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => handleRemoveTaint(taint)}
                  >
                    {tc('actions.delete')}
                  </Button>
                }
              >
                <Descriptions column={1}>
                  <Descriptions.Item label={t('detail.taintKey')}>{taint.key}</Descriptions.Item>
                  {taint.value && <Descriptions.Item label={t('detail.taintValue')}>{taint.value}</Descriptions.Item>}
                  <Descriptions.Item label={t('detail.taintEffect')}>
                    <Tag color={
                      taint.effect === 'NoSchedule' ? 'orange' :
                      taint.effect === 'PreferNoSchedule' ? 'blue' : 'red'
                    }>
                      {taint.effect}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label={t('detail.description')}>
                    {taint.effect === 'NoSchedule' && t('detail.noScheduleDesc')}
                    {taint.effect === 'PreferNoSchedule' && t('detail.preferNoScheduleDesc')}
                    {taint.effect === 'NoExecute' && t('detail.noExecuteDesc')}
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            ))
          ) : (
            <EmptyState description={t('detail.noTaints')} />
          )}

          <Button
            type="dashed"
            icon={<PlusOutlined />}
            onClick={() => setTaintModalVisible(true)}
            style={{ marginTop: 16 }}
          >
            {t('detail.addTaint')}
          </Button>
        </Card>
      ),
    },
    {
      key: 'terminal',
      label: (
        <span>
          <CodeOutlined />
          {t('actions.ssh')}
        </span>
      ),
      children: (
        <SSHTerminal
          nodeIP={node?.addresses?.find(addr => addr.type === 'InternalIP')?.address || ''}
          nodeName={nodeName}
          clusterId={clusterId}
        />
      ),
    },
  ];
}

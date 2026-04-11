import React from 'react';
import { Button, Space, Table, Progress, Tag, Alert, Empty } from 'antd';
import { ReloadOutlined, DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { WorkloadEfficiency, NamespaceEfficiency } from '../../../services/costService';
import { ResourceService } from '../../../services/costService';

interface WasteResourcesTabProps {
  wasteItems: WorkloadEfficiency[];
  wasteItemsLoading: boolean;
  nsEfficiency: NamespaceEfficiency[];
  clusterId: string;
  onRefresh: () => void;
}

export const WasteResourcesTab: React.FC<WasteResourcesTabProps> = ({
  wasteItems,
  wasteItemsLoading,
  nsEfficiency,
  clusterId,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh} loading={wasteItemsLoading}>
          {t('common:actions.refresh')}
        </Button>
        <Button
          icon={<DownloadOutlined />}
          href={ResourceService.getWasteExportURL(clusterId)}
          target="_blank"
          disabled={wasteItems.length === 0}
        >
          匯出 CSV
        </Button>
      </Space>

      <Alert
        type="warning"
        showIcon
        message="以下工作負載的 CPU 效率低於 20%，代表申請的資源遠超實際使用量，建議降低 CPU requests。"
        style={{ marginBottom: 16 }}
      />

      {wasteItems.length === 0 && !wasteItemsLoading ? (
        <Empty description={nsEfficiency.length > 0 && !nsEfficiency[0].has_metrics ? '需要 Prometheus 監控資料才能識別低效工作負載' : '目前無低效工作負載'} />
      ) : (
        <Table
          rowKey={(r: WorkloadEfficiency) => `${r.namespace}/${r.kind}/${r.name}`}
          loading={wasteItemsLoading}
          dataSource={wasteItems}
          size="small"
          scroll={{ x: 900 }}
          pagination={{ pageSize: 20 }}
          columns={[
            { title: '命名空間', dataIndex: 'namespace', key: 'namespace', width: 140 },
            { title: '工作負載', dataIndex: 'name', key: 'name', ellipsis: true },
            { title: '類型', dataIndex: 'kind', key: 'kind', width: 110,
              render: (k: string) => <Tag color={{ Deployment: 'blue', StatefulSet: 'purple', DaemonSet: 'cyan' }[k] ?? 'default'}>{k}</Tag> },
            { title: '副本', dataIndex: 'replicas', key: 'replicas', width: 70 },
            {
              title: 'CPU 效率',
              render: (_: unknown, r: WorkloadEfficiency) => (
                <Progress percent={+(r.cpu_efficiency * 100).toFixed(1)} size="small"
                  status="exception" format={p => `${p}%`} style={{ width: 110 }} />
              ),
              sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => a.cpu_efficiency - b.cpu_efficiency,
            },
            { title: 'CPU 申請 (m)', dataIndex: 'cpu_request_millicores', key: 'cpu_req', render: (v: number) => v.toFixed(0) },
            { title: 'CPU 使用 (m)', dataIndex: 'cpu_usage_millicores', key: 'cpu_usage', render: (v: number) => v.toFixed(1) },
            { title: '記憶體效率', render: (_: unknown, r: WorkloadEfficiency) => (
              <Progress percent={+(r.memory_efficiency * 100).toFixed(1)} size="small"
                status={r.memory_efficiency < 0.2 ? 'exception' : 'active'} format={p => `${p}%`} style={{ width: 110 }} />
            )},
            {
              title: '廢棄分數',
              render: (_: unknown, r: WorkloadEfficiency) => (
                <Tag color={r.waste_score > 0.7 ? 'red' : 'orange'}>{(r.waste_score * 100).toFixed(0)}</Tag>
              ),
              sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => b.waste_score - a.waste_score,
              defaultSortOrder: 'ascend' as const,
            },
            {
              title: '建議 CPU (m)',
              key: 'rec_cpu',
              render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
                ? <Tag color="geekblue">{r.rightsizing.cpu_recommended_millicores.toFixed(0)}</Tag>
                : '—',
            },
            {
              title: '建議記憶體 (MiB)',
              key: 'rec_mem',
              render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
                ? <Tag color="geekblue">{r.rightsizing.memory_recommended_mib.toFixed(0)}</Tag>
                : '—',
            },
          ]}
        />
      )}
    </div>
  );
};

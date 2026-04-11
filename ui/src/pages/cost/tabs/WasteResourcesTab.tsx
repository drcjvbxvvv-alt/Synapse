import React from 'react';
import { Button, Space, Table, Progress, Tag, Alert, theme } from 'antd';
import { ReloadOutlined, DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import EmptyState from '../../../components/EmptyState';
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
  const { token } = theme.useToken();

  return (
    <div>
      <Space style={{ marginBottom: token.marginMD }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh} loading={wasteItemsLoading}>
          {t('common:actions.refresh')}
        </Button>
        <Button
          icon={<DownloadOutlined />}
          href={ResourceService.getWasteExportURL(clusterId)}
          target="_blank"
          disabled={wasteItems.length === 0}
        >
          {t('cost:export.button')}
        </Button>
      </Space>

      <Alert
        type="warning"
        showIcon
        message={t('cost:wasteResources.alertMessage')}
        style={{ marginBottom: token.marginMD }}
      />

      {wasteItems.length === 0 && !wasteItemsLoading ? (
        <EmptyState description={nsEfficiency.length > 0 && !nsEfficiency[0].has_metrics ? t('cost:wasteResources.emptyNoMetrics') : t('cost:wasteResources.emptyNoWaste')} />
      ) : (
        <Table
          rowKey={(r: WorkloadEfficiency) => `${r.namespace}/${r.kind}/${r.name}`}
          loading={wasteItemsLoading}
          dataSource={wasteItems}
          size="small"
          scroll={{ x: 900 }}
          pagination={{ pageSize: 20 }}
          columns={[
            { title: t('cost:wasteResources.table.namespace'), dataIndex: 'namespace', key: 'namespace', width: 140 },
            { title: t('cost:wasteResources.table.workload'), dataIndex: 'name', key: 'name', ellipsis: true },
            { title: t('cost:wasteResources.table.type'), dataIndex: 'kind', key: 'kind', width: 110,
              render: (k: string) => <Tag color={{ Deployment: 'blue', StatefulSet: 'purple', DaemonSet: 'cyan' }[k] ?? 'default'}>{k}</Tag> },
            { title: t('cost:wasteResources.table.replicas'), dataIndex: 'replicas', key: 'replicas', width: 70 },
            {
              title: t('cost:wasteResources.table.cpuEfficiency'),
              render: (_: unknown, r: WorkloadEfficiency) => (
                <Progress percent={+(r.cpu_efficiency * 100).toFixed(1)} size="small"
                  status="exception" format={p => `${p}%`} style={{ width: 110 }} />
              ),
              sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => a.cpu_efficiency - b.cpu_efficiency,
            },
            { title: t('cost:wasteResources.table.cpuRequest'), dataIndex: 'cpu_request_millicores', key: 'cpu_req', render: (v: number) => v.toFixed(0) },
            { title: t('cost:wasteResources.table.cpuUsage'), dataIndex: 'cpu_usage_millicores', key: 'cpu_usage', render: (v: number) => v.toFixed(1) },
            { title: t('cost:wasteResources.table.memEfficiency'), render: (_: unknown, r: WorkloadEfficiency) => (
              <Progress percent={+(r.memory_efficiency * 100).toFixed(1)} size="small"
                status={r.memory_efficiency < 0.2 ? 'exception' : 'active'} format={p => `${p}%`} style={{ width: 110 }} />
            )},
            {
              title: t('cost:wasteResources.table.wasteScore'),
              render: (_: unknown, r: WorkloadEfficiency) => (
                <Tag color={r.waste_score > 0.7 ? 'red' : 'orange'}>{(r.waste_score * 100).toFixed(0)}</Tag>
              ),
              sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => b.waste_score - a.waste_score,
              defaultSortOrder: 'ascend' as const,
            },
            {
              title: t('cost:wasteResources.table.recommendedCPU'),
              key: 'rec_cpu',
              render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
                ? <Tag color="geekblue">{r.rightsizing.cpu_recommended_millicores.toFixed(0)}</Tag>
                : '—',
            },
            {
              title: t('cost:wasteResources.table.recommendedMem'),
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

import React from 'react';
import { Button, Space, Table, Progress, Tag } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { WorkloadEfficiency } from '../../../services/costService';

interface WorkloadEfficiencyTabProps {
  wlEfficiency: WorkloadEfficiency[];
  wlEffTotal: number;
  wlEffPage: number;
  wlEffLoading: boolean;
  wlEffNs: string;
  onPageChange: (page: number) => void;
  onRefresh: () => void;
}

export const WorkloadEfficiencyTab: React.FC<WorkloadEfficiencyTabProps> = ({
  wlEfficiency,
  wlEffTotal,
  wlEffPage,
  wlEffLoading,
  onPageChange,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh} loading={wlEffLoading}>
          {t('common:actions.refresh')}
        </Button>
      </Space>

      <Table
        rowKey={(r: WorkloadEfficiency) => `${r.namespace}/${r.kind}/${r.name}`}
        loading={wlEffLoading}
        dataSource={wlEfficiency}
        size="small"
        scroll={{ x: 1400 }}
        pagination={{
          current: wlEffPage,
          total: wlEffTotal,
          pageSize: 20,
          onChange: onPageChange,
        }}
        columns={[
          { title: t('cost:table.namespace'), dataIndex: 'namespace', key: 'namespace', width: 130, fixed: 'left' },
          { title: t('cost:table.workload'), dataIndex: 'name', key: 'name', width: 160, ellipsis: true },
          { title: t('cost:table.type'), dataIndex: 'kind', key: 'kind', width: 100,
            render: (k: string) => <Tag color={{ Deployment: 'blue', StatefulSet: 'purple', DaemonSet: 'cyan' }[k] ?? 'default'}>{k}</Tag> },
          { title: t('cost:table.replicas'), dataIndex: 'replicas', key: 'replicas', width: 60 },
          { title: t('cost:table.cpuRequest'), dataIndex: 'cpu_request_millicores', key: 'cpu_req', width: 110, render: (v: number) => v.toFixed(0) },
          {
            title: t('cost:occupancy.cpuEfficiency'), dataIndex: 'cpu_efficiency', key: 'cpu_eff', width: 160,
            render: (v: number, r: WorkloadEfficiency) => r.has_metrics ? (
              <Progress percent={+(v * 100).toFixed(1)} size="small"
                status={v < 0.2 ? 'exception' : v < 0.5 ? 'active' : 'normal'}
                format={p => `${p}%`} style={{ width: 120 }} />
            ) : <Tag>{t('cost:occupancy.needsPrometheus')}</Tag>,
            sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => a.cpu_efficiency - b.cpu_efficiency,
          },
          { title: t('cost:table.memRequest'), dataIndex: 'memory_request_mib', key: 'mem_req', width: 130, render: (v: number) => v.toFixed(0) },
          {
            title: t('cost:occupancy.memEfficiency'), dataIndex: 'memory_efficiency', key: 'mem_eff', width: 160,
            render: (v: number, r: WorkloadEfficiency) => r.has_metrics ? (
              <Progress percent={+(v * 100).toFixed(1)} size="small"
                status={v < 0.2 ? 'exception' : v < 0.5 ? 'active' : 'normal'}
                format={p => `${p}%`} style={{ width: 120 }} />
            ) : <Tag>{t('cost:occupancy.needsPrometheus')}</Tag>,
          },
          {
            title: t('cost:wasteResources.table.wasteScore'),
            dataIndex: 'waste_score',
            key: 'waste',
            width: 90,
            render: (v: number, r: WorkloadEfficiency) => r.has_metrics ? (
              <Tag color={v > 0.7 ? 'red' : v > 0.4 ? 'orange' : 'green'}>
                {(v * 100).toFixed(0)}
              </Tag>
            ) : '—',
            sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => b.waste_score - a.waste_score,
            defaultSortOrder: 'ascend' as const,
          },
          {
            title: t('cost:wasteResources.table.recommendedCPU'),
            key: 'rec_cpu',
            width: 110,
            render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
              ? <Tag color="geekblue">{r.rightsizing.cpu_recommended_millicores.toFixed(0)}</Tag>
              : '—',
          },
          {
            title: t('cost:wasteResources.table.recommendedMem'),
            key: 'rec_mem',
            width: 130,
            render: (_: unknown, r: WorkloadEfficiency) => r.rightsizing
              ? <Tag color="geekblue">{r.rightsizing.memory_recommended_mib.toFixed(0)}</Tag>
              : '—',
          },
        ]}
      />
    </div>
  );
};

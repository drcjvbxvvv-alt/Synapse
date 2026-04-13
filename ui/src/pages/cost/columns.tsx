import { Progress, Tag, Typography } from 'antd';
import type { TFunction } from 'i18next';
import type { CostItem, WasteItem, WorkloadEfficiency } from '../../services/costService';

const { Text } = Typography;

// Utility cell renderer for utilization progress
export const utilCell = (val: number) => (
  <Progress
    percent={Number(val.toFixed(1))}
    size="small"
    status={val < 10 ? 'exception' : val < 50 ? 'active' : 'normal'}
    format={p => `${p}%`}
  />
);

// Efficiency cell renderer
export const efficiencyCell = (val: number) => {
  const color = val < 20 ? 'red' : val < 50 ? 'orange' : 'green';
  return (
    <Progress
      percent={Number(val.toFixed(1))}
      size="small"
      strokeColor={color}
      format={p => `${p}%`}
    />
  );
};

export const getNamespaceCostColumns = (t: TFunction) => [
  { title: t('cost:table.namespace'), dataIndex: 'name', key: 'name' },
  { title: t('cost:table.cpuRequest'), dataIndex: 'cpu_request', key: 'cpu_request', render: (v: number) => v.toFixed(1) },
  { title: t('cost:table.cpuUtil'), dataIndex: 'cpu_util', key: 'cpu_util', render: utilCell },
  { title: t('cost:table.memRequest'), dataIndex: 'mem_request', key: 'mem_request', render: (v: number) => v.toFixed(1) },
  { title: t('cost:table.memUtil'), dataIndex: 'mem_util', key: 'mem_util', render: utilCell },
  { title: t('cost:table.podCount'), dataIndex: 'pod_count', key: 'pod_count' },
  {
    title: t('cost:table.estCost'), dataIndex: 'est_cost', key: 'est_cost',
    render: (v: number, row: CostItem) => <Text strong>{`${row.currency} ${v.toFixed(4)}`}</Text>,
    sorter: (a: CostItem, b: CostItem) => b.est_cost - a.est_cost,
  },
];

export const getWorkloadCostColumns = (t: TFunction) => [
  { title: t('cost:table.workload'), dataIndex: 'name', key: 'name', ellipsis: true },
  { title: t('cost:table.cpuRequest'), dataIndex: 'cpu_request', key: 'cpu_request', render: (v: number) => v.toFixed(1) },
  { title: t('cost:table.cpuUtil'), dataIndex: 'cpu_util', key: 'cpu_util', render: utilCell },
  { title: t('cost:table.memRequest'), dataIndex: 'mem_request', key: 'mem_request', render: (v: number) => v.toFixed(1) },
  { title: t('cost:table.memUtil'), dataIndex: 'mem_util', key: 'mem_util', render: utilCell },
  {
    title: t('cost:table.estCost'), dataIndex: 'est_cost', key: 'est_cost',
    render: (v: number, row: CostItem) => `${row.currency} ${v.toFixed(4)}`,
  },
];

export const getWasteColumns = (t: TFunction) => [
  { title: t('cost:table.namespace'), dataIndex: 'namespace', key: 'namespace' },
  { title: t('cost:table.workload'), dataIndex: 'workload', key: 'workload', ellipsis: true },
  { title: t('cost:table.cpuRequest'), dataIndex: 'cpu_request', key: 'cpu_request', render: (v: number) => v.toFixed(1) },
  {
    title: t('cost:table.cpuUtil'), dataIndex: 'cpu_util', key: 'cpu_util',
    render: (v: number) => <Tag color="red">{v.toFixed(1)}%</Tag>,
  },
  { title: t('cost:table.memRequest'), dataIndex: 'mem_request', key: 'mem_request', render: (v: number) => v.toFixed(1) },
  { title: t('cost:table.days'), dataIndex: 'days', key: 'days' },
  {
    title: t('cost:table.wastedCost'), dataIndex: 'wasted_cost', key: 'wasted_cost',
    render: (v: number, row: WasteItem) => (
      <Text type="danger" strong>{`${row.currency} ${v.toFixed(4)}`}</Text>
    ),
  },
];

export const getOccupancyColumns = (t: TFunction) => [
  { title: t('cost:table.namespace'), dataIndex: 'namespace', key: 'namespace' },
  { title: t('cost:occupancy.cpuRequestCol'), dataIndex: 'cpu_request_millicores', key: 'cpu_request_millicores', render: (v: number) => v.toFixed(0) },
  { title: t('cost:occupancy.cpuOccupancyCol'), dataIndex: 'cpu_occupancy_percent', key: 'cpu_occupancy_percent', render: (v: number) => `${v.toFixed(2)}%` },
  { title: t('cost:occupancy.memRequestCol'), dataIndex: 'memory_request_mib', key: 'memory_request_mib', render: (v: number) => v.toFixed(0) },
  { title: t('cost:occupancy.memOccupancyCol'), dataIndex: 'memory_occupancy_percent', key: 'memory_occupancy_percent', render: (v: number) => `${v.toFixed(2)}%` },
  { title: t('cost:occupancy.podCountCol'), dataIndex: 'pod_count', key: 'pod_count' },
];

export const getNamespaceEfficiencyColumns = (t: TFunction) => [
  { title: t('cost:table.namespace'), dataIndex: 'namespace', key: 'namespace' },
  {
    title: 'CPU 效率', dataIndex: 'cpu_efficiency', key: 'cpu_eff', width: 170,
    render: efficiencyCell,
    sorter: (a: { cpu_efficiency: number }, b: { cpu_efficiency: number }) => a.cpu_efficiency - b.cpu_efficiency,
  },
  {
    title: '記憶體效率', dataIndex: 'memory_efficiency', key: 'mem_eff', width: 170,
    render: efficiencyCell,
    sorter: (a: { memory_efficiency: number }, b: { memory_efficiency: number }) => a.memory_efficiency - b.memory_efficiency,
  },
  { title: 'CPU Request (m)', dataIndex: 'cpu_request_millicores', key: 'cpu_req', render: (v: number) => v.toFixed(0) },
  { title: 'CPU Usage (m)', dataIndex: 'cpu_usage_millicores', key: 'cpu_use', render: (v: number) => v.toFixed(0) },
  { title: 'Mem Request (MiB)', dataIndex: 'memory_request_mib', key: 'mem_req', render: (v: number) => v.toFixed(0) },
  { title: 'Mem Usage (MiB)', dataIndex: 'memory_usage_mib', key: 'mem_use', render: (v: number) => v.toFixed(0) },
];

export const getWorkloadEfficiencyColumns = (t: TFunction) => [
  { title: t('cost:table.namespace'), dataIndex: 'namespace', key: 'namespace', width: 140 },
  { title: t('cost:table.workload'), dataIndex: 'name', key: 'name', ellipsis: true },
  {
    title: 'CPU 效率', dataIndex: 'cpu_efficiency', key: 'cpu_eff', width: 160,
    render: efficiencyCell,
    sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => a.cpu_efficiency - b.cpu_efficiency,
  },
  {
    title: '記憶體效率', dataIndex: 'memory_efficiency', key: 'mem_eff', width: 160,
    render: efficiencyCell,
    sorter: (a: WorkloadEfficiency, b: WorkloadEfficiency) => a.memory_efficiency - b.memory_efficiency,
  },
  {
    key: 'waste',
    title: '浪費警告',
    render: (_: unknown, row: WorkloadEfficiency) => (row.cpu_efficiency < 20 || row.memory_efficiency < 20 ? <Tag color="red">低效</Tag> : null),
    width: 90,
  },
];

export const getWasteItemColumns = (t: TFunction) => [
  { title: t('cost:table.namespace'), dataIndex: 'namespace', key: 'namespace' },
  { title: t('cost:table.workload'), dataIndex: 'name', key: 'name', ellipsis: true },
  { title: 'CPU 效率', dataIndex: 'cpu_efficiency', key: 'cpu_eff', render: (v: number) => <Tag color="red">{v.toFixed(1)}%</Tag> },
  { title: '記憶體效率', dataIndex: 'memory_efficiency', key: 'mem_eff', render: (v: number) => <Tag color="red">{v.toFixed(1)}%</Tag> },
  { title: 'CPU Request (m)', dataIndex: 'cpu_request_millicores', key: 'cpu_req', render: (v: number) => v.toFixed(0) },
  { title: 'Mem Request (MiB)', dataIndex: 'memory_request_mib', key: 'mem_req', render: (v: number) => v.toFixed(0) },
];

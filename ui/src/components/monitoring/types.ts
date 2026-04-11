export interface DataPoint {
  timestamp: number;
  value: number;
}

export interface MetricSeries {
  current: number;
  series: DataPoint[];
}

export interface NetworkMetrics {
  in: MetricSeries;
  out: MetricSeries;
}

export interface PodMetrics {
  total: number;
  running: number;
  pending: number;
  failed: number;
}

export interface NetworkPPS {
  in: MetricSeries;
  out: MetricSeries;
}

export interface NetworkDrops {
  receive: MetricSeries;
  transmit: MetricSeries;
}

export interface DiskIOPS {
  read: MetricSeries;
  write: MetricSeries;
}

export interface DiskThroughput {
  read: MetricSeries;
  write: MetricSeries;
}

export interface MultiSeriesDataPoint {
  timestamp: number;
  values: { [podName: string]: number };
}

export interface MultiSeriesMetric {
  series: MultiSeriesDataPoint[];
}

export interface ClusterOverview {
  total_cpu_cores: number;
  total_memory: number;
  worker_nodes: number;
  cpu_usage_rate?: MetricSeries;
  memory_usage_rate?: MetricSeries;
  max_pods: number;
  created_pods: number;
  available_pods: number;
  pod_usage_rate: number;
  etcd_has_leader: boolean;
  apiserver_availability: number;
  cpu_request_ratio?: MetricSeries;
  cpu_limit_ratio?: MetricSeries;
  mem_request_ratio?: MetricSeries;
  mem_limit_ratio?: MetricSeries;
  apiserver_request_rate?: MetricSeries;
}

export interface NodeMetricItem {
  node_name: string;
  cpu_usage_rate: number;
  memory_usage_rate: number;
  cpu_cores: number;
  total_memory: number;
  status: string;
}

export interface ClusterMetricsData {
  cpu?: MetricSeries;
  memory?: MetricSeries;
  network?: NetworkMetrics;
  storage?: MetricSeries;
  pods?: PodMetrics;
  cpu_request?: MetricSeries;
  cpu_limit?: MetricSeries;
  memory_request?: MetricSeries;
  memory_limit?: MetricSeries;
  probe_failures?: MetricSeries;
  container_restarts?: MetricSeries;
  network_pps?: NetworkPPS;
  threads?: MetricSeries;
  network_drops?: NetworkDrops;
  cpu_throttling?: MetricSeries;
  cpu_throttling_time?: MetricSeries;
  disk_iops?: DiskIOPS;
  disk_throughput?: DiskThroughput;
  cpu_usage_absolute?: MetricSeries;
  memory_usage_bytes?: MetricSeries;
  oom_kills?: MetricSeries;
  cluster_overview?: ClusterOverview;
  node_list?: NodeMetricItem[];
  cpu_multi?: MultiSeriesMetric;
  memory_multi?: MultiSeriesMetric;
  container_restarts_multi?: MultiSeriesMetric;
  oom_kills_multi?: MultiSeriesMetric;
  probe_failures_multi?: MultiSeriesMetric;
  network_pps_multi?: MultiSeriesMetric;
  threads_multi?: MultiSeriesMetric;
  network_drops_multi?: MultiSeriesMetric;
  cpu_throttling_multi?: MultiSeriesMetric;
  cpu_throttling_time_multi?: MultiSeriesMetric;
  disk_iops_multi?: MultiSeriesMetric;
  disk_throughput_multi?: MultiSeriesMetric;
}

export interface MonitoringChartsProps {
  clusterId: string;
  clusterName?: string;
  nodeName?: string;
  namespace?: string;
  podName?: string;
  workloadName?: string;
  type: 'cluster' | 'node' | 'pod' | 'workload';
  lazyLoad?: boolean;
}

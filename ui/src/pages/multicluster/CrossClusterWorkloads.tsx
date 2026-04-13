import React, { useState, useEffect, useCallback } from 'react';
import {
  App, Badge, Button, Card, Col, Input, Row, Select, Space, Statistic, Table, Tag, Tooltip, Typography,
} from 'antd';
import {
  ReloadOutlined, SearchOutlined, WarningOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import { crossClusterService, type CrossClusterWorkload } from '../../services/imageService';

const { Text } = Typography;
const { Option } = Select;

type ClusterStat = {
  clusterId: number;
  clusterName: string;
  deployments: number;
  statefulSets: number;
  daemonSets: number;
  degraded: number;
};

const CrossClusterWorkloads: React.FC = () => {
  const { t } = useTranslation(['multicluster', 'common']);
  const { message } = App.useApp();
  const [items, setItems] = useState<CrossClusterWorkload[]>([]);
  const [stats, setStats] = useState<ClusterStat[]>([]);
  const [loading, setLoading] = useState(false);
  const [filterKind, setFilterKind] = useState('');
  const [filterName, setFilterName] = useState('');
  const [filterNS, setFilterNS] = useState('');

  const loadStats = useCallback(async () => {
    try {
      const res = await crossClusterService.getStats();
      setStats(res.data.clusters || []);
    } catch {
      // ignore
    }
  }, []);

  const loadWorkloads = useCallback(async () => {
    setLoading(true);
    try {
      const res = await crossClusterService.listWorkloads({
        kind: filterKind || undefined,
        name: filterName || undefined,
        namespace: filterNS || undefined,
      });
      setItems(res.data.items || []);
    } catch {
      message.error(t('multicluster:messages.loadError'));
    } finally {
      setLoading(false);
    }
  }, [filterKind, filterName, filterNS, message, t]);

  useEffect(() => {
    loadStats();
    loadWorkloads();
  }, [loadStats, loadWorkloads]);

  const totalDegraded = stats.reduce((s, c) => s + c.degraded, 0);
  const totalWorkloads = stats.reduce((s, c) => s + c.deployments + c.statefulSets + c.daemonSets, 0);

  const columns: ColumnsType<CrossClusterWorkload> = [
    {
      title: t('multicluster:table.cluster'),
      dataIndex: 'clusterName',
      width: 130,
    },
    {
      title: t('multicluster:table.namespace'),
      dataIndex: 'namespace',
      width: 140,
    },
    {
      title: t('multicluster:table.type'),
      dataIndex: 'kind',
      width: 110,
      render: (kind: string) => {
        const color = kind === 'Deployment' ? 'blue' : kind === 'StatefulSet' ? 'purple' : 'cyan';
        return <Tag color={color}>{kind}</Tag>;
      },
    },
    {
      title: t('multicluster:table.name'),
      dataIndex: 'name',
      ellipsis: true,
    },
    {
      title: t('multicluster:table.replicas'),
      width: 90,
      render: (_, r) => `${r.ready} / ${r.replicas}`,
    },
    {
      title: t('multicluster:table.status'),
      dataIndex: 'status',
      width: 90,
      render: (s: string) =>
        s === 'healthy'
          ? <Badge status="success" text={t('multicluster:table.healthy')} />
          : <Badge status="error" text={t('multicluster:table.degraded')} />,
    },
    {
      title: t('multicluster:table.images'),
      dataIndex: 'images',
      render: (imgs: string[]) => (
        <Space direction="vertical" size={0}>
          {imgs.map((img, i) => (
            <Tooltip key={i} title={img}>
              <Text code style={{ fontSize: 11, maxWidth: 300, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {img}
              </Text>
            </Tooltip>
          ))}
        </Space>
      ),
    },
    {
      title: t('multicluster:table.createdAt'),
      dataIndex: 'createdAt',
      width: 160,
      render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm'),
    },
  ];

  return (
    <div>
      {/* 統計卡片 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card size="small">
            <Statistic title={t('multicluster:stats.clusters')} value={stats.length} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title={t('multicluster:stats.totalWorkloads')} value={totalWorkloads} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title={t('multicluster:stats.degradedWorkloads')}
              value={totalDegraded}
              valueStyle={{ color: totalDegraded > 0 ? '#ff4d4f' : '#52c41a' }}
              prefix={totalDegraded > 0 ? <WarningOutlined /> : undefined}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title={t('multicluster:stats.searchResults')} value={items.length} />
          </Card>
        </Col>
      </Row>

      {/* 工作負載列表 */}
      <Card
        title={t('multicluster:title')}
        extra={
          <Button icon={<ReloadOutlined />} onClick={loadWorkloads} loading={loading}>
            {t('multicluster:messages.refresh')}
          </Button>
        }
      >
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            placeholder={t('multicluster:filters.type')}
            value={filterKind}
            onChange={setFilterKind}
            style={{ width: 130 }}
            allowClear
          >
            <Option value="Deployment">Deployment</Option>
            <Option value="StatefulSet">StatefulSet</Option>
            <Option value="DaemonSet">DaemonSet</Option>
          </Select>
          <Input
            placeholder={t('multicluster:filters.nameFilter')}
            value={filterName}
            onChange={(e) => setFilterName(e.target.value)}
            style={{ width: 180 }}
            prefix={<SearchOutlined />}
            allowClear
          />
          <Input
            placeholder={t('multicluster:filters.namespaceFilter')}
            value={filterNS}
            onChange={(e) => setFilterNS(e.target.value)}
            style={{ width: 160 }}
            allowClear
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={loadWorkloads}>
            {t('multicluster:filters.search')}
          </Button>
        </Space>

        <Table
          rowKey={(r) => `${r.clusterId}-${r.namespace}-${r.kind}-${r.name}`}
          columns={columns}
          dataSource={items}
          loading={loading}
          scroll={{ x: 1200 }}
          pagination={{ pageSize: 20, showTotal: (total) => t('multicluster:messages.total', { count: total }) }}
        />
      </Card>
    </div>
  );
};

export default CrossClusterWorkloads;

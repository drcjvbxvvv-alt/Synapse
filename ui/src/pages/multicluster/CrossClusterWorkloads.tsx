import React, { useState, useEffect, useCallback } from 'react';
import {
  App, Badge, Button, Card, Col, Input, Row, Select, Space, Statistic, Table, Tag, Tooltip, Typography,
} from 'antd';
import {
  ReloadOutlined, SearchOutlined, WarningOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
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
      message.error('載入跨叢集工作負載失敗');
    } finally {
      setLoading(false);
    }
  }, [filterKind, filterName, filterNS, message]);

  useEffect(() => {
    loadStats();
    loadWorkloads();
  }, [loadStats, loadWorkloads]);

  const totalDegraded = stats.reduce((s, c) => s + c.degraded, 0);
  const totalWorkloads = stats.reduce((s, c) => s + c.deployments + c.statefulSets + c.daemonSets, 0);

  const columns: ColumnsType<CrossClusterWorkload> = [
    {
      title: '叢集',
      dataIndex: 'clusterName',
      width: 130,
    },
    {
      title: '命名空間',
      dataIndex: 'namespace',
      width: 140,
    },
    {
      title: '類型',
      dataIndex: 'kind',
      width: 110,
      render: (kind: string) => {
        const color = kind === 'Deployment' ? 'blue' : kind === 'StatefulSet' ? 'purple' : 'cyan';
        return <Tag color={color}>{kind}</Tag>;
      },
    },
    {
      title: '名稱',
      dataIndex: 'name',
      ellipsis: true,
    },
    {
      title: '副本',
      width: 90,
      render: (_, r) => `${r.ready} / ${r.replicas}`,
    },
    {
      title: '狀態',
      dataIndex: 'status',
      width: 90,
      render: (s: string) =>
        s === 'healthy'
          ? <Badge status="success" text="正常" />
          : <Badge status="error" text="異常" />,
    },
    {
      title: '映像',
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
      title: '建立時間',
      dataIndex: 'createdAt',
      width: 160,
      render: (v: string) => new Date(v).toLocaleString('zh-TW'),
    },
  ];

  return (
    <div>
      {/* 統計卡片 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card size="small">
            <Statistic title="叢集數" value={stats.length} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="工作負載總數" value={totalWorkloads} />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title="異常工作負載"
              value={totalDegraded}
              valueStyle={{ color: totalDegraded > 0 ? '#ff4d4f' : '#52c41a' }}
              prefix={totalDegraded > 0 ? <WarningOutlined /> : undefined}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small">
            <Statistic title="搜尋結果" value={items.length} suffix="筆" />
          </Card>
        </Col>
      </Row>

      {/* 工作負載列表 */}
      <Card
        title="跨叢集工作負載"
        extra={
          <Button icon={<ReloadOutlined />} onClick={loadWorkloads} loading={loading}>
            重新整理
          </Button>
        }
      >
        <Space style={{ marginBottom: 16 }} wrap>
          <Select
            placeholder="類型"
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
            placeholder="名稱（模糊）"
            value={filterName}
            onChange={(e) => setFilterName(e.target.value)}
            style={{ width: 180 }}
            prefix={<SearchOutlined />}
            allowClear
          />
          <Input
            placeholder="命名空間"
            value={filterNS}
            onChange={(e) => setFilterNS(e.target.value)}
            style={{ width: 160 }}
            allowClear
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={loadWorkloads}>
            搜尋
          </Button>
        </Space>

        <Table
          rowKey={(r) => `${r.clusterId}-${r.namespace}-${r.kind}-${r.name}`}
          columns={columns}
          dataSource={items}
          loading={loading}
          scroll={{ x: 1200 }}
          pagination={{ pageSize: 20, showTotal: (t) => `共 ${t} 筆` }}
        />
      </Card>
    </div>
  );
};

export default CrossClusterWorkloads;

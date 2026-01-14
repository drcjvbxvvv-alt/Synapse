import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Card,
  Table,
  Tag,
  Button,
  Space,
  Row,
  Col,
  Statistic,
  message,
  Tooltip,
  Badge,
  Typography,
  Empty,
  Spin,
  Progress,
} from 'antd';
import {
  AlertOutlined,
  ReloadOutlined,
  FireOutlined,
  WarningOutlined,
  ClusterOutlined,
  RightOutlined,
  ExclamationCircleOutlined,
  StopOutlined,
  CheckCircleOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import 'dayjs/locale/zh-cn';
import { overviewService } from '../../services/overviewService';
import type { GlobalAlertStats, ClusterAlertCount } from '../../services/overviewService';

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

const { Title, Text } = Typography;

// 卡片样式
const cardStyle = { 
  boxShadow: '0 1px 4px rgba(0,0,0,0.08)', 
  borderRadius: 8 
};

const GlobalAlertCenter: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [alertStats, setAlertStats] = useState<GlobalAlertStats | null>(null);

  // 加载数据
  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      const response = await overviewService.getAlertStats();
      setAlertStats(response.data);
    } catch (error) {
      console.error('加载告警统计失败:', error);
      message.error('加载告警统计失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // 获取严重程度颜色（未使用，保留以备将来使用）
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const getSeverityColor = (severity: string) => {
    switch (severity?.toLowerCase()) {
      case 'critical':
        return '#ff4d4f';
      case 'warning':
        return '#faad14';
      case 'info':
        return '#1890ff';
      default:
        return '#d9d9d9';
    }
  };

  // 集群告警表格列
  const clusterColumns: ColumnsType<ClusterAlertCount> = [
    {
      title: '集群名称',
      dataIndex: 'clusterName',
      key: 'clusterName',
      render: (text: string) => (
        <Space>
          <ClusterOutlined style={{ color: '#1890ff' }} />
          <Text strong>{text}</Text>
        </Space>
      ),
    },
    {
      title: '触发中告警',
      dataIndex: 'firing',
      key: 'firing',
      width: 140,
      sorter: (a, b) => a.firing - b.firing,
      defaultSortOrder: 'descend',
      render: (value: number) => (
        <Badge
          count={value}
          showZero
          style={{ 
            backgroundColor: value > 0 ? '#ff4d4f' : '#52c41a',
            fontWeight: 600,
          }}
        />
      ),
    },
    {
      title: '告警总数',
      dataIndex: 'total',
      key: 'total',
      width: 120,
      sorter: (a, b) => a.total - b.total,
      render: (value: number) => (
        <Text type={value > 0 ? undefined : 'secondary'}>{value}</Text>
      ),
    },
    {
      title: '告警状态',
      key: 'status',
      width: 200,
      render: (_: unknown, record: ClusterAlertCount) => {
        if (record.total === 0) {
          return (
            <Space>
              <CheckCircleOutlined style={{ color: '#52c41a' }} />
              <Text type="secondary">无告警</Text>
            </Space>
          );
        }
        const firingPercent = record.total > 0 ? (record.firing / record.total) * 100 : 0;
        return (
          <Tooltip title={`触发中: ${record.firing} / 总数: ${record.total}`}>
            <Progress
              percent={100}
              success={{ percent: 100 - firingPercent }}
              size="small"
              showInfo={false}
              strokeColor="#ff4d4f"
              trailColor="#52c41a"
            />
          </Tooltip>
        );
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_: unknown, record: ClusterAlertCount) => (
        <Button
          type="link"
          size="small"
          icon={<RightOutlined />}
          onClick={() => navigate(`/clusters/${record.clusterId}/alerts`)}
        >
          查看详情
        </Button>
      ),
    },
  ];

  // 严重程度分布
  const renderSeverityDistribution = () => {
    if (!alertStats?.bySeverity) return null;
    
    const severities = [
      { key: 'critical', label: 'Critical', icon: <FireOutlined />, color: '#ff4d4f' },
      { key: 'warning', label: 'Warning', icon: <WarningOutlined />, color: '#faad14' },
      { key: 'info', label: 'Info', icon: <InfoCircleOutlined />, color: '#1890ff' },
    ];

    return (
      <Row gutter={16}>
        {severities.map(({ key, label, icon, color }) => (
          <Col span={8} key={key}>
            <div style={{ 
              display: 'flex', 
              alignItems: 'center', 
              gap: 8,
              padding: '8px 12px',
              background: `${color}10`,
              borderRadius: 6,
              borderLeft: `3px solid ${color}`,
            }}>
              <span style={{ color, fontSize: 18 }}>{icon}</span>
              <div>
                <Text type="secondary" style={{ fontSize: 12 }}>{label}</Text>
                <div style={{ fontSize: 20, fontWeight: 600, color }}>
                  {alertStats.bySeverity[key] || 0}
                </div>
              </div>
            </div>
          </Col>
        ))}
      </Row>
    );
  };

  if (loading && !alertStats) {
    return (
      <div style={{ 
        display: 'flex', 
        justifyContent: 'center', 
        alignItems: 'center', 
        height: 'calc(100vh - 200px)' 
      }}>
        <Spin size="large" tip="加载中..." />
      </div>
    );
  }

  if (!alertStats || alertStats.enabledCount === 0) {
    return (
      <div style={{ padding: 24 }}>
        <Card style={cardStyle}>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space direction="vertical" align="center">
                <Text>暂无已配置告警的集群</Text>
                <Text type="secondary">请在集群配置中心配置 Alertmanager</Text>
              </Space>
            }
          >
            <Button type="primary" onClick={() => navigate('/clusters')}>
              前往集群管理
            </Button>
          </Empty>
        </Card>
      </div>
    );
  }

  // 获取有告警的集群列表
  const clustersWithAlerts = alertStats.byCluster || [];
  const firingClusters = clustersWithAlerts.filter(c => c.firing > 0);

  return (
    <div style={{ padding: '0 4px' }}>
      {/* 顶部工具栏 */}
      <div style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        alignItems: 'center', 
        marginBottom: 16,
        padding: '12px 16px',
        background: '#fff',
        borderRadius: 8,
        boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
      }}>
        <Space>
          <AlertOutlined style={{ fontSize: 20, color: '#f59e0b' }} />
          <Title level={4} style={{ margin: 0 }}>全局告警中心</Title>
          <Tag color={alertStats.enabledCount > 0 ? 'blue' : 'default'}>
            {alertStats.enabledCount} 个集群已配置
          </Tag>
        </Space>
        <Button 
          icon={<ReloadOutlined spin={loading} />} 
          onClick={loadData} 
          loading={loading}
        >
          刷新
        </Button>
      </div>

      {/* 统计卡片 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card bordered={false} style={{ ...cardStyle, height: 140 }} bodyStyle={{ padding: '20px 16px' }}>
            <Statistic
              title={<span style={{ color: '#6b7280' }}><AlertOutlined /> 告警总数</span>}
              value={alertStats.total}
              valueStyle={{ color: '#1f2937', fontSize: 32, fontWeight: 700 }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card 
            bordered={false} 
            style={{ 
              ...cardStyle, 
              height: 140,
              cursor: alertStats.firing > 0 ? 'pointer' : 'default',
              borderLeft: alertStats.firing > 0 ? '4px solid #ff4d4f' : undefined,
            }} 
            bodyStyle={{ padding: '20px 16px' }}
          >
            <Statistic
              title={<span style={{ color: '#6b7280' }}><FireOutlined style={{ color: '#ff4d4f' }} /> 触发中</span>}
              value={alertStats.firing}
              valueStyle={{ color: alertStats.firing > 0 ? '#ff4d4f' : '#9ca3af', fontSize: 32, fontWeight: 700 }}
            />
            {firingClusters.length > 0 && (
              <div style={{ marginTop: 4, fontSize: 12, color: '#6b7280' }}>
                {firingClusters.length} 个集群有告警
              </div>
            )}
          </Card>
        </Col>
        <Col span={6}>
          <Card bordered={false} style={{ ...cardStyle, height: 140 }} bodyStyle={{ padding: '20px 16px' }}>
            <Statistic
              title={<span style={{ color: '#6b7280' }}><StopOutlined style={{ color: '#faad14' }} /> 已静默</span>}
              value={alertStats.suppressed}
              valueStyle={{ color: alertStats.suppressed > 0 ? '#faad14' : '#9ca3af', fontSize: 32, fontWeight: 700 }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card bordered={false} style={{ ...cardStyle, height: 140 }} bodyStyle={{ padding: '20px 16px' }}>
            <Statistic
              title={<span style={{ color: '#6b7280' }}><ExclamationCircleOutlined style={{ color: '#ff4d4f' }} /> 严重告警</span>}
              value={alertStats.bySeverity?.critical || 0}
              valueStyle={{ color: (alertStats.bySeverity?.critical || 0) > 0 ? '#ff4d4f' : '#9ca3af', fontSize: 32, fontWeight: 700 }}
            />
          </Card>
        </Col>
      </Row>

      {/* 严重程度分布 */}
      <Card 
        title={
          <Space>
            <WarningOutlined style={{ color: '#f59e0b' }} />
            <span>告警严重程度分布</span>
          </Space>
        }
        bordered={false}
        style={{ ...cardStyle, marginBottom: 16 }}
        bodyStyle={{ padding: 16 }}
      >
        {renderSeverityDistribution()}
      </Card>

      {/* 集群告警分布表格 */}
      <Card
        title={
          <Space>
            <ClusterOutlined style={{ color: '#1890ff' }} />
            <span>集群告警分布</span>
            <Badge 
              count={firingClusters.length} 
              style={{ backgroundColor: firingClusters.length > 0 ? '#ff4d4f' : '#52c41a' }}
              showZero
            />
          </Space>
        }
        bordered={false}
        style={cardStyle}
        bodyStyle={{ padding: '0 16px 16px' }}
      >
        <Table
          columns={clusterColumns}
          dataSource={clustersWithAlerts}
          rowKey="clusterId"
          loading={loading}
          pagination={false}
          size="middle"
          locale={{ emptyText: '暂无已配置告警的集群' }}
          rowClassName={(record) => record.firing > 0 ? 'alert-row-firing' : ''}
        />
      </Card>

      {/* 添加行样式 */}
      <style>{`
        .alert-row-firing {
          background-color: #fff2f0;
        }
        .alert-row-firing:hover > td {
          background-color: #ffebe8 !important;
        }
      `}</style>
    </div>
  );
};

export default GlobalAlertCenter;


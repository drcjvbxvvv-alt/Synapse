import React, { useState } from 'react';
import {
  App, Button, Card, Input, Space, Table, Tag, Tooltip, Typography,
} from 'antd';
import {
  SearchOutlined, SyncOutlined, InfoCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { imageService, type ImageIndex } from '../../services/imageService';

const { Text } = Typography;

const ImageSearch: React.FC = () => {
  const { message } = App.useApp();
  const [query, setQuery] = useState('');
  const [tag, setTag] = useState('');
  const [items, setItems] = useState<ImageIndex[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [syncStatus, setSyncStatus] = useState<{ indexed: number; lastSyncAt: string } | null>(null);
  const [page, setPage] = useState(1);

  const search = async (p = 1) => {
    setLoading(true);
    setPage(p);
    try {
      const res = await imageService.search({ q: query, tag, page: p, limit: 20 });
      setItems(res.data.items || []);
      setTotal(res.data.total);
    } catch {
      message.error('搜尋失敗');
    } finally {
      setLoading(false);
    }
  };

  const loadStatus = async () => {
    try {
      const res = await imageService.syncStatus();
      setSyncStatus(res.data);
    } catch {
      // ignore
    }
  };

  const handleSync = async () => {
    setSyncing(true);
    try {
      const res = await imageService.sync();
      message.success(`同步完成，共索引 ${res.data.indexed} 筆映像`);
      loadStatus();
      if (query || tag) search(1);
    } catch {
      message.error('同步失敗');
    } finally {
      setSyncing(false);
    }
  };

  React.useEffect(() => { loadStatus(); }, []);

  const columns: ColumnsType<ImageIndex> = [
    {
      title: '叢集',
      dataIndex: 'clusterName',
      width: 120,
    },
    {
      title: '命名空間',
      dataIndex: 'namespace',
      width: 140,
    },
    {
      title: '工作負載',
      width: 200,
      render: (_, r) => (
        <Space>
          <Tag color="blue">{r.workloadKind}</Tag>
          <Text>{r.workloadName}</Text>
        </Space>
      ),
    },
    {
      title: '容器',
      dataIndex: 'containerName',
      width: 140,
    },
    {
      title: '映像',
      dataIndex: 'image',
      render: (img: string) => (
        <Text code copyable style={{ fontSize: 12 }}>
          {img}
        </Text>
      ),
    },
    {
      title: 'Tag',
      dataIndex: 'imageTag',
      width: 120,
      render: (tag: string) => <Tag>{tag || 'latest'}</Tag>,
    },
    {
      title: '最後同步',
      dataIndex: 'lastSyncAt',
      width: 160,
      render: (v: string) => new Date(v).toLocaleString('zh-TW'),
    },
  ];

  return (
    <Card
      title={
        <Space>
          <SearchOutlined />
          Image Tag 全域搜尋
          {syncStatus && (
            <Tooltip title={`最後同步：${new Date(syncStatus.lastSyncAt).toLocaleString('zh-TW')}`}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                <InfoCircleOutlined /> 已索引 {syncStatus.indexed} 筆
              </Text>
            </Tooltip>
          )}
        </Space>
      }
      extra={
        <Button
          icon={<SyncOutlined spin={syncing} />}
          onClick={handleSync}
          loading={syncing}
        >
          掃描並同步索引
        </Button>
      }
    >
      <Space style={{ marginBottom: 16, width: '100%' }} wrap>
        <Input
          placeholder="映像名稱（如 nginx）"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onPressEnter={() => search(1)}
          style={{ width: 240 }}
          prefix={<SearchOutlined />}
          allowClear
        />
        <Input
          placeholder="Tag（如 1.21）"
          value={tag}
          onChange={(e) => setTag(e.target.value)}
          onPressEnter={() => search(1)}
          style={{ width: 160 }}
          allowClear
        />
        <Button type="primary" icon={<SearchOutlined />} onClick={() => search(1)}>
          搜尋
        </Button>
      </Space>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={items}
        loading={loading}
        scroll={{ x: 1100 }}
        pagination={{
          current: page,
          total,
          pageSize: 20,
          onChange: (p) => search(p),
          showTotal: (t) => `共 ${t} 筆`,
        }}
      />
    </Card>
  );
};

export default ImageSearch;

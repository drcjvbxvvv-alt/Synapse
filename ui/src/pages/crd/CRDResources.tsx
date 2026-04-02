import React, { useEffect, useState } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  App,
  Button,
  Card,
  Drawer,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import {
  ArrowLeftOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { crdService, type CRDResourceItem } from '../../services/crdService';

const { Text, Paragraph } = Typography;

interface LocationState {
  kind?: string;
  namespaced?: boolean;
}

const CRDResources: React.FC = () => {
  const { clusterId, group, version, plural } = useParams<{
    clusterId: string;
    group: string;
    version: string;
    plural: string;
  }>();
  const location = useLocation();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { message, modal } = App.useApp();

  const state = (location.state ?? {}) as LocationState;
  const kind = state.kind ?? plural;
  const namespaced = state.namespaced ?? true;

  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<CRDResourceItem[]>([]);
  const [search, setSearch] = useState('');
  const [namespace, setNamespace] = useState<string>('');
  const [namespaces, setNamespaces] = useState<string[]>([]);

  // Detail drawer
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailData, setDetailData] = useState<Record<string, unknown> | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const fetchItems = async () => {
    if (!clusterId || !group || !version || !plural) return;
    setLoading(true);
    try {
      const decodedGroup = decodeURIComponent(group);
      const res = await crdService.listCRDResources(clusterId, {
        group: decodedGroup,
        version,
        plural,
        namespace: namespace || undefined,
      });
      const allItems = res.items ?? [];
      setItems(allItems);
      // 收集命名空間清單
      const nsSet = new Set(allItems.map((i) => i.namespace).filter(Boolean) as string[]);
      setNamespaces(Array.from(nsSet).sort());
    } catch {
      message.error(t('crd.loadResourcesFailed', '加载资源失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchItems();
  }, [clusterId, group, version, plural, namespace]);

  const handleView = async (record: CRDResourceItem) => {
    if (!clusterId || !group || !version || !plural) return;
    setDetailVisible(true);
    setDetailLoading(true);
    try {
      const decodedGroup = decodeURIComponent(group);
      const ns = record.namespace ?? '_';
      const data = await crdService.getCRDResource(clusterId, ns, record.name, {
        group: decodedGroup,
        version,
        plural,
      });
      setDetailData(data as Record<string, unknown>);
    } catch {
      message.error(t('crd.loadDetailFailed', '加载详情失败'));
    } finally {
      setDetailLoading(false);
    }
  };

  const handleDelete = (record: CRDResourceItem) => {
    modal.confirm({
      title: t('crd.confirmDelete', '确认删除'),
      content: t('crd.confirmDeleteMsg', `确定要删除 ${record.name} 吗？此操作不可逆。`),
      okType: 'danger',
      onOk: async () => {
        try {
          const decodedGroup = decodeURIComponent(group ?? '');
          const ns = record.namespace ?? '_';
          await crdService.deleteCRDResource(clusterId!, ns, record.name, {
            group: decodedGroup,
            version: version!,
            plural: plural!,
          });
          message.success(t('common.deleteSuccess', '删除成功'));
          fetchItems();
        } catch {
          message.error(t('common.deleteFailed', '删除失败'));
        }
      },
    });
  };

  const filtered = items.filter(
    (i) =>
      !search ||
      i.name.toLowerCase().includes(search.toLowerCase()) ||
      (i.namespace ?? '').toLowerCase().includes(search.toLowerCase())
  );

  const columns: ColumnsType<CRDResourceItem> = [
    {
      title: t('common.name', '名称'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <Text strong>{name}</Text>,
    },
    ...(namespaced
      ? [
          {
            title: t('common.namespace', '命名空间'),
            dataIndex: 'namespace',
            key: 'namespace',
            render: (ns: string) => <Tag color="blue">{ns}</Tag>,
          },
        ]
      : []),
    {
      title: t('common.createdAt', '创建时间'),
      dataIndex: 'created',
      key: 'created',
      render: (v: string) => new Date(v).toLocaleString(),
    },
    {
      title: t('common.labels', '标签'),
      key: 'labels',
      render: (_: unknown, record: CRDResourceItem) =>
        Object.entries(record.labels ?? {})
          .slice(0, 3)
          .map(([k, v]) => (
            <Tag key={k} style={{ marginBottom: 2 }}>
              {k}={v}
            </Tag>
          )),
    },
    {
      title: t('common.actions', '操作'),
      key: 'actions',
      width: 140,
      render: (_: unknown, record: CRDResourceItem) => (
        <Space>
          <Button
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleView(record)}
          >
            {t('common.view', '查看')}
          </Button>
          <Button
            size="small"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record)}
          >
            {t('common.delete', '删除')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Card
        title={
          <Space>
            <Button
              type="text"
              icon={<ArrowLeftOutlined />}
              onClick={() => navigate(`/clusters/${clusterId}/crds`)}
            />
            <Text strong>
              {kind}
              <Text type="secondary" style={{ fontWeight: 400, marginLeft: 8 }}>
                {decodeURIComponent(group ?? '')} / {version}
              </Text>
            </Text>
          </Space>
        }
        extra={
          <Space>
            {namespaced && (
              <Select
                allowClear
                placeholder={t('common.allNamespaces', '所有命名空间')}
                value={namespace || undefined}
                onChange={(v) => setNamespace(v ?? '')}
                style={{ width: 180 }}
              >
                {namespaces.map((ns) => (
                  <Select.Option key={ns} value={ns}>
                    {ns}
                  </Select.Option>
                ))}
              </Select>
            )}
            <Input
              prefix={<SearchOutlined />}
              placeholder={t('common.search', '搜索名称')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              allowClear
              style={{ width: 200 }}
            />
            <ReloadOutlined
              onClick={fetchItems}
              spin={loading}
              style={{ cursor: 'pointer', fontSize: 16 }}
            />
          </Space>
        }
      >
        <Table<CRDResourceItem>
          rowKey={(r) => `${r.namespace ?? '_'}/${r.name}`}
          columns={columns}
          dataSource={filtered}
          loading={loading}
          virtual
          scroll={{ y: 500 }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />
      </Card>

      <Drawer
        title={t('crd.resourceDetail', '资源详情')}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
        width="50%"
        loading={detailLoading}
      >
        {detailData && (
          <Paragraph>
            <pre style={{ fontSize: 12, overflowX: 'auto' }}>
              {JSON.stringify(detailData, null, 2)}
            </pre>
          </Paragraph>
        )}
      </Drawer>
    </>
  );
};

export default CRDResources;

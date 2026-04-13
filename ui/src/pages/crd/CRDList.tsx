import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { usePermission } from '../../hooks/usePermission';
import { App, Card, Collapse, Input, Space, Table, Tag, Typography } from 'antd';
import { ApiOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { crdService, type CRDInfo } from '../../services/crdService';

const { Text } = Typography;

const CRDList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { message } = App.useApp();
  const { canWrite } = usePermission();

  const [loading, setLoading] = useState(false);
  const [crds, setCrds] = useState<CRDInfo[]>([]);
  const [search, setSearch] = useState('');

  const fetchCRDs = useCallback(async () => {
    if (!clusterId) return;
    setLoading(true);
    try {
      const res = await crdService.listCRDs(clusterId);
      setCrds(res.items ?? []);
    } catch {
      message.error(t('crd.loadFailed', '載入 CRD 列表失敗'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  useEffect(() => {
    fetchCRDs();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clusterId]);

  // 按 API Group 分組
  const grouped = useMemo(() => {
    const filtered = crds.filter(
      (c) =>
        !search ||
        c.kind.toLowerCase().includes(search.toLowerCase()) ||
        c.group.toLowerCase().includes(search.toLowerCase()) ||
        c.plural.toLowerCase().includes(search.toLowerCase())
    );
    const map: Record<string, CRDInfo[]> = {};
    for (const crd of filtered) {
      if (!map[crd.group]) map[crd.group] = [];
      map[crd.group].push(crd);
    }
    return Object.entries(map).sort(([a], [b]) => a.localeCompare(b));
  }, [crds, search]);

  const columns: ColumnsType<CRDInfo> = [
    {
      title: t('crd.kind', 'Kind'),
      dataIndex: 'kind',
      key: 'kind',
      width: 200,
      render: (kind: string) => <Text strong>{kind}</Text>,
    },
    {
      title: t('crd.plural', '資源名稱 (Plural)'),
      dataIndex: 'plural',
      key: 'plural',
      width: 200,
    },
    {
      title: t('crd.version', '版本'),
      dataIndex: 'version',
      key: 'version',
      width: 100,
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: t('crd.scope', '範圍'),
      dataIndex: 'namespaced',
      key: 'namespaced',
      width: 120,
      render: (namespaced: boolean) =>
        namespaced ? (
          <Tag color="geekblue">Namespaced</Tag>
        ) : (
          <Tag color="purple">Cluster</Tag>
        ),
    },
    ...(canWrite() ? [{
      title: t('common.actions', '操作'),
      key: 'actions',
      render: (_: unknown, record: CRDInfo) => (
        <a
          onClick={() =>
            navigate(
              `/clusters/${clusterId}/crds/${encodeURIComponent(record.group)}/${record.version}/${record.plural}`,
              { state: { kind: record.kind, namespaced: record.namespaced } }
            )
          }
        >
          {t('crd.viewResources', '檢視例項')}
        </a>
      ),
    }] : []),
  ];

  const collapseItems = grouped.map(([group, items]) => ({
    key: group,
    label: (
      <Space>
        <ApiOutlined />
        <Text strong>{group}</Text>
        <Tag>{items.length}</Tag>
      </Space>
    ),
    children: (
      <Table<CRDInfo>
        rowKey="plural"
        columns={columns}
        dataSource={items}
        pagination={false}
        size="small"
        virtual
        scroll={{ y: 400 }}
      />
    ),
  }));

  return (
    <Card
      title={
        <Space>
          <ApiOutlined />
          {t('crd.title', 'CRD 管理')}
        </Space>
      }
      extra={
        <Space>
          <Input
            prefix={<SearchOutlined />}
            placeholder={t('crd.searchPlaceholder', '搜尋 Kind / Group')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            allowClear
            style={{ width: 220 }}
          />
          <ReloadOutlined
            onClick={fetchCRDs}
            spin={loading}
            style={{ cursor: 'pointer', fontSize: 16 }}
          />
        </Space>
      }
    >
      {grouped.length === 0 && !loading ? (
        <Text type="secondary">
          {search
            ? t('crd.noMatch', '未找到匹配的 CRD')
            : t('crd.noCRD', '該叢集未安裝任何自定義 CRD')}
        </Text>
      ) : (
        <Collapse
          items={collapseItems}
          defaultActiveKey={grouped.map(([g]) => g)}
          size="small"
        />
      )}
    </Card>
  );
};

export default CRDList;

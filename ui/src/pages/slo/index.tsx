import React, { useState, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Switch,
  Tooltip,
  Popconfirm,
  Input,
  Flex,
  App,
  theme,
  Progress,
  Typography,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  SearchOutlined,
  LineChartOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { sloService, type SLO, type SLOStatus } from '../../services/sloService';
import EmptyState from '../../components/EmptyState';
import SLOFormModal from './SLOFormModal';
import SLOStatusDrawer from './SLOStatusDrawer';

const { Text } = Typography;

// ── Status tag helpers ────────────────────────────────────────────────────────

const STATUS_COLOR: Record<string, string> = {
  ok:       'success',
  warning:  'warning',
  critical: 'error',
  unknown:  'default',
};

const STATUS_LABEL: Record<string, string> = {
  ok:       '正常',
  warning:  '警告',
  critical: '嚴重',
  unknown:  '未知',
};

// ── Component ─────────────────────────────────────────────────────────────────

const SLOListPage: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['common']);
  const queryClient = useQueryClient();

  const cid = Number(clusterId ?? 0);

  const [search, setSearch] = useState('');
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<SLO | null>(null);
  const [statusDrawerSLO, setStatusDrawerSLO] = useState<SLO | null>(null);

  // ── Queries ──────────────────────────────────────────────────────────────────

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['slos', cid],
    queryFn: () => sloService.list(cid),
    enabled: cid > 0,
    staleTime: 30_000,
  });

  const items: SLO[] = (data?.items ?? []).filter((s) =>
    s.name.toLowerCase().includes(search.toLowerCase()) ||
    s.namespace.toLowerCase().includes(search.toLowerCase())
  );

  // ── Mutations ────────────────────────────────────────────────────────────────

  const deleteMutation = useMutation({
    mutationFn: (id: number) => sloService.delete(cid, id),
    onSuccess: () => {
      message.success('SLO 已刪除');
      queryClient.invalidateQueries({ queryKey: ['slos', cid] });
    },
    onError: () => message.error('刪除失敗'),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ slo, enabled }: { slo: SLO; enabled: boolean }) =>
      sloService.update(cid, slo.id, { ...slo, enabled }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['slos', cid] }),
    onError: () => message.error('更新失敗'),
  });

  // ── Handlers ─────────────────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    setFormOpen(true);
  }, []);

  const handleEdit = useCallback((slo: SLO) => {
    setEditing(slo);
    setFormOpen(true);
  }, []);

  const handleFormClose = useCallback(() => {
    setFormOpen(false);
    setEditing(null);
  }, []);

  const handleFormSuccess = useCallback(() => {
    handleFormClose();
    queryClient.invalidateQueries({ queryKey: ['slos', cid] });
  }, [handleFormClose, queryClient, cid]);

  // ── Columns ───────────────────────────────────────────────────────────────────

  const columns: TableColumnsType<SLO> = [
    {
      title: '名稱',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      width: 200,
      render: (name: string) => (
        <Text strong style={{ fontSize: token.fontSize }}>{name}</Text>
      ),
    },
    {
      title: '命名空間',
      dataIndex: 'namespace',
      key: 'namespace',
      width: 140,
      render: (ns: string) => ns || <Text type="secondary">叢集層級</Text>,
    },
    {
      title: '類型',
      dataIndex: 'sli_type',
      key: 'sli_type',
      width: 120,
      render: (t: string) => <Tag>{t}</Tag>,
    },
    {
      title: '目標',
      dataIndex: 'target',
      key: 'target',
      width: 110,
      render: (v: number) => (
        <Text style={{ fontVariantNumeric: 'tabular-nums' }}>
          {(v * 100).toFixed(3)}%
        </Text>
      ),
    },
    {
      title: '計算視窗',
      dataIndex: 'window',
      key: 'window',
      width: 90,
      render: (w: string) => <Tag color="blue">{w}</Tag>,
    },
    {
      title: '燃燒率閾值',
      key: 'burn',
      width: 140,
      render: (_: unknown, r: SLO) => (
        <Space size={4}>
          <Tooltip title="警告閾值">
            <Tag color="warning">{r.burn_rate_warning}x</Tag>
          </Tooltip>
          <Tooltip title="嚴重閾值">
            <Tag color="error">{r.burn_rate_critical}x</Tag>
          </Tooltip>
        </Space>
      ),
    },
    {
      title: '啟用',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled: boolean, record: SLO) => (
        <Switch
          size="small"
          checked={enabled}
          loading={toggleMutation.isPending}
          onChange={(checked) => toggleMutation.mutate({ slo: record, enabled: checked })}
        />
      ),
    },
    {
      title: '建立時間',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (v: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(v).format('YYYY-MM-DD HH:mm')}
        </Text>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      fixed: 'right',
      render: (_: unknown, record: SLO) => (
        <Space size={0}>
          <Tooltip title="查看狀態">
            <Button
              type="link"
              size="small"
              icon={<LineChartOutlined />}
              onClick={() => setStatusDrawerSLO(record)}
            />
          </Tooltip>
          <Tooltip title={t('actions.edit')}>
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record)}
            />
          </Tooltip>
          <Popconfirm
            title="確認刪除"
            description={`此操作將永久刪除「${record.name}」，無法復原。`}
            onConfirm={() => deleteMutation.mutate(record.id)}
            okText={t('actions.delete')}
            okButtonProps={{ danger: true }}
            cancelText={t('actions.cancel')}
          >
            <Tooltip title={t('actions.delete')}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // ── Render ────────────────────────────────────────────────────────────────────

  return (
    <div style={{ padding: token.paddingLG }}>
      <Card variant="borderless">
        {/* Toolbar */}
        <Flex
          justify="space-between"
          align="center"
          style={{ marginBottom: token.marginMD }}
        >
          <Space>
            <Input
              prefix={<SearchOutlined />}
              placeholder="搜尋 SLO 名稱或命名空間"
              allowClear
              style={{ width: 260 }}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </Space>
          <Space>
            <Tooltip title={t('actions.refresh')}>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => refetch()}
                loading={isLoading}
              />
            </Tooltip>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              新增 SLO
            </Button>
          </Space>
        </Flex>

        {/* Table */}
        <Table<SLO>
          columns={columns}
          dataSource={items}
          rowKey="id"
          loading={isLoading}
          size="small"
          scroll={{ x: 'max-content' }}
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 筆`,
          }}
          locale={{
            emptyText: <EmptyState description={t('messages.noData')} />,
          }}
        />
      </Card>

      {/* Create / Edit modal */}
      <SLOFormModal
        open={formOpen}
        clusterId={cid}
        slo={editing}
        onClose={handleFormClose}
        onSuccess={handleFormSuccess}
      />

      {/* Status drawer */}
      {statusDrawerSLO && (
        <SLOStatusDrawer
          clusterId={cid}
          slo={statusDrawerSLO}
          open={!!statusDrawerSLO}
          onClose={() => setStatusDrawerSLO(null)}
        />
      )}
    </div>
  );
};

export default SLOListPage;

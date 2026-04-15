/**
 * RolloutList — Argo Rollouts 列表頁（P3-3）
 *
 * 功能：
 *  - 檢查 CRD 是否安裝，未安裝時顯示提示
 *  - 表格列表，支援命名空間篩選與搜尋
 *  - Canary 權重進度條、BlueGreen 策略標籤
 *  - 行內 Promote / PromoteFull / Abort 操作
 *  - 點擊名稱進入 RolloutDetail
 */
import React, { useState, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Table,
  Button,
  Input,
  Select,
  Space,
  Flex,
  Tag,
  Tooltip,
  Popconfirm,
  Progress,
  Typography,
  App,
  theme,
  Alert,
  Spin,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  ReloadOutlined,
  SearchOutlined,
  DeleteOutlined,
  FastForwardOutlined,
  StopOutlined,
  RocketOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import rolloutService, { type RolloutInfo } from '../../services/rolloutService';
import EmptyState from '../../components/EmptyState';

const { Text } = Typography;

// ─── Status tag ───────────────────────────────────────────────────────────────

function RolloutStatusTag({ status }: { status: string }) {
  const { t } = useTranslation('rollout');
  const colorMap: Record<string, string> = {
    Healthy: 'success',
    Stopped: 'default',
    Degraded: 'error',
  };
  return (
    <Tag color={colorMap[status] ?? 'processing'}>
      {t(`rollout:status.${status}`, { defaultValue: status })}
    </Tag>
  );
}

// ─── Canary weight cell ───────────────────────────────────────────────────────

function CanaryWeightCell({ rollout }: { rollout: RolloutInfo }) {
  const { t } = useTranslation('rollout');
  if (rollout.strategy !== 'Canary') return <Text type="secondary">—</Text>;

  const weight = rollout.current_weight ?? 0;
  const stepInfo =
    rollout.current_step_count != null
      ? `${rollout.current_step_index ?? 0}/${rollout.current_step_count}`
      : null;

  return (
    <div style={{ minWidth: 120 }}>
      <Flex align="center" gap={8}>
        <Progress
          percent={weight}
          size="small"
          style={{ flex: 1, marginBottom: 0 }}
          status={weight === 100 ? 'success' : 'active'}
        />
        <Text style={{ fontSize: 12, whiteSpace: 'nowrap' }}>{weight}%</Text>
      </Flex>
      {stepInfo && (
        <Text type="secondary" style={{ fontSize: 11 }}>
          {t('rollout:detail.stepIndex')} {stepInfo}
        </Text>
      )}
    </div>
  );
}

// ─── Main component ───────────────────────────────────────────────────────────

const RolloutList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['rollout', 'common']);
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const cid = Number(clusterId ?? 0);

  const [search, setSearch] = useState('');
  const [namespace, setNamespace] = useState<string>('');

  // ─── CRD check ──────────────────────────────────────────────────────────────

  const { data: crdData, isLoading: crdLoading } = useQuery({
    queryKey: ['rollout-crd', cid],
    queryFn: () => rolloutService.checkCRD(cid),
    enabled: cid > 0,
    staleTime: 60_000,
  });

  // ─── Namespaces ─────────────────────────────────────────────────────────────

  const { data: nsData } = useQuery({
    queryKey: ['rollout-namespaces', cid],
    queryFn: () => rolloutService.getNamespaces(cid),
    enabled: cid > 0 && crdData?.installed === true,
    staleTime: 30_000,
  });

  const namespaceOptions = [
    { label: t('common:search.all', { defaultValue: 'All Namespaces' }), value: '' },
    ...(nsData?.namespaces ?? []).map((ns) => ({ label: ns, value: ns })),
  ];

  // ─── Rollout list ────────────────────────────────────────────────────────────

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['rollouts', cid, namespace, search],
    queryFn: () => rolloutService.list(cid, { namespace: namespace || undefined, search: search || undefined }),
    enabled: cid > 0 && crdData?.installed === true,
    staleTime: 15_000,
  });

  const items = data?.items ?? [];

  // ─── Mutations ───────────────────────────────────────────────────────────────

  const promoteMutation = useMutation({
    mutationFn: ({ ns, name }: { ns: string; name: string }) =>
      rolloutService.promote(cid, ns, name),
    onSuccess: () => {
      message.success(t('rollout:messages.promoteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['rollouts', cid] });
    },
    onError: () => message.error(t('rollout:messages.promoteFailed')),
  });

  const promoteFullMutation = useMutation({
    mutationFn: ({ ns, name }: { ns: string; name: string }) =>
      rolloutService.promoteFull(cid, ns, name),
    onSuccess: () => {
      message.success(t('rollout:messages.promoteFullSuccess'));
      queryClient.invalidateQueries({ queryKey: ['rollouts', cid] });
    },
    onError: () => message.error(t('rollout:messages.promoteFullFailed')),
  });

  const abortMutation = useMutation({
    mutationFn: ({ ns, name }: { ns: string; name: string }) =>
      rolloutService.abort(cid, ns, name),
    onSuccess: () => {
      message.success(t('rollout:messages.abortSuccess'));
      queryClient.invalidateQueries({ queryKey: ['rollouts', cid] });
    },
    onError: () => message.error(t('rollout:messages.abortFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: ({ ns, name }: { ns: string; name: string }) =>
      rolloutService.delete(cid, ns, name),
    onSuccess: () => {
      message.success(t('rollout:messages.deleteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['rollouts', cid] });
    },
    onError: () => message.error(t('rollout:messages.deleteFailed')),
  });

  // ─── Handlers ────────────────────────────────────────────────────────────────

  const handleViewDetail = useCallback(
    (r: RolloutInfo) => navigate(`/clusters/${cid}/rollouts/${r.namespace}/${r.name}`),
    [cid, navigate],
  );

  // ─── Columns ─────────────────────────────────────────────────────────────────

  const columns: TableColumnsType<RolloutInfo> = [
    {
      title: t('rollout:table.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string, record) => (
        <Button
          type="link"
          style={{ padding: 0, fontWeight: 600 }}
          onClick={() => handleViewDetail(record)}
        >
          {name}
        </Button>
      ),
    },
    {
      title: t('rollout:table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 140,
      render: (ns: string) => <Tag>{ns}</Tag>,
    },
    {
      title: t('rollout:table.strategy'),
      dataIndex: 'strategy',
      key: 'strategy',
      width: 110,
      render: (strategy: string) => (
        <Tag color={strategy === 'Canary' ? 'blue' : 'purple'}>
          {t(`rollout:strategy.${strategy}`, { defaultValue: strategy })}
        </Tag>
      ),
    },
    {
      title: t('rollout:table.status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => <RolloutStatusTag status={status} />,
    },
    {
      title: t('rollout:table.replicas'),
      key: 'replicas',
      width: 100,
      render: (_, r) => (
        <Text style={{ fontSize: token.fontSizeSM }}>
          {r.ready_replicas}/{r.replicas}
        </Text>
      ),
    },
    {
      title: t('rollout:table.weight'),
      key: 'weight',
      width: 180,
      render: (_, r) => <CanaryWeightCell rollout={r} />,
    },
    {
      title: t('rollout:table.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (time: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(time).format('YYYY-MM-DD HH:mm')}
        </Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 160,
      fixed: 'right',
      render: (_, record) => (
        <RolloutActions
          record={record}
          onPromote={() => promoteMutation.mutate({ ns: record.namespace, name: record.name })}
          onPromoteFull={() => promoteFullMutation.mutate({ ns: record.namespace, name: record.name })}
          onAbort={() => abortMutation.mutate({ ns: record.namespace, name: record.name })}
          onDelete={() => deleteMutation.mutate({ ns: record.namespace, name: record.name })}
        />
      ),
    },
  ];

  // ─── CRD not installed ───────────────────────────────────────────────────────

  if (crdLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 200 }}>
        <Spin />
      </div>
    );
  }

  if (crdData && !crdData.installed) {
    return (
      <div style={{ marginBottom: token.marginLG }}>
        <Typography.Title level={4} style={{ margin: 0, marginBottom: token.marginXS }}>
          {t('rollout:page.title')}
        </Typography.Title>
        <Text type="secondary">{t('rollout:page.subtitle')}</Text>
        <Alert
          style={{ marginTop: token.marginLG }}
          type="warning"
          showIcon
          message={t('rollout:crd.notInstalled')}
          description={t('rollout:crd.notInstalledDesc')}
        />
      </div>
    );
  }

  // ─── Render ──────────────────────────────────────────────────────────────────

  return (
    <>
      <div style={{ marginBottom: token.marginLG }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('rollout:page.title')}
        </Typography.Title>
        <Text type="secondary">{t('rollout:page.subtitle')}</Text>
      </div>

      <Card variant="borderless">
        {/* Toolbar */}
        <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
          <Space>
            <Input
              prefix={<SearchOutlined />}
              placeholder={t('common:search.placeholder')}
              allowClear
              style={{ width: 220 }}
              onChange={(e) => setSearch(e.target.value)}
            />
            <Select
              style={{ width: 180 }}
              value={namespace}
              onChange={setNamespace}
              options={namespaceOptions}
            />
          </Space>
          <Tooltip title={t('common:actions.refresh')}>
            <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
          </Tooltip>
        </Flex>

        <Table<RolloutInfo>
          columns={columns}
          dataSource={items}
          rowKey={(r) => `${r.namespace}/${r.name}`}
          loading={isLoading}
          size="small"
          scroll={{ x: 'max-content' }}
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => t('common:pagination.total', { total }),
          }}
          locale={{
            emptyText: <EmptyState description={t('common:messages.noData')} />,
          }}
        />
      </Card>
    </>
  );
};

// ─── Row actions ──────────────────────────────────────────────────────────────

interface RolloutActionsProps {
  record: RolloutInfo;
  onPromote: () => void;
  onPromoteFull: () => void;
  onAbort: () => void;
  onDelete: () => void;
}

const RolloutActions: React.FC<RolloutActionsProps> = ({
  record, onPromote, onPromoteFull, onAbort, onDelete,
}) => {
  const { t } = useTranslation(['rollout', 'common']);

  return (
    <Space size={0}>
      {/* Promote */}
      <Popconfirm
        title={t('rollout:confirm.promoteTitle')}
        description={t('rollout:confirm.promoteDesc', { name: record.name })}
        onConfirm={onPromote}
        okText={t('common:actions.confirm')}
        cancelText={t('common:actions.cancel')}
      >
        <Tooltip title={t('rollout:actions.promote')}>
          <Button type="link" size="small" icon={<FastForwardOutlined />} />
        </Tooltip>
      </Popconfirm>

      {/* Promote Full — Canary only */}
      {record.strategy === 'Canary' && (
        <Popconfirm
          title={t('rollout:confirm.promoteFullTitle')}
          description={t('rollout:confirm.promoteFullDesc', { name: record.name })}
          onConfirm={onPromoteFull}
          okText={t('common:actions.confirm')}
          cancelText={t('common:actions.cancel')}
        >
          <Tooltip title={t('rollout:actions.promoteFull')}>
            <Button type="link" size="small" icon={<RocketOutlined />} />
          </Tooltip>
        </Popconfirm>
      )}

      {/* Abort */}
      <Popconfirm
        title={t('rollout:confirm.abortTitle')}
        description={t('rollout:confirm.abortDesc', { name: record.name })}
        onConfirm={onAbort}
        okText={t('common:actions.confirm')}
        cancelText={t('common:actions.cancel')}
        okButtonProps={{ danger: true }}
      >
        <Tooltip title={t('rollout:actions.abort')}>
          <Button type="link" size="small" danger icon={<StopOutlined />} />
        </Tooltip>
      </Popconfirm>

      {/* Delete */}
      <Popconfirm
        title={t('common:confirm.deleteTitle')}
        description={t('common:confirm.deleteDesc', { name: record.name })}
        onConfirm={onDelete}
        okText={t('common:actions.delete')}
        okButtonProps={{ danger: true }}
        cancelText={t('common:actions.cancel')}
      >
        <Tooltip title={t('common:actions.delete')}>
          <Button type="link" size="small" danger icon={<DeleteOutlined />} />
        </Tooltip>
      </Popconfirm>
    </Space>
  );
};

export default RolloutList;

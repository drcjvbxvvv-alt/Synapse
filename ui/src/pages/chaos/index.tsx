import React, { useState, useCallback } from 'react';
import { useParams } from 'react-router-dom';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Tooltip,
  Popconfirm,
  Input,
  Flex,
  App,
  theme,
  Alert,
  Typography,
  Select,
  Tabs,
} from 'antd';
import type { TableColumnsType, TabsProps } from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  DeleteOutlined,
  SearchOutlined,
  ThunderboltOutlined,
  EyeOutlined,
  ScheduleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import {
  chaosService,
  type ChaosExperiment,
  type ChaosSchedule,
  type ChaosKind,
} from '../../services/chaosService';
import EmptyState from '../../components/EmptyState';
import ChaosFormModal from './ChaosFormModal';
import ChaosDetailDrawer from './ChaosDetailDrawer';
import ScheduleFormModal from './ScheduleFormModal';

const { Text } = Typography;

// ── Phase / kind colour maps ──────────────────────────────────────────────────

const PHASE_COLOR: Record<string, string> = {
  Running:   'processing',
  Injecting: 'processing',
  Waiting:   'warning',
  Paused:    'default',
  Finished:  'success',
  Failed:    'error',
  Stopped:   'default',
};

const KIND_COLOR: Record<ChaosKind, string> = {
  PodChaos:     'volcano',
  NetworkChaos: 'geekblue',
  StressChaos:  'purple',
  HTTPChaos:    'cyan',
  IOChaos:      'gold',
};

// ── Experiments tab ───────────────────────────────────────────────────────────

interface ExperimentsTabProps {
  clusterId: string;
  installed: boolean;
  onCreateClick: () => void;
}

const ExperimentsTab: React.FC<ExperimentsTabProps> = ({ clusterId, installed, onCreateClick }) => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['common']);
  const queryClient = useQueryClient();

  const [searchText, setSearchText] = useState('');
  const [nsFilter, setNsFilter]     = useState('');
  const [kindFilter, setKindFilter] = useState('');
  const [detailItem, setDetailItem] = useState<ChaosExperiment | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['chaos-experiments', clusterId, nsFilter],
    queryFn: () => chaosService.listExperiments(clusterId, nsFilter || undefined),
    enabled: !!clusterId,
    staleTime: 15_000,
  });

  const experiments: ChaosExperiment[] = data?.items ?? [];

  const deleteMutation = useMutation({
    mutationFn: (exp: ChaosExperiment) =>
      chaosService.deleteExperiment(clusterId, exp.namespace, exp.kind, exp.name),
    onSuccess: () => {
      message.success(t('common:messages.success'));
      queryClient.invalidateQueries({ queryKey: ['chaos-experiments', clusterId] });
    },
    onError: () => message.error(t('common:messages.failed')),
  });

  const handleView = useCallback((exp: ChaosExperiment) => {
    setDetailItem(exp);
    setDrawerOpen(true);
  }, []);

  const filtered = experiments.filter((e: ChaosExperiment) => {
    const matchName = e.name.toLowerCase().includes(searchText.toLowerCase());
    const matchKind = kindFilter ? e.kind === kindFilter : true;
    return matchName && matchKind;
  });

  const nsOptions = Array.from(new Set(experiments.map((e: ChaosExperiment) => e.namespace))).map(
    (ns) => ({ label: ns, value: ns }),
  );

  const activeCount = experiments.filter(
    (e: ChaosExperiment) => e.phase === 'Running' || e.phase === 'Injecting',
  ).length;

  const columns: TableColumnsType<ChaosExperiment> = [
    {
      title: '名稱',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string, record) => (
        <Button type="link" style={{ padding: 0 }} onClick={() => handleView(record)}>
          {name}
        </Button>
      ),
    },
    {
      title: 'Namespace',
      dataIndex: 'namespace',
      key: 'namespace',
      width: 150,
    },
    {
      title: '類型',
      dataIndex: 'kind',
      key: 'kind',
      width: 130,
      render: (kind: ChaosKind) => (
        <Tag color={KIND_COLOR[kind] ?? 'default'}>{kind}</Tag>
      ),
    },
    {
      title: '狀態',
      dataIndex: 'phase',
      key: 'phase',
      width: 120,
      render: (phase: string) => (
        <Tag color={PHASE_COLOR[phase] ?? 'default'}>{phase || '—'}</Tag>
      ),
    },
    {
      title: '持續時間',
      dataIndex: 'duration',
      key: 'duration',
      width: 110,
      render: (dur: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{dur || '—'}</Text>
      ),
    },
    {
      title: '建立時間',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (ts: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '—'}
        </Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 100,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Tooltip title="查看詳情">
            <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handleView(record)} />
          </Tooltip>
          <Popconfirm
            title={t('common:confirm.deleteTitle')}
            description={t('common:confirm.deleteDesc', { name: record.name })}
            onConfirm={() => deleteMutation.mutate(record)}
            okText={t('common:actions.delete')}
            okButtonProps={{ danger: true }}
            cancelText={t('common:actions.cancel')}
          >
            <Tooltip title={t('common:actions.delete')}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <>
      {activeCount > 0 && (
        <Alert
          type="warning"
          showIcon
          icon={<ThunderboltOutlined />}
          message={`${activeCount} 個實驗注入中`}
          description="目前有混沌實驗正在注入故障，相關 Namespace 的 SLO 告警已自動暫停。"
          style={{ marginBottom: token.marginMD }}
        />
      )}

      <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
        <Space>
          <Input
            prefix={<SearchOutlined />}
            placeholder="搜尋實驗名稱"
            allowClear
            style={{ width: 220 }}
            onChange={(e) => setSearchText(e.target.value)}
          />
          <Select
            placeholder="Namespace"
            allowClear
            style={{ width: 160 }}
            options={nsOptions}
            onChange={(v: string | undefined) => setNsFilter(v ?? '')}
          />
          <Select
            placeholder="類型"
            allowClear
            style={{ width: 150 }}
            options={[
              { label: 'PodChaos',     value: 'PodChaos' },
              { label: 'NetworkChaos', value: 'NetworkChaos' },
              { label: 'StressChaos',  value: 'StressChaos' },
              { label: 'HTTPChaos',    value: 'HTTPChaos' },
              { label: 'IOChaos',      value: 'IOChaos' },
            ]}
            onChange={(v: string | undefined) => setKindFilter(v ?? '')}
          />
        </Space>
        <Space>
          <Tooltip title={t('common:actions.refresh')}>
            <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
          </Tooltip>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={onCreateClick}
            disabled={!installed}
          >
            建立實驗
          </Button>
        </Space>
      </Flex>

      <Table<ChaosExperiment>
        columns={columns}
        dataSource={filtered}
        rowKey="uid"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (total) => t('common:pagination.total', { total }),
        }}
        locale={{
          emptyText: (
            <EmptyState
              description={
                !installed
                  ? 'Chaos Mesh 未安裝，無法列出實驗'
                  : t('common:messages.noData')
              }
            />
          ),
        }}
      />

      <ChaosDetailDrawer
        open={drawerOpen}
        experiment={detailItem}
        clusterId={clusterId}
        onClose={() => { setDrawerOpen(false); setDetailItem(null); }}
      />
    </>
  );
};

// ── Schedules tab ─────────────────────────────────────────────────────────────

interface SchedulesTabProps {
  clusterId: string;
  installed: boolean;
  onCreateClick: () => void;
}

const SchedulesTab: React.FC<SchedulesTabProps> = ({ clusterId, installed, onCreateClick }) => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['common']);
  const queryClient = useQueryClient();

  const [searchText, setSearchText] = useState('');

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['chaos-schedules', clusterId],
    queryFn: () => chaosService.listSchedules(clusterId),
    enabled: !!clusterId,
    staleTime: 30_000,
  });

  const schedules: ChaosSchedule[] = data?.items ?? [];

  const deleteMutation = useMutation({
    mutationFn: (s: ChaosSchedule) =>
      chaosService.deleteSchedule(clusterId, s.namespace, s.name),
    onSuccess: () => {
      message.success(t('common:messages.success'));
      queryClient.invalidateQueries({ queryKey: ['chaos-schedules', clusterId] });
    },
    onError: () => message.error(t('common:messages.failed')),
  });

  const filtered = schedules.filter((s: ChaosSchedule) =>
    s.name.toLowerCase().includes(searchText.toLowerCase()),
  );

  const columns: TableColumnsType<ChaosSchedule> = [
    {
      title: '名稱',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
    },
    {
      title: 'Namespace',
      dataIndex: 'namespace',
      key: 'namespace',
      width: 150,
    },
    {
      title: 'Cron',
      dataIndex: 'cron_expr',
      key: 'cron_expr',
      width: 160,
      render: (v: string) => <Text code style={{ fontSize: token.fontSizeSM }}>{v || '—'}</Text>,
    },
    {
      title: '類型',
      dataIndex: 'type',
      key: 'type',
      width: 130,
      render: (kind: string) => (
        <Tag color={KIND_COLOR[kind as ChaosKind] ?? 'default'}>{kind || '—'}</Tag>
      ),
    },
    {
      title: '狀態',
      dataIndex: 'suspended',
      key: 'suspended',
      width: 80,
      render: (v: boolean) =>
        v ? <Tag color="warning">已暫停</Tag> : <Tag color="success">啟用</Tag>,
    },
    {
      title: '上次執行',
      dataIndex: 'last_run_time',
      key: 'last_run_time',
      width: 160,
      render: (ts: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '—'}
        </Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 80,
      fixed: 'right',
      render: (_, record) => (
        <Popconfirm
          title={t('common:confirm.deleteTitle')}
          description={t('common:confirm.deleteDesc', { name: record.name })}
          onConfirm={() => deleteMutation.mutate(record)}
          okText={t('common:actions.delete')}
          okButtonProps={{ danger: true }}
          cancelText={t('common:actions.cancel')}
        >
          <Tooltip title={t('common:actions.delete')}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />} />
          </Tooltip>
        </Popconfirm>
      ),
    },
  ];

  return (
    <>
      <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
        <Input
          prefix={<SearchOutlined />}
          placeholder="搜尋排程名稱"
          allowClear
          style={{ width: 240 }}
          onChange={(e) => setSearchText(e.target.value)}
        />
        <Space>
          <Tooltip title={t('common:actions.refresh')}>
            <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
          </Tooltip>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={onCreateClick}
            disabled={!installed}
          >
            建立排程
          </Button>
        </Space>
      </Flex>

      <Table<ChaosSchedule>
        columns={columns}
        dataSource={filtered}
        rowKey="name"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (total) => t('common:pagination.total', { total }),
        }}
        locale={{
          emptyText: (
            <EmptyState
              description={!installed ? 'Chaos Mesh 未安裝' : t('common:messages.noData')}
            />
          ),
        }}
      />
    </>
  );
};

// ── Page ──────────────────────────────────────────────────────────────────────

const ChaosPage: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { token } = theme.useToken();
  const queryClient = useQueryClient();

  const [expFormOpen, setExpFormOpen]     = useState(false);
  const [schedFormOpen, setSchedFormOpen] = useState(false);

  const { data: status } = useQuery({
    queryKey: ['chaos-status', clusterId],
    queryFn: () => chaosService.getStatus(clusterId!),
    enabled: !!clusterId,
    staleTime: 60_000,
  });

  const installed = status?.installed ?? true;

  const tabs: TabsProps['items'] = [
    {
      key: 'experiments',
      label: (
        <span>
          <ThunderboltOutlined />
          {' '}實驗
        </span>
      ),
      children: (
        <ExperimentsTab
          clusterId={clusterId!}
          installed={installed}
          onCreateClick={() => setExpFormOpen(true)}
        />
      ),
    },
    {
      key: 'schedules',
      label: (
        <span>
          <ScheduleOutlined />
          {' '}排程
        </span>
      ),
      children: (
        <SchedulesTab
          clusterId={clusterId!}
          installed={installed}
          onCreateClick={() => setSchedFormOpen(true)}
        />
      ),
    },
  ];

  return (
    <div style={{ padding: token.paddingLG }}>
      <div style={{ marginBottom: token.marginLG }}>
        <div style={{ fontSize: token.fontSizeLG, fontWeight: 600, color: token.colorText }}>
          混沌工程
        </div>
        <div style={{ color: token.colorTextSecondary, fontSize: token.fontSizeSM, marginTop: 4 }}>
          透過 Chaos Mesh 注入故障，驗證系統韌性
        </div>
      </div>

      {status && !status.installed && (
        <Alert
          type="warning"
          showIcon
          icon={<ThunderboltOutlined />}
          message="Chaos Mesh 未安裝"
          description="此叢集尚未安裝 Chaos Mesh。請先安裝才能建立混沌實驗。"
          style={{ marginBottom: token.marginMD }}
        />
      )}

      <Card variant="borderless">
        <Tabs items={tabs} />
      </Card>

      <ChaosFormModal
        open={expFormOpen}
        clusterId={clusterId!}
        onClose={() => setExpFormOpen(false)}
        onSuccess={() => {
          setExpFormOpen(false);
          queryClient.invalidateQueries({ queryKey: ['chaos-experiments', clusterId] });
        }}
      />

      <ScheduleFormModal
        open={schedFormOpen}
        clusterId={clusterId!}
        onClose={() => setSchedFormOpen(false)}
        onSuccess={() => {
          setSchedFormOpen(false);
          queryClient.invalidateQueries({ queryKey: ['chaos-schedules', clusterId] });
        }}
      />
    </div>
  );
};

export default ChaosPage;

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
import { usePermission } from '../../hooks/usePermission';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import {
  chaosService,
  type ChaosExperiment,
  type ChaosSchedule,
  type ChaosKind,
} from '../../services/chaosService';
import EmptyState from '../../components/EmptyState';
import NotInstalledCard from '../../components/NotInstalledCard';
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
  const { t } = useTranslation(['chaos', 'common']);
  const { canWrite } = usePermission();
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
      title: t('chaos:table.name'),
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
      title: t('chaos:table.kind'),
      dataIndex: 'kind',
      key: 'kind',
      width: 130,
      render: (kind: ChaosKind) => (
        <Tag color={KIND_COLOR[kind] ?? 'default'}>{kind}</Tag>
      ),
    },
    {
      title: t('chaos:table.phase'),
      dataIndex: 'phase',
      key: 'phase',
      width: 120,
      render: (phase: string) => (
        <Tag color={PHASE_COLOR[phase] ?? 'default'}>{phase || '—'}</Tag>
      ),
    },
    {
      title: t('chaos:table.duration'),
      dataIndex: 'duration',
      key: 'duration',
      width: 110,
      render: (dur: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{dur || '—'}</Text>
      ),
    },
    {
      title: t('chaos:table.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (ts: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '—'}
        </Text>
      ),
    },
    ...(canWrite() ? [{
      title: t('common:table.actions'),
      key: 'actions',
      width: 100,
      fixed: 'right' as const,
      render: (_: unknown, record: ChaosExperiment) => (
        <Space size={0}>
          <Tooltip title={t('chaos:actions.viewDetail')}>
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
    }] : []),
  ];

  return (
    <>
      {activeCount > 0 && (
        <Alert
          type="warning"
          showIcon
          icon={<ThunderboltOutlined />}
          message={t('chaos:injectingAlert.message', { count: activeCount })}
          description={t('chaos:injectingAlert.desc')}
          style={{ marginBottom: token.marginMD }}
        />
      )}

      <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
        <Space>
          <Input
            prefix={<SearchOutlined />}
            placeholder={t('chaos:search.experimentPlaceholder')}
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
            placeholder={t('chaos:search.kindPlaceholder')}
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
            {t('chaos:actions.createExperiment')}
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
                  ? t('chaos:notInstalled.emptyExperiments')
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
  const { t } = useTranslation(['chaos', 'common']);
  const { canWrite } = usePermission();
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
      title: t('chaos:table.name'),
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
      title: t('chaos:table.kind'),
      dataIndex: 'type',
      key: 'type',
      width: 130,
      render: (kind: string) => (
        <Tag color={KIND_COLOR[kind as ChaosKind] ?? 'default'}>{kind || '—'}</Tag>
      ),
    },
    {
      title: t('chaos:table.phase'),
      dataIndex: 'suspended',
      key: 'suspended',
      width: 80,
      render: (v: boolean) =>
        v
          ? <Tag color="warning">{t('chaos:table.suspended')}</Tag>
          : <Tag color="success">{t('chaos:table.active')}</Tag>,
    },
    {
      title: t('chaos:table.lastRun'),
      dataIndex: 'last_run_time',
      key: 'last_run_time',
      width: 160,
      render: (ts: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '—'}
        </Text>
      ),
    },
    ...(canWrite() ? [{
      title: t('common:table.actions'),
      key: 'actions',
      width: 80,
      fixed: 'right' as const,
      render: (_: unknown, record: ChaosSchedule) => (
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
    }] : []),
  ];

  return (
    <>
      <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
        <Input
          prefix={<SearchOutlined />}
          placeholder={t('chaos:search.schedulePlaceholder')}
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
            {t('chaos:actions.createSchedule')}
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
              description={!installed ? t('chaos:notInstalled.emptySchedules') : t('common:messages.noData')}
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
  const { t } = useTranslation(['chaos', 'common']);
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
          {' '}{t('chaos:tabs.experiments')}
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
          {' '}{t('chaos:tabs.schedules')}
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
          {t('chaos:page.title')}
        </div>
        <div style={{ color: token.colorTextSecondary, fontSize: token.fontSizeSM, marginTop: 4 }}>
          {t('chaos:page.subtitle')}
        </div>
      </div>

      {status && !status.installed ? (
        <NotInstalledCard
          title={t('chaos:notInstalled.title')}
          description={t('chaos:notInstalled.desc')}
          command="helm install chaos-mesh chaos-mesh/chaos-mesh --namespace=chaos-mesh --create-namespace"
          docsUrl="https://chaos-mesh.org/docs/production-installation-using-helm/"
        />
      ) : (
        <Card variant="borderless">
          <Tabs items={tabs} />
        </Card>
      )}

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

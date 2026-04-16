/**
 * CIEngineRunViewer — Drawer for viewing a CI engine run (M19c).
 *
 * Shows run phase, logs (polling every 3 s while running), steps table,
 * and artifacts tab. Cancel button available while run is active.
 */
import React, { useCallback, useState } from 'react';
import {
  Drawer,
  Tabs,
  Table,
  Tag,
  Button,
  Popconfirm,
  Space,
  Tooltip,
  Spin,
  Typography,
  App,
  theme,
  Flex,
  Badge,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  StopOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import ciEngineService, {
  type RunStatus,
  type StepStatus,
  type Artifact,
  type RunPhase,
  RUN_PHASE_TERMINAL,
} from '../../../services/ciEngineService';
import EmptyState from '../../../components/EmptyState';

const { Text } = Typography;

// ─── Terminal-style log display (hardcoded dark theme per CLAUDE.md §1.4) ───

const TERMINAL_COLORS = {
  bg: '#1e1e1e',
  text: '#d4d4d4',
} as const;

// ─── Phase tag helper ────────────────────────────────────────────────────────

const PHASE_TAG_COLOR: Record<RunPhase, string> = {
  pending:   'default',
  running:   'processing',
  success:   'success',
  failed:    'error',
  cancelled: 'warning',
  unknown:   'default',
};

function PhaseTag({ phase, t }: { phase: RunPhase; t: (k: string) => string }) {
  return (
    <Tag color={PHASE_TAG_COLOR[phase] ?? 'default'}>
      {t(`cicd:ciEngine.runViewer.phase.${phase}`)}
    </Tag>
  );
}

// ─── Props ───────────────────────────────────────────────────────────────────

export interface CIEngineRunViewerProps {
  open: boolean;
  engineId: number;
  engineName: string;
  runId: string;
  onClose: () => void;
}

// ─── Component ───────────────────────────────────────────────────────────────

const CIEngineRunViewer: React.FC<CIEngineRunViewerProps> = ({
  open,
  engineId,
  engineName,
  runId,
  onClose,
}) => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['cicd', 'common']);
  const queryClient = useQueryClient();

  const [activeTab, setActiveTab] = useState('logs');

  // ── Run status — poll while running ────────────────────────────────────────

  const { data: runStatus, isLoading: statusLoading } = useQuery<RunStatus>({
    queryKey: ['ci-engine-run', engineId, runId],
    queryFn: () => ciEngineService.getRun(engineId, runId),
    enabled: open && !!runId,
    refetchInterval: (query) => {
      const phase = query.state.data?.phase;
      return phase && RUN_PHASE_TERMINAL.has(phase) ? false : 3_000;
    },
    staleTime: 0,
  });

  const isTerminal = runStatus?.phase ? RUN_PHASE_TERMINAL.has(runStatus.phase) : false;

  // ── Logs — poll while running ───────────────────────────────────────────────

  const { data: logContent, isLoading: logsLoading, refetch: refetchLogs } = useQuery<string>({
    queryKey: ['ci-engine-logs', engineId, runId],
    queryFn: () => ciEngineService.fetchLogs(engineId, runId),
    enabled: open && !!runId && activeTab === 'logs',
    refetchInterval: isTerminal ? false : 3_000,
    staleTime: 0,
  });

  // ── Artifacts ───────────────────────────────────────────────────────────────

  const { data: artifactsData, isLoading: artifactsLoading } = useQuery({
    queryKey: ['ci-engine-artifacts', engineId, runId],
    queryFn: () => ciEngineService.getArtifacts(engineId, runId),
    enabled: open && !!runId && activeTab === 'artifacts',
    staleTime: 15_000,
  });

  // ── Cancel ──────────────────────────────────────────────────────────────────

  const cancelMutation = useMutation({
    mutationFn: () => ciEngineService.cancelRun(engineId, runId),
    onSuccess: () => {
      message.success(t('cicd:ciEngine.runViewer.cancelSuccess'));
      queryClient.invalidateQueries({ queryKey: ['ci-engine-run', engineId, runId] });
    },
    onError: () => message.error(t('cicd:ciEngine.runViewer.cancelFailed')),
  });

  // ── Columns ─────────────────────────────────────────────────────────────────

  const stepColumns: TableColumnsType<StepStatus> = [
    {
      title: t('cicd:ciEngine.runViewer.steps.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
    },
    {
      title: t('cicd:ciEngine.runViewer.steps.phase'),
      dataIndex: 'phase',
      key: 'phase',
      width: 110,
      render: (p: RunPhase) => <PhaseTag phase={p} t={t} />,
    },
    {
      title: t('cicd:ciEngine.runViewer.steps.startedAt'),
      dataIndex: 'started_at',
      key: 'started_at',
      width: 145,
      render: (v?: string) => v
        ? <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{dayjs(v).format('HH:mm:ss')}</Text>
        : <Text type="secondary">—</Text>,
    },
    {
      title: t('cicd:ciEngine.runViewer.steps.finishedAt'),
      dataIndex: 'finished_at',
      key: 'finished_at',
      width: 145,
      render: (v?: string) => v
        ? <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{dayjs(v).format('HH:mm:ss')}</Text>
        : <Text type="secondary">—</Text>,
    },
  ];

  const artifactColumns: TableColumnsType<Artifact> = [
    {
      title: t('cicd:ciEngine.runViewer.artifacts.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
    },
    {
      title: t('cicd:ciEngine.runViewer.artifacts.kind'),
      dataIndex: 'kind',
      key: 'kind',
      width: 120,
      render: (k: string) => <Tag>{k}</Tag>,
    },
    {
      title: t('cicd:ciEngine.runViewer.artifacts.size'),
      dataIndex: 'size_bytes',
      key: 'size_bytes',
      width: 100,
      render: (n?: number) => n != null
        ? <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{formatBytes(n)}</Text>
        : <Text type="secondary">—</Text>,
    },
    {
      title: t('cicd:ciEngine.runViewer.artifacts.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 145,
      render: (v: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(v).format('YYYY-MM-DD HH:mm')}
        </Text>
      ),
    },
  ];

  // ── Render helpers ───────────────────────────────────────────────────────────

  const renderLogs = useCallback(() => {
    if (logsLoading && !logContent) {
      return (
        <Flex justify="center" style={{ padding: token.paddingLG }}>
          <Spin />
        </Flex>
      );
    }
    if (!logContent) {
      return <EmptyState description={t('cicd:ciEngine.runViewer.noLogs')} />;
    }
    return (
      <div
        style={{
          background: TERMINAL_COLORS.bg,
          color: TERMINAL_COLORS.text,
          fontFamily: 'monospace',
          fontSize: token.fontSizeSM,
          padding: token.padding,
          borderRadius: token.borderRadius,
          overflowX: 'auto',
          overflowY: 'auto',
          maxHeight: 480,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          lineHeight: 1.6,
        }}
      >
        {logContent}
      </div>
    );
  }, [logContent, logsLoading, t, token]);

  const renderSteps = useCallback(() => {
    const steps = runStatus?.steps ?? [];
    return (
      <Table<StepStatus>
        columns={stepColumns}
        dataSource={steps}
        rowKey="name"
        size="small"
        pagination={false}
        loading={statusLoading}
        locale={{ emptyText: <EmptyState description={t('cicd:ciEngine.runViewer.noSteps')} /> }}
      />
    );
  }, [runStatus?.steps, statusLoading, stepColumns, t]);

  const renderArtifacts = useCallback(() => {
    const arts = artifactsData?.items ?? [];
    return (
      <Table<Artifact>
        columns={artifactColumns}
        dataSource={arts}
        rowKey="name"
        size="small"
        pagination={false}
        loading={artifactsLoading}
        locale={{ emptyText: <EmptyState description={t('cicd:ciEngine.runViewer.noArtifacts')} /> }}
      />
    );
  }, [artifactsData?.items, artifactsLoading, artifactColumns, t]);

  // ── Drawer title ─────────────────────────────────────────────────────────────

  const drawerTitle = (
    <Flex align="center" gap={token.marginSM}>
      <span>{t('cicd:ciEngine.runViewer.title', { runId })}</span>
      {runStatus && <PhaseTag phase={runStatus.phase} t={t} />}
      {!isTerminal && !statusLoading && (
        <Badge status="processing" />
      )}
    </Flex>
  );

  const drawerExtra = (
    <Space>
      {!isTerminal && (
        <Popconfirm
          title={t('cicd:ciEngine.runViewer.cancelConfirm')}
          onConfirm={() => cancelMutation.mutate()}
          okText={t('common:actions.confirm')}
          cancelText={t('common:actions.cancel')}
          okButtonProps={{ danger: true }}
        >
          <Button
            danger
            icon={<StopOutlined />}
            loading={cancelMutation.isPending}
            size="small"
          >
            {t('cicd:ciEngine.runViewer.cancel')}
          </Button>
        </Popconfirm>
      )}
      <Tooltip title={t('common:actions.refresh')}>
        <Button
          icon={<ReloadOutlined />}
          size="small"
          onClick={() => {
            queryClient.invalidateQueries({ queryKey: ['ci-engine-run', engineId, runId] });
            if (activeTab === 'logs') refetchLogs();
          }}
        />
      </Tooltip>
    </Space>
  );

  const tabItems = [
    {
      key: 'logs',
      label: t('cicd:ciEngine.runViewer.tabs.logs'),
      children: renderLogs(),
    },
    {
      key: 'steps',
      label: t('cicd:ciEngine.runViewer.tabs.steps'),
      children: renderSteps(),
    },
    {
      key: 'artifacts',
      label: t('cicd:ciEngine.runViewer.tabs.artifacts'),
      children: renderArtifacts(),
    },
  ];

  return (
    <Drawer
      title={drawerTitle}
      open={open}
      onClose={onClose}
      width={800}
      extra={drawerExtra}
      destroyOnHidden
    >
      {statusLoading && !runStatus ? (
        <Flex justify="center" style={{ padding: token.paddingXL }}>
          <Spin size="large" />
        </Flex>
      ) : (
        <>
          {runStatus?.message && (
            <Text
              type={runStatus.phase === 'failed' ? 'danger' : 'secondary'}
              style={{ display: 'block', marginBottom: token.marginMD, fontSize: token.fontSizeSM }}
            >
              {runStatus.message}
            </Text>
          )}
          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            items={tabItems}
            size="small"
          />
        </>
      )}
    </Drawer>
  );
};

export default CIEngineRunViewer;

// ─── Utils ───────────────────────────────────────────────────────────────────

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

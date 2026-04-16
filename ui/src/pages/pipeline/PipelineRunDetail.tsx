/**
 * PipelineRunDetail — Pipeline Run 詳情頁（P3-2）
 *
 * 功能：
 *  - ReactFlow DAG 顯示真實 StepRun 狀態（由 API 驅動）
 *  - 活躍 Run 自動 3s 輪詢刷新
 *  - 點擊步驟節點開啟 StepLogViewer（SSE 串流日誌）
 *  - 取消 / 重跑操作
 *  - Breadcrumb 導航回 Pipeline 列表
 */
import React, { useState, useCallback, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  ReactFlow,
  Background,
  Controls,
  BackgroundVariant,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  Button,
  Space,
  Tag,
  Breadcrumb,
  Descriptions,
  App,
  theme,
  Spin,
  Typography,
  Flex,
  Tooltip,
  Popconfirm,
  Modal,
  Input,
} from 'antd';
import type { Node, Edge } from '@xyflow/react';
import {
  ArrowLeftOutlined,
  ReloadOutlined,
  StopOutlined,
  RedoOutlined,
  LoadingOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  AuditOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import pipelineService, { type StepRun, type PipelineRun } from '../../services/pipelineService';
import RunPipelineNode, { toVisualStatus, type RunPipelineNodeData } from './components/RunPipelineNode';
import StepLogViewer from './components/StepLogViewer';
import EmptyState from '../../components/EmptyState';

const { Text, Title } = Typography;

// ─── Node types ───────────────────────────────────────────────────────────────

const NODE_TYPES = { runStep: RunPipelineNode };
const X_GAP = 210;
const Y_CENTER = 120;

// ─── Graph helpers ────────────────────────────────────────────────────────────

function buildRunNodes(
  steps: StepRun[],
  onClickStep: (sr: StepRun) => void,
): Node<RunPipelineNodeData>[] {
  return [...steps]
    .sort((a, b) => a.step_index - b.step_index)
    .map((sr, i) => ({
      id: String(sr.id),
      type: 'runStep',
      position: { x: 60 + i * X_GAP, y: Y_CENTER },
      data: { stepRun: sr, onClick: onClickStep },
    }));
}

function buildRunEdges(steps: StepRun[]): Edge[] {
  const sorted = [...steps].sort((a, b) => a.step_index - b.step_index);
  return sorted.slice(0, -1).map((sr, i) => {
    const next = sorted[i + 1];
    const srcVis = toVisualStatus(sr.status);
    const dstVis = toVisualStatus(next.status);
    const isActive = dstVis === 'running';
    const isOk     = dstVis === 'success' || dstVis === 'skipped';
    const isFailed = srcVis === 'failed';
    return {
      id: `${sr.id}->${next.id}`,
      source: String(sr.id),
      target: String(next.id),
      animated: isActive,
      style: {
        stroke: isFailed ? '#ef4444' : isOk ? '#22c55e' : isActive ? '#3b82f6' : '#cbd5e1',
        strokeWidth: isOk || isActive ? 2.5 : 1.5,
        strokeDasharray: isFailed ? '6 4' : undefined,
        transition: 'stroke 0.4s ease, stroke-width 0.4s ease',
      },
    };
  });
}

// ─── Status tag ───────────────────────────────────────────────────────────────

function RunStatusTag({ status }: { status: PipelineRun['status'] }) {
  const { t } = useTranslation('pipeline');
  switch (status) {
    case 'queued':           return <Tag color="default"     icon={<ClockCircleOutlined />}>{t('run.status.queued')}</Tag>;
    case 'running':          return <Tag color="processing"  icon={<LoadingOutlined spin />}>{t('run.status.running')}</Tag>;
    case 'success':          return <Tag color="success"     icon={<CheckCircleOutlined />}>{t('run.status.success')}</Tag>;
    case 'failed':           return <Tag color="error"       icon={<CloseCircleOutlined />}>{t('run.status.failed')}</Tag>;
    case 'cancelled':        return <Tag color="warning">{t('run.status.cancelled')}</Tag>;
    case 'waiting_approval': return <Tag color="gold">{t('run.status.waiting_approval')}</Tag>;
    default:                 return <Tag>{status}</Tag>;
  }
}

// ─── Duration helper ──────────────────────────────────────────────────────────

function formatDuration(startedAt: string | null, finishedAt: string | null): string {
  if (!startedAt) return '—';
  const start = new Date(startedAt).getTime();
  const end = finishedAt ? new Date(finishedAt).getTime() : Date.now();
  const ms = end - start;
  if (ms < 60_000) return `${(ms / 1000).toFixed(0)}s`;
  const m = Math.floor(ms / 60_000);
  const s = Math.floor((ms % 60_000) / 1000);
  return `${m}m ${s}s`;
}

// ─── Main component ───────────────────────────────────────────────────────────

const PipelineRunDetail: React.FC = () => {
  const { pipelineId, runId } = useParams<{
    pipelineId: string;
    runId: string;
  }>();
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['pipeline', 'common']);
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const pid = Number(pipelineId ?? 0);
  const rid = Number(runId ?? 0);

  // ─── State ──────────────────────────────────────────────────────────────────

  const [selectedStep, setSelectedStep] = useState<StepRun | null>(null);
  const [logDrawerOpen, setLogDrawerOpen] = useState(false);
  const [rejectModalOpen, setRejectModalOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState('');

  // ─── Query ──────────────────────────────────────────────────────────────────

  const isActive = useCallback((run?: PipelineRun) =>
    run?.status === 'queued' || run?.status === 'running' || run?.status === 'waiting_approval',
    [],
  );

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['pipeline-run', pid, rid],
    queryFn: () => pipelineService.getRun(pid, rid),
    enabled: pid > 0 && rid > 0,
    refetchInterval: (query) =>
      isActive(query.state.data?.run) ? 3000 : false,
    staleTime: 2000,
  });

  const run = data?.run;
  const steps = data?.steps ?? [];

  // ─── ReactFlow graph ─────────────────────────────────────────────────────────

  const handleClickStep = useCallback((sr: StepRun) => {
    setSelectedStep(sr);
    setLogDrawerOpen(true);
  }, []);

  // Nodes and edges are server-driven (nodesDraggable=false).
  // Rebuild on every query refresh — no ReactFlow internal state needed.
  const syncedNodes = useMemo(
    () => buildRunNodes(steps, handleClickStep),
    [steps, handleClickStep],
  );
  const syncedEdges = useMemo(() => buildRunEdges(steps), [steps]);

  // ─── SSE log URL ─────────────────────────────────────────────────────────────

  const logUrl = selectedStep
    ? pipelineService.getStepLogUrl(pid, rid, selectedStep.id)
    : null;

  // ─── Mutations ───────────────────────────────────────────────────────────────

  const cancelMutation = useMutation({
    mutationFn: () => pipelineService.cancelRun(pid, rid),
    onSuccess: () => {
      message.success(t('pipeline:runDetail.cancelSuccess'));
      queryClient.invalidateQueries({ queryKey: ['pipeline-run', pid, rid] });
    },
    onError: () => message.error(t('pipeline:runDetail.cancelFailed')),
  });

  const rerunMutation = useMutation({
    mutationFn: (fromFailed: boolean) => pipelineService.rerun(pid, rid, fromFailed),
    onSuccess: (newRun) => {
      message.success(t('pipeline:run.triggered', { id: newRun.id }));
      navigate(`/pipelines/${pid}/runs/${newRun.id}`);
    },
    onError: () => message.error(t('pipeline:messages.triggerFailed')),
  });

  const approveMutation = useMutation({
    mutationFn: (stepRunId: number) => pipelineService.approveStep(pid, rid, stepRunId),
    onSuccess: () => {
      message.success(t('pipeline:runDetail.approval.approveSuccess'));
      queryClient.invalidateQueries({ queryKey: ['pipeline-run', pid, rid] });
    },
    onError: () => message.error(t('pipeline:runDetail.approval.approveFailed')),
  });

  const rejectMutation = useMutation({
    mutationFn: ({ stepRunId, reason }: { stepRunId: number; reason: string }) =>
      pipelineService.rejectStep(pid, rid, stepRunId, reason || undefined),
    onSuccess: () => {
      message.success(t('pipeline:runDetail.approval.rejectSuccess'));
      setRejectModalOpen(false);
      setRejectReason('');
      queryClient.invalidateQueries({ queryKey: ['pipeline-run', pid, rid] });
    },
    onError: () => message.error(t('pipeline:runDetail.approval.rejectFailed')),
  });

  // ─── Render ───────────────────────────────────────────────────────────────────

  if (isLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!run) {
    return <EmptyState description={t('common:messages.noData')} />;
  }

  const hasFailed = steps.some(s => s.status === 'failed');
  const isMutating = cancelMutation.isPending || rerunMutation.isPending;
  const waitingStep = steps.find(s => s.status === 'waiting_approval') ?? null;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
      {/* CSS keyframes */}
      <style>{`
        @keyframes pulse-ring {
          0%   { transform: scale(1);    opacity: 0.8; }
          70%  { transform: scale(1.18); opacity: 0;   }
          100% { transform: scale(1.18); opacity: 0;   }
        }
        @keyframes shake {
          0%,100% { transform: translateX(0);   }
          20%     { transform: translateX(-6px); }
          40%     { transform: translateX(6px);  }
          60%     { transform: translateX(-4px); }
          80%     { transform: translateX(4px);  }
        }
      `}</style>

      {/* ── Header ─────────────────────────────────────────────────────────── */}
      <div style={{
        background: token.colorBgContainer,
        borderBottom: `1px solid ${token.colorBorder}`,
        padding: `${token.paddingSM}px ${token.paddingLG}px`,
      }}>
        {/* Breadcrumb */}
        <Breadcrumb
          style={{ marginBottom: token.marginXS }}
          items={[
            {
              title: (
                <Button
                  type="link"
                  size="small"
                  icon={<ArrowLeftOutlined />}
                  style={{ padding: 0 }}
                  onClick={() => navigate('/pipelines')}
                >
                  {t('pipeline:page.title')}
                </Button>
              ),
            },
            { title: t('pipeline:runDetail.title', { id: run.id }) },
          ]}
        />

        <Flex justify="space-between" align="center">
          <Flex align="center" gap={token.marginSM}>
            <Title level={5} style={{ margin: 0 }}>
              {t('pipeline:runDetail.title', { id: run.id })}
            </Title>
            <RunStatusTag status={run.status} />
            <Tag>{t(`pipeline:run.triggerType.${run.trigger_type}`)}</Tag>
          </Flex>

          <Space>
            <Tooltip title={t('common:actions.refresh')}>
              <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
            </Tooltip>
            {isActive(run) && (
              <Popconfirm
                title={t('pipeline:runDetail.cancelRun')}
                onConfirm={() => cancelMutation.mutate()}
                okText={t('common:actions.confirm')}
                cancelText={t('common:actions.cancel')}
              >
                <Button danger icon={<StopOutlined />} loading={cancelMutation.isPending}>
                  {t('pipeline:runDetail.cancelRun')}
                </Button>
              </Popconfirm>
            )}
            {(run.status === 'failed' || run.status === 'cancelled') && (
              <Space size={4}>
                <Button
                  icon={<RedoOutlined />}
                  loading={rerunMutation.isPending}
                  onClick={() => rerunMutation.mutate(false)}
                  disabled={isMutating}
                >
                  {t('pipeline:runDetail.rerun')}
                </Button>
                {hasFailed && (
                  <Button
                    icon={<RedoOutlined />}
                    loading={rerunMutation.isPending}
                    onClick={() => rerunMutation.mutate(true)}
                    disabled={isMutating}
                  >
                    {t('pipeline:runDetail.rerunFromFailed')}
                  </Button>
                )}
              </Space>
            )}
          </Space>
        </Flex>

        {/* Run meta */}
        <Descriptions
          size="small"
          column={4}
          style={{ marginTop: token.marginSM }}
        >
          <Descriptions.Item label={t('pipeline:runDetail.queuedAt')}>
            <Text style={{ fontSize: token.fontSizeSM }}>
              {dayjs(run.queued_at).format('YYYY-MM-DD HH:mm:ss')}
            </Text>
          </Descriptions.Item>
          {run.started_at && (
            <Descriptions.Item label={t('pipeline:runDetail.startedAt')}>
              <Text style={{ fontSize: token.fontSizeSM }}>
                {dayjs(run.started_at).format('YYYY-MM-DD HH:mm:ss')}
              </Text>
            </Descriptions.Item>
          )}
          {run.finished_at && (
            <Descriptions.Item label={t('pipeline:runDetail.finishedAt')}>
              <Text style={{ fontSize: token.fontSizeSM }}>
                {dayjs(run.finished_at).format('YYYY-MM-DD HH:mm:ss')}
              </Text>
            </Descriptions.Item>
          )}
          <Descriptions.Item label={t('pipeline:runDetail.elapsed')}>
            <Text style={{ fontSize: token.fontSizeSM }}>
              {formatDuration(run.started_at, run.finished_at)}
            </Text>
          </Descriptions.Item>
        </Descriptions>
      </div>

      {/* ── Approval Banner ─────────────────────────────────────────────────── */}
      {waitingStep && (
        <div style={{
          background: '#fffbeb',
          borderBottom: `1px solid #fcd34d`,
          padding: `${token.paddingSM}px ${token.paddingLG}px`,
        }}>
          <Flex justify="space-between" align="center">
            <Space>
              <AuditOutlined style={{ color: '#d97706', fontSize: 16 }} />
              <Text style={{ color: '#92400e' }}>
                {t('pipeline:runDetail.approval.waiting', { step: waitingStep.step_name })}
              </Text>
            </Space>
            <Space>
              <Popconfirm
                title={t('pipeline:runDetail.approval.approveConfirm')}
                onConfirm={() => approveMutation.mutate(waitingStep.id)}
                okText={t('common:actions.confirm')}
                cancelText={t('common:actions.cancel')}
              >
                <Button
                  type="primary"
                  size="small"
                  loading={approveMutation.isPending}
                >
                  {t('pipeline:runDetail.approval.approve')}
                </Button>
              </Popconfirm>
              <Button
                danger
                size="small"
                onClick={() => setRejectModalOpen(true)}
              >
                {t('pipeline:runDetail.approval.reject')}
              </Button>
            </Space>
          </Flex>
        </div>
      )}

      {/* ── Reject Reason Modal ──────────────────────────────────────────────── */}
      <Modal
        title={t('pipeline:runDetail.approval.rejectTitle')}
        open={rejectModalOpen}
        onCancel={() => { setRejectModalOpen(false); setRejectReason(''); }}
        onOk={() => waitingStep && rejectMutation.mutate({ stepRunId: waitingStep.id, reason: rejectReason })}
        okText={t('pipeline:runDetail.approval.reject')}
        okButtonProps={{ danger: true, loading: rejectMutation.isPending }}
        cancelText={t('common:actions.cancel')}
        destroyOnHidden
        width={480}
      >
        <Input.TextArea
          value={rejectReason}
          onChange={e => setRejectReason(e.target.value)}
          placeholder={t('pipeline:runDetail.approval.rejectReasonPlaceholder')}
          rows={3}
          style={{ marginTop: 8 }}
        />
      </Modal>

      {/* ── DAG Canvas ──────────────────────────────────────────────────────── */}
      <div style={{ flex: 1, minHeight: 0, position: 'relative' }}>
        {steps.length === 0 ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
            <EmptyState description={t('pipeline:runDetail.noSteps')} />
          </div>
        ) : (
          <ReactFlow
            nodes={syncedNodes}
            edges={syncedEdges}
            nodeTypes={NODE_TYPES}
            fitView
            fitViewOptions={{ padding: 0.4 }}
            nodesDraggable={false}
            nodesConnectable={false}
            elementsSelectable={false}
            panOnDrag
            zoomOnScroll
            minZoom={0.4}
            maxZoom={2}
          >
            <Background variant={BackgroundVariant.Dots} gap={20} size={1} color={token.colorBorder} />
            <Controls showInteractive={false} />

            {/* Legend */}
            <div style={{
              position: 'absolute', bottom: 16, left: '50%', transform: 'translateX(-50%)',
              background: token.colorBgContainer,
              border: `1px solid ${token.colorBorder}`,
              borderRadius: token.borderRadius,
              padding: '6px 16px',
              display: 'flex', alignItems: 'center', gap: 16,
              boxShadow: token.boxShadowSecondary,
              zIndex: 10,
              fontSize: token.fontSizeSM,
              color: token.colorTextSecondary,
            }}>
              💡 {t('common:messages.clickToView', { defaultValue: 'Click any step to view logs' })}
            </div>
          </ReactFlow>
        )}
      </div>

      {/* ── Step Log Drawer ─────────────────────────────────────────────────── */}
      <StepLogViewer
        open={logDrawerOpen}
        onClose={() => setLogDrawerOpen(false)}
        stepRun={selectedStep}
        logUrl={logUrl}
      />
    </div>
  );
};

export default PipelineRunDetail;

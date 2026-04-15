/**
 * RunPipelineNode — ReactFlow custom node for real StepRun data
 *
 * Reuses STATUS_STYLES colour tokens from PipelineNode.
 * Maps API StepRun.status → visual StepStatus.
 */
import React from 'react';
import { Handle, Position } from '@xyflow/react';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  ClockCircleOutlined,
  MinusCircleOutlined,
  PauseCircleOutlined,
  BranchesOutlined,
  ContainerOutlined,
  SecurityScanOutlined,
  CloudUploadOutlined,
  RocketOutlined,
  CodeOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import type { StepStatus } from '../pipelineTypes';
import { STATUS_STYLES } from './PipelineNode';
import type { StepRun } from '../../../services/pipelineService';

// ─── Status mapping ───────────────────────────────────────────────────────────

export function toVisualStatus(apiStatus: string): StepStatus {
  switch (apiStatus) {
    case 'pending':           return 'idle';
    case 'running':           return 'running';
    case 'waiting_approval':  return 'running';
    case 'success':           return 'success';
    case 'failed':            return 'failed';
    case 'skipped':           return 'skipped';
    default:                  return 'idle';
  }
}

// ─── Icon by step type ────────────────────────────────────────────────────────

function stepTypeIcon(stepType: string): React.ReactNode {
  const t = stepType.toLowerCase();
  if (t.includes('git') || t.includes('clone') || t.includes('checkout')) return <BranchesOutlined />;
  if (t.includes('docker') || t.includes('kaniko') || t.includes('build')) return <ContainerOutlined />;
  if (t.includes('trivy') || t.includes('scan') || t.includes('security')) return <SecurityScanOutlined />;
  if (t.includes('push') || t.includes('registry') || t.includes('harbor')) return <CloudUploadOutlined />;
  if (t.includes('deploy') || t.includes('rollout') || t.includes('k8s')) return <RocketOutlined />;
  if (t.includes('test') || t.includes('junit')) return <ThunderboltOutlined />;
  if (t.includes('script') || t.includes('shell') || t.includes('run')) return <CodeOutlined />;
  return <CodeOutlined />;
}

// ─── Duration helper ──────────────────────────────────────────────────────────

function formatDuration(startedAt: string | null, finishedAt: string | null): string | undefined {
  if (!startedAt) return undefined;
  const start = new Date(startedAt).getTime();
  const end = finishedAt ? new Date(finishedAt).getTime() : Date.now();
  const ms = end - start;
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  const m = Math.floor(ms / 60_000);
  const s = Math.floor((ms % 60_000) / 1000);
  return `${m}m ${s}s`;
}

// ─── Props ────────────────────────────────────────────────────────────────────

export interface RunPipelineNodeData extends Record<string, unknown> {
  stepRun: StepRun;
  onClick: (sr: StepRun) => void;
}

// ─── Component ────────────────────────────────────────────────────────────────

const RunPipelineNode: React.FC<{ data: RunPipelineNodeData }> = ({ data }) => {
  const { stepRun, onClick } = data;
  const vStatus = toVisualStatus(stepRun.status);
  const style = STATUS_STYLES[vStatus];
  const elapsed = formatDuration(stepRun.started_at, stepRun.finished_at);
  const isActive = vStatus !== 'idle';
  const isWaitingApproval = stepRun.status === 'waiting_approval';

  const statusIcon = () => {
    if (isWaitingApproval) return <PauseCircleOutlined style={{ color: '#f59e0b' }} />;
    switch (vStatus) {
      case 'running': return <LoadingOutlined style={{ color: style.text }} spin />;
      case 'success': return <CheckCircleOutlined style={{ color: '#22c55e' }} />;
      case 'failed':  return <CloseCircleOutlined style={{ color: '#ef4444' }} />;
      case 'skipped': return <MinusCircleOutlined style={{ color: '#94a3b8' }} />;
      default:        return <ClockCircleOutlined style={{ color: '#94a3b8' }} />;
    }
  };

  return (
    <div
      onClick={() => isActive && onClick(stepRun)}
      style={{
        background: style.bg,
        border: `2px solid ${isWaitingApproval ? '#f59e0b' : style.border}`,
        borderRadius: 12,
        padding: '10px 16px',
        minWidth: 140,
        cursor: isActive ? 'pointer' : 'default',
        boxShadow: style.shadow ?? '0 1px 4px rgba(0,0,0,0.06)',
        transition: 'all 0.35s cubic-bezier(0.4,0,0.2,1)',
        animation: vStatus === 'failed' ? 'shake 0.4s ease-in-out' : undefined,
        position: 'relative',
        userSelect: 'none',
      }}
    >
      <Handle
        type="target"
        position={Position.Left}
        style={{ background: style.border, border: 'none', width: 10, height: 10 }}
      />

      {/* Pulse ring for running */}
      {vStatus === 'running' && (
        <div style={{
          position: 'absolute', inset: -6, borderRadius: 16,
          border: `2px solid ${style.border}`,
          animation: 'pulse-ring 1.5s ease-out infinite',
          pointerEvents: 'none',
        }} />
      )}

      {/* Header: type icon + status icon */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
        <span style={{ fontSize: 18, color: style.text }}>
          {stepTypeIcon(stepRun.step_type)}
        </span>
        {statusIcon()}
      </div>

      {/* Step name */}
      <div style={{ fontSize: 13, fontWeight: 700, color: style.text, lineHeight: 1.2 }}>
        {stepRun.step_name}
      </div>
      <div style={{ fontSize: 11, color: '#94a3b8', marginTop: 2 }}>
        {stepRun.step_type}
      </div>

      {/* Duration */}
      {elapsed && vStatus === 'running' && (
        <div style={{ fontSize: 11, color: style.text, marginTop: 4, fontVariantNumeric: 'tabular-nums' }}>
          ⏱ {elapsed}
        </div>
      )}
      {elapsed && vStatus === 'success' && (
        <div style={{ fontSize: 11, color: '#22c55e', marginTop: 4 }}>
          ✓ {elapsed}
        </div>
      )}
      {elapsed && vStatus === 'failed' && (
        <div style={{ fontSize: 11, color: '#ef4444', marginTop: 4 }}>
          ✗ {elapsed}
        </div>
      )}

      {/* Retry badge */}
      {stepRun.retry_count > 0 && (
        <div style={{ fontSize: 10, color: '#f59e0b', marginTop: 2 }}>
          retry {stepRun.retry_count}/{stepRun.max_retries}
        </div>
      )}

      <Handle
        type="source"
        position={Position.Right}
        style={{ background: style.border, border: 'none', width: 10, height: 10 }}
      />
    </div>
  );
};

export default RunPipelineNode;

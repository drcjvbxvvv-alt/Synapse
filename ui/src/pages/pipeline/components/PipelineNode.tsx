/**
 * PipelineNode — ReactFlow 自訂節點元件
 */
import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Tag } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  ClockCircleOutlined,
  MinusCircleOutlined,
} from '@ant-design/icons';
import type { StepDef, StepStatus } from '../pipelineTypes';

// ─── 節點顏色 Token ───────────────────────────────────────────────────────────

export const STATUS_STYLES: Record<StepStatus, {
  bg: string; border: string; text: string; shadow?: string; badgeColor?: string;
}> = {
  idle:    { bg: '#f8fafc', border: '#cbd5e1', text: '#64748b' },
  running: { bg: '#eff6ff', border: '#3b82f6', text: '#1e40af', shadow: '0 0 0 4px rgba(59,130,246,0.15)', badgeColor: '#3b82f6' },
  success: { bg: '#f0fdf4', border: '#22c55e', text: '#166534', shadow: '0 0 0 3px rgba(34,197,94,0.12)', badgeColor: '#22c55e' },
  failed:  { bg: '#fef2f2', border: '#ef4444', text: '#991b1b', shadow: '0 0 0 4px rgba(239,68,68,0.15)', badgeColor: '#ef4444' },
  skipped: { bg: '#f8fafc', border: '#94a3b8', text: '#94a3b8' },
};

// ─── Props ────────────────────────────────────────────────────────────────────

export interface PipelineNodeData {
  step: StepDef;
  status: StepStatus;
  elapsed?: string;
  onClick: (step: StepDef) => void;
  isFailScenario?: boolean;
}

// ─── 元件 ─────────────────────────────────────────────────────────────────────

const PipelineNode: React.FC<{ data: PipelineNodeData }> = ({ data }) => {
  const { step, status, elapsed, onClick, isFailScenario } = data;
  const style = STATUS_STYLES[status];

  const statusIcon = () => {
    switch (status) {
      case 'running': return <LoadingOutlined style={{ color: style.text }} spin />;
      case 'success': return <CheckCircleOutlined style={{ color: '#22c55e' }} />;
      case 'failed':  return <CloseCircleOutlined style={{ color: '#ef4444' }} />;
      case 'skipped': return <MinusCircleOutlined style={{ color: '#94a3b8' }} />;
      default:        return <ClockCircleOutlined style={{ color: '#94a3b8' }} />;
    }
  };

  // Trivy 失敗場景：Trivy 節點顯示 CVE badge
  const showCveBadge = isFailScenario && step.id === 'trivy-scan' && status === 'failed';

  return (
    <div
      onClick={() => (status !== 'idle') && onClick(step)}
      style={{
        background: style.bg,
        border: `2px solid ${style.border}`,
        borderRadius: 12,
        padding: '10px 16px',
        minWidth: 140,
        cursor: status !== 'idle' ? 'pointer' : 'default',
        boxShadow: style.shadow ?? '0 1px 4px rgba(0,0,0,0.06)',
        transition: 'all 0.35s cubic-bezier(0.4,0,0.2,1)',
        animation: status === 'failed' ? 'shake 0.4s ease-in-out' : undefined,
        position: 'relative',
        userSelect: 'none',
      }}
    >
      <Handle type="target" position={Position.Left}
        style={{ background: style.border, border: 'none', width: 10, height: 10 }} />

      {/* 執行中脈衝光暈環 */}
      {status === 'running' && (
        <div style={{
          position: 'absolute', inset: -6, borderRadius: 16,
          border: `2px solid ${style.border}`,
          animation: 'pulse-ring 1.5s ease-out infinite',
          pointerEvents: 'none',
        }} />
      )}

      {/* 頂部：圖示 + 狀態燈 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
        <span style={{ fontSize: 20, color: style.text }}>{step.icon}</span>
        {statusIcon()}
      </div>

      {/* 步驟名稱 */}
      <div style={{ fontSize: 13, fontWeight: 700, color: style.text, lineHeight: 1.2 }}>
        {step.label}
      </div>
      <div style={{ fontSize: 11, color: '#94a3b8', marginTop: 2 }}>
        {step.subLabel}
      </div>

      {/* 執行時間 */}
      {elapsed && status === 'running' && (
        <div style={{ fontSize: 11, color: style.text, marginTop: 4, fontVariantNumeric: 'tabular-nums' }}>
          ⏱ {elapsed}
        </div>
      )}
      {elapsed && status === 'success' && (
        <div style={{ fontSize: 11, color: '#22c55e', marginTop: 4 }}>
          ✓ {elapsed}
        </div>
      )}

      {/* Trivy CVE badge */}
      {showCveBadge && (
        <div style={{ marginTop: 6 }}>
          <Tag color="error" style={{ fontSize: 10, lineHeight: '16px', padding: '0 6px' }}>
            3 CRITICAL
          </Tag>
        </div>
      )}

      <Handle type="source" position={Position.Right}
        style={{ background: style.border, border: 'none', width: 10, height: 10 }} />
    </div>
  );
};

export default PipelineNode;

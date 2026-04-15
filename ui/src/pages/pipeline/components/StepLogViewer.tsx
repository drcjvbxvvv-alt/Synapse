/**
 * StepLogViewer — SSE-based step log Drawer
 *
 * Opens a Drawer and streams logs via useSSELog hook.
 * Terminal dark-theme display with colour coding, blinking cursor,
 * auto-scroll to bottom, and copy-all button.
 */
import React, { useEffect, useRef } from 'react';
import { Drawer, Space, Tag, Button, Tooltip, Flex } from 'antd';
import {
  CopyOutlined,
  LoadingOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useSSELog } from '../../../hooks/useSSELog';
import { toVisualStatus } from './RunPipelineNode';
import type { StepRun } from '../../../services/pipelineService';

// ─── Log line colour coding ───────────────────────────────────────────────────

function lineColor(line: string): string | undefined {
  if (line.includes('CRITICAL') || line.includes('❌') || line.includes('ERROR') || line.includes('FAILED')) return '#f87171';
  if (line.includes('✅') || line.includes('BUILD SUCCESS') || line.includes('successfully')) return '#86efac';
  if (line.includes('WARN') || line.includes('HIGH') || line.includes('MEDIUM')) return '#fbbf24';
  if (line.startsWith('═') || line.startsWith('┌') || line.startsWith('│') || line.startsWith('└') || line.startsWith('├')) return '#475569';
  return '#e2e8f0';
}

// ─── Props ────────────────────────────────────────────────────────────────────

interface StepLogViewerProps {
  open: boolean;
  onClose: () => void;
  stepRun: StepRun | null;
  logUrl: string | null;
}

// ─── Component ────────────────────────────────────────────────────────────────

const StepLogViewer: React.FC<StepLogViewerProps> = ({ open, onClose, stepRun, logUrl }) => {
  const { t } = useTranslation(['pipeline', 'common']);
  const bottomRef = useRef<HTMLDivElement>(null);

  // Only stream while the drawer is open
  const { lines, status } = useSSELog({ url: open ? logUrl : null });

  // Auto-scroll on new lines
  useEffect(() => {
    if (status === 'open' || lines.length > 0) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [lines, status]);

  const handleCopy = () => {
    navigator.clipboard.writeText(lines.join('\n'));
  };

  const vStatus = stepRun ? toVisualStatus(stepRun.status) : 'idle';

  // ─── Status indicator ──────────────────────────────────────────────────────

  const streamBadge = () => {
    switch (status) {
      case 'connecting': return <Tag icon={<LoadingOutlined spin />} color="processing">Connecting…</Tag>;
      case 'open':       return <Tag icon={<SyncOutlined spin />} color="processing">Streaming</Tag>;
      case 'closed':     return <Tag color="default">Complete</Tag>;
      case 'error':      return <Tag color="error">Error</Tag>;
      default:           return null;
    }
  };

  // ─── Drawer title ──────────────────────────────────────────────────────────

  const title = stepRun ? (
    <Flex align="center" gap={8}>
      <span style={{ fontWeight: 700 }}>{stepRun.step_name}</span>
      <Tag style={{ fontWeight: 400, fontSize: 11 }}>{stepRun.step_type}</Tag>
      {vStatus === 'success' && <Tag color="success" icon={<CheckCircleOutlined />}>{t('pipeline:run.status.success')}</Tag>}
      {vStatus === 'failed'  && <Tag color="error"   icon={<CloseCircleOutlined />}>{t('pipeline:run.status.failed')}</Tag>}
      {vStatus === 'running' && <Tag color="processing" icon={<LoadingOutlined spin />}>{t('pipeline:run.status.running')}</Tag>}
      {streamBadge()}
    </Flex>
  ) : t('pipeline:runDetail.stepLogs');

  return (
    <Drawer
      title={title}
      open={open}
      onClose={onClose}
      width={620}
      styles={{ body: { padding: 0, display: 'flex', flexDirection: 'column' } }}
      extra={
        <Tooltip title={t('common:actions.copy')}>
          <Button icon={<CopyOutlined />} size="small" onClick={handleCopy} disabled={lines.length === 0} />
        </Tooltip>
      }
    >
      {/* CSS keyframes injected once */}
      <style>{`
        @keyframes log-fade-in {
          from { opacity: 0; transform: translateY(2px); }
          to   { opacity: 1; transform: translateY(0); }
        }
        @keyframes blink { 50% { opacity: 0; } }
      `}</style>

      <div style={{
        flex: 1,
        background: '#0f172a',
        padding: '16px 20px',
        fontFamily: '"JetBrains Mono", "Fira Code", Menlo, monospace',
        fontSize: 12,
        lineHeight: 1.75,
        color: '#e2e8f0',
        overflowY: 'auto',
        minHeight: 0,
      }}>
        {/* Connecting placeholder */}
        {status === 'connecting' && lines.length === 0 && (
          <div style={{ color: '#64748b' }}>
            <LoadingOutlined spin style={{ marginRight: 8 }} />
            Connecting to log stream…
          </div>
        )}

        {/* Log lines */}
        {lines.map((line, i) => (
          <div
            key={i}
            style={{
              color: lineColor(line),
              minHeight: '1em',
              whiteSpace: 'pre',
              animation: 'log-fade-in 0.12s ease forwards',
            }}
          >
            {line || '\u00A0'}
          </div>
        ))}

        {/* Blinking cursor while streaming */}
        {status === 'open' && (
          <span style={{
            display: 'inline-block', width: 8, height: 14,
            background: '#3b82f6', marginLeft: 2, verticalAlign: 'middle',
            animation: 'blink 1s step-end infinite',
          }} />
        )}

        {/* Error placeholder */}
        {status === 'error' && (
          <div style={{ color: '#f87171', marginTop: 8 }}>
            ⚠ Log stream error — check network or try again.
          </div>
        )}

        {/* Auto-scroll anchor */}
        <div ref={bottomRef} />
      </div>

      {/* Footer: line count */}
      <div style={{
        background: '#1e293b',
        borderTop: '1px solid #334155',
        padding: '6px 20px',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        fontSize: 11,
        color: '#64748b',
      }}>
        <Space size={12}>
          {streamBadge()}
          <span>{lines.length} {lines.length === 1 ? 'line' : 'lines'}</span>
        </Space>
        {stepRun?.error && (
          <span style={{ color: '#f87171' }}>Exit: {stepRun.exit_code ?? '—'}</span>
        )}
      </div>
    </Drawer>
  );
};

export default StepLogViewer;

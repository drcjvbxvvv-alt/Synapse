/**
 * LogDrawer — 步驟日誌 Drawer（逐行打字效果）
 */
import React from 'react';
import { Drawer, Space, Tag } from 'antd';
import type { StepDef, StepStatus } from '../pipelineTypes';

interface LogDrawerProps {
  open: boolean;
  onClose: () => void;
  activeStep: StepDef | null;
  statuses: Record<string, StepStatus>;
  logLines: string[];
}

const LogDrawer: React.FC<LogDrawerProps> = ({ open, onClose, activeStep, statuses, logLines }) => (
  <Drawer
    title={
      <Space>
        <span style={{ fontSize: 16 }}>{activeStep?.icon}</span>
        <span>{activeStep?.label}</span>
        <Tag style={{ fontSize: 11 }}>{activeStep?.subLabel}</Tag>
        {activeStep && statuses[activeStep.id] === 'success' && <Tag color="success">成功</Tag>}
        {activeStep && statuses[activeStep.id] === 'failed'  && <Tag color="error">失敗</Tag>}
      </Space>
    }
    open={open}
    onClose={onClose}
    width={560}
    styles={{ body: { padding: 0 } }}
  >
    <div style={{
      background: '#0f172a',
      minHeight: '100%',
      padding: '16px 20px',
      fontFamily: '"JetBrains Mono", "Fira Code", Menlo, monospace',
      fontSize: 12,
      lineHeight: 1.7,
      color: '#e2e8f0',
      overflowY: 'auto',
    }}>
      {logLines.map((line, i) => {
        const isError   = line.includes('CRITICAL') || line.includes('❌') || line.includes('FAILED');
        const isWarning = line.includes('HIGH') || line.includes('MEDIUM');
        const isSuccess = line.includes('✅') || line.includes('BUILD SUCCESS') || line.includes('successfully');
        const isSep     = line.startsWith('═') || line.startsWith('┌') || line.startsWith('│') || line.startsWith('└') || line.startsWith('├');
        return (
          <div
            key={i}
            className="pipeline-log-line"
            style={{
              color: isError   ? '#f87171'
                   : isSuccess ? '#86efac'
                   : isWarning ? '#fbbf24'
                   : isSep     ? '#475569'
                   : line === '' ? undefined : '#e2e8f0',
              minHeight: '1em',
              whiteSpace: 'pre',
            }}
          >
            {line || '\u00A0'}
          </div>
        );
      })}
      {/* 閃爍游標 */}
      {open && (
        <span style={{
          display: 'inline-block', width: 8, height: 14,
          background: '#3b82f6', marginLeft: 2, verticalAlign: 'middle',
          animation: 'blink 1s step-end infinite',
        }} />
      )}
      <style>{`@keyframes blink { 50% { opacity: 0; } }`}</style>
    </div>
  </Drawer>
);

export default LogDrawer;

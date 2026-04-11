/**
 * PipelineRunDemo — CI/CD Pipeline 視覺化動畫展示元件（設計參照用）
 *
 * 功能展示：
 *  - ReactFlow DAG 節點圖，左→右線性排列
 *  - 節點狀態動畫：等待 / 執行中（脈衝光暈）/ 成功（綠✓）/ 失敗（紅✗震動）/ 跳過
 *  - 邊狀態動畫：流動虛線（執行中）/ 實線綠色（成功）/ 虛線紅色（失敗下游跳過）
 *  - 場景切換：「成功流程」/ 「Trivy 掃描失敗」
 *  - 點擊節點展開右側 Drawer 顯示模擬 Log 輸出（逐行打字效果）
 *  - 速度控制（1× / 2× / 0.5×）
 *  - 自動循環播放
 */
import React from 'react';
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
  Tooltip,
  Typography,
  Switch,
  Select,
  Divider,
} from 'antd';
import {
  PlayCircleOutlined,
  ReloadOutlined,
  BugOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
} from '@ant-design/icons';
import PipelineNode from './components/PipelineNode';
import LogDrawer from './components/LogDrawer';
import { usePipelineRunDemo } from './hooks/usePipelineRunDemo';

const { Text, Title } = Typography;

const nodeTypes = { pipeline: PipelineNode };

// ─── 主元件 ──────────────────────────────────────────────────────────────────

const PipelineRunDemo: React.FC = () => {
  const {
    scenario, setScenario,
    speedMult, setSpeedMult,
    autoLoop, setAutoLoop,
    running,
    statuses,
    activeStep,
    logLines,
    drawerOpen, setDrawerOpen,
    nodes, edges, onNodesChange, onEdgesChange,
    overallStatus,
    totalElapsed,
    allStatuses,
    run,
    reset,
  } = usePipelineRunDemo();

  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column', background: '#f5f7fa' }}>

      {/* ── CSS keyframes（注入一次）── */}
      <style>{`
        @keyframes pulse-ring {
          0%   { transform: scale(1);   opacity: 0.8; }
          70%  { transform: scale(1.18); opacity: 0; }
          100% { transform: scale(1.18); opacity: 0; }
        }
        @keyframes shake {
          0%,100% { transform: translateX(0); }
          20%     { transform: translateX(-6px); }
          40%     { transform: translateX(6px); }
          60%     { transform: translateX(-4px); }
          80%     { transform: translateX(4px); }
        }
        .pipeline-log-line {
          animation: fadeInUp 0.15s ease forwards;
        }
        @keyframes fadeInUp {
          from { opacity: 0; transform: translateY(4px); }
          to   { opacity: 1; transform: translateY(0); }
        }
      `}</style>

      {/* ── Header 工具列 ── */}
      <div style={{
        background: '#fff',
        borderBottom: '1px solid #e8ecf0',
        padding: '12px 24px',
        display: 'flex',
        alignItems: 'center',
        gap: 16,
        flexWrap: 'wrap',
      }}>
        <div style={{ flex: 1 }}>
          <Title level={5} style={{ margin: 0, color: '#1e293b' }}>
            CI/CD Pipeline 視覺化展示
          </Title>
          <Text type="secondary" style={{ fontSize: 12 }}>
            設計參照原型 — backend-service · commit abc1234 · 觸發：git push → main
          </Text>
        </div>

        {/* 整體狀態 badge */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {overallStatus === 'idle' && <Tag color="default">等待執行</Tag>}
          {overallStatus === 'running' && <Tag color="processing" icon={<LoadingOutlined spin />}>執行中 {totalElapsed ?? ''}</Tag>}
          {overallStatus === 'success' && <Tag color="success" icon={<CheckCircleOutlined />}>Pipeline 成功 {totalElapsed}</Tag>}
          {overallStatus === 'failed'  && <Tag color="error"   icon={<CloseCircleOutlined />}>Pipeline 失敗</Tag>}
        </div>

        <Divider type="vertical" />

        {/* 場景切換 */}
        <Space>
          <Text style={{ fontSize: 12, color: '#64748b' }}>場景：</Text>
          <Select
            value={scenario}
            onChange={v => { setScenario(v); reset(); }}
            size="small"
            style={{ width: 160 }}
            options={[
              { value: 'success',     label: '✅ 全部成功' },
              { value: 'trivy-fail',  label: '❌ Trivy 掃描失敗' },
            ]}
          />
        </Space>

        {/* 速度 */}
        <Space>
          <Text style={{ fontSize: 12, color: '#64748b' }}>速度：</Text>
          <Select
            value={speedMult}
            onChange={setSpeedMult}
            size="small"
            style={{ width: 80 }}
            options={[
              { value: 0.5, label: '0.5×' },
              { value: 1,   label: '1×' },
              { value: 2,   label: '2×' },
              { value: 4,   label: '4×' },
            ]}
          />
        </Space>

        {/* 自動循環 */}
        <Space>
          <Text style={{ fontSize: 12, color: '#64748b' }}>自動循環</Text>
          <Switch size="small" checked={autoLoop} onChange={setAutoLoop} />
        </Space>

        <Divider type="vertical" />

        <Space>
          <Tooltip title="重置">
            <Button icon={<ReloadOutlined />} size="small" onClick={reset} disabled={!running && allStatuses.length === 0} />
          </Tooltip>
          <Button
            type="primary"
            icon={running ? <LoadingOutlined spin /> : <PlayCircleOutlined />}
            size="small"
            onClick={run}
            disabled={running}
          >
            {running ? '執行中...' : '執行 Pipeline'}
          </Button>
        </Space>
      </div>

      {/* ── 圖例 ── */}
      <div style={{
        background: '#fff',
        borderBottom: '1px solid #e8ecf0',
        padding: '8px 24px',
        display: 'flex',
        gap: 20,
        alignItems: 'center',
      }}>
        <Text style={{ fontSize: 11, color: '#64748b', marginRight: 4 }}>節點狀態：</Text>
        {([
          ['idle',    '#94a3b8', '等待'],
          ['running', '#3b82f6', '執行中'],
          ['success', '#22c55e', '成功'],
          ['failed',  '#ef4444', '失敗'],
          ['skipped', '#94a3b8', '跳過'],
        ] as const).map(([, color, label]) => (
          <Space key={label} size={4}>
            <span style={{
              display: 'inline-block', width: 10, height: 10, borderRadius: '50%', background: color,
            }} />
            <Text style={{ fontSize: 11, color: '#64748b' }}>{label}</Text>
          </Space>
        ))}
        <Divider type="vertical" />
        <Text style={{ fontSize: 11, color: '#64748b' }}>
          💡 點擊任意已執行的節點可查看步驟日誌
        </Text>
      </div>

      {/* ── ReactFlow 主畫布 ── */}
      <div style={{ flex: 1, position: 'relative' }}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.4 }}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable={false}
          panOnDrag
          zoomOnScroll
          minZoom={0.5}
          maxZoom={2}
        >
          <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="#e2e8f0" />
          <Controls showInteractive={false} />

          {/* 場景說明 panel */}
          {scenario === 'trivy-fail' && (
            <div style={{
              position: 'absolute', bottom: 20, left: '50%', transform: 'translateX(-50%)',
              background: '#fef2f2', border: '1px solid #fca5a5', borderRadius: 8,
              padding: '8px 16px', display: 'flex', alignItems: 'center', gap: 8,
              boxShadow: '0 2px 8px rgba(0,0,0,0.08)', zIndex: 10,
            }}>
              <BugOutlined style={{ color: '#ef4444' }} />
              <Text style={{ fontSize: 12, color: '#991b1b' }}>
                Trivy 掃描發現 <strong>3 個 CRITICAL 漏洞</strong>，Pipeline 中止，映像不會推送至 Harbor
              </Text>
            </div>
          )}
        </ReactFlow>
      </div>

      {/* ── 步驟日誌 Drawer ── */}
      <LogDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        activeStep={activeStep}
        statuses={statuses}
        logLines={logLines}
      />
    </div>
  );
};

export default PipelineRunDemo;

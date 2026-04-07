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
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  Handle,
  Position,
  BackgroundVariant,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  Button,
  Drawer,
  Space,
  Tag,
  Tooltip,
  Typography,
  Switch,
  Select,
  Badge,
  Divider,
} from 'antd';
import {
  PlayCircleOutlined,
  ReloadOutlined,
  BugOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  ClockCircleOutlined,
  MinusCircleOutlined,
  SecurityScanOutlined,
  CloudUploadOutlined,
  RocketOutlined,
  CodeOutlined,
  ContainerOutlined,
} from '@ant-design/icons';

const { Text, Title } = Typography;

// ─── 型別 ────────────────────────────────────────────────────────────────────

type StepStatus = 'idle' | 'running' | 'success' | 'failed' | 'skipped';

interface StepDef {
  id: string;
  label: string;
  subLabel: string;
  icon: React.ReactNode;
  duration: number;      // ms，模擬執行時間
  mockLogs: string[];
}

// ─── 步驟定義 ─────────────────────────────────────────────────────────────────

const STEPS: StepDef[] = [
  {
    id: 'build-jar',
    label: 'Build JAR',
    subLabel: 'Maven 3.9',
    icon: <CodeOutlined />,
    duration: 3200,
    mockLogs: [
      '[INFO] Scanning for projects...',
      '[INFO] Building backend-service 2.1.4-SNAPSHOT',
      '[INFO] --- maven-compiler-plugin:3.11:compile ---',
      '[INFO] Compiling 247 source files to /workspace/target/classes',
      '[INFO] --- maven-surefire-plugin:3.0:test ---',
      '[INFO] Tests run: 142, Failures: 0, Errors: 0, Skipped: 3',
      '[INFO] --- maven-jar-plugin:3.3:jar ---',
      '[INFO] Building jar: /workspace/target/backend-service.jar',
      '[INFO] BUILD SUCCESS',
      '[INFO] Total time: 52.3 s',
    ],
  },
  {
    id: 'build-image',
    label: 'Build Image',
    subLabel: 'Kaniko',
    icon: <ContainerOutlined />,
    duration: 4100,
    mockLogs: [
      'INFO[0000] Retrieving image manifest openjdk:17-slim',
      'INFO[0002] Unpacking rootfs as cmd COPY requires it.',
      'INFO[0004] COPY target/backend-service.jar /app/app.jar',
      'INFO[0004] RUN addgroup -S app && adduser -S app -G app',
      'INFO[0006] EXPOSE 8080',
      'INFO[0006] ENTRYPOINT ["java", "-jar", "/app/app.jar"]',
      'INFO[0007] Pushing image to harbor.company.com/prod/backend-service:abc1234',
      'INFO[0009] Pushed image digest: sha256:4f2a9c...',
    ],
  },
  {
    id: 'trivy-scan',
    label: 'Trivy Scan',
    subLabel: 'Security',
    icon: <SecurityScanOutlined />,
    duration: 2800,
    mockLogs: [
      '2024-04-06T10:23:01Z INFO  Vulnerability scanning is enabled',
      '2024-04-06T10:23:01Z INFO  Detected OS: debian 11.8',
      '2024-04-06T10:23:03Z INFO  Scanning packages...',
      '2024-04-06T10:23:05Z INFO  Scanning library vulnerabilities...',
      '',
      'harbor.company.com/prod/backend-service:abc1234',
      '═══════════════════════════════════',
      'Total: 3 (CRITICAL: 0, HIGH: 1, MEDIUM: 2, LOW: 0)',
      '',
      '┌─────────────────┬──────────────────┬──────────┬──────────────────┐',
      '│ Library         │ Vulnerability    │ Severity │ Fixed Version    │',
      '├─────────────────┼──────────────────┼──────────┼──────────────────┤',
      '│ log4j-core      │ CVE-2024-23672   │ HIGH     │ 2.23.1           │',
      '│ commons-text    │ CVE-2024-11024   │ MEDIUM   │ 1.11.0           │',
      '│ jackson-databind│ CVE-2024-28849   │ MEDIUM   │ 2.16.2           │',
      '└─────────────────┴──────────────────┴──────────┴──────────────────┘',
      '',
      '✅ No CRITICAL vulnerabilities. Threshold not exceeded.',
    ],
  },
  {
    id: 'push-harbor',
    label: 'Push Harbor',
    subLabel: 'Registry',
    icon: <CloudUploadOutlined />,
    duration: 1800,
    mockLogs: [
      'Logging into harbor.company.com...',
      'Login Succeeded',
      'Pushing harbor.company.com/prod/backend-service:abc1234',
      'The push refers to repository [harbor.company.com/prod/backend-service]',
      '4f2a9c...: Pushed',
      'abc1234: digest: sha256:9d8e2f... size: 1847',
      '',
      '✅ Image successfully pushed to Harbor',
      '   Repository: harbor.company.com/prod/backend-service',
      '   Tag:        abc1234',
      '   Digest:     sha256:9d8e2f...',
    ],
  },
  {
    id: 'deploy',
    label: 'Deploy',
    subLabel: 'K8s',
    icon: <RocketOutlined />,
    duration: 2400,
    mockLogs: [
      'Applying manifests to cluster: production-k8s',
      'Namespace: app-production',
      '',
      'deployment.apps/backend-service configured',
      'service/backend-service unchanged',
      '',
      'Waiting for rollout to finish...',
      'Waiting for deployment "backend-service" rollout to finish: 1 out of 3 new replicas have been updated...',
      'Waiting for deployment "backend-service" rollout to finish: 2 out of 3 new replicas have been updated...',
      'Waiting for deployment "backend-service" rollout to finish: 1 old replicas are pending termination...',
      'Waiting for deployment "backend-service" rollout to finish: 0 old replicas are pending termination...',
      '',
      '✅ deployment "backend-service" successfully rolled out',
      '   Ready: 3/3  |  Up-to-date: 3  |  Available: 3',
    ],
  },
];

// Trivy 失敗場景的 log
const TRIVY_FAIL_LOGS = [
  '2024-04-06T10:23:01Z INFO  Vulnerability scanning is enabled',
  '2024-04-06T10:23:01Z INFO  Detected OS: debian 11.8',
  '2024-04-06T10:23:03Z INFO  Scanning packages...',
  '',
  'harbor.company.com/prod/backend-service:abc1234',
  '═══════════════════════════════════',
  'Total: 8 (CRITICAL: 3, HIGH: 4, MEDIUM: 1)',
  '',
  '┌──────────────────┬──────────────────┬──────────┬──────────────────┐',
  '│ Library          │ Vulnerability    │ Severity │ Fixed Version    │',
  '├──────────────────┼──────────────────┼──────────┼──────────────────┤',
  '│ log4j-core       │ CVE-2021-44228   │ CRITICAL │ 2.17.1           │',
  '│ log4j-core       │ CVE-2021-45105   │ CRITICAL │ 2.17.1           │',
  '│ openssl          │ CVE-2023-0286    │ CRITICAL │ 3.0.9            │',
  '│ spring-webmvc    │ CVE-2024-22233   │ HIGH     │ 6.1.4            │',
  '└──────────────────┴──────────────────┴──────────┴──────────────────┘',
  '',
  '❌ CRITICAL vulnerabilities found: 3',
  '   Threshold: CRITICAL=0',
  '   Pipeline FAILED — image will NOT be pushed to Harbor',
];

// ─── 節點顏色 Token ───────────────────────────────────────────────────────────

const STATUS_STYLES: Record<StepStatus, {
  bg: string; border: string; text: string; shadow?: string; badgeColor?: string;
}> = {
  idle:    { bg: '#f8fafc', border: '#cbd5e1', text: '#64748b' },
  running: { bg: '#eff6ff', border: '#3b82f6', text: '#1e40af', shadow: '0 0 0 4px rgba(59,130,246,0.15)', badgeColor: '#3b82f6' },
  success: { bg: '#f0fdf4', border: '#22c55e', text: '#166534', shadow: '0 0 0 3px rgba(34,197,94,0.12)', badgeColor: '#22c55e' },
  failed:  { bg: '#fef2f2', border: '#ef4444', text: '#991b1b', shadow: '0 0 0 4px rgba(239,68,68,0.15)', badgeColor: '#ef4444' },
  skipped: { bg: '#f8fafc', border: '#94a3b8', text: '#94a3b8' },
};

// ─── 自訂節點元件 ─────────────────────────────────────────────────────────────

interface PipelineNodeData {
  step: StepDef;
  status: StepStatus;
  elapsed?: string;
  onClick: (step: StepDef) => void;
  isFailScenario?: boolean;
}

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

const nodeTypes = { pipeline: PipelineNode };

// ─── 節點 / 邊 初始值 ─────────────────────────────────────────────────────────

const X_GAP = 210;
const Y = 120;

function buildNodes(
  statuses: Record<string, StepStatus>,
  elapsed: Record<string, string>,
  onClickStep: (step: StepDef) => void,
  isFailScenario: boolean,
): Node[] {
  return STEPS.map((step, i) => ({
    id: step.id,
    type: 'pipeline',
    position: { x: 60 + i * X_GAP, y: Y },
    data: {
      step,
      status: statuses[step.id] ?? 'idle',
      elapsed: elapsed[step.id],
      onClick: onClickStep,
      isFailScenario,
    },
  }));
}

function buildEdges(statuses: Record<string, StepStatus>): Edge[] {
  return STEPS.slice(0, -1).map((step, i) => {
    const nextStep = STEPS[i + 1];
    const srcStatus = statuses[step.id] ?? 'idle';
    const isRunning = statuses[nextStep.id] === 'running';
    const isSuccess = statuses[nextStep.id] === 'success' || statuses[nextStep.id] === 'skipped';
    const isFailed  = srcStatus === 'failed';

    return {
      id: `${step.id}->${nextStep.id}`,
      source: step.id,
      target: nextStep.id,
      animated: isRunning,
      style: {
        stroke: isFailed ? '#ef4444'
          : isSuccess  ? '#22c55e'
          : isRunning  ? '#3b82f6'
          : '#cbd5e1',
        strokeWidth: isSuccess || isRunning ? 2.5 : 1.5,
        strokeDasharray: isFailed ? '6 4' : undefined,
        transition: 'stroke 0.4s ease, stroke-width 0.4s ease',
      },
    };
  });
}

// ─── 主元件 ──────────────────────────────────────────────────────────────────

type Scenario = 'success' | 'trivy-fail';

const PipelineRunDemo: React.FC = () => {
  const [scenario, setScenario] = useState<Scenario>('success');
  const [speedMult, setSpeedMult] = useState(1);
  const [autoLoop, setAutoLoop]   = useState(false);
  const [running, setRunning]     = useState(false);

  const [statuses, setStatuses]   = useState<Record<string, StepStatus>>({});
  const [elapsed, setElapsed]     = useState<Record<string, string>>({});
  const [activeStep, setActiveStep] = useState<StepDef | null>(null);
  const [logLines, setLogLines]   = useState<string[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const stopRef   = useRef(false);
  const timerRef  = useRef<ReturnType<typeof setTimeout> | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  // 同步 nodes / edges
  const syncGraph = useCallback(
    (s: Record<string, StepStatus>, e: Record<string, string>) => {
      setNodes(buildNodes(s, e, handleClickStep, scenario === 'trivy-fail'));
      setEdges(buildEdges(s));
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [scenario]
  );

  useEffect(() => { syncGraph(statuses, elapsed); }, [statuses, elapsed, syncGraph]);

  // 點擊節點 → 開 Drawer 顯示 log
  const handleClickStep = useCallback((step: StepDef) => {
    setActiveStep(step);
    setDrawerOpen(true);
    // 決定顯示哪組 log
    const isFail = scenario === 'trivy-fail' && step.id === 'trivy-scan';
    const lines  = isFail ? TRIVY_FAIL_LOGS : step.mockLogs;
    // 逐行打字效果
    setLogLines([]);
    lines.forEach((line, i) => {
      setTimeout(() => {
        setLogLines(prev => [...prev, line]);
      }, i * 60);
    });
  }, [scenario]);

  // 步驟執行模擬
  const runStep = useCallback(
    (stepIndex: number, localStatuses: Record<string, StepStatus>, localElapsed: Record<string, string>) =>
      new Promise<{ statuses: Record<string, StepStatus>; elapsed: Record<string, string> }>(resolve => {
        if (stopRef.current) { resolve({ statuses: localStatuses, elapsed: localElapsed }); return; }

        const step = STEPS[stepIndex];
        const dur  = Math.round(step.duration / speedMult);

        // 標記 running
        const s1 = { ...localStatuses, [step.id]: 'running' as StepStatus };
        const e1 = { ...localElapsed };
        setStatuses(s1); setElapsed(e1); syncGraph(s1, e1);

        // elapsed timer
        const start = Date.now();
        intervalRef.current = setInterval(() => {
          const sec = ((Date.now() - start) / 1000).toFixed(1);
          const e2 = { ...e1, [step.id]: `${sec}s` };
          setElapsed(e2);
          syncGraph(s1, e2);
        }, 200);

        timerRef.current = setTimeout(() => {
          clearInterval(intervalRef.current!);
          const sec    = (dur / 1000).toFixed(1);
          const failed = scenario === 'trivy-fail' && step.id === 'trivy-scan';
          const status: StepStatus = failed ? 'failed' : 'success';
          const s2 = { ...s1, [step.id]: status };
          const e2 = { ...e1, [step.id]: `${sec}s` };
          setStatuses(s2); setElapsed(e2); syncGraph(s2, e2);
          resolve({ statuses: s2, elapsed: e2 });
        }, dur);
      }),
    [scenario, speedMult, syncGraph]
  );

  const reset = useCallback(() => {
    stopRef.current = true;
    clearTimeout(timerRef.current!);
    clearInterval(intervalRef.current!);
    setTimeout(() => {
      stopRef.current = false;
      setRunning(false);
      setStatuses({});
      setElapsed({});
      setDrawerOpen(false);
      setLogLines([]);
    }, 50);
  }, []);

  const run = useCallback(async () => {
    reset();
    // 等 reset 完成
    await new Promise(r => setTimeout(r, 100));
    setRunning(true);
    stopRef.current = false;

    let s: Record<string, StepStatus> = {};
    let e: Record<string, string>     = {};

    for (let i = 0; i < STEPS.length; i++) {
      if (stopRef.current) break;
      const result = await runStep(i, s, e);
      s = result.statuses;
      e = result.elapsed;

      // Trivy 失敗：把後續步驟標為 skipped
      if (scenario === 'trivy-fail' && STEPS[i].id === 'trivy-scan') {
        const skipped = { ...s };
        STEPS.slice(i + 1).forEach(st => { skipped[st.id] = 'skipped'; });
        setStatuses(skipped);
        syncGraph(skipped, e);
        break;
      }
    }

    setRunning(false);

    // 自動循環
    if (autoLoop && !stopRef.current) {
      timerRef.current = setTimeout(() => run(), 2000);
    }
  }, [reset, runStep, scenario, autoLoop, syncGraph]);

  // cleanup on unmount
  useEffect(() => () => {
    stopRef.current = true;
    clearTimeout(timerRef.current!);
    clearInterval(intervalRef.current!);
  }, []);

  // ── 計算整體狀態 ──
  const allStatuses = Object.values(statuses);
  const overallStatus: 'idle' | 'running' | 'success' | 'failed' =
    running ? 'running'
    : allStatuses.includes('failed') ? 'failed'
    : allStatuses.length > 0 && STEPS.every(s => statuses[s.id] === 'success') ? 'success'
    : 'idle';

  const totalElapsed = (() => {
    const nums = Object.values(elapsed).map(v => parseFloat(v)).filter(Boolean);
    return nums.length ? `${nums.reduce((a, b) => a + b, 0).toFixed(1)}s` : null;
  })();

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
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
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
          {drawerOpen && (
            <span style={{
              display: 'inline-block', width: 8, height: 14,
              background: '#3b82f6', marginLeft: 2, verticalAlign: 'middle',
              animation: 'blink 1s step-end infinite',
            }} />
          )}
          <style>{`@keyframes blink { 50% { opacity: 0; } }`}</style>
        </div>
      </Drawer>
    </div>
  );
};

export default PipelineRunDemo;

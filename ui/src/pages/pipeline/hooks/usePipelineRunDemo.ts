/**
 * usePipelineRunDemo — Pipeline 動畫展示的所有狀態與控制邏輯
 */
import { useState, useCallback, useEffect, useRef } from 'react';
import { useNodesState, useEdgesState, type Node, type Edge } from '@xyflow/react';
import { STEPS, TRIVY_FAIL_LOGS } from '../pipelineTypes';
import type { StepDef, StepStatus, Scenario } from '../pipelineTypes';
import { buildNodes, buildEdges } from '../components/pipelineGraph';

export function usePipelineRunDemo() {
  const [scenario, setScenario] = useState<Scenario>('success');
  const [speedMult, setSpeedMult] = useState(1);
  const [autoLoop, setAutoLoop]   = useState(false);
  const [running, setRunning]     = useState(false);

  const [statuses, setStatuses]     = useState<Record<string, StepStatus>>({});
  const [elapsed, setElapsed]       = useState<Record<string, string>>({});
  const [activeStep, setActiveStep] = useState<StepDef | null>(null);
  const [logLines, setLogLines]     = useState<string[]>([]);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const stopRef     = useRef(false);
  const timerRef    = useRef<ReturnType<typeof setTimeout> | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  // Sync nodes / edges whenever statuses or elapsed change
  const syncGraph = useCallback(
    (s: Record<string, StepStatus>, e: Record<string, string>) => {
      setNodes(buildNodes(s, e, handleClickStep, scenario === 'trivy-fail'));
      setEdges(buildEdges(s));
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [scenario]
  );

  useEffect(() => { syncGraph(statuses, elapsed); }, [statuses, elapsed, syncGraph]);

  // Click node → open Drawer with typewriter log effect
  const handleClickStep = useCallback((step: StepDef) => {
    setActiveStep(step);
    setDrawerOpen(true);
    const isFail = scenario === 'trivy-fail' && step.id === 'trivy-scan';
    const lines  = isFail ? TRIVY_FAIL_LOGS : step.mockLogs;
    setLogLines([]);
    lines.forEach((line, i) => {
      setTimeout(() => {
        setLogLines(prev => [...prev, line]);
      }, i * 60);
    });
  }, [scenario]);

  // Simulate running a single step
  const runStep = useCallback(
    (stepIndex: number, localStatuses: Record<string, StepStatus>, localElapsed: Record<string, string>) =>
      new Promise<{ statuses: Record<string, StepStatus>; elapsed: Record<string, string> }>(resolve => {
        if (stopRef.current) { resolve({ statuses: localStatuses, elapsed: localElapsed }); return; }

        const step = STEPS[stepIndex];
        const dur  = Math.round(step.duration / speedMult);

        const s1 = { ...localStatuses, [step.id]: 'running' as StepStatus };
        const e1 = { ...localElapsed };
        setStatuses(s1); setElapsed(e1); syncGraph(s1, e1);

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

      if (scenario === 'trivy-fail' && STEPS[i].id === 'trivy-scan') {
        const skipped = { ...s };
        STEPS.slice(i + 1).forEach(st => { skipped[st.id] = 'skipped'; });
        setStatuses(skipped);
        syncGraph(skipped, e);
        break;
      }
    }

    setRunning(false);

    if (autoLoop && !stopRef.current) {
      timerRef.current = setTimeout(() => run(), 2000);
    }
  }, [reset, runStep, scenario, autoLoop, syncGraph]);

  // Cleanup on unmount
  useEffect(() => () => {
    stopRef.current = true;
    clearTimeout(timerRef.current!);
    clearInterval(intervalRef.current!);
  }, []);

  // Derived state
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

  return {
    // State
    scenario, setScenario,
    speedMult, setSpeedMult,
    autoLoop, setAutoLoop,
    running,
    statuses,
    activeStep,
    logLines,
    drawerOpen, setDrawerOpen,
    // Graph
    nodes, edges, onNodesChange, onEdgesChange,
    // Derived
    overallStatus,
    totalElapsed,
    allStatuses,
    // Actions
    run,
    reset,
  };
}

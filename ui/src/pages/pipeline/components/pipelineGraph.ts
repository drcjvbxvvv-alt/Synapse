/**
 * ReactFlow ノード/エッジ構築ヘルパー
 */
import type { Node, Edge } from '@xyflow/react';
import { STEPS } from '../pipelineTypes';
import type { StepDef, StepStatus } from '../pipelineTypes';
import type { PipelineNodeData } from './PipelineNode';

const X_GAP = 210;
const Y = 120;

export function buildNodes(
  statuses: Record<string, StepStatus>,
  elapsed: Record<string, string>,
  onClickStep: (step: StepDef) => void,
  isFailScenario: boolean,
): Node<PipelineNodeData>[] {
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

export function buildEdges(statuses: Record<string, StepStatus>): Edge[] {
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

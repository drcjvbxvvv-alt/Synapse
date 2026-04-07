import React, { useMemo, useCallback } from 'react';
import { Tag, Badge, Tooltip } from 'antd';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  EdgeLabelRenderer,
  type Node,
  type Edge,
  type NodeProps,
  type EdgeProps,
  getBezierPath,
  Position,
  Handle,
  type NodeMouseHandler,
} from '@xyflow/react';
import dagre from '@dagrejs/dagre';
import '@xyflow/react/dist/style.css';
import type { NetworkNode, NetworkEdge } from '../../services/networkTopologyService';

// ---- Constants ----

const NODE_W = 160;
const NODE_H = 72;

const WORKLOAD_KIND_COLOR: Record<string, string> = {
  Deployment: '#1677ff',
  StatefulSet: '#722ed1',
  DaemonSet: '#13c2c2',
  Job: '#fa8c16',
  Pod: '#8c8c8c',
  ReplicaSet: '#1677ff',
};

const HEALTH_EDGE: Record<string, { stroke: string; particle: string; dur: string }> = {
  healthy:  { stroke: '#52c41a', particle: '#52c41a', dur: '1.8s' },
  degraded: { stroke: '#fa8c16', particle: '#fa8c16', dur: '3.5s' },
  down:     { stroke: '#ff4d4f', particle: '#ff4d4f', dur: '6s'   },
  unknown:  { stroke: '#d9d9d9', particle: '#bfbfbf', dur: '2.5s' },
};

// ---- Custom node data types ----

interface WorkloadNodeData extends Record<string, unknown> {
  name: string;
  namespace: string;
  workloadKind: string;
  readyCount: number;
  totalCount: number;
}

interface ServiceNodeData extends Record<string, unknown> {
  name: string;
  namespace: string;
  serviceType: string;
  clusterIP?: string;
}

interface IngressNodeData extends Record<string, unknown> {
  name: string;
  namespace: string;
  ingressClass?: string;
}

// ---- Custom nodes ----

const WorkloadNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as WorkloadNodeData;
  const color = WORKLOAD_KIND_COLOR[d.workloadKind] ?? '#8c8c8c';
  const healthy = d.readyCount === d.totalCount && d.totalCount > 0;
  const status = healthy ? 'success' : d.readyCount > 0 ? 'warning' : 'error';

  return (
    <div
      style={{
        width: NODE_W,
        minHeight: NODE_H,
        background: '#fff',
        border: `1.5px solid ${color}`,
        borderRadius: 8,
        padding: '6px 10px',
        boxSizing: 'border-box',
      }}
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, marginBottom: 2 }}>
        <Tag
          color={color}
          style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', marginRight: 0 }}
        >
          {d.workloadKind}
        </Tag>
        <Badge status={status} />
      </div>
      <div style={{ fontSize: 12, fontWeight: 600, wordBreak: 'break-all', lineHeight: 1.3 }}>
        {d.name}
      </div>
      <div style={{ fontSize: 10, color: '#8c8c8c', marginTop: 1 }}>
        {d.namespace}
      </div>
      <div style={{ fontSize: 10, color: d.readyCount < d.totalCount ? '#fa8c16' : '#52c41a', marginTop: 2 }}>
        {d.readyCount}/{d.totalCount} Ready
      </div>
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

const ServiceNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as ServiceNodeData;

  return (
    <div
      style={{
        width: NODE_W,
        minHeight: NODE_H,
        background: '#fff',
        border: '1.5px solid #fa8c16',
        borderRadius: 8,
        padding: '6px 10px',
        boxSizing: 'border-box',
      }}
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ marginBottom: 2 }}>
        <Tag
          color="orange"
          style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}
        >
          Service
        </Tag>
        <Tag
          style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', marginLeft: 2 }}
        >
          {d.serviceType || 'ClusterIP'}
        </Tag>
      </div>
      <div style={{ fontSize: 12, fontWeight: 600, wordBreak: 'break-all', lineHeight: 1.3 }}>
        {d.name}
      </div>
      <div style={{ fontSize: 10, color: '#8c8c8c', marginTop: 1 }}>
        {d.namespace}
      </div>
      {d.clusterIP && d.clusterIP !== 'None' && (
        <div style={{ fontSize: 10, color: '#8c8c8c', marginTop: 1 }}>
          {d.clusterIP}
        </div>
      )}
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

const IngressNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as IngressNodeData;
  return (
    <div
      style={{
        width: NODE_W,
        minHeight: NODE_H,
        background: '#fff',
        border: '1.5px solid #722ed1',
        borderRadius: 8,
        padding: '6px 10px',
        boxSizing: 'border-box',
      }}
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ marginBottom: 2, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
        <Tag color="purple" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>
          Ingress
        </Tag>
        {d.ingressClass && (
          <Tag style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>
            {d.ingressClass}
          </Tag>
        )}
      </div>
      <div style={{ fontSize: 12, fontWeight: 600, wordBreak: 'break-all', lineHeight: 1.3 }}>
        {d.name}
      </div>
      <div style={{ fontSize: 10, color: '#8c8c8c', marginTop: 1 }}>
        {d.namespace}
      </div>
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

// ---- Particle edge ----

const PARTICLE_COUNT = 3;

interface ParticleEdgeData extends Record<string, unknown> {
  kind?: string;
  health?: string;
  ports?: string;
  requestRate?: number;
  errorRate?: number;
  latencyP99ms?: number;
}

function resolveEdgeStyle(d: ParticleEdgeData): { stroke: string; particle: string; dur: string } {
  // Ingress edges: purple
  if (d.kind === 'ingress') {
    return { stroke: '#722ed1', particle: '#722ed1', dur: '2s' };
  }
  // Istio actual call-direction edges: cyan, speed driven by error rate
  if (d.kind === 'istio-flow') {
    if ((d.errorRate ?? 0) > 0.2)  return { stroke: '#ff4d4f', particle: '#ff4d4f', dur: '1.5s' };
    if ((d.errorRate ?? 0) > 0.05) return { stroke: '#fa8c16', particle: '#fa8c16', dur: '2.5s' };
    return { stroke: '#13c2c2', particle: '#13c2c2', dur: '1.8s' };
  }
  // Istio metrics override static health colour when available
  if (d.requestRate !== undefined) {
    if ((d.errorRate ?? 0) > 0.2)    return { stroke: '#ff4d4f', particle: '#ff4d4f', dur: '1.5s' };
    if ((d.errorRate ?? 0) > 0.05)   return { stroke: '#fa8c16', particle: '#fa8c16', dur: '2.5s' };
    if ((d.latencyP99ms ?? 0) > 500) return { stroke: '#faad14', particle: '#faad14', dur: '2.5s' };
    if (d.requestRate > 0)            return { stroke: '#52c41a', particle: '#52c41a', dur: '1.8s' };
  }
  return HEALTH_EDGE[d.health ?? 'unknown'] ?? HEALTH_EDGE.unknown;
}

const ParticleEdge: React.FC<EdgeProps> = ({
  id, sourceX, sourceY, targetX, targetY,
  sourcePosition, targetPosition, data,
}) => {
  const d = (data ?? {}) as ParticleEdgeData;
  const style = resolveEdgeStyle(d);
  const pathId = `pe-${id}`;
  const hasIstio = d.requestRate !== undefined;

  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX, sourceY, sourcePosition,
    targetX, targetY, targetPosition,
  });

  // Label: show rps for istio-flow edges; error rate for any edge with high error
  const label = d.kind === 'istio-flow' && d.requestRate !== undefined
    ? (d.requestRate > 0
        ? `${d.requestRate.toFixed(1)} rps${(d.errorRate ?? 0) > 0.01 ? ` · ${((d.errorRate ?? 0) * 100).toFixed(0)}% err` : ''}`
        : null)
    : hasIstio && (d.errorRate ?? 0) > 0.01
    ? `${((d.errorRate ?? 0) * 100).toFixed(1)}% err`
    : null;

  return (
    <>
      <g>
        <path
          id={pathId}
          d={edgePath}
          stroke={style.stroke}
          strokeWidth={1.5}
          fill="none"
          strokeOpacity={0.6}
        />
        <path d={edgePath} fill="none" stroke="transparent" strokeWidth={12} />
        {Array.from({ length: PARTICLE_COUNT }).map((_, i) => (
          <circle key={i} r={3} fill={style.particle} fillOpacity={0.85}>
            <animateMotion
              dur={style.dur}
              begin={`${(i / PARTICLE_COUNT) * parseFloat(style.dur)}s`}
              repeatCount="indefinite"
            >
              <mpath href={`#${pathId}`} />
            </animateMotion>
          </circle>
        ))}
      </g>
      {label && (
        <EdgeLabelRenderer>
          <div
            style={{
              position: 'absolute',
              transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
              pointerEvents: 'all',
              fontSize: 10,
              background: style.stroke,
              color: '#fff',
              borderRadius: 3,
              padding: '1px 4px',
              fontWeight: 600,
              whiteSpace: 'nowrap',
            }}
          >
            {label}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
};

// ---- nodeTypes / edgeTypes (stable references outside component) ----

const nodeTypes = {
  workload: WorkloadNode,
  service: ServiceNode,
  ingress: IngressNode,
};

const edgeTypes = {
  particle: ParticleEdge,
};

// ---- Dagre layout ----

const runDagreLayout = (nodes: Node[], edges: Edge[]): Node[] => {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: 'LR', nodesep: 50, ranksep: 100 });

  nodes.forEach((n) => g.setNode(n.id, { width: NODE_W, height: NODE_H }));
  edges.forEach((e) => g.setEdge(e.source, e.target));
  dagre.layout(g);

  return nodes.map((n) => {
    const pos = g.node(n.id);
    return { ...n, position: { x: pos.x - NODE_W / 2, y: pos.y - NODE_H / 2 } };
  });
};

// ---- Main component ----

interface ClusterTopologyGraphProps {
  topoNodes: NetworkNode[];
  topoEdges: NetworkEdge[];
  onNodeClick?: (node: NetworkNode) => void;
}

const ClusterTopologyGraph: React.FC<ClusterTopologyGraphProps> = ({ topoNodes, topoEdges, onNodeClick }) => {
  const handleNodeClick = useCallback<NodeMouseHandler>(
    (_evt, rfNode) => {
      const original = topoNodes.find((n) => n.id === rfNode.id);
      if (original) onNodeClick?.(original);
    },
    [topoNodes, onNodeClick],
  );

  const { nodes, edges } = useMemo(() => {
    const rfNodes: Node[] = topoNodes.map((n) => {
      if (n.kind === 'Service') {
        return {
          id: n.id,
          type: 'service',
          position: { x: 0, y: 0 },
          sourcePosition: Position.Right,
          targetPosition: Position.Left,
          data: {
            name: n.name,
            namespace: n.namespace,
            serviceType: n.serviceType ?? 'ClusterIP',
            clusterIP: n.clusterIP,
          } as ServiceNodeData,
        };
      }
      if (n.kind === 'Ingress') {
        return {
          id: n.id,
          type: 'ingress',
          position: { x: 0, y: 0 },
          sourcePosition: Position.Right,
          targetPosition: Position.Left,
          data: {
            name: n.name,
            namespace: n.namespace,
            ingressClass: n.ingressClass,
          } as IngressNodeData,
        };
      }
      return {
        id: n.id,
        type: 'workload',
        position: { x: 0, y: 0 },
        sourcePosition: Position.Right,
        targetPosition: Position.Left,
        data: {
          name: n.name,
          namespace: n.namespace,
          workloadKind: n.workloadKind ?? 'Pod',
          readyCount: n.readyCount,
          totalCount: n.totalCount,
        } as WorkloadNodeData,
      };
    });

    const rfEdges: Edge[] = topoEdges.map((e, i) => ({
      id: `e-${i}-${e.source}-${e.target}`,
      source: e.source,
      target: e.target,
      type: 'particle',
      data: {
        kind: e.kind,
        health: e.health,
        ports: e.ports,
        requestRate: e.requestRate,
        errorRate: e.errorRate,
        latencyP99ms: e.latencyP99ms,
      } as ParticleEdgeData,
    }));

    const layoutedNodes = runDagreLayout(rfNodes, rfEdges);
    return { nodes: layoutedNodes, edges: rfEdges };
  }, [topoNodes, topoEdges]);

  return (
    <div style={{ height: 560, border: '1px solid #f0f0f0', borderRadius: 8, overflow: 'hidden' }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        nodesDraggable
        nodesConnectable={false}
        elementsSelectable
        onNodeClick={handleNodeClick}
      >
        <Background />
        <Controls />
        <MiniMap
          nodeColor={(n) =>
            n.type === 'service'  ? '#fa8c16' :
            n.type === 'ingress'  ? '#722ed1' :
            WORKLOAD_KIND_COLOR[(n.data as WorkloadNodeData).workloadKind ?? ''] ?? '#8c8c8c'
          }
        />
      </ReactFlow>
    </div>
  );
};

export default ClusterTopologyGraph;

// Export legend data for the parent tab
export const HEALTH_LEGEND = Object.entries(HEALTH_EDGE).map(([k, v]) => ({
  key: k,
  color: v.stroke,
}));

export { WORKLOAD_KIND_COLOR };

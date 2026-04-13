import React, { useMemo, useCallback, useRef } from 'react';
import { Tag, Badge, Tooltip, Button } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import { toPng } from 'html-to-image';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  Panel,
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
import { WORKLOAD_KIND_COLOR } from './constants';

// ---- Constants ----

const NODE_W = 160; // px: 適合顯示 18 字元名稱 + badge，不至於撐開 Dagre 間距
const NODE_H = 72;  // px: 容納 kind tag + name + namespace + ready count 四行

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

// ---- Helpers ----

const MAX_LABEL = 18;
const truncate = (s: string) => s.length > MAX_LABEL ? s.slice(0, MAX_LABEL) + '…' : s;

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
      <Tooltip title={d.name.length > MAX_LABEL ? d.name : undefined} placement="top">
        <div style={{ fontSize: 12, fontWeight: 600, lineHeight: 1.3, overflow: 'hidden', whiteSpace: 'nowrap' }}>
          {truncate(d.name)}
        </div>
      </Tooltip>
      <div style={{ fontSize: 10, color: '#8c8c8c', marginTop: 1, overflow: 'hidden', whiteSpace: 'nowrap' }}>
        {truncate(d.namespace)}
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
      <Tooltip title={d.name.length > MAX_LABEL ? d.name : undefined} placement="top">
        <div style={{ fontSize: 12, fontWeight: 600, lineHeight: 1.3, overflow: 'hidden', whiteSpace: 'nowrap' }}>
          {truncate(d.name)}
        </div>
      </Tooltip>
      <div style={{ fontSize: 10, color: '#8c8c8c', marginTop: 1, overflow: 'hidden', whiteSpace: 'nowrap' }}>
        {truncate(d.namespace)}
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
      <Tooltip title={d.name.length > MAX_LABEL ? d.name : undefined} placement="top">
        <div style={{ fontSize: 12, fontWeight: 600, lineHeight: 1.3, overflow: 'hidden', whiteSpace: 'nowrap' }}>
          {truncate(d.name)}
        </div>
      </Tooltip>
      <div style={{ fontSize: 10, color: '#8c8c8c', marginTop: 1, overflow: 'hidden', whiteSpace: 'nowrap' }}>
        {truncate(d.namespace)}
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
  policyStatus?: string;
  policyName?: string;
  // Phase F: Hubble
  hubbleFlowRate?: number;
  hubbleDropRate?: number;
  hubbleDropReason?: string;
}

interface EdgeStyle {
  stroke: string;
  particle: string;
  dur: string;
  dashed?: boolean;
  noParticles?: boolean;
}

function resolveEdgeStyle(d: ParticleEdgeData): EdgeStyle {
  // NetworkPolicy overlay (Phase E) — checked first, overrides all other styles
  if (d.policyStatus === 'policy-deny') {
    return { stroke: '#ff4d4f', particle: '#ff4d4f', dur: '4s', dashed: true, noParticles: true };
  }
  if (d.policyStatus === 'policy-restricted') {
    return { stroke: '#fa8c16', particle: '#fa8c16', dur: '3s', dashed: true, noParticles: true };
  }
  if (d.policyStatus === 'policy-allow') {
    return { stroke: '#1677ff', particle: '#1677ff', dur: '2s' };
  }
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
  // Hubble drop-rate override (Phase F)
  if (d.hubbleFlowRate !== undefined || d.hubbleDropRate !== undefined) {
    if ((d.hubbleDropRate ?? 0) > 0.5)  return { stroke: '#ff4d4f', particle: '#ff4d4f', dur: '1.5s' };
    if ((d.hubbleDropRate ?? 0) > 0.1)  return { stroke: '#fa8c16', particle: '#fa8c16', dur: '2.5s' };
    if ((d.hubbleFlowRate ?? 0) > 0)    return { stroke: '#52c41a', particle: '#52c41a', dur: '1.8s' };
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

  // Label logic
  const label = d.policyStatus === 'policy-deny'
    ? `🔒 ${d.policyName ?? 'blocked'}`
    : d.policyStatus === 'policy-restricted'
    ? `⚠ ${d.policyName ?? 'restricted'}`
    : d.kind === 'istio-flow' && d.requestRate !== undefined && d.requestRate > 0
    ? `${d.requestRate.toFixed(1)} rps${(d.errorRate ?? 0) > 0.01 ? ` · ${((d.errorRate ?? 0) * 100).toFixed(0)}% err` : ''}`
    : hasIstio && (d.errorRate ?? 0) > 0.01
    ? `${((d.errorRate ?? 0) * 100).toFixed(1)}% err`
    : (d.hubbleDropRate ?? 0) > 0.01
    ? `⬇ ${(d.hubbleDropRate! * 100).toFixed(0)}% drop`
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
          strokeOpacity={0.7}
          strokeDasharray={style.dashed ? '7 4' : undefined}
        />
        <path d={edgePath} fill="none" stroke="transparent" strokeWidth={12} />
        {!style.noParticles && Array.from({ length: PARTICLE_COUNT }).map((_, i) => (
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
  const containerRef = useRef<HTMLDivElement>(null);

  const exportPng = useCallback(() => {
    const el = containerRef.current?.querySelector<HTMLElement>('.react-flow__renderer');
    if (!el) return;
    toPng(el, { backgroundColor: '#fff', pixelRatio: 2 })
      .then((dataUrl) => {
        const a = document.createElement('a');
        a.href = dataUrl;
        a.download = 'topology.png';
        a.click();
      })
      .catch(() => {/* ignore */});
  }, []);

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
        policyStatus: e.policyStatus,
        policyName: e.policyName,
        hubbleFlowRate: e.hubbleFlowRate,
        hubbleDropRate: e.hubbleDropRate,
        hubbleDropReason: e.hubbleDropReason,
      } as ParticleEdgeData,
    }));

    const layoutedNodes = runDagreLayout(rfNodes, rfEdges);
    return { nodes: layoutedNodes, edges: rfEdges };
  }, [topoNodes, topoEdges]);

  return (
    <div ref={containerRef} style={{ height: 'calc(100vh - 320px)', minHeight: 480, border: '1px solid #f0f0f0', borderRadius: 8, overflow: 'hidden' }}>
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
        <Panel position="top-right">
          <Button
            size="small"
            icon={<DownloadOutlined />}
            onClick={exportPng}
            style={{ background: 'rgba(255,255,255,0.9)', fontSize: 11 }}
          >
            PNG
          </Button>
        </Panel>
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
// eslint-disable-next-line react-refresh/only-export-components
export const HEALTH_LEGEND = Object.entries(HEALTH_EDGE).map(([k, v]) => ({
  key: k,
  color: v.stroke,
}));

// eslint-disable-next-line react-refresh/only-export-components
export { WORKLOAD_KIND_COLOR };

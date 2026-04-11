import React, { useMemo } from 'react';
import { Tag, Tooltip, Badge } from 'antd';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  MarkerType,
  type Node,
  type Edge,
  type NodeProps,
  Position,
  Handle,
} from '@xyflow/react';
import dagre from '@dagrejs/dagre';
import '@xyflow/react/dist/style.css';
import type { ClusterSection, CrossEdge, NetworkNode } from '../../services/networkTopologyService';
import { WORKLOAD_KIND_COLOR, HEALTH_COLOR } from '../network/constants';

// ── Layout constants ────────────────────────────────────────────────────────

const NODE_W = 160;
const NODE_H = 72;
const CLUSTER_PADDING   = 40;   // inner padding around nodes
const CLUSTER_HEADER_H  = 36;   // header bar height
const CLUSTER_SPACING   = 80;   // gap between cluster groups
const CLUSTER_MIN_W     = 220;  // minimum group width for empty clusters
const CLUSTER_MIN_H     = 80;

// Palette for cluster group borders/headers (cycles if > 10 clusters)
const CLUSTER_PALETTE = [
  '#1677ff', '#52c41a', '#722ed1', '#fa8c16',
  '#13c2c2', '#eb2f96', '#2f54eb', '#389e0d',
  '#cf1322', '#d48806',
];

const MAX_LABEL = 18;
const truncate = (s: string) => s.length > MAX_LABEL ? s.slice(0, MAX_LABEL) + '…' : s;

// ── Custom node types ───────────────────────────────────────────────────────

interface WorkloadData extends Record<string, unknown> {
  name: string; namespace: string; workloadKind: string;
  readyCount: number; totalCount: number;
}
interface ServiceData extends Record<string, unknown> {
  name: string; namespace: string; serviceType?: string; clusterIP?: string;
}
interface IngressData extends Record<string, unknown> {
  name: string; namespace: string; ingressClass?: string;
}
interface ClusterGroupData extends Record<string, unknown> {
  clusterName: string; clusterId: number; color: string; nodeCount: number;
}

const WorkloadNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as WorkloadData;
  const color = WORKLOAD_KIND_COLOR[d.workloadKind] ?? '#8c8c8c';
  const status = d.readyCount === d.totalCount && d.totalCount > 0 ? 'success'
    : d.readyCount > 0 ? 'warning' : 'error';
  return (
    <div style={{ width: NODE_W, minHeight: NODE_H, background: '#fff', border: `1.5px solid ${color}`, borderRadius: 8, padding: '6px 10px', boxSizing: 'border-box' }}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, marginBottom: 2 }}>
        <Tag color={color} style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', marginRight: 0 }}>{d.workloadKind}</Tag>
        <Badge status={status} />
      </div>
      <Tooltip title={d.name.length > MAX_LABEL ? d.name : undefined} placement="top">
        <div style={{ fontSize: 12, fontWeight: 600, overflow: 'hidden', whiteSpace: 'nowrap' }}>{truncate(d.name)}</div>
      </Tooltip>
      <div style={{ fontSize: 10, color: '#8c8c8c', overflow: 'hidden', whiteSpace: 'nowrap' }}>{truncate(d.namespace)}</div>
      <div style={{ fontSize: 10, color: d.readyCount < d.totalCount ? '#fa8c16' : '#52c41a' }}>{d.readyCount}/{d.totalCount} Ready</div>
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

const ServiceNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as ServiceData;
  return (
    <div style={{ width: NODE_W, minHeight: NODE_H, background: '#fff', border: '1.5px solid #fa8c16', borderRadius: 8, padding: '6px 10px', boxSizing: 'border-box' }}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ marginBottom: 2 }}>
        <Tag color="orange" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>Service</Tag>
        <Tag style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', marginLeft: 2 }}>{d.serviceType || 'ClusterIP'}</Tag>
      </div>
      <Tooltip title={d.name.length > MAX_LABEL ? d.name : undefined} placement="top">
        <div style={{ fontSize: 12, fontWeight: 600, overflow: 'hidden', whiteSpace: 'nowrap' }}>{truncate(d.name)}</div>
      </Tooltip>
      <div style={{ fontSize: 10, color: '#8c8c8c', overflow: 'hidden', whiteSpace: 'nowrap' }}>{truncate(d.namespace)}</div>
      {d.clusterIP && d.clusterIP !== 'None' && (
        <div style={{ fontSize: 10, color: '#8c8c8c' }}>{d.clusterIP}</div>
      )}
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

const IngressNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as IngressData;
  return (
    <div style={{ width: NODE_W, minHeight: NODE_H, background: '#fff', border: '1.5px solid #722ed1', borderRadius: 8, padding: '6px 10px', boxSizing: 'border-box' }}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ marginBottom: 2 }}>
        <Tag color="purple" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>Ingress</Tag>
        {d.ingressClass && <Tag style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', marginLeft: 2 }}>{d.ingressClass}</Tag>}
      </div>
      <Tooltip title={d.name.length > MAX_LABEL ? d.name : undefined} placement="top">
        <div style={{ fontSize: 12, fontWeight: 600, overflow: 'hidden', whiteSpace: 'nowrap' }}>{truncate(d.name)}</div>
      </Tooltip>
      <div style={{ fontSize: 10, color: '#8c8c8c', overflow: 'hidden', whiteSpace: 'nowrap' }}>{truncate(d.namespace)}</div>
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

// Cluster group node — renders a labelled box with colored header
const ClusterGroupNode: React.FC<NodeProps> = (props) => {
  const d = props.data as ClusterGroupData;
  const nodeStyle = (props as unknown as { style?: { width?: number; height?: number } }).style;
  const w = nodeStyle?.width ?? CLUSTER_MIN_W;
  const h = nodeStyle?.height ?? CLUSTER_MIN_H;
  return (
    <div
      style={{
        width: w,
        height: h,
        border: `2px solid ${d.color}`,
        borderRadius: 12,
        overflow: 'hidden',
        background: 'transparent',
        boxSizing: 'border-box',
        pointerEvents: 'none',
      }}
    >
      {/* Header bar */}
      <div
        style={{
          height: CLUSTER_HEADER_H,
          background: d.color,
          display: 'flex',
          alignItems: 'center',
          paddingLeft: 12,
          gap: 8,
        }}
      >
        <span style={{ color: '#fff', fontWeight: 700, fontSize: 13 }}>{d.clusterName}</span>
        <Tag
          style={{
            fontSize: 10,
            lineHeight: '16px',
            padding: '0 4px',
            marginRight: 0,
            background: 'rgba(255,255,255,0.25)',
            border: 'none',
            color: '#fff',
          }}
        >
          {d.nodeCount} nodes
        </Tag>
      </div>
    </div>
  );
};

// ── Layout ──────────────────────────────────────────────────────────────────

function buildNodeData(n: NetworkNode): Record<string, unknown> {
  if (n.kind === 'Service') {
    return { name: n.name, namespace: n.namespace, serviceType: n.serviceType, clusterIP: n.clusterIP };
  }
  if (n.kind === 'Ingress') {
    return { name: n.name, namespace: n.namespace, ingressClass: n.ingressClass };
  }
  return { name: n.name, namespace: n.namespace, workloadKind: n.workloadKind ?? 'Pod', readyCount: n.readyCount, totalCount: n.totalCount };
}

function nodeTypeOf(n: NetworkNode): string {
  if (n.kind === 'Service') return 'serviceNode';
  if (n.kind === 'Ingress') return 'ingressNode';
  return 'workloadNode';
}

function computeLayout(sections: ClusterSection[], crossEdges: CrossEdge[]): { nodes: Node[]; edges: Edge[] } {
  const rfNodes: Node[] = [];
  const rfEdges: Edge[] = [];
  let offsetX = 0;

  sections.forEach((sec, si) => {
    const color = CLUSTER_PALETTE[si % CLUSTER_PALETTE.length];

    if (sec.nodes.length === 0) {
      rfNodes.push({
        id: `cluster-${sec.clusterId}`,
        type: 'clusterGroup',
        position: { x: offsetX, y: 0 },
        data: { clusterName: sec.clusterName, clusterId: sec.clusterId, color, nodeCount: 0 } satisfies ClusterGroupData,
        style: { width: CLUSTER_MIN_W, height: CLUSTER_MIN_H },
        draggable: false,
        selectable: false,
      });
      offsetX += CLUSTER_MIN_W + CLUSTER_SPACING;
      return;
    }

    // Run Dagre within this cluster
    const g = new dagre.graphlib.Graph();
    g.setGraph({ rankdir: 'TB', nodesep: 50, ranksep: 50 });
    g.setDefaultEdgeLabel(() => ({}));
    for (const n of sec.nodes) g.setNode(n.id, { width: NODE_W, height: NODE_H });
    for (const e of sec.edges) g.setEdge(e.source, e.target);
    dagre.layout(g);

    // Bounding box
    let maxX = 0, maxY = 0;
    for (const n of sec.nodes) {
      const pos = g.node(n.id);
      if (pos) {
        maxX = Math.max(maxX, pos.x + NODE_W / 2);
        maxY = Math.max(maxY, pos.y + NODE_H / 2);
      }
    }
    const groupW = maxX + CLUSTER_PADDING * 2;
    const groupH = maxY + CLUSTER_PADDING * 2 + CLUSTER_HEADER_H;

    // Group node
    rfNodes.push({
      id: `cluster-${sec.clusterId}`,
      type: 'clusterGroup',
      position: { x: offsetX, y: 0 },
      data: { clusterName: sec.clusterName, clusterId: sec.clusterId, color, nodeCount: sec.nodes.length } satisfies ClusterGroupData,
      style: { width: groupW, height: groupH },
      draggable: false,
      selectable: false,
    });

    // Child nodes
    for (const n of sec.nodes) {
      const pos = g.node(n.id);
      if (!pos) continue;
      rfNodes.push({
        id: n.id,
        type: nodeTypeOf(n),
        parentId: `cluster-${sec.clusterId}`,
        extent: 'parent',
        position: {
          x: CLUSTER_PADDING + pos.x - NODE_W / 2,
          y: CLUSTER_HEADER_H + CLUSTER_PADDING + pos.y - NODE_H / 2,
        },
        data: buildNodeData(n),
        draggable: false,
      });
    }

    // Intra-cluster edges
    for (const e of sec.edges) {
      const stroke = HEALTH_COLOR[e.health] ?? '#d9d9d9';
      rfEdges.push({
        id: `e:${e.source}:${e.target}`,
        source: e.source,
        target: e.target,
        type: 'smoothstep',
        style: { stroke, strokeWidth: 1.5 },
        animated: e.health === 'healthy',
        label: e.ports,
        labelStyle: { fontSize: 9 },
      });
    }

    offsetX += groupW + CLUSTER_SPACING;
  });

  // Cross-cluster edges
  for (const ce of crossEdges) {
    rfEdges.push({
      id: `cross:${ce.source}:${ce.target}`,
      source: ce.source,
      target: ce.target,
      type: 'smoothstep',
      style: { stroke: '#722ed1', strokeWidth: 2, strokeDasharray: '6 3' },
      label: ce.label,
      labelStyle: { fontSize: 10, fill: '#722ed1', fontWeight: 600 },
      markerEnd: { type: MarkerType.ArrowClosed, color: '#722ed1' },
    });
  }

  return { nodes: rfNodes, edges: rfEdges };
}

// ── Graph component ─────────────────────────────────────────────────────────

const nodeTypes = {
  workloadNode:  WorkloadNode,
  serviceNode:   ServiceNode,
  ingressNode:   IngressNode,
  clusterGroup:  ClusterGroupNode,
};

interface Props {
  sections: ClusterSection[];
  crossEdges: CrossEdge[];
}

const MultiClusterTopologyGraph: React.FC<Props> = ({ sections, crossEdges }) => {
  const { nodes, edges } = useMemo(
    () => computeLayout(sections, crossEdges),
    [sections, crossEdges],
  );

  return (
    <div style={{ width: '100%', height: 600 }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        minZoom={0.15}
        maxZoom={2}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={16} color="#e8eaec" />
        <Controls showInteractive={false} />
        <MiniMap zoomable pannable nodeStrokeWidth={2} />
      </ReactFlow>
    </div>
  );
};

export default MultiClusterTopologyGraph;

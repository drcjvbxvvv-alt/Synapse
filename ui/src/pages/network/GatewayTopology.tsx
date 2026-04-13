
import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import EmptyState from '@/components/EmptyState';
import { Button, Spin, Tag, App, Badge, Tooltip } from 'antd';
import { ReloadOutlined, DownloadOutlined } from '@ant-design/icons';
import { toPng } from 'html-to-image';
import { useTranslation } from 'react-i18next';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  Panel,
  type Node,
  type Edge,
  type NodeProps,
  Position,
  Handle,
} from '@xyflow/react';
import dagre from '@dagrejs/dagre';
import '@xyflow/react/dist/style.css';
import { gatewayService } from '../../services/gatewayService';
import type { GatewayTabProps, TopologyNode } from './gatewayTypes';

// ---- Constants ----

const NODE_W = 160; // px
const NODE_H = 72;  // px

const KIND_COLOR: Record<string, string> = {
  GatewayClass: '#722ed1',
  Gateway:      '#1677ff',
  HTTPRoute:    '#13c2c2',
  GRPCRoute:    '#52c41a',
  Service:      '#fa8c16',
};

const MAX_LABEL = 18;
const truncate = (s: string) => s.length > MAX_LABEL ? s.slice(0, MAX_LABEL) + '…' : s;

// ---- Custom node data types ----

interface GatewayClassNodeData extends Record<string, unknown> {
  name: string;
  status?: string;
}

interface GatewayNodeData extends Record<string, unknown> {
  name: string;
  namespace: string;
  status?: string;
  subKind?: string;
}

interface RouteNodeData extends Record<string, unknown> {
  name: string;
  namespace: string;
  kind: string;
  hostnames?: string[];
}

interface ServiceNodeData extends Record<string, unknown> {
  name: string;
  namespace: string;
}

// ---- Custom node components ----

const GatewayClassNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as GatewayClassNodeData;
  const color = KIND_COLOR.GatewayClass;
  const accepted = d.status === 'Accepted';
  const status = accepted ? 'success' : 'warning';

  return (
    <div style={{ width: NODE_W, minHeight: NODE_H, background: '#fff', border: `1.5px solid ${color}`, borderRadius: 8, padding: '6px 10px', boxSizing: 'border-box' }}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, marginBottom: 2 }}>
        <Tag color="purple" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', marginRight: 0 }}>
          GatewayClass
        </Tag>
        <Badge status={status} />
      </div>
      <Tooltip title={d.name.length > MAX_LABEL ? d.name : undefined} placement="top">
        <div style={{ fontSize: 12, fontWeight: 600, lineHeight: 1.3, overflow: 'hidden', whiteSpace: 'nowrap' }}>
          {truncate(d.name)}
        </div>
      </Tooltip>
      {d.status && (
        <div style={{ fontSize: 10, color: accepted ? '#52c41a' : '#fa8c16', marginTop: 2 }}>
          {d.status}
        </div>
      )}
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

const GatewayNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as GatewayNodeData;
  const color = KIND_COLOR.Gateway;
  const ready = d.status === 'Ready';
  const status = ready ? 'success' : d.status ? 'warning' : 'default';

  return (
    <div style={{ width: NODE_W, minHeight: NODE_H, background: '#fff', border: `1.5px solid ${color}`, borderRadius: 8, padding: '6px 10px', boxSizing: 'border-box' }}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, marginBottom: 2, flexWrap: 'wrap' }}>
        <Tag color="blue" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px', marginRight: 0 }}>
          Gateway
        </Tag>
        {d.subKind && (
          <Tag style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>
            {d.subKind}
          </Tag>
        )}
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
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

const RouteNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as RouteNodeData;
  const isHTTP = d.kind === 'HTTPRoute';
  const color = isHTTP ? KIND_COLOR.HTTPRoute : KIND_COLOR.GRPCRoute;
  const tagColor = isHTTP ? 'cyan' : 'green';
  const firstHost = d.hostnames?.[0];

  return (
    <div style={{ width: NODE_W, minHeight: NODE_H, background: '#fff', border: `1.5px solid ${color}`, borderRadius: 8, padding: '6px 10px', boxSizing: 'border-box' }}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ marginBottom: 2 }}>
        <Tag color={tagColor} style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>
          {d.kind}
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
      {firstHost && (
        <Tooltip title={firstHost.length > MAX_LABEL ? firstHost : undefined} placement="bottom">
          <div style={{ fontSize: 10, color: color, marginTop: 2, overflow: 'hidden', whiteSpace: 'nowrap' }}>
            {truncate(firstHost)}
            {(d.hostnames?.length ?? 0) > 1 && ` +${(d.hostnames?.length ?? 0) - 1}`}
          </div>
        </Tooltip>
      )}
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

const GwServiceNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as ServiceNodeData;

  return (
    <div style={{ width: NODE_W, minHeight: NODE_H, background: '#fff', border: '1.5px solid #fa8c16', borderRadius: 8, padding: '6px 10px', boxSizing: 'border-box' }}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div style={{ marginBottom: 2 }}>
        <Tag color="orange" style={{ fontSize: 10, lineHeight: '16px', padding: '0 4px' }}>
          Service
        </Tag>
      </div>
      <Tooltip title={d.name.length > MAX_LABEL ? d.name : undefined} placement="top">
        <div style={{ fontSize: 12, fontWeight: 600, lineHeight: 1.3, overflow: 'hidden', whiteSpace: 'nowrap' }}>
          {truncate(d.name)}
        </div>
      </Tooltip>
      <div style={{ fontSize: 10, color: '#8c8c8c', marginTop: 1, overflow: 'hidden', whiteSpace: 'nowrap' }}>
        {truncate(d.namespace ?? '')}
      </div>
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

// ---- nodeTypes map (stable reference) ----

const nodeTypes = {
  gatewayClass: GatewayClassNode,
  gateway:      GatewayNode,
  route:        RouteNode,
  gwService:    GwServiceNode,
};

// ---- Dagre layout ----

const runDagreLayout = (nodes: Node[], edges: Edge[]) => {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: 'LR', nodesep: 40, ranksep: 80 });
  nodes.forEach((n) => g.setNode(n.id, { width: NODE_W, height: NODE_H }));
  edges.forEach((e) => g.setEdge(e.source, e.target));
  dagre.layout(g);
  return nodes.map((n) => {
    const pos = g.node(n.id);
    return { ...n, position: { x: pos.x - NODE_W / 2, y: pos.y - NODE_H / 2 } };
  });
};

// ---- Build flow elements ----

const buildFlowElements = (topoNodes: TopologyNode[]) => {
  const nodes: Node[] = topoNodes.map((n) => {
    if (n.kind === 'GatewayClass') {
      return {
        id: n.id, type: 'gatewayClass',
        data: { name: n.name, status: n.status },
        position: { x: 0, y: 0 },
        sourcePosition: Position.Right, targetPosition: Position.Left,
      };
    }
    if (n.kind === 'Gateway') {
      return {
        id: n.id, type: 'gateway',
        data: { name: n.name, namespace: n.namespace ?? '', status: n.status, subKind: n.subKind },
        position: { x: 0, y: 0 },
        sourcePosition: Position.Right, targetPosition: Position.Left,
      };
    }
    if (n.kind === 'HTTPRoute' || n.kind === 'GRPCRoute') {
      return {
        id: n.id, type: 'route',
        data: { name: n.name, namespace: n.namespace ?? '', kind: n.kind, hostnames: n.hostnames },
        position: { x: 0, y: 0 },
        sourcePosition: Position.Right, targetPosition: Position.Left,
      };
    }
    // Service
    return {
      id: n.id, type: 'gwService',
      data: { name: n.name, namespace: n.namespace ?? '' },
      position: { x: 0, y: 0 },
      sourcePosition: Position.Right, targetPosition: Position.Left,
    };
  });

  return nodes;
};

// ---- Component ----

const GatewayTopology: React.FC<GatewayTabProps> = ({ clusterId }) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');
  const [loading, setLoading] = useState(false);
  const [topoNodes, setTopoNodes] = useState<TopologyNode[]>([]);
  const [topoEdges, setTopoEdges] = useState<{ source: string; target: string }[]>([]);
  const containerRef = useRef<HTMLDivElement>(null);

  const loadTopology = useCallback(async () => {
    setLoading(true);
    try {
      const data = await gatewayService.getTopology(clusterId);
      setTopoNodes(data.nodes ?? []);
      setTopoEdges(data.edges ?? []);
    } catch {
      message.error(t('gatewayapi.topology.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  useEffect(() => { loadTopology(); }, [loadTopology]);

  const { nodes, edges } = useMemo(() => {
    const rawNodes = buildFlowElements(topoNodes);
    const rawEdges: Edge[] = topoEdges.map((e, i) => ({
      id: `e-${i}`,
      source: e.source,
      target: e.target,
      type: 'smoothstep',
      animated: false,
      style: { stroke: '#bfbfbf', strokeWidth: 1.5 },
      markerEnd: { type: 'arrowclosed' as const, color: '#bfbfbf' },
    }));
    const layoutedNodes = runDagreLayout(rawNodes, rawEdges);
    return { nodes: layoutedNodes, edges: rawEdges };
  }, [topoNodes, topoEdges]);

  const exportPng = useCallback(async () => {
    const el = containerRef.current?.querySelector<HTMLElement>('.react-flow__renderer');
    if (!el) return;
    try {
      const url = await toPng(el, { backgroundColor: '#fff', pixelRatio: 2 });
      const a = document.createElement('a');
      a.href = url;
      a.download = `gateway-topology-${clusterId}.png`;
      a.click();
    } catch {
      message.error(t('gatewayapi.topology.exportError'));
    }
  }, [clusterId, message, t]);

  const miniMapNodeColor = useCallback(
    (n: Node) => KIND_COLOR[(n.data as { kind?: string }).kind ?? n.type?.replace('gwService', 'Service').replace('gateway', 'Gateway').replace('gatewayClass', 'GatewayClass').replace('route', '') ?? ''] ?? '#bfbfbf',
    [],
  );

  return (
    <div>
      <div style={{ marginBottom: 12, display: 'flex', gap: 8, alignItems: 'center' }}>
        <Button icon={<ReloadOutlined />} onClick={loadTopology} loading={loading}>
          {t('gatewayapi.topology.refresh')}
        </Button>
        <div style={{ flex: 1 }} />
        <div style={{ display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap' }}>
          {Object.entries(KIND_COLOR).map(([kind, color]) => (
            <Tag key={kind} color={color} style={{ fontSize: 11 }}>{kind}</Tag>
          ))}
        </div>
      </div>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin tip={t('gatewayapi.topology.loading')} />
        </div>
      ) : nodes.length === 0 ? (
        <EmptyState description={t('gatewayapi.topology.empty')} style={{ padding: 60 }} />
      ) : (
        <div ref={containerRef} style={{ height: 560, border: '1px solid #f0f0f0', borderRadius: 8, overflow: 'hidden' }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            fitView
            fitViewOptions={{ padding: 0.2 }}
            nodesDraggable
            nodesConnectable={false}
            elementsSelectable
          >
            <Background />
            <Controls />
            <MiniMap nodeColor={miniMapNodeColor} />
            <Panel position="top-right">
              <Button
                size="small"
                icon={<DownloadOutlined />}
                onClick={exportPng}
                style={{ background: '#fff', boxShadow: '0 1px 4px rgba(0,0,0,.15)' }}
              >
                PNG
              </Button>
            </Panel>
          </ReactFlow>
        </div>
      )}
    </div>
  );
};

export default GatewayTopology;

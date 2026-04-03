import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  MarkerType,
  Handle,
  Position,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import dagre from '@dagrejs/dagre';
import { Alert, Button, Empty, Select, Spin, Tag, Tooltip } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { MeshService, type MeshNode, type MeshEdge } from '../../services/meshService';

const NODE_W = 180;
const NODE_H = 80;

// Error rate color coding: green < 1%, yellow 1–5%, red > 5%
function errorRateColor(rate: number): string {
  if (rate > 0.05) return '#ff4d4f';
  if (rate > 0.01) return '#faad14';
  return '#52c41a';
}

// ---- Custom node component ----
interface MeshNodeData extends Record<string, unknown> {
  label: string;
  namespace: string;
  rps: number;
  errorRate: number;
  p99ms: number;
}

const MeshNodeComponent: React.FC<{ data: MeshNodeData }> = ({ data }) => {
  const color = errorRateColor(data.errorRate);
  return (
    <div
      style={{
        background: '#fff',
        border: `2px solid ${color}`,
        borderRadius: 8,
        padding: '8px 12px',
        minWidth: NODE_W,
        textAlign: 'center',
        boxShadow: '0 2px 6px rgba(0,0,0,0.1)',
      }}
    >
      <Handle type="target" position={Position.Left} style={{ background: color }} />
      <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 4 }}>{data.label}</div>
      <Tag color="blue" style={{ fontSize: 10 }}>{data.namespace}</Tag>
      {data.rps > 0 && (
        <div style={{ marginTop: 4, fontSize: 11, color: '#666' }}>
          <Tooltip title="Requests per second">
            <span>{data.rps.toFixed(2)} RPS</span>
          </Tooltip>
          {' · '}
          <Tooltip title="Error rate">
            <span style={{ color }}>{(data.errorRate * 100).toFixed(1)}% err</span>
          </Tooltip>
        </div>
      )}
      <Handle type="source" position={Position.Right} style={{ background: color }} />
    </div>
  );
};

const nodeTypes = { meshNode: MeshNodeComponent };

// ---- Dagre layout ----
function layoutGraph(nodes: Node[], edges: Edge[]): { nodes: Node[]; edges: Edge[] } {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: 'LR', nodesep: 60, ranksep: 120 });

  nodes.forEach(n => g.setNode(n.id, { width: NODE_W, height: NODE_H }));
  edges.forEach(e => g.setEdge(e.source, e.target));

  dagre.layout(g);

  const laidOutNodes = nodes.map(n => {
    const pos = g.node(n.id);
    return { ...n, position: { x: pos.x - NODE_W / 2, y: pos.y - NODE_H / 2 } };
  });

  return { nodes: laidOutNodes, edges };
}

// ---- Convert API data to ReactFlow data ----
function toFlowData(apiNodes: MeshNode[], apiEdges: MeshEdge[]): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = apiNodes.map(n => ({
    id: n.id,
    type: 'meshNode',
    position: { x: 0, y: 0 },
    data: {
      label: n.name,
      namespace: n.namespace,
      rps: n.rps ?? 0,
      errorRate: n.errorRate ?? 0,
      p99ms: n.p99ms ?? 0,
    } as MeshNodeData,
  }));

  const edges: Edge[] = apiEdges.map(e => ({
    id: e.id,
    source: e.source,
    target: e.target,
    markerEnd: { type: MarkerType.ArrowClosed },
    style: { strokeWidth: Math.max(1, Math.min(6, e.rps)) },
    label: e.rps > 0 ? `${e.rps.toFixed(1)} rps` : undefined,
  }));

  return layoutGraph(nodes, edges);
}

interface Props {
  clusterId: string;
  namespaces: string[];
}

const ServiceTopologyGraph: React.FC<Props> = ({ clusterId, namespaces }) => {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [namespace, setNamespace] = useState('');
  const [istioInstalled, setIstioInstalled] = useState<boolean | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Check Istio status first
      const statusRes = await MeshService.getStatus(clusterId);
      const status = (statusRes as unknown as { data: { installed: boolean } }).data ?? statusRes;
      if (!status.installed) {
        setIstioInstalled(false);
        setLoading(false);
        return;
      }
      setIstioInstalled(true);

      const topoRes = await MeshService.getTopology(clusterId, namespace || undefined);
      const topo = (topoRes as unknown as { data: { nodes: MeshNode[]; edges: MeshEdge[] } }).data ?? topoRes;
      const { nodes: fn, edges: fe } = toFlowData(topo.nodes ?? [], topo.edges ?? []);
      setNodes(fn);
      setEdges(fe);
    } catch {
      setError('取得拓撲失敗');
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, setNodes, setEdges]);

  useEffect(() => {
    load();
  }, [load]);

  const nsOptions = useMemo(() => namespaces.map(ns => ({ value: ns, label: ns })), [namespaces]);

  if (istioInstalled === false) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description="Istio 未安裝，無法顯示服務拓撲圖"
      />
    );
  }

  return (
    <div>
      <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
        <Select
          allowClear
          placeholder="篩選命名空間"
          style={{ width: 200 }}
          value={namespace || undefined}
          onChange={v => setNamespace(v ?? '')}
          options={nsOptions}
        />
        <Button icon={<ReloadOutlined />} onClick={load}>重新整理</Button>
      </div>
      {error && <Alert type="error" message={error} style={{ marginBottom: 12 }} />}
      <Spin spinning={loading}>
        <div style={{ height: 500, border: '1px solid #f0f0f0', borderRadius: 8 }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            nodeTypes={nodeTypes}
            fitView
          >
            <Background />
            <Controls />
            <MiniMap />
          </ReactFlow>
        </div>
      </Spin>
    </div>
  );
};

export default ServiceTopologyGraph;

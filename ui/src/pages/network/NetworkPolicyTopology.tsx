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
  Panel,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import dagre from '@dagrejs/dagre';
import { Alert, Badge, Button, Select, Spin, Tag, Tooltip, Typography } from 'antd';
import { ReloadOutlined, ApartmentOutlined, WarningOutlined } from '@ant-design/icons';
import { NetworkPolicyService, type ConflictItem, type TopologyNode as ApiNode } from '../../services/networkPolicyService';

const { Text } = Typography;

// ---- Node colours ----
const NODE_COLORS: Record<string, { bg: string; border: string; text: string }> = {
  podgroup:  { bg: '#f0fdf4', border: '#22c55e', text: '#166534' },
  namespace: { bg: '#eff6ff', border: '#3b82f6', text: '#1e3a8a' },
  ipblock:   { bg: '#fff7ed', border: '#f97316', text: '#9a3412' },
  external:  { bg: '#f8fafc', border: '#94a3b8', text: '#475569' },
};

// ---- Custom node component ----
const TopologyNodeComponent: React.FC<{ data: { label: string; nodeType: string; namespace?: string; policyCount?: number } }> = ({ data }) => {
  const colors = NODE_COLORS[data.nodeType] || NODE_COLORS.external;
  return (
    <div style={{
      background: colors.bg,
      border: `2px solid ${colors.border}`,
      borderRadius: 8,
      padding: '8px 14px',
      minWidth: 140,
      textAlign: 'center',
      boxShadow: '0 2px 6px rgba(0,0,0,0.08)',
    }}>
      <Handle type="target" position={Position.Left} style={{ background: colors.border }} />
      <div style={{ fontSize: 11, color: colors.text, fontWeight: 600, marginBottom: 2 }}>
        {data.nodeType === 'podgroup' && '📦 Pod Group'}
        {data.nodeType === 'namespace' && '🏷️ Namespace'}
        {data.nodeType === 'ipblock' && '🌐 IP Block'}
        {data.nodeType === 'external' && '☁️ External'}
      </div>
      <div style={{ fontSize: 13, color: '#1e293b', fontWeight: 500, wordBreak: 'break-all' }}>
        {data.label}
      </div>
      {data.namespace && (
        <div style={{ fontSize: 10, color: '#64748b', marginTop: 2 }}>{data.namespace}</div>
      )}
      {(data.policyCount ?? 0) > 0 && (
        <Badge count={data.policyCount} style={{ marginTop: 4 }} />
      )}
      <Handle type="source" position={Position.Right} style={{ background: colors.border }} />
    </div>
  );
};

const nodeTypes = { topology: TopologyNodeComponent };

// ---- Dagre layout ----
const NODE_W = 170;
const NODE_H = 80;

function applyDagreLayout(nodes: Node[], edges: Edge[]): Node[] {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: 'LR', ranksep: 80, nodesep: 40 });
  nodes.forEach(n => g.setNode(n.id, { width: NODE_W, height: NODE_H }));
  edges.forEach(e => g.setEdge(e.source, e.target));
  dagre.layout(g);
  return nodes.map(n => {
    const pos = g.node(n.id);
    return { ...n, position: { x: pos.x - NODE_W / 2, y: pos.y - NODE_H / 2 } };
  });
}

// ---- Build ReactFlow nodes/edges from API data ----
function buildFlowElements(apiNodes: ApiNode[], apiEdges: { id: string; source: string; target: string; label?: string; direction: string; policy: string }[]) {
  const nodes: Node[] = apiNodes.map(n => ({
    id: n.id,
    type: 'topology',
    position: { x: 0, y: 0 },
    data: { label: n.label, nodeType: n.type, namespace: n.namespace, policyCount: n.policyCount },
  }));

  const edges: Edge[] = apiEdges.map(e => ({
    id: e.id,
    source: e.source,
    target: e.target,
    label: e.label,
    animated: true,
    style: { stroke: e.direction === 'ingress' ? '#22c55e' : '#3b82f6', strokeWidth: 2 },
    markerEnd: { type: MarkerType.ArrowClosed, color: e.direction === 'ingress' ? '#22c55e' : '#3b82f6' },
    labelStyle: { fontSize: 10, fill: '#475569' },
    labelBgStyle: { fill: '#f8fafc', fillOpacity: 0.9 },
  }));

  const layoutedNodes = applyDagreLayout(nodes, edges);
  return { nodes: layoutedNodes, edges };
}

// ---- Props ----
interface Props {
  clusterId: string;
  namespaces: string[];
}

const NetworkPolicyTopology: React.FC<Props> = ({ clusterId, namespaces }) => {
  const [ns, setNs] = useState<string>('_all_');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflicts, setConflicts] = useState<ConflictItem[]>([]);
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [topoRes, confRes] = await Promise.all([
        NetworkPolicyService.getTopology(clusterId, ns),
        NetworkPolicyService.getConflicts(clusterId, ns),
      ]);
      const topo = (topoRes as { data?: { nodes: ApiNode[]; edges: { id: string; source: string; target: string; label?: string; direction: string; policy: string }[] } }).data;
      if (topo && topo.nodes.length > 0) {
        const { nodes: n, edges: e } = buildFlowElements(topo.nodes, topo.edges);
        setNodes(n);
        setEdges(e);
      } else {
        setNodes([]);
        setEdges([]);
      }
      const conf = (confRes as { data?: { conflicts: ConflictItem[] } }).data;
      setConflicts(conf?.conflicts ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [clusterId, ns, setNodes, setEdges]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const nsOptions = useMemo(() => [
    { value: '_all_', label: '全部命名空間' },
    ...namespaces.map(n => ({ value: n, label: n })),
  ], [namespaces]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* Toolbar */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <ApartmentOutlined style={{ fontSize: 16, color: '#3b82f6' }} />
        <Text strong>流量拓撲圖</Text>
        <Select
          value={ns}
          onChange={setNs}
          options={nsOptions}
          style={{ width: 200 }}
          placeholder="選擇命名空間"
        />
        <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>重新整理</Button>
        {conflicts.length > 0 && (
          <Tooltip title={conflicts.map(c => `${c.policyA} ⚡ ${c.policyB}: ${c.reason}`).join('\n')}>
            <Tag icon={<WarningOutlined />} color="warning">
              {conflicts.length} 個策略衝突
            </Tag>
          </Tooltip>
        )}
      </div>

      {/* Legend */}
      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
        {Object.entries(NODE_COLORS).map(([type, colors]) => (
          <div key={type} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <div style={{ width: 12, height: 12, borderRadius: 2, background: colors.bg, border: `2px solid ${colors.border}` }} />
            <Text style={{ fontSize: 12, color: '#64748b' }}>
              {type === 'podgroup' ? 'Pod 群組' : type === 'namespace' ? 'Namespace' : type === 'ipblock' ? 'IP Block' : 'External'}
            </Text>
          </div>
        ))}
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <div style={{ width: 20, height: 2, background: '#22c55e' }} />
          <Text style={{ fontSize: 12, color: '#64748b' }}>Ingress</Text>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <div style={{ width: 20, height: 2, background: '#3b82f6' }} />
          <Text style={{ fontSize: 12, color: '#64748b' }}>Egress</Text>
        </div>
      </div>

      {/* Conflict alerts */}
      {conflicts.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {conflicts.map((c, i) => (
            <Alert
              key={i}
              type="warning"
              showIcon
              message={
                <span>
                  <Tag color="orange">{c.namespace}</Tag>
                  <strong>{c.policyA}</strong> ⚡ <strong>{c.policyB}</strong>：{c.reason}
                </span>
              }
            />
          ))}
        </div>
      )}

      {error && <Alert type="error" message={error} showIcon />}

      {/* ReactFlow canvas */}
      <Spin spinning={loading}>
        <div style={{ height: 520, border: '1px solid #e2e8f0', borderRadius: 8, overflow: 'hidden', background: '#fafafa' }}>
          {nodes.length === 0 && !loading ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: '#94a3b8' }}>
              此命名空間尚無 NetworkPolicy 資料
            </div>
          ) : (
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              nodeTypes={nodeTypes}
              fitView
              minZoom={0.2}
              maxZoom={2}
            >
              <Background color="#e2e8f0" gap={20} />
              <Controls />
              <MiniMap nodeStrokeWidth={3} zoomable pannable />
              <Panel position="bottom-right">
                <Text style={{ fontSize: 11, color: '#94a3b8' }}>
                  {nodes.length} 節點 · {edges.length} 連線
                </Text>
              </Panel>
            </ReactFlow>
          )}
        </div>
      </Spin>
    </div>
  );
};

export default NetworkPolicyTopology;

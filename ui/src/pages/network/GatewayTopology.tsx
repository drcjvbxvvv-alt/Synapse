import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { Button, Spin, Tag, Empty, App } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type Node,
  type Edge,
  Position,
} from '@xyflow/react';
import dagre from '@dagrejs/dagre';
import '@xyflow/react/dist/style.css';
import { gatewayService } from '../../services/gatewayService';
import type { GatewayTabProps, TopologyNode, TopologyEdge } from './gatewayTypes';

const NODE_WIDTH = 180;
const NODE_HEIGHT = 60;

const KIND_COLORS: Record<string, string> = {
  GatewayClass: '#722ed1',
  Gateway: '#1677ff',
  HTTPRoute: '#13c2c2',
  GRPCRoute: '#52c41a',
  Service: '#fa8c16',
};

const runDagreLayout = (nodes: Node[], edges: Edge[]) => {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: 'LR', nodesep: 40, ranksep: 80 });

  nodes.forEach((n) => g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT }));
  edges.forEach((e) => g.setEdge(e.source, e.target));
  dagre.layout(g);

  return nodes.map((n) => {
    const pos = g.node(n.id);
    return {
      ...n,
      position: { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 },
    };
  });
};

const buildFlowElements = (topoNodes: TopologyNode[], topoEdges: TopologyEdge[]) => {
  const nodes: Node[] = topoNodes.map((n) => ({
    id: n.id,
    type: 'default',
    data: {
      label: (
        <div style={{ textAlign: 'center', lineHeight: 1.4 }}>
          <Tag color={KIND_COLORS[n.kind] ?? '#8c8c8c'} style={{ marginBottom: 2, fontSize: 10 }}>
            {n.kind}
          </Tag>
          <div style={{ fontSize: 12, fontWeight: 600, wordBreak: 'break-all' }}>{n.name}</div>
          {n.namespace && (
            <div style={{ fontSize: 10, color: '#8c8c8c' }}>{n.namespace}</div>
          )}
        </div>
      ),
    },
    position: { x: 0, y: 0 },
    sourcePosition: Position.Right,
    targetPosition: Position.Left,
    style: {
      width: NODE_WIDTH,
      minHeight: NODE_HEIGHT,
      borderColor: KIND_COLORS[n.kind] ?? '#d9d9d9',
      borderWidth: 1.5,
      borderRadius: 8,
      background: '#fff',
      padding: '6px 8px',
    },
  }));

  const edges: Edge[] = topoEdges.map((e, i) => ({
    id: `e-${i}`,
    source: e.source,
    target: e.target,
    type: 'smoothstep',
    animated: false,
    style: { stroke: '#bfbfbf' },
  }));

  const layoutedNodes = runDagreLayout(nodes, edges);
  return { nodes: layoutedNodes, edges };
};

const GatewayTopology: React.FC<GatewayTabProps> = ({ clusterId }) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');
  const [loading, setLoading] = useState(false);
  const [topoNodes, setTopoNodes] = useState<TopologyNode[]>([]);
  const [topoEdges, setTopoEdges] = useState<TopologyEdge[]>([]);

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

  const { nodes, edges } = useMemo(
    () => buildFlowElements(topoNodes, topoEdges),
    [topoNodes, topoEdges],
  );

  return (
    <div>
      <div style={{ marginBottom: 12, display: 'flex', gap: 8, alignItems: 'center' }}>
        <Button icon={<ReloadOutlined />} onClick={loadTopology} loading={loading}>
          {t('gatewayapi.topology.refresh')}
        </Button>
        <div style={{ flex: 1 }} />
        <div style={{ display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap' }}>
          {Object.entries(KIND_COLORS).map(([kind, color]) => (
            <Tag key={kind} color={color} style={{ fontSize: 11 }}>{kind}</Tag>
          ))}
        </div>
      </div>

      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin tip={t('gatewayapi.topology.loading')} />
        </div>
      ) : nodes.length === 0 ? (
        <Empty description={t('gatewayapi.topology.empty')} style={{ padding: 60 }} />
      ) : (
        <div style={{ height: 560, border: '1px solid #f0f0f0', borderRadius: 8, overflow: 'hidden' }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            fitView
            fitViewOptions={{ padding: 0.2 }}
            nodesDraggable
            nodesConnectable={false}
            elementsSelectable
          >
            <Background />
            <Controls />
            <MiniMap nodeColor={(n) => KIND_COLORS[(n.data as { label: React.ReactNode; kind?: string }).kind ?? ''] ?? '#bfbfbf'} />
          </ReactFlow>
        </div>
      )}
    </div>
  );
};

export default GatewayTopology;

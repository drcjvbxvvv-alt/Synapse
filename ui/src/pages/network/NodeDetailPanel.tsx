import React from 'react';
import { Drawer, Badge, Tag, Descriptions, Divider, Typography, Space } from 'antd';
import { useTranslation } from 'react-i18next';
import type { NetworkNode, NetworkEdge } from '../../services/networkTopologyService';

const { Text } = Typography;

interface NodeDetailPanelProps {
  node: NetworkNode | null;
  edges: NetworkEdge[];
  onClose: () => void;
}

const WORKLOAD_KIND_COLOR: Record<string, string> = {
  Deployment:  '#1677ff',
  StatefulSet: '#722ed1',
  DaemonSet:   '#13c2c2',
  Job:         '#fa8c16',
  Pod:         '#8c8c8c',
  ReplicaSet:  '#1677ff',
};

const HEALTH_COLOR: Record<string, string> = {
  healthy:  '#52c41a',
  degraded: '#fa8c16',
  down:     '#ff4d4f',
  unknown:  '#d9d9d9',
};

const HealthBadge: React.FC<{ health: string }> = ({ health }) => {
  const status =
    health === 'healthy'  ? 'success' :
    health === 'degraded' ? 'warning' :
    health === 'down'     ? 'error'   : 'default';
  return <Badge status={status} text={health} />;
};

const NodeDetailPanel: React.FC<NodeDetailPanelProps> = ({ node, edges, onClose }) => {
  const { t } = useTranslation('network');

  if (!node) return null;

  const isWorkload = node.kind === 'Workload';
  const isIngress  = node.kind === 'Ingress';
  const kindColor  = isWorkload
    ? (WORKLOAD_KIND_COLOR[node.workloadKind ?? ''] ?? '#8c8c8c')
    : isIngress ? '#722ed1' : '#fa8c16';

  // Edges connected to this node
  const outEdges = edges.filter((e) => e.source === node.id); // Service → Workload
  const inEdges  = edges.filter((e) => e.target === node.id); // something → this node

  const title = (
    <Space>
      <Tag color={kindColor} style={{ marginRight: 0 }}>
        {isWorkload ? (node.workloadKind ?? 'Workload') : 'Service'}
      </Tag>
      <span>{node.name}</span>
    </Space>
  );

  return (
    <Drawer
      title={title}
      open={!!node}
      onClose={onClose}
      width={400}
      styles={{ body: { padding: '16px 20px' } }}
    >
      {/* Basic info */}
      <Descriptions column={1} size="small" bordered={false}>
        <Descriptions.Item label={t('clusterTopology.detail.namespace')}>
          <Text code>{node.namespace}</Text>
        </Descriptions.Item>
        <Descriptions.Item label={t('clusterTopology.detail.kind')}>
          <Tag color={kindColor}>
            {isWorkload
              ? (node.workloadKind ?? 'Workload')
              : isIngress
              ? 'Ingress'
              : `Service / ${node.serviceType ?? 'ClusterIP'}`}
          </Tag>
        </Descriptions.Item>

        {/* Workload-specific */}
        {isWorkload && (
          <Descriptions.Item label={t('clusterTopology.detail.readiness')}>
            <Badge
              status={node.readyCount === node.totalCount && node.totalCount > 0 ? 'success' : node.readyCount > 0 ? 'warning' : 'error'}
              text={`${node.readyCount} / ${node.totalCount} Ready`}
            />
          </Descriptions.Item>
        )}

        {/* Ingress-specific */}
        {isIngress && node.ingressClass && (
          <Descriptions.Item label={t('clusterTopology.detail.ingressClass')}>
            <Tag>{node.ingressClass}</Tag>
          </Descriptions.Item>
        )}

        {/* Service-specific */}
        {!isWorkload && !isIngress && node.clusterIP && node.clusterIP !== 'None' && (
          <Descriptions.Item label="ClusterIP">
            <Text code>{node.clusterIP}</Text>
          </Descriptions.Item>
        )}
        {!isWorkload && !isIngress && node.serviceType && (
          <Descriptions.Item label={t('clusterTopology.detail.serviceType')}>
            <Tag>{node.serviceType}</Tag>
          </Descriptions.Item>
        )}
      </Descriptions>

      {/* Labels */}
      {node.labels && Object.keys(node.labels).length > 0 && (
        <>
          <Divider style={{ margin: '12px 0' }} />
          <div style={{ fontSize: 12, color: '#8c8c8c', marginBottom: 6 }}>
            {t('clusterTopology.detail.labels')}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
            {Object.entries(node.labels).map(([k, v]) => (
              <Tag key={k} style={{ fontSize: 11, fontFamily: 'monospace' }}>
                {k}={v}
              </Tag>
            ))}
          </div>
        </>
      )}

      {/* Outbound connections (Service → Workload) */}
      {outEdges.length > 0 && (
        <>
          <Divider style={{ margin: '12px 0' }} />
          <div style={{ fontSize: 12, color: '#8c8c8c', marginBottom: 8 }}>
            {t('clusterTopology.detail.connectedWorkloads')}
          </div>
          {outEdges.map((e, i) => (
            <ConnectionRow key={i} edge={e} targetId={e.target} direction="out" />
          ))}
        </>
      )}

      {/* Inbound connections (→ this node) */}
      {inEdges.length > 0 && (
        <>
          <Divider style={{ margin: '12px 0' }} />
          <div style={{ fontSize: 12, color: '#8c8c8c', marginBottom: 8 }}>
            {t('clusterTopology.detail.connectedServices')}
          </div>
          {inEdges.map((e, i) => (
            <ConnectionRow key={i} edge={e} targetId={e.source} direction="in" />
          ))}
        </>
      )}
    </Drawer>
  );
};

// Individual connection row
interface ConnectionRowProps {
  edge: NetworkEdge;
  targetId: string;
  direction: 'in' | 'out';
}

const ConnectionRow: React.FC<ConnectionRowProps> = ({ edge, targetId, direction }) => {
  // Parse node ID: "workload/ns/kind/name" or "service/ns/name"
  const parts = targetId.split('/');
  const displayName = parts[parts.length - 1];
  const displayKind = parts[0] === 'service' ? 'Service' : (parts[2] ?? parts[0]);

  const hasIstio = edge.requestRate !== undefined;

  return (
    <div
      style={{
        background: '#fafafa',
        border: '1px solid #f0f0f0',
        borderRadius: 6,
        padding: '8px 10px',
        marginBottom: 6,
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Space size={4}>
          <Tag style={{ fontSize: 10, padding: '0 4px' }}>{displayKind}</Tag>
          <Text style={{ fontSize: 12 }} strong>{displayName}</Text>
        </Space>
        <HealthBadge health={edge.health} />
      </div>

      {edge.kind === 'istio-flow' && (
        <div style={{ fontSize: 10, color: '#13c2c2', marginTop: 3, fontWeight: 500 }}>
          Istio 實際流量
        </div>
      )}
      {edge.policyStatus === 'policy-deny' && (
        <div style={{ fontSize: 10, color: '#ff4d4f', marginTop: 3, fontWeight: 500 }}>
          🔒 NetworkPolicy 封鎖：{edge.policyName}
        </div>
      )}
      {edge.policyStatus === 'policy-restricted' && (
        <div style={{ fontSize: 10, color: '#fa8c16', marginTop: 3, fontWeight: 500 }}>
          ⚠ NetworkPolicy 限制來源：{edge.policyName}
        </div>
      )}
      {edge.policyStatus === 'policy-allow' && (
        <div style={{ fontSize: 10, color: '#1677ff', marginTop: 3, fontWeight: 500 }}>
          ✓ NetworkPolicy 明確允許：{edge.policyName}
        </div>
      )}
      {edge.ports && (
        <div style={{ fontSize: 11, color: '#8c8c8c', marginTop: 4 }}>
          Ports: <Text code style={{ fontSize: 11 }}>{edge.ports}</Text>
        </div>
      )}

      {hasIstio && (
        <div style={{ display: 'flex', gap: 12, marginTop: 4, fontSize: 11 }}>
          {edge.requestRate !== undefined && (
            <span style={{ color: '#1677ff' }}>
              {edge.requestRate.toFixed(2)} rps
            </span>
          )}
          {edge.errorRate !== undefined && edge.errorRate > 0 && (
            <span style={{ color: HEALTH_COLOR[edge.health] ?? '#8c8c8c' }}>
              {(edge.errorRate * 100).toFixed(1)}% err
            </span>
          )}
          {edge.latencyP99ms !== undefined && edge.latencyP99ms > 0 && (
            <span style={{ color: '#8c8c8c' }}>
              P99 {edge.latencyP99ms.toFixed(0)}ms
            </span>
          )}
        </div>
      )}
    </div>
  );
};

export default NodeDetailPanel;

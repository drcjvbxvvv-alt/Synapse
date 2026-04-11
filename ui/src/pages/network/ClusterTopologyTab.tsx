
import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import EmptyState from '@/components/EmptyState';
import { Button, Input, Spin, Tag, App, Space, Tooltip, Switch } from 'antd';
import { ReloadOutlined, ApiOutlined, SyncOutlined, SearchOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { networkTopologyService } from '../../services/networkTopologyService';
import type { NetworkNode, NetworkEdge, TopologyIntegrationStatus } from '../../services/networkTopologyService';
import ClusterTopologyGraph, { HEALTH_LEGEND, WORKLOAD_KIND_COLOR } from './ClusterTopologyGraph';
import NodeDetailPanel from './NodeDetailPanel';

interface ClusterTopologyTabProps {
  clusterId: string;
}

const ClusterTopologyTab: React.FC<ClusterTopologyTabProps> = ({ clusterId }) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');

  const [loading, setLoading] = useState(false);
  const [nodes, setNodes] = useState<NetworkNode[]>([]);
  const [edges, setEdges] = useState<NetworkEdge[]>([]);
  const [integrations, setIntegrations] = useState<TopologyIntegrationStatus | null>(null);
  const [enrich, setEnrich] = useState(false);
  const [selectedNode, setSelectedNode] = useState<NetworkNode | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [showIstioFlows, setShowIstioFlows] = useState(true);
  const [showPolicy, setShowPolicy] = useState(false);
  const [showHubble, setShowHubble] = useState(false);
  const [searchText, setSearchText] = useState('');
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const isInteractingRef = useRef(false);

  // Load integration status
  useEffect(() => {
    networkTopologyService.getIntegrations(clusterId)
      .then(setIntegrations)
      .catch(() => setIntegrations({ cilium: false, istio: false, hubbleMetrics: false }));
  }, [clusterId]);

  const loadTopology = useCallback(async () => {
    setLoading(true);
    try {
      const data = await networkTopologyService.getTopology(
        clusterId,
        undefined,
        enrich,
        showPolicy,
        showHubble,
      );
      setNodes(data.nodes ?? []);
      setEdges(data.edges ?? []);
    } catch {
      message.error(t('clusterTopology.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, enrich, showPolicy, showHubble, message, t]);

  useEffect(() => {
    loadTopology();
  }, [loadTopology]);

  // Pause auto-refresh when detail panel is open
  useEffect(() => {
    isInteractingRef.current = selectedNode !== null;
  }, [selectedNode]);

  // Auto-refresh every 15 seconds when enabled, skipped during interaction
  useEffect(() => {
    if (timerRef.current) clearInterval(timerRef.current);
    if (autoRefresh) {
      timerRef.current = setInterval(() => {
        if (!isInteractingRef.current) loadTopology();
      }, 15000);
    }
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [autoRefresh, loadTopology]);

  // Filter nodes by search text
  const filteredNodes = useMemo(() => {
    if (!searchText.trim()) return nodes;
    const lower = searchText.toLowerCase();
    return nodes.filter((n) => n.name.toLowerCase().includes(lower) || n.namespace.toLowerCase().includes(lower));
  }, [nodes, searchText]);

  const filteredEdges = useMemo(() => {
    if (!searchText.trim()) return edges;
    const nodeIds = new Set(filteredNodes.map((n) => n.id));
    return edges.filter((e) => nodeIds.has(e.source) || nodeIds.has(e.target));
  }, [edges, filteredNodes, searchText]);

  return (
    <div>
      {/* Toolbar */}
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12, flexWrap: 'wrap' }}>
        <Input
          allowClear
          prefix={<SearchOutlined />}
          placeholder={t('clusterTopology.searchPlaceholder')}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          style={{ width: 180 }}
        />
        <Button icon={<ReloadOutlined />} loading={loading} onClick={loadTopology}>
          {t('clusterTopology.refresh')}
        </Button>
        <Tooltip title={autoRefresh ? t('clusterTopology.autoRefreshStop') : t('clusterTopology.autoRefreshStart')}>
          <Button
            icon={<SyncOutlined spin={autoRefresh} />}
            type={autoRefresh ? 'primary' : 'default'}
            onClick={() => setAutoRefresh((v) => !v)}
          />
        </Tooltip>

        {/* Integration badges */}
        {integrations?.istio && (
          <Tooltip title={`Istio ${integrations.istioVersion ?? ''} ${t('clusterTopology.detected')}`}>
            <Tag icon={<ApiOutlined />} color="geekblue" style={{ cursor: 'default' }}>
              Istio {integrations.istioVersion}
            </Tag>
          </Tooltip>
        )}
        {integrations?.cilium && (
          <Tooltip title={`Cilium ${integrations.ciliumVersion ?? ''} ${t('clusterTopology.detected')}`}>
            <Tag icon={<ApiOutlined />} color="cyan" style={{ cursor: 'default' }}>
              Cilium {integrations.ciliumVersion}
            </Tag>
          </Tooltip>
        )}

        {/* Istio metrics enrich toggle */}
        {integrations?.istio && (
          <Tooltip title={t('clusterTopology.enrichTooltip')}>
            <Space size={4}>
              <Switch
                size="small"
                checked={enrich}
                onChange={setEnrich}
              />
              <span style={{ fontSize: 12 }}>{t('clusterTopology.enrichLabel')}</span>
            </Space>
          </Tooltip>
        )}

        {/* NetworkPolicy overlay toggle */}
        <Tooltip title={t('clusterTopology.policyTooltip')}>
          <Space size={4}>
            <Switch
              size="small"
              checked={showPolicy}
              onChange={setShowPolicy}
            />
            <span style={{ fontSize: 12 }}>{t('clusterTopology.policyLabel')}</span>
          </Space>
        </Tooltip>

        {/* Istio call-direction flow toggle (only useful when enrich is on) */}
        {integrations?.istio && enrich && (
          <Tooltip title={t('clusterTopology.istioFlowTooltip')}>
            <Space size={4}>
              <Switch
                size="small"
                checked={showIstioFlows}
                onChange={setShowIstioFlows}
              />
              <span style={{ fontSize: 12 }}>{t('clusterTopology.istioFlowLabel')}</span>
            </Space>
          </Tooltip>
        )}

        {/* Hubble real flow toggle (only when Hubble Prometheus metrics are confirmed available) */}
        {integrations?.hubbleMetrics && (
          <Tooltip title={t('clusterTopology.hubbleTooltip')}>
            <Space size={4}>
              <Switch
                size="small"
                checked={showHubble}
                onChange={setShowHubble}
              />
              <span style={{ fontSize: 12 }}>{t('clusterTopology.hubbleLabel')}</span>
            </Space>
          </Tooltip>
        )}

        <div style={{ flex: 1 }} />

        {/* Legend */}
        <Space wrap size={4}>
          {Object.entries(WORKLOAD_KIND_COLOR).slice(0, 4).map(([kind, color]) => (
            <Tag key={kind} color={color} style={{ fontSize: 11 }}>{kind}</Tag>
          ))}
          <Tag color="orange"  style={{ fontSize: 11 }}>Service</Tag>
          <Tag color="purple"  style={{ fontSize: 11 }}>Ingress</Tag>
          {integrations?.istio && enrich && (
            <span style={{ fontSize: 11, display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ display: 'inline-block', width: 20, height: 2, background: '#13c2c2', borderRadius: 1 }} />
              {t('clusterTopology.istioFlowLabel')}
            </span>
          )}
          {showPolicy && (
            <>
              <span style={{ fontSize: 11, display: 'flex', alignItems: 'center', gap: 4 }}>
                <span style={{ display: 'inline-block', width: 20, height: 2, background: '#ff4d4f', borderRadius: 1, borderTop: '2px dashed #ff4d4f' }} />
                {t('clusterTopology.policyDeny')}
              </span>
              <span style={{ fontSize: 11, display: 'flex', alignItems: 'center', gap: 4 }}>
                <span style={{ display: 'inline-block', width: 20, height: 2, background: '#1677ff', borderRadius: 1 }} />
                {t('clusterTopology.policyAllow')}
              </span>
            </>
          )}
        </Space>
        <Space wrap size={4}>
          {HEALTH_LEGEND.map(({ key, color }) => (
            <span key={key} style={{ fontSize: 11, display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ display: 'inline-block', width: 20, height: 2, background: color, borderRadius: 1 }} />
              {t(`clusterTopology.health.${key}`)}
            </span>
          ))}
        </Space>
      </div>

      {/* Graph */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}>
          <Spin tip={t('clusterTopology.loading')} />
        </div>
      ) : nodes.length === 0 ? (
        <EmptyState description={t('clusterTopology.empty')} style={{ padding: 60 }} />
      ) : (
        <ClusterTopologyGraph
          topoNodes={filteredNodes}
          topoEdges={showIstioFlows ? filteredEdges : filteredEdges.filter((e) => e.kind !== 'istio-flow')}
          onNodeClick={setSelectedNode}
        />
      )}

      <NodeDetailPanel
        node={selectedNode}
        edges={edges}
        onClose={() => setSelectedNode(null)}
      />
    </div>
  );
};

export default ClusterTopologyTab;

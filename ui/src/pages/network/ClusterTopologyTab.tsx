import React, { useState, useEffect, useCallback } from 'react';
import { Button, Select, Spin, Empty, Tag, App, Space } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { networkTopologyService } from '../../services/networkTopologyService';
import type { NetworkNode, NetworkEdge } from '../../services/networkTopologyService';
import ClusterTopologyGraph, { HEALTH_LEGEND, WORKLOAD_KIND_COLOR } from './ClusterTopologyGraph';
import { namespaceService } from '../../services/namespaceService';

interface ClusterTopologyTabProps {
  clusterId: string;
}

const ClusterTopologyTab: React.FC<ClusterTopologyTabProps> = ({ clusterId }) => {
  const { message } = App.useApp();
  const { t } = useTranslation('network');

  const [loading, setLoading] = useState(false);
  const [nodes, setNodes] = useState<NetworkNode[]>([]);
  const [edges, setEdges] = useState<NetworkEdge[]>([]);
  const [allNamespaces, setAllNamespaces] = useState<string[]>([]);
  const [selectedNs, setSelectedNs] = useState<string[]>([]);

  // Load namespace list for filter
  useEffect(() => {
    namespaceService.getNamespaces(clusterId)
      .then((res) => {
        const list = (res as { items?: { name: string }[] }).items?.map((n) => n.name) ?? [];
        setAllNamespaces(list);
      })
      .catch(() => {});
  }, [clusterId]);

  const loadTopology = useCallback(async () => {
    setLoading(true);
    try {
      const data = await networkTopologyService.getTopology(
        clusterId,
        selectedNs.length > 0 ? selectedNs : undefined,
      );
      setNodes(data.nodes ?? []);
      setEdges(data.edges ?? []);
    } catch {
      message.error(t('clusterTopology.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, selectedNs, message, t]);

  useEffect(() => {
    loadTopology();
  }, [loadTopology]);

  return (
    <div>
      {/* Toolbar */}
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12, flexWrap: 'wrap' }}>
        <Select
          mode="multiple"
          allowClear
          placeholder={t('clusterTopology.namespaceFilter')}
          value={selectedNs}
          onChange={setSelectedNs}
          style={{ minWidth: 240, maxWidth: 480 }}
          options={allNamespaces.map((ns) => ({ value: ns, label: ns }))}
          maxTagCount="responsive"
        />
        <Button icon={<ReloadOutlined />} loading={loading} onClick={loadTopology}>
          {t('clusterTopology.refresh')}
        </Button>
        <div style={{ flex: 1 }} />

        {/* Legend */}
        <Space wrap size={4}>
          {Object.entries(WORKLOAD_KIND_COLOR).slice(0, 4).map(([kind, color]) => (
            <Tag key={kind} color={color} style={{ fontSize: 11 }}>{kind}</Tag>
          ))}
          <Tag color="orange" style={{ fontSize: 11 }}>Service</Tag>
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
        <Empty description={t('clusterTopology.empty')} style={{ padding: 60 }} />
      ) : (
        <ClusterTopologyGraph topoNodes={nodes} topoEdges={edges} />
      )}
    </div>
  );
};

export default ClusterTopologyTab;

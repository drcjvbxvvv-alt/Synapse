import React, { useState } from 'react';
import { Button, Select, Spin, Alert, Space, Tag, Tooltip, theme, Empty } from 'antd';
import EmptyState from '@/components/EmptyState';
import { ApartmentOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { networkTopologyService } from '../../services/networkTopologyService';
import MultiClusterTopologyGraph from './MultiClusterTopologyGraph';

// Cluster list reuses the existing cluster API shape
interface ClusterOption {
  value: number;
  label: string;
}

interface ClusterListResponse {
  items?: { id: number; name: string }[];
  data?: { id: number; name: string }[];
}

const MultiClusterTopologyPage: React.FC = () => {
  const { t } = useTranslation(['multicluster', 'common']);
  const { token } = theme.useToken();
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);
  const [queryIDs, setQueryIDs] = useState<number[]>([]);

  // ── Fetch cluster list for the selector ──────────────────────────────────
  const { data: clusterListData } = useQuery<ClusterListResponse>({
    queryKey: ['clusters-list-for-mc-topo'],
    queryFn: () =>
      import('../../utils/api').then((m) => m.request.get('/clusters')),
    staleTime: 60_000,
  });

  const clusterOptions: ClusterOption[] = (
    clusterListData?.items ?? clusterListData?.data ?? []
  ).map((c) => ({ value: c.id, label: c.name }));

  // ── Fetch topology (only when queryIDs is non-empty) ─────────────────────
  const {
    data: topo,
    isFetching,
    isError,
    refetch,
  } = useQuery({
    queryKey: ['multi-cluster-topology', queryIDs],
    queryFn: () => networkTopologyService.getMultiClusterTopology(queryIDs),
    enabled: queryIDs.length > 0,
    staleTime: 30_000,
  });

  const handleFetch = () => {
    if (selectedIDs.length === 0) return;
    setQueryIDs([...selectedIDs]);
  };

  const totalNodes = topo?.clusters.reduce((s, c) => s + c.nodes.length, 0) ?? 0;
  const crossCount  = topo?.crossEdges.length ?? 0;

  return (
    <div style={{ padding: token.paddingLG }}>
      {/* Toolbar */}
      <div style={{ display: 'flex', gap: token.marginSM, alignItems: 'center', flexWrap: 'wrap', marginBottom: token.marginMD }}>
        <Select
          mode="multiple"
          allowClear
          placeholder={t('multicluster:topology.selectPlaceholder')}
          style={{ minWidth: 320, maxWidth: 600 }}
          options={clusterOptions}
          value={selectedIDs}
          onChange={setSelectedIDs}
          maxTagCount="responsive"
          maxCount={10}
        />
        <Button
          type="primary"
          icon={<ApartmentOutlined />}
          onClick={handleFetch}
          disabled={selectedIDs.length < 2}
        >
          {t('multicluster:topology.viewButton')}
        </Button>
        {queryIDs.length > 0 && (
          <Tooltip title={t('multicluster:topology.refreshTooltip')}>
            <Button icon={<ReloadOutlined />} loading={isFetching} onClick={() => refetch()} />
          </Tooltip>
        )}

        {/* Stats */}
        {topo && (
          <Space size={4} style={{ marginLeft: token.marginSM }}>
            <Tag color="blue">{t('multicluster:topology.clusterCount', { count: topo.clusters.length })}</Tag>
            <Tag color="green">{t('multicluster:topology.nodeCount', { count: totalNodes })}</Tag>
            {crossCount > 0 && (
              <Tag color="purple">{t('multicluster:topology.crossEdgeCount', { count: crossCount })}</Tag>
            )}
          </Space>
        )}
      </div>

      {/* Cross-cluster edge hint */}
      {topo && crossCount === 0 && (
        <Alert
          type="info"
          showIcon
          message={t('multicluster:topology.noCrossEdgeDetected')}
          description={
            <>
              {t('multicluster:topology.noCrossEdgeHint')}
              <code style={{ marginLeft: 8 }}>synapse.io/cross-cluster: &quot;&lt;targetClusterID&gt;/&lt;namespace&gt;/&lt;service&gt;&quot;</code>
            </>
          }
          style={{ marginBottom: token.marginMD }}
          closable
        />
      )}

      {/* Graph area */}
      {queryIDs.length === 0 ? (
        <Empty
          image={<ApartmentOutlined style={{ fontSize: 64, color: token.colorTextTertiary }} />}
          description={t('multicluster:topology.emptyDescription')}
          style={{ paddingTop: 80 }}
        />
      ) : isFetching ? (
        <div style={{ textAlign: 'center', paddingTop: 80 }}>
          <Spin tip={t('multicluster:topology.loading')} size="large" />
        </div>
      ) : isError ? (
        <Alert
          type="error"
          showIcon
          message={t('multicluster:topology.loadError')}
          description={t('multicluster:topology.loadErrorDescription')}
          style={{ marginTop: token.marginMD }}
        />
      ) : topo && topo.clusters.every((c) => c.nodes.length === 0) ? (
        <EmptyState description={t('common:messages.noNetworkResources')} style={{ paddingTop: 60 }} />
      ) : topo ? (
        <MultiClusterTopologyGraph
          sections={topo.clusters}
          crossEdges={topo.crossEdges}
        />
      ) : null}

      {/* Cross-cluster edge legend (only shown when edges exist) */}
      {crossCount > 0 && (
        <div style={{ marginTop: token.marginMD, display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: token.colorTextSecondary }}>
          <span
            style={{
              display: 'inline-block',
              width: 28,
              height: 2,
              borderTop: '2px dashed #722ed1',
              verticalAlign: 'middle',
            }}
          />
          <span style={{ color: '#722ed1', fontWeight: 600 }}>{t('multicluster:topology.crossEdgeLegend')}</span>
          <span dangerouslySetInnerHTML={{ __html: t('multicluster:topology.crossEdgeLegendDescription') }} />
        </div>
      )}
    </div>
  );
};

export default MultiClusterTopologyPage;

import React, { useState, useCallback, useEffect } from 'react';
import {
  Button,
  Select,
  Drawer,
  Switch,
  Space,
  Tag,
  Tooltip,
  Flex,
  Typography,
  Divider,
  App,
  theme,
} from 'antd';
import { EditOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { TableColumnsType } from 'antd';
import EmptyState from '../../components/EmptyState';
import { TableListLayout } from '../../components/TableListLayout';
import type { Cluster, ClusterPermission, FeaturePolicyResponse } from '../../types';
import { clusterService } from '../../services/clusterService';
import * as permissionService from '../../services/permissionService';
import { parseApiError } from '../../utils/api';

const { Text } = Typography;

// ─── Feature 分類定義 ──────────────────────────────────────────────────────

interface FeatureGroup {
  groupKey: string;
  labelKey: string;
  features: { key: string; labelKey: string; descKey: string }[];
}

const FEATURE_GROUPS: FeatureGroup[] = [
  {
    groupKey: 'workload',
    labelKey: 'featurePolicy.group.workload',
    features: [
      { key: 'workload:view',   labelKey: 'featurePolicy.key.workloadView',   descKey: 'featurePolicy.desc.workloadView' },
      { key: 'workload:write',  labelKey: 'featurePolicy.key.workloadWrite',  descKey: 'featurePolicy.desc.workloadWrite' },
      { key: 'workload:delete', labelKey: 'featurePolicy.key.workloadDelete', descKey: 'featurePolicy.desc.workloadDelete' },
    ],
  },
  {
    groupKey: 'network',
    labelKey: 'featurePolicy.group.network',
    features: [
      { key: 'network:view',   labelKey: 'featurePolicy.key.networkView',   descKey: 'featurePolicy.desc.networkView' },
      { key: 'network:write',  labelKey: 'featurePolicy.key.networkWrite',  descKey: 'featurePolicy.desc.networkWrite' },
      { key: 'network:delete', labelKey: 'featurePolicy.key.networkDelete', descKey: 'featurePolicy.desc.networkDelete' },
    ],
  },
  {
    groupKey: 'storage',
    labelKey: 'featurePolicy.group.storage',
    features: [
      { key: 'storage:view',   labelKey: 'featurePolicy.key.storageView',   descKey: 'featurePolicy.desc.storageView' },
      { key: 'storage:write',  labelKey: 'featurePolicy.key.storageWrite',  descKey: 'featurePolicy.desc.storageWrite' },
      { key: 'storage:delete', labelKey: 'featurePolicy.key.storageDelete', descKey: 'featurePolicy.desc.storageDelete' },
    ],
  },
  {
    groupKey: 'node',
    labelKey: 'featurePolicy.group.node',
    features: [
      { key: 'node:view',   labelKey: 'featurePolicy.key.nodeView',   descKey: 'featurePolicy.desc.nodeView' },
      { key: 'node:manage', labelKey: 'featurePolicy.key.nodeManage', descKey: 'featurePolicy.desc.nodeManage' },
    ],
  },
  {
    groupKey: 'config',
    labelKey: 'featurePolicy.group.config',
    features: [
      { key: 'config:view',   labelKey: 'featurePolicy.key.configView',   descKey: 'featurePolicy.desc.configView' },
      { key: 'config:write',  labelKey: 'featurePolicy.key.configWrite',  descKey: 'featurePolicy.desc.configWrite' },
      { key: 'config:delete', labelKey: 'featurePolicy.key.configDelete', descKey: 'featurePolicy.desc.configDelete' },
    ],
  },
  {
    groupKey: 'terminal',
    labelKey: 'featurePolicy.group.terminal',
    features: [
      { key: 'terminal:pod',  labelKey: 'featurePolicy.key.terminalPod',  descKey: 'featurePolicy.desc.terminalPod' },
      { key: 'terminal:node', labelKey: 'featurePolicy.key.terminalNode', descKey: 'featurePolicy.desc.terminalNode' },
    ],
  },
  {
    groupKey: 'observability',
    labelKey: 'featurePolicy.group.observability',
    features: [
      { key: 'logs:view',         labelKey: 'featurePolicy.key.logsView',         descKey: 'featurePolicy.desc.logsView' },
      { key: 'monitoring:view',   labelKey: 'featurePolicy.key.monitoringView',   descKey: 'featurePolicy.desc.monitoringView' },
      { key: 'alerts:view',       labelKey: 'featurePolicy.key.alertsView',       descKey: 'featurePolicy.desc.alertsView' },
      { key: 'event_alerts:view', labelKey: 'featurePolicy.key.eventAlertsView',  descKey: 'featurePolicy.desc.eventAlertsView' },
      { key: 'cost:view',         labelKey: 'featurePolicy.key.costView',         descKey: 'featurePolicy.desc.costView' },
      { key: 'security:view',     labelKey: 'featurePolicy.key.securityView',     descKey: 'featurePolicy.desc.securityView' },
      { key: 'certificates:view', labelKey: 'featurePolicy.key.certificatesView', descKey: 'featurePolicy.desc.certificatesView' },
      { key: 'slo:view',          labelKey: 'featurePolicy.key.sloView',          descKey: 'featurePolicy.desc.sloView' },
      { key: 'chaos:view',        labelKey: 'featurePolicy.key.chaosView',        descKey: 'featurePolicy.desc.chaosView' },
      { key: 'compliance:view',   labelKey: 'featurePolicy.key.complianceView',   descKey: 'featurePolicy.desc.complianceView' },
    ],
  },
  {
    groupKey: 'helm',
    labelKey: 'featurePolicy.group.helm',
    features: [
      { key: 'helm:view',  labelKey: 'featurePolicy.key.helmView',  descKey: 'featurePolicy.desc.helmView' },
      { key: 'helm:write', labelKey: 'featurePolicy.key.helmWrite', descKey: 'featurePolicy.desc.helmWrite' },
    ],
  },
  {
    groupKey: 'tools',
    labelKey: 'featurePolicy.group.tools',
    features: [
      { key: 'export',       labelKey: 'featurePolicy.key.export',       descKey: 'featurePolicy.desc.export' },
      { key: 'ai_assistant', labelKey: 'featurePolicy.key.aiAssistant', descKey: 'featurePolicy.desc.aiAssistant' },
    ],
  },
];

// 表格摘要欄位（最受關注的幾個功能）
const SUMMARY_FEATURES = ['terminal:pod', 'terminal:node', 'export', 'ai_assistant'];

// per-type ceiling（前端鏡像後端 FeatureCeilings，僅用於表格摘要顯示）
const CEILING: Record<string, string[]> = {
  admin:    ['workload:view','workload:write','workload:delete','network:view','network:write','network:delete','storage:view','storage:write','storage:delete','node:view','node:manage','config:view','config:write','config:delete','terminal:pod','terminal:node','logs:view','monitoring:view','alerts:view','event_alerts:view','cost:view','security:view','certificates:view','slo:view','chaos:view','compliance:view','helm:view','helm:write','export','ai_assistant'],
  ops:      ['workload:view','workload:write','workload:delete','network:view','network:write','network:delete','storage:view','storage:delete','node:view','config:view','config:write','config:delete','terminal:pod','terminal:node','logs:view','monitoring:view','alerts:view','event_alerts:view','cost:view','security:view','certificates:view','slo:view','chaos:view','compliance:view','helm:view','helm:write','export','ai_assistant'],
  dev:      ['workload:view','workload:write','workload:delete','network:view','network:write','network:delete','config:view','config:write','config:delete','terminal:pod','logs:view','monitoring:view','alerts:view','event_alerts:view','security:view','slo:view','compliance:view','helm:view','export','ai_assistant'],
  readonly: ['workload:view','network:view','storage:view','node:view','config:view','logs:view','monitoring:view','helm:view'],
  custom:   ['workload:view','workload:write','workload:delete','network:view','network:write','network:delete','storage:view','storage:write','storage:delete','node:view','node:manage','config:view','config:write','config:delete','terminal:pod','terminal:node','logs:view','monitoring:view','alerts:view','event_alerts:view','cost:view','security:view','certificates:view','slo:view','chaos:view','compliance:view','helm:view','helm:write','export','ai_assistant'],
};

// ─── 主元件 ────────────────────────────────────────────────────────────────

const FeaturePolicyPage: React.FC = () => {
  const { t } = useTranslation('permission');
  const { token } = theme.useToken();
  const { message } = App.useApp();

  // ── 叢集選擇 ──────────────────────────────────────────────────────────
  const [clusters, setClusters] = useState<{ value: number; label: string }[]>([]);
  const [clustersLoading, setClustersLoading] = useState(false);
  const [selectedClusterId, setSelectedClusterId] = useState<number | undefined>(undefined);

  // ── 權限列表 ──────────────────────────────────────────────────────────
  const [permissions, setPermissions] = useState<ClusterPermission[]>([]);
  const [permLoading, setPermLoading] = useState(false);

  // ── Drawer ──────────────────────────────────────────────────────────
  const [drawerVisible, setDrawerVisible] = useState(false);
  const [editingPerm, setEditingPerm] = useState<ClusterPermission | null>(null);
  const [featureData, setFeatureData] = useState<FeaturePolicyResponse | null>(null);
  const [featureLoading, setFeatureLoading] = useState(false);
  const [saveLoading, setSaveLoading] = useState(false);
  const [draftPolicy, setDraftPolicy] = useState<Record<string, boolean>>({});

  // ── 載入叢集清單 ──────────────────────────────────────────────────
  const loadClusters = useCallback(async () => {
    setClustersLoading(true);
    try {
      const res = await clusterService.getClusters({ pageSize: 500 });
      const items: Cluster[] = res.items ?? [];
      setClusters(items.map((c) => ({ value: Number(c.id), label: c.name })));
    } catch {
      message.error(t('featurePolicy.messages.loadClusterFailed'));
    } finally {
      setClustersLoading(false);
    }
  }, [message, t]);

  useEffect(() => { loadClusters(); }, [loadClusters]);

  // ── 載入叢集權限列表 ──────────────────────────────────────────────
  const loadPermissions = useCallback(async (clusterId: number) => {
    setPermLoading(true);
    try {
      const res = await permissionService.getClusterPermissions(clusterId);
      setPermissions(res);
    } catch (err) {
      const msg = parseApiError(err);
      if (msg) message.error(msg);
    } finally {
      setPermLoading(false);
    }
  }, [message]);

  useEffect(() => {
    if (selectedClusterId != null) loadPermissions(selectedClusterId);
    else setPermissions([]);
  }, [selectedClusterId, loadPermissions]);

  // ── 開啟 Drawer ───────────────────────────────────────────────────
  const handleEdit = useCallback(async (record: ClusterPermission) => {
    setEditingPerm(record);
    setDrawerVisible(true);
    setFeatureData(null);
    setDraftPolicy({});
    setFeatureLoading(true);
    try {
      const data = await permissionService.getFeaturePolicy(record.id);
      setFeatureData(data);
      setDraftPolicy(data.policy ?? {});
    } catch (err) {
      const msg = parseApiError(err);
      if (msg) message.error(msg);
    } finally {
      setFeatureLoading(false);
    }
  }, [message]);

  // ── 儲存 ─────────────────────────────────────────────────────────
  const handleSave = useCallback(async () => {
    if (!editingPerm) return;
    setSaveLoading(true);
    try {
      await permissionService.updateFeaturePolicy(editingPerm.id, draftPolicy);
      message.success(t('featurePolicy.messages.saveSuccess'));
      setDrawerVisible(false);
      if (selectedClusterId != null) loadPermissions(selectedClusterId);
    } catch (err) {
      const msg = parseApiError(err);
      if (msg) message.error(msg);
    } finally {
      setSaveLoading(false);
    }
  }, [editingPerm, draftPolicy, message, t, selectedClusterId, loadPermissions]);

  // ─── 表格欄位 ─────────────────────────────────────────────────────
  const columns: TableColumnsType<ClusterPermission> = [
    {
      title: t('columns.subject'),
      key: 'subject',
      ellipsis: true,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.username ?? record.user_group_name ?? '—'}</Text>
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
            {record.username ? t('columns.user') : t('columns.userGroup')}
          </Text>
        </Space>
      ),
    },
    {
      title: t('columns.permissionType'),
      dataIndex: 'permission_type',
      key: 'permission_type',
      width: 120,
      render: (type: string) => (
        <Tag color={permissionService.permissionTypeColors[type] ?? 'default'}>
          {permissionService.getPermissionTypeName(type)}
        </Tag>
      ),
    },
    // 摘要欄
    ...SUMMARY_FEATURES.map((featureKey) => ({
      title: t(`featurePolicy.summaryKey.${featureKey.replace(':', '_').replace('ai_assistant', 'aiAssistant')}`),
      key: featureKey,
      width: 100,
      align: 'center' as const,
      render: (_: unknown, record: ClusterPermission) => (
        <FeatureSummaryCell
          permType={record.permission_type}
          featureKey={featureKey}
          featurePolicyJSON={record.feature_policy}
          t={t}
        />
      ),
    })),
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 80,
      fixed: 'right',
      render: (_, record) => (
        <Tooltip title={t('featurePolicy.actions.edit')}>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          />
        </Tooltip>
      ),
    },
  ];

  // ─── 渲染 ─────────────────────────────────────────────────────────
  return (
    <div style={{ padding: token.paddingLG }}>
      {/* 頁首 */}
      <div style={{ marginBottom: token.marginLG }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('featurePolicy.page.title')}
        </Typography.Title>
        <Text type="secondary">{t('featurePolicy.page.subtitle')}</Text>
      </div>

      {/* 叢集選擇器 */}
      <div style={{ marginBottom: token.marginMD }}>
        <Select
          style={{ width: 280 }}
          placeholder={t('filter.selectCluster')}
          loading={clustersLoading}
          options={clusters}
          value={selectedClusterId}
          onChange={(v) => setSelectedClusterId(v)}
          showSearch
          filterOption={(input, opt) =>
            String(opt?.label ?? '').toLowerCase().includes(input.toLowerCase())
          }
        />
      </div>

      {/* 表格 */}
      <TableListLayout<ClusterPermission>
        onRefresh={selectedClusterId != null ? () => loadPermissions(selectedClusterId) : undefined}
        refreshing={permLoading}
        tableProps={{
          columns,
          dataSource: permissions,
          rowKey: 'id',
          loading: permLoading,
          pagination: {
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => t('pagination.total', { total }),
          },
          locale: {
            emptyText: selectedClusterId == null
              ? <EmptyState type="not-configured" description={t('featurePolicy.messages.selectCluster')} />
              : <EmptyState description={t('common:messages.noData')} />,
          },
        }}
      />

      {/* 編輯 Drawer */}
      <Drawer
        title={
          editingPerm
            ? t('featurePolicy.drawer.title', {
                name: editingPerm.username ?? editingPerm.user_group_name ?? '—',
                type: permissionService.getPermissionTypeName(editingPerm.permission_type),
              })
            : ''
        }
        open={drawerVisible}
        onClose={() => setDrawerVisible(false)}
        width={600}
        footer={
          <Flex justify="flex-end" gap={token.marginSM}>
            <Button onClick={() => setDrawerVisible(false)}>{t('common:actions.cancel')}</Button>
            <Button type="primary" loading={saveLoading} onClick={handleSave}>
              {t('featurePolicy.drawer.save')}
            </Button>
          </Flex>
        }
      >
        {featureLoading || !featureData ? (
          <EmptyState description={t('common:messages.loading')} />
        ) : (
          <FeaturePolicyForm
            featureData={featureData}
            draftPolicy={draftPolicy}
            onChange={setDraftPolicy}
            t={t}
            token={token}
          />
        )}
      </Drawer>
    </div>
  );
};

// ─── FeatureSummaryCell ─────────────────────────────────────────────────────

interface FeatureSummaryCellProps {
  permType: string;
  featureKey: string;
  featurePolicyJSON?: string;
  t: (key: string) => string;
}

const FeatureSummaryCell: React.FC<FeatureSummaryCellProps> = ({
  permType,
  featureKey,
  featurePolicyJSON,
  t,
}) => {
  const ceiling = CEILING[permType] ?? [];

  if (!ceiling.includes(featureKey)) {
    return (
      <Tooltip title={t('featurePolicy.summary.notInCeiling')}>
        <Text type="secondary">—</Text>
      </Tooltip>
    );
  }

  let policyDisabled = false;
  if (featurePolicyJSON) {
    try {
      const policy = JSON.parse(featurePolicyJSON) as Record<string, boolean>;
      if (policy[featureKey] === false) policyDisabled = true;
    } catch {
      // ignore malformed JSON
    }
  }

  if (policyDisabled) {
    return (
      <Tooltip title={t('featurePolicy.summary.policyDisabled')}>
        <Text type="danger">✗</Text>
      </Tooltip>
    );
  }

  return (
    <Tooltip title={t('featurePolicy.summary.inCeiling')}>
      <Text style={{ color: '#52c41a' }}>✓</Text>
    </Tooltip>
  );
};

// ─── FeaturePolicyForm ──────────────────────────────────────────────────────

interface FeaturePolicyFormProps {
  featureData: FeaturePolicyResponse;
  draftPolicy: Record<string, boolean>;
  onChange: (next: Record<string, boolean>) => void;
  t: (key: string, opts?: Record<string, unknown>) => string;
  token: ReturnType<typeof theme.useToken>['token'];
}

const FeaturePolicyForm: React.FC<FeaturePolicyFormProps> = ({
  featureData,
  draftPolicy,
  onChange,
  t,
  token,
}) => {
  const handleToggle = (featureKey: string, enabled: boolean) => {
    onChange({ ...draftPolicy, [featureKey]: enabled });
  };

  const isDraftEnabled = (featureKey: string): boolean => {
    if (featureKey in draftPolicy) return draftPolicy[featureKey];
    return featureData.ceiling.includes(featureKey);
  };

  return (
    <div>
      {FEATURE_GROUPS.map((group, gi) => (
        <div key={group.groupKey}>
          {gi > 0 && <Divider style={{ margin: `${token.marginSM}px 0` }} />}
          <Text
            strong
            style={{
              display: 'block',
              marginBottom: token.marginXS,
              color: token.colorTextSecondary,
              fontSize: token.fontSizeSM,
              textTransform: 'uppercase',
              letterSpacing: '0.5px',
            }}
          >
            {t(group.labelKey)}
          </Text>
          <Space direction="vertical" size={token.marginSM} style={{ width: '100%' }}>
            {group.features.map((feat) => {
              const inCeiling = featureData.ceiling.includes(feat.key);
              const enabled = isDraftEnabled(feat.key);
              return (
                <Flex key={feat.key} justify="space-between" align="center">
                  <div>
                    <Text>{t(feat.labelKey)}</Text>
                    <br />
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      {inCeiling
                        ? t(feat.descKey)
                        : t('featurePolicy.drawer.notInCeiling', {
                            type: featureData.permission_type,
                          })}
                    </Text>
                  </div>
                  <Tooltip
                    title={
                      !inCeiling
                        ? t('featurePolicy.drawer.notInCeiling', {
                            type: featureData.permission_type,
                          })
                        : undefined
                    }
                  >
                    <Switch
                      checked={inCeiling && enabled}
                      disabled={!inCeiling}
                      onChange={(val) => handleToggle(feat.key, val)}
                      size="small"
                    />
                  </Tooltip>
                </Flex>
              );
            })}
          </Space>
        </div>
      ))}
    </div>
  );
};

export default FeaturePolicyPage;

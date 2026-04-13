import React, { useState, useCallback, useEffect, useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import EmptyState from '../../components/EmptyState';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Input,
  Select,
  Modal,
  Form,
  Drawer,
  App,
  Tooltip,
  Dropdown,
  Row,
  Col,
  Statistic,
  Flex,
  Typography,
  theme,
} from 'antd';
import type { MenuProps } from 'antd';
import {
  ReloadOutlined,
  PlusOutlined,
  HistoryOutlined,
  UploadOutlined,
  RollbackOutlined,
  DeleteOutlined,
  SearchOutlined,
  MoreOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  AppstoreOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import helmService from '../../services/helmService';
import { usePermission } from '../../hooks/usePermission';
import type {
  HelmRelease,
  HelmHistory,
  HelmRepo,
  ChartInfo,
  InstallReleaseRequest,
  UpgradeReleaseRequest,
} from '../../services/helmService';

dayjs.extend(relativeTime);

const { TextArea } = Input;
const { Text } = Typography;

// ─── Status ───────────────────────────────────────────────────────────────────

const statusColour = (status: string): string => {
  switch (status) {
    case 'deployed':      return 'success';
    case 'failed':        return 'error';
    case 'pending-install':
    case 'pending-upgrade':
    case 'pending-rollback':
    case 'uninstalling':  return 'processing';
    case 'superseded':    return 'default';
    default:              return 'blue';
  }
};

// ─── Component ────────────────────────────────────────────────────────────────

const HelmList: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { t } = useTranslation('helm');
  const { message, modal } = App.useApp();
  const { hasFeature } = usePermission();
  const { token } = theme.useToken();

  // ── state ────────────────────────────────────────────────────────────────
  const [releases, setReleases] = useState<HelmRelease[]>([]);
  const [loading, setLoading] = useState(false);
  const [namespace, setNamespace] = useState('');
  const [searchText, setSearchText] = useState('');

  // Install modal
  const [installVisible, setInstallVisible] = useState(false);
  const [installLoading, setInstallLoading] = useState(false);
  const [repos, setRepos] = useState<HelmRepo[]>([]);
  const [charts, setCharts] = useState<ChartInfo[]>([]);
  const [chartKeyword, setChartKeyword] = useState('');
  const [installForm] = Form.useForm<InstallReleaseRequest>();

  // Upgrade modal
  const [upgradeVisible, setUpgradeVisible] = useState(false);
  const [upgradeLoading, setUpgradeLoading] = useState(false);
  const [upgradeTarget, setUpgradeTarget] = useState<HelmRelease | null>(null);
  const [upgradeForm] = Form.useForm<UpgradeReleaseRequest>();

  // Rollback modal
  const [rollbackVisible, setRollbackVisible] = useState(false);
  const [rollbackLoading, setRollbackLoading] = useState(false);
  const [rollbackTarget, setRollbackTarget] = useState<HelmRelease | null>(null);
  const [rollbackRevision, setRollbackRevision] = useState<number>(0);
  const [historyItems, setHistoryItems] = useState<HelmHistory[]>([]);

  // History drawer
  const [historyVisible, setHistoryVisible] = useState(false);
  const [historyTarget, setHistoryTarget] = useState<HelmRelease | null>(null);

  // ── derived data ─────────────────────────────────────────────────────────
  const namespaceOptions = useMemo(() => {
    const ns = [...new Set(releases.map((r) => r.namespace))].sort();
    return ns.map((n) => ({ label: n, value: n }));
  }, [releases]);

  const stats = useMemo(() => ({
    total:    releases.length,
    deployed: releases.filter((r) => r.status === 'deployed').length,
    failed:   releases.filter((r) => r.status === 'failed').length,
    pending:  releases.filter((r) => r.status.startsWith('pending') || r.status === 'uninstalling').length,
  }), [releases]);

  const filteredReleases = useMemo(() =>
    releases.filter((r) =>
      (!namespace || r.namespace === namespace) &&
      (!searchText ||
        r.name.toLowerCase().includes(searchText.toLowerCase()) ||
        r.chart.toLowerCase().includes(searchText.toLowerCase()))
    ),
    [releases, namespace, searchText]
  );

  // ── fetch ─────────────────────────────────────────────────────────────────
  const fetchReleases = useCallback(async () => {
    if (!clusterId) return;
    setLoading(true);
    try {
      const resp = await helmService.listReleases(clusterId);
      const items = Array.isArray(resp) ? resp : (resp as { items: HelmRelease[] }).items ?? [];
      setReleases(items);
    } catch {
      message.error(t('fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message, t]);

  const fetchRepos = useCallback(async () => {
    try {
      const resp = await helmService.listRepos();
      setRepos(Array.isArray(resp) ? resp : []);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { fetchReleases(); }, [fetchReleases]);

  // ── install ───────────────────────────────────────────────────────────────
  const handleOpenInstall = async () => {
    await fetchRepos();
    setCharts([]);
    setChartKeyword('');
    installForm.resetFields();
    setInstallVisible(true);
  };

  const handleSearchCharts = async (keyword: string) => {
    setChartKeyword(keyword);
    try {
      const resp = await helmService.searchCharts(keyword);
      setCharts(Array.isArray(resp) ? resp : []);
    } catch { /* ignore */ }
  };

  const handleInstall = async () => {
    try {
      const values = await installForm.validateFields();
      setInstallLoading(true);
      await helmService.installRelease(clusterId!, values);
      message.success(t('installSuccess'));
      setInstallVisible(false);
      fetchReleases();
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'errorFields' in err) return;
      message.error(t('installError'));
    } finally {
      setInstallLoading(false);
    }
  };

  // ── upgrade ───────────────────────────────────────────────────────────────
  const handleOpenUpgrade = async (record: HelmRelease) => {
    setUpgradeTarget(record);
    try {
      const vals = await helmService.getValues(clusterId!, record.namespace, record.name, false);
      upgradeForm.setFieldsValue({ values: JSON.stringify(vals, null, 2) });
    } catch {
      upgradeForm.resetFields();
    }
    setUpgradeVisible(true);
  };

  const handleUpgrade = async () => {
    if (!upgradeTarget) return;
    try {
      const values = await upgradeForm.validateFields();
      setUpgradeLoading(true);
      await helmService.upgradeRelease(clusterId!, upgradeTarget.namespace, upgradeTarget.name, values);
      message.success(t('upgradeSuccess'));
      setUpgradeVisible(false);
      fetchReleases();
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'errorFields' in err) return;
      message.error(t('upgradeError'));
    } finally {
      setUpgradeLoading(false);
    }
  };

  // ── rollback ──────────────────────────────────────────────────────────────
  const handleOpenRollback = async (record: HelmRelease) => {
    setRollbackTarget(record);
    setRollbackRevision(0);
    try {
      const resp = await helmService.getHistory(clusterId!, record.namespace, record.name);
      setHistoryItems(Array.isArray(resp) ? resp : []);
    } catch {
      setHistoryItems([]);
    }
    setRollbackVisible(true);
  };

  const handleRollback = async () => {
    if (!rollbackTarget || !rollbackRevision) return;
    setRollbackLoading(true);
    try {
      await helmService.rollbackRelease(clusterId!, rollbackTarget.namespace, rollbackTarget.name, rollbackRevision);
      message.success(t('rollbackSuccess'));
      setRollbackVisible(false);
      fetchReleases();
    } catch {
      message.error(t('rollbackError'));
    } finally {
      setRollbackLoading(false);
    }
  };

  // ── uninstall ─────────────────────────────────────────────────────────────
  const handleUninstall = (record: HelmRelease) => {
    modal.confirm({
      title: t('uninstallConfirm'),
      content: t('uninstallConfirmContent', { name: record.name }),
      okType: 'danger',
      okText: t('uninstall'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          await helmService.uninstallRelease(clusterId!, record.namespace, record.name);
          message.success(t('uninstallSuccess'));
          fetchReleases();
        } catch {
          message.error(t('uninstallError'));
        }
      },
    });
  };

  // ── history drawer ────────────────────────────────────────────────────────
  const handleShowHistory = async (record: HelmRelease) => {
    setHistoryTarget(record);
    try {
      const resp = await helmService.getHistory(clusterId!, record.namespace, record.name);
      setHistoryItems(Array.isArray(resp) ? resp : []);
    } catch {
      setHistoryItems([]);
    }
    setHistoryVisible(true);
  };

  // ── row action menu ───────────────────────────────────────────────────────
  const getActionMenuItems = (record: HelmRelease): MenuProps['items'] => [
    {
      key: 'history',
      icon: <HistoryOutlined />,
      label: t('history'),
      onClick: () => handleShowHistory(record),
    },
    ...(hasFeature('helm:write', clusterId) ? [
      {
        key: 'upgrade',
        icon: <UploadOutlined />,
        label: t('upgrade'),
        onClick: () => handleOpenUpgrade(record),
      },
      {
        key: 'rollback',
        icon: <RollbackOutlined />,
        label: t('rollback'),
        onClick: () => handleOpenRollback(record),
      },
      { type: 'divider' as const },
      {
        key: 'uninstall',
        icon: <DeleteOutlined />,
        label: t('uninstall'),
        danger: true,
        onClick: () => handleUninstall(record),
      },
    ] : []),
  ];

  // ── columns ───────────────────────────────────────────────────────────────
  const columns: ColumnsType<HelmRelease> = [
    {
      title: t('name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (name: string, record) => (
        <Button type="link" style={{ padding: 0 }} onClick={() => handleShowHistory(record)}>
          {name}
        </Button>
      ),
    },
    {
      title: t('namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 140,
    },
    {
      title: t('chart'),
      key: 'chart',
      ellipsis: true,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text style={{ fontSize: token.fontSize }}>{record.chart}</Text>
          {record.app_version && (
            <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
              app {record.app_version}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: t('version'),
      dataIndex: 'version',
      key: 'version',
      width: 90,
    },
    {
      title: t('status'),
      dataIndex: 'status',
      key: 'status',
      width: 130,
      render: (status: string) => (
        <Tag color={statusColour(status)}>{t(`statusLabel.${status.replace(/-/g, '_')}`, status)}</Tag>
      ),
    },
    {
      title: t('revision'),
      dataIndex: 'revision',
      key: 'revision',
      width: 80,
      align: 'center',
      render: (rev: number) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>#{rev}</Text>
      ),
    },
    {
      title: t('updatedAt'),
      dataIndex: 'updated_at',
      key: 'updated_at',
      width: 120,
      render: (t_: string) => t_ ? (
        <Tooltip title={dayjs(t_).format('YYYY-MM-DD HH:mm:ss')}>
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
            {dayjs(t_).fromNow()}
          </Text>
        </Tooltip>
      ) : '—',
    },
    {
      title: t('actions'),
      key: 'actions',
      width: 64,
      fixed: 'right',
      render: (_, record) => (
        <Dropdown menu={{ items: getActionMenuItems(record) }} trigger={['click']}>
          <Button size="small" icon={<MoreOutlined />} />
        </Dropdown>
      ),
    },
  ];

  const historyColumns: ColumnsType<HelmHistory> = [
    {
      title: t('revision'),
      dataIndex: 'revision',
      key: 'revision',
      width: 80,
      render: (rev: number) => (
        <Text strong={rev === historyTarget?.revision}>
          #{rev}{rev === historyTarget?.revision ? ` (${t('current')})` : ''}
        </Text>
      ),
    },
    { title: t('chart'), dataIndex: 'chart', key: 'chart' },
    { title: t('appVersion'), dataIndex: 'app_version', key: 'app_version', width: 110 },
    {
      title: t('status'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (s: string) => <Tag color={statusColour(s)}>{t(`statusLabel.${s.replace(/-/g, '_')}`, s)}</Tag>,
    },
    {
      title: t('updatedAt'),
      dataIndex: 'updated_at',
      key: 'updated_at',
      width: 160,
      render: (t_: string) => t_ ? dayjs(t_).format('YYYY-MM-DD HH:mm') : '—',
    },
    { title: t('description'), dataIndex: 'description', key: 'description', ellipsis: true },
  ];

  // ── render ────────────────────────────────────────────────────────────────
  return (
    <div style={{ padding: token.paddingLG }}>

      {/* ── Stat cards ── */}
      <Row gutter={token.marginSM} style={{ marginBottom: token.marginMD }}>
        <Col span={6}>
          <Card variant="borderless" size="small">
            <Statistic
              title={t('stats.total')}
              value={stats.total}
              prefix={<AppstoreOutlined style={{ color: token.colorPrimary }} />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card variant="borderless" size="small">
            <Statistic
              title={t('stats.deployed')}
              value={stats.deployed}
              valueStyle={{ color: token.colorSuccess }}
              prefix={<CheckCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card variant="borderless" size="small">
            <Statistic
              title={t('stats.failed')}
              value={stats.failed}
              valueStyle={{ color: stats.failed > 0 ? token.colorError : token.colorTextSecondary }}
              prefix={<CloseCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card variant="borderless" size="small">
            <Statistic
              title={t('stats.pending')}
              value={stats.pending}
              valueStyle={{ color: stats.pending > 0 ? token.colorWarning : token.colorTextSecondary }}
              prefix={<SyncOutlined spin={stats.pending > 0} />}
            />
          </Card>
        </Col>
      </Row>

      {/* ── Main table card ── */}
      <Card variant="borderless">
        {/* Toolbar */}
        <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
          <Space>
            <Input
              prefix={<SearchOutlined />}
              placeholder={t('search')}
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              style={{ width: 240 }}
              allowClear
            />
            <Select
              placeholder={t('allNamespaces')}
              allowClear
              style={{ width: 160 }}
              value={namespace || undefined}
              onChange={(v) => setNamespace(v ?? '')}
              options={namespaceOptions}
            />
          </Space>
          <Space>
            <Tooltip title={t('refresh')}>
              <Button icon={<ReloadOutlined />} onClick={fetchReleases} loading={loading} />
            </Tooltip>
            {hasFeature('helm:write', clusterId) && (
              <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenInstall}>
                {t('install')}
              </Button>
            )}
          </Space>
        </Flex>

        <Table<HelmRelease>
          rowKey={(r) => `${r.namespace}/${r.name}`}
          columns={columns}
          dataSource={filteredReleases}
          loading={loading}
          size="middle"
          pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (total) => `${total} releases` }}
          scroll={{ x: 'max-content' }}
          locale={{ emptyText: <EmptyState description={t('noReleases')} /> }}
        />
      </Card>

      {/* ── Install Modal ── */}
      <Modal
        title={t('installRelease')}
        open={installVisible}
        onCancel={() => setInstallVisible(false)}
        onOk={handleInstall}
        confirmLoading={installLoading}
        width={640}
        destroyOnClose
      >
        <Form form={installForm} layout="vertical">
          <Form.Item name="release_name" label={t('releaseName')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="namespace" label={t('namespace')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="repo_name" label={t('repoName')} rules={[{ required: true }]}>
            <Select placeholder={t('selectRepo')} options={repos.map((r) => ({ label: r.name, value: r.name }))} />
          </Form.Item>
          <Form.Item label={t('searchChart')}>
            <Input.Search
              placeholder={t('chartKeyword')}
              onSearch={handleSearchCharts}
              value={chartKeyword}
              onChange={(e) => setChartKeyword(e.target.value)}
            />
          </Form.Item>
          <Form.Item name="chart_name" label={t('chartName')} rules={[{ required: true }]}>
            <Select
              placeholder={t('selectChart')}
              showSearch
              options={charts.map((c) => ({
                label: `${c.name} ${c.version} — ${c.description}`,
                value: c.name,
              }))}
            />
          </Form.Item>
          <Form.Item name="version" label={t('version')}>
            <Input placeholder={t('latestVersion')} />
          </Form.Item>
          <Form.Item name="values" label={t('values')}>
            <TextArea rows={6} placeholder="key: value" style={{ fontFamily: 'monospace' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* ── Upgrade Modal ── */}
      <Modal
        title={`${t('upgradeRelease')} — ${upgradeTarget?.name}`}
        open={upgradeVisible}
        onCancel={() => setUpgradeVisible(false)}
        onOk={handleUpgrade}
        confirmLoading={upgradeLoading}
        width={640}
        destroyOnClose
      >
        <Form form={upgradeForm} layout="vertical">
          <Form.Item name="version" label={t('version')}>
            <Input placeholder={t('keepCurrentVersion')} />
          </Form.Item>
          <Form.Item name="values" label={t('values')}>
            <TextArea rows={10} style={{ fontFamily: 'monospace' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* ── Rollback Modal ── */}
      <Modal
        title={`${t('rollbackRelease')} — ${rollbackTarget?.name}`}
        open={rollbackVisible}
        onCancel={() => setRollbackVisible(false)}
        onOk={handleRollback}
        confirmLoading={rollbackLoading}
        destroyOnClose
      >
        <Form layout="vertical">
          <Form.Item label={t('selectRevision')} required>
            <Select
              style={{ width: '100%' }}
              value={rollbackRevision || undefined}
              onChange={(v) => setRollbackRevision(v)}
              placeholder={t('chooseRevision')}
              options={historyItems
                .filter((h) => h.revision !== rollbackTarget?.revision)
                .map((h) => ({
                  label: `#${h.revision} — ${h.chart} · ${t(`statusLabel.${h.status.replace(/-/g, '_')}`, h.status)} · ${h.updated_at ? dayjs(h.updated_at).format('MM-DD HH:mm') : ''}`,
                  value: h.revision,
                }))}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* ── History Drawer ── */}
      <Drawer
        title={`${t('releaseHistory')} — ${historyTarget?.name}`}
        open={historyVisible}
        onClose={() => setHistoryVisible(false)}
        width={800}
      >
        <Table<HelmHistory>
          scroll={{ x: 'max-content' }}
          rowKey="revision"
          columns={historyColumns}
          dataSource={historyItems}
          pagination={false}
          size="small"
          rowClassName={(r) => r.revision === historyTarget?.revision ? 'ant-table-row-selected' : ''}
          locale={{ emptyText: <EmptyState description={t('noHistory')} /> }}
        />
      </Drawer>
    </div>
  );
};

export default HelmList;

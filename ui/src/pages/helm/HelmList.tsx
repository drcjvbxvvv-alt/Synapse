import React, { useState, useCallback, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
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
} from 'antd';
import {
  ReloadOutlined,
  PlusOutlined,
  HistoryOutlined,
  UploadOutlined,
  RollbackOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import helmService from '../../services/helmService';
import type {
  HelmRelease,
  HelmHistory,
  HelmRepo,
  ChartInfo,
  InstallReleaseRequest,
  UpgradeReleaseRequest,
} from '../../services/helmService';

const { Option } = Select;
const { TextArea } = Input;

// Status colour mapping
const statusColour = (status: string): string => {
  switch (status) {
    case 'deployed':
      return 'green';
    case 'failed':
      return 'red';
    case 'pending-install':
    case 'pending-upgrade':
    case 'pending-rollback':
      return 'orange';
    case 'superseded':
      return 'default';
    default:
      return 'blue';
  }
};

const HelmList: React.FC = () => {
  const { id: clusterId } = useParams<{ id: string }>();
  const { t } = useTranslation('helm');
  const { message, modal } = App.useApp();

  // ---- state ----
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

  // ---- data fetch ----
  const fetchReleases = useCallback(async () => {
    if (!clusterId) return;
    setLoading(true);
    try {
      const resp = await helmService.listReleases(clusterId, namespace || undefined);
      const items = Array.isArray(resp) ? resp : (resp as { items: HelmRelease[] }).items ?? [];
      setReleases(items);
    } catch (err) {
      message.error(t('fetchError', 'Failed to fetch releases'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, message, t]);

  const fetchRepos = useCallback(async () => {
    try {
      const resp = await helmService.listRepos();
      setRepos(Array.isArray(resp) ? resp : []);
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    fetchReleases();
  }, [fetchReleases]);

  // ---- filtered data ----
  const filteredReleases = releases.filter(
    (r) =>
      !searchText ||
      r.name.toLowerCase().includes(searchText.toLowerCase()) ||
      r.chart.toLowerCase().includes(searchText.toLowerCase())
  );

  // ---- install ----
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
    } catch {
      // ignore
    }
  };

  const handleInstall = async () => {
    try {
      const values = await installForm.validateFields();
      setInstallLoading(true);
      await helmService.installRelease(clusterId!, values);
      message.success(t('installSuccess', 'Release installed successfully'));
      setInstallVisible(false);
      fetchReleases();
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'errorFields' in err) return; // validation error
      message.error(t('installError', 'Failed to install release'));
    } finally {
      setInstallLoading(false);
    }
  };

  // ---- upgrade ----
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
      await helmService.upgradeRelease(
        clusterId!,
        upgradeTarget.namespace,
        upgradeTarget.name,
        values
      );
      message.success(t('upgradeSuccess', 'Release upgraded successfully'));
      setUpgradeVisible(false);
      fetchReleases();
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'errorFields' in err) return;
      message.error(t('upgradeError', 'Failed to upgrade release'));
    } finally {
      setUpgradeLoading(false);
    }
  };

  // ---- rollback ----
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
      await helmService.rollbackRelease(
        clusterId!,
        rollbackTarget.namespace,
        rollbackTarget.name,
        rollbackRevision
      );
      message.success(t('rollbackSuccess', 'Release rolled back successfully'));
      setRollbackVisible(false);
      fetchReleases();
    } catch {
      message.error(t('rollbackError', 'Failed to rollback release'));
    } finally {
      setRollbackLoading(false);
    }
  };

  // ---- uninstall ----
  const handleUninstall = (record: HelmRelease) => {
    modal.confirm({
      title: t('uninstallConfirm', 'Uninstall Release'),
      content: t('uninstallConfirmContent', `Are you sure you want to uninstall "${record.name}"?`),
      okType: 'danger',
      onOk: async () => {
        try {
          await helmService.uninstallRelease(clusterId!, record.namespace, record.name);
          message.success(t('uninstallSuccess', 'Release uninstalled successfully'));
          fetchReleases();
        } catch {
          message.error(t('uninstallError', 'Failed to uninstall release'));
        }
      },
    });
  };

  // ---- history drawer ----
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

  // ---- columns ----
  const columns: ColumnsType<HelmRelease> = [
    {
      title: t('name', 'Name'),
      dataIndex: 'name',
      key: 'name',
      sorter: (a, b) => a.name.localeCompare(b.name),
    },
    {
      title: t('namespace', 'Namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
    },
    {
      title: t('chart', 'Chart'),
      dataIndex: 'chart',
      key: 'chart',
    },
    {
      title: t('version', 'Version'),
      dataIndex: 'version',
      key: 'version',
    },
    {
      title: t('appVersion', 'App Version'),
      dataIndex: 'app_version',
      key: 'app_version',
    },
    {
      title: t('status', 'Status'),
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={statusColour(status)}>{status}</Tag>
      ),
    },
    {
      title: t('revision', 'Revision'),
      dataIndex: 'revision',
      key: 'revision',
      width: 90,
    },
    {
      title: t('updatedAt', 'Updated At'),
      dataIndex: 'updated_at',
      key: 'updated_at',
    },
    {
      title: t('actions', 'Actions'),
      key: 'actions',
      width: 160,
      render: (_, record) => (
        <Space size="small">
          <Tooltip title={t('history', 'History')}>
            <Button
              size="small"
              icon={<HistoryOutlined />}
              onClick={() => handleShowHistory(record)}
            />
          </Tooltip>
          <Tooltip title={t('upgrade', 'Upgrade')}>
            <Button
              size="small"
              icon={<UploadOutlined />}
              onClick={() => handleOpenUpgrade(record)}
            />
          </Tooltip>
          <Tooltip title={t('rollback', 'Rollback')}>
            <Button
              size="small"
              icon={<RollbackOutlined />}
              onClick={() => handleOpenRollback(record)}
            />
          </Tooltip>
          <Tooltip title={t('uninstall', 'Uninstall')}>
            <Button
              size="small"
              danger
              icon={<DeleteOutlined />}
              onClick={() => handleUninstall(record)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ];

  const historyColumns: ColumnsType<HelmHistory> = [
    { title: t('revision', 'Revision'), dataIndex: 'revision', key: 'revision' },
    { title: t('chart', 'Chart'), dataIndex: 'chart', key: 'chart' },
    { title: t('appVersion', 'App Version'), dataIndex: 'app_version', key: 'app_version' },
    {
      title: t('status', 'Status'),
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => <Tag color={statusColour(s)}>{s}</Tag>,
    },
    { title: t('updatedAt', 'Updated At'), dataIndex: 'updated_at', key: 'updated_at' },
    { title: t('description', 'Description'), dataIndex: 'description', key: 'description' },
  ];

  return (
    <>
      <Card
        title={t('helmReleases', 'Helm Releases')}
        extra={
          <Space>
            <Input
              placeholder={t('search', 'Search by name or chart')}
              prefix={<ReloadOutlined />}
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              style={{ width: 220 }}
              allowClear
            />
            <Select
              placeholder={t('allNamespaces', 'All Namespaces')}
              allowClear
              style={{ width: 180 }}
              value={namespace || undefined}
              onChange={(v) => setNamespace(v ?? '')}
            >
              <Option value="">{t('allNamespaces', 'All Namespaces')}</Option>
            </Select>
            <Button icon={<ReloadOutlined />} onClick={fetchReleases}>
              {t('refresh', 'Refresh')}
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenInstall}>
              {t('install', 'Install')}
            </Button>
          </Space>
        }
      >
        <Table<HelmRelease>
          rowKey={(r) => `${r.namespace}/${r.name}`}
          columns={columns}
          dataSource={filteredReleases}
          loading={loading}
          pagination={{ pageSize: 20, showSizeChanger: true }}
          scroll={{ x: 'max-content' }}
          locale={{ emptyText: <EmptyState description={t('noReleases')} /> }}
        />
      </Card>

      {/* Install Modal */}
      <Modal
        title={t('installRelease', 'Install Helm Release')}
        open={installVisible}
        onCancel={() => setInstallVisible(false)}
        onOk={handleInstall}
        confirmLoading={installLoading}
        width={640}
        destroyOnClose
      >
        <Form form={installForm} layout="vertical">
          <Form.Item
            name="release_name"
            label={t('releaseName', 'Release Name')}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="namespace"
            label={t('namespace', 'Namespace')}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="repo_name"
            label={t('repoName', 'Repository')}
            rules={[{ required: true }]}
          >
            <Select placeholder={t('selectRepo', 'Select Repository')}>
              {repos.map((r) => (
                <Option key={r.name} value={r.name}>
                  {r.name}
                </Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item label={t('searchChart', 'Search Chart')}>
            <Input.Search
              placeholder={t('chartKeyword', 'Enter chart keyword')}
              onSearch={handleSearchCharts}
              value={chartKeyword}
              onChange={(e) => setChartKeyword(e.target.value)}
            />
          </Form.Item>
          <Form.Item
            name="chart_name"
            label={t('chartName', 'Chart')}
            rules={[{ required: true }]}
          >
            <Select placeholder={t('selectChart', 'Select Chart')} showSearch>
              {charts.map((c) => (
                <Option key={`${c.repo_name}/${c.name}`} value={c.name}>
                  {c.name} ({c.version}) — {c.description}
                </Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="version" label={t('version', 'Version')}>
            <Input placeholder={t('latestVersion', 'Leave blank for latest')} />
          </Form.Item>
          <Form.Item name="values" label={t('values', 'Values (YAML)')}>
            <TextArea rows={6} placeholder="key: value" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Upgrade Modal */}
      <Modal
        title={`${t('upgradeRelease', 'Upgrade')} — ${upgradeTarget?.name}`}
        open={upgradeVisible}
        onCancel={() => setUpgradeVisible(false)}
        onOk={handleUpgrade}
        confirmLoading={upgradeLoading}
        width={640}
        destroyOnClose
      >
        <Form form={upgradeForm} layout="vertical">
          <Form.Item name="version" label={t('version', 'Version')}>
            <Input placeholder={t('keepCurrentVersion', 'Leave blank to keep current version')} />
          </Form.Item>
          <Form.Item name="values" label={t('values', 'Values (YAML)')}>
            <TextArea rows={10} />
          </Form.Item>
        </Form>
      </Modal>

      {/* Rollback Modal */}
      <Modal
        title={`${t('rollbackRelease', 'Rollback')} — ${rollbackTarget?.name}`}
        open={rollbackVisible}
        onCancel={() => setRollbackVisible(false)}
        onOk={handleRollback}
        confirmLoading={rollbackLoading}
        destroyOnClose
      >
        <Form layout="vertical">
          <Form.Item label={t('selectRevision', 'Select Revision')} required>
            <Select
              style={{ width: '100%' }}
              value={rollbackRevision || undefined}
              onChange={(v) => setRollbackRevision(v)}
              placeholder={t('chooseRevision', 'Choose a revision to rollback to')}
            >
              {historyItems
                .filter((h) => h.revision !== rollbackTarget?.revision)
                .map((h) => (
                  <Option key={h.revision} value={h.revision}>
                    #{h.revision} — {h.chart} ({h.status}) — {h.updated_at}
                  </Option>
                ))}
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* History Drawer */}
      <Drawer
        title={`${t('releaseHistory', 'Release History')} — ${historyTarget?.name}`}
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
          locale={{ emptyText: <EmptyState description={t('noHistory')} /> }}
        />
      </Drawer>
    </>
  );
};

export default HelmList;

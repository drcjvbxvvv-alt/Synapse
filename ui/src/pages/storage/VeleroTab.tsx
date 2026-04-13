import React, { useState, useEffect, useCallback } from 'react';
import {
  Table, Button, Space, Tag, Badge, Modal, Form, Input, Switch,
  Popconfirm, Typography, Spin, App, Tabs, Tooltip, Progress,
} from 'antd';
import {
  ReloadOutlined, PlusOutlined, PlayCircleOutlined,
} from '@ant-design/icons';
import NotInstalledCard from '../../components/NotInstalledCard';
import { useTranslation } from 'react-i18next';
import EmptyState from '../../components/EmptyState';
import { usePermission } from '../../hooks/usePermission';
import {
  snapshotService, backupPhaseColor, restorePhaseColor,
  type VeleroBackupInfo, type VeleroRestoreInfo, type VeleroScheduleInfo,
} from '../../services/snapshotService';

const { Text } = Typography;

interface VeleroTabProps { clusterId: string }

// ─── Backup sub-panel ───────────────────────────────────────────────────────

const BackupPanel: React.FC<{ clusterId: string; veleroNS: string }> = ({ clusterId, veleroNS }) => {
  const { t } = useTranslation(['storage', 'common']);
  const { message } = App.useApp();
  const { canWrite } = usePermission();
  const [loading, setLoading] = useState(true);
  const [backups, setBackups] = useState<VeleroBackupInfo[]>([]);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [selectedBackup, setSelectedBackup] = useState<VeleroBackupInfo | null>(null);
  const [restoreForm] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await snapshotService.listBackups(clusterId, veleroNS);
      setBackups(res.data.items ?? []);
    } catch {
      message.error(t('velero.fetchBackupError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, veleroNS, message, t]);

  useEffect(() => { load(); }, [load]);

  const openRestore = (b: VeleroBackupInfo) => {
    setSelectedBackup(b);
    restoreForm.resetFields();
    setRestoreOpen(true);
  };

  const handleRestore = async () => {
    try {
      const values = await restoreForm.validateFields();
      setRestoring(true);
      await snapshotService.triggerRestore(clusterId, {
        backupName: selectedBackup!.name,
        restoreName: values.restoreName,
        veleroNS,
        includedNamespaces: values.includedNamespaces
          ? values.includedNamespaces.split(',').map((s: string) => s.trim()).filter(Boolean)
          : undefined,
      });
      message.success(t('velero.restoreTriggered'));
      setRestoreOpen(false);
    } catch (e: unknown) {
      if (e && typeof e === 'object' && 'errorFields' in e) return;
      message.error(t('velero.restoreError'));
    } finally {
      setRestoring(false);
    }
  };

  const columns = [
    {
      title: t('velero.name'), dataIndex: 'name', key: 'name',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t('velero.status'), dataIndex: 'phase', key: 'phase',
      render: (v: string) => <Badge status={backupPhaseColor(v) as 'success' | 'error' | 'processing' | 'warning' | 'default'} text={v || '—'} />,
    },
    {
      title: t('velero.namespaces'), dataIndex: 'includedNamespaces', key: 'includedNamespaces',
      render: (v: string[]) => v?.length ? v.map(ns => <Tag key={ns}>{ns}</Tag>) : <Tag>All</Tag>,
    },
    {
      title: t('velero.progress'), key: 'progress',
      render: (_: unknown, r: VeleroBackupInfo) => {
        if (!r.progress) return '—';
        const { totalItems = 0, itemsBackedUp = 0 } = r.progress;
        if (totalItems === 0) return '—';
        return <Progress percent={Math.round((itemsBackedUp / totalItems) * 100)} size="small" style={{ width: 120 }} />;
      },
    },
    {
      title: t('velero.expiration'), dataIndex: 'expiration', key: 'expiration',
      render: (v: string) => v ? new Date(v).toLocaleDateString() : '—',
    },
    {
      title: t('velero.storageLocation'), dataIndex: 'storageLocation', key: 'storageLocation',
      render: (v: string) => v || '—',
    },
    {
      title: t('velero.createdAt'), dataIndex: 'createdAt', key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
    {
      title: t('velero.actions'), key: 'actions', fixed: 'right' as const, width: 100,
      render: (_: unknown, r: VeleroBackupInfo) => canWrite() ? (
        <Tooltip title={t('velero.triggerRestore')}>
          <Button
            type="link" size="small" icon={<PlayCircleOutlined />}
            disabled={r.phase !== 'Completed'}
            onClick={() => openRestore(r)}
          >
            {t('velero.restore')}
          </Button>
        </Tooltip>
      ) : null,
    },
  ];

  return (
    <>
      <Space style={{ marginBottom: 12 }}>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={load}>{t('velero.refresh')}</Button>
      </Space>
      <Table
        rowKey="name" columns={columns} dataSource={backups} loading={loading}
        size="small" scroll={{ x: 1000 }}
        locale={{ emptyText: <EmptyState /> }}
        pagination={{ pageSize: 20, showSizeChanger: true, showTotal: tot => t('velero.total', { total: tot }) }}
      />
      <Modal
        title={t('velero.restoreTitle', { name: selectedBackup?.name })}
        open={restoreOpen}
        onCancel={() => setRestoreOpen(false)}
        onOk={handleRestore}
        confirmLoading={restoring}
        okText={t('velero.confirm')}
        cancelText={t('velero.cancel')}
      >
        <Form form={restoreForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="restoreName" label={t('velero.restoreName')}>
            <Input placeholder={t('velero.restoreNamePlaceholder')} />
          </Form.Item>
          <Form.Item name="includedNamespaces" label={t('velero.includedNamespaces')}>
            <Input placeholder={t('velero.namespacesPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

// ─── Restore sub-panel ──────────────────────────────────────────────────────

const RestorePanel: React.FC<{ clusterId: string; veleroNS: string }> = ({ clusterId, veleroNS }) => {
  const { t } = useTranslation(['storage', 'common']);
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [restores, setRestores] = useState<VeleroRestoreInfo[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await snapshotService.listRestores(clusterId, veleroNS);
      setRestores(res.data.items ?? []);
    } catch {
      message.error(t('velero.fetchRestoreError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, veleroNS, message, t]);

  useEffect(() => { load(); }, [load]);

  const columns = [
    {
      title: t('velero.name'), dataIndex: 'name', key: 'name',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t('velero.backupName'), dataIndex: 'backupName', key: 'backupName',
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: t('velero.status'), dataIndex: 'phase', key: 'phase',
      render: (v: string) => <Badge status={restorePhaseColor(v) as 'success' | 'error' | 'processing' | 'warning' | 'default'} text={v || '—'} />,
    },
    {
      title: t('velero.warnings'), dataIndex: 'warnings', key: 'warnings',
      render: (v: number) => v > 0 ? <Tag color="orange">{v}</Tag> : '0',
    },
    {
      title: t('velero.errors'), dataIndex: 'errors', key: 'errors',
      render: (v: number) => v > 0 ? <Tag color="red">{v}</Tag> : '0',
    },
    {
      title: t('velero.createdAt'), dataIndex: 'createdAt', key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
  ];

  return (
    <>
      <Space style={{ marginBottom: 12 }}>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={load}>{t('velero.refresh')}</Button>
      </Space>
      <Table
        rowKey="name" columns={columns} dataSource={restores} loading={loading}
        size="small" scroll={{ x: 800 }}
        locale={{ emptyText: <EmptyState /> }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />
    </>
  );
};

// ─── Schedule sub-panel ─────────────────────────────────────────────────────

const SchedulePanel: React.FC<{ clusterId: string; veleroNS: string }> = ({ clusterId, veleroNS }) => {
  const { t } = useTranslation(['storage', 'common']);
  const { message } = App.useApp();
  const { canWrite } = usePermission();
  const [loading, setLoading] = useState(true);
  const [schedules, setSchedules] = useState<VeleroScheduleInfo[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await snapshotService.listSchedules(clusterId, veleroNS);
      setSchedules(res.data.items ?? []);
    } catch {
      message.error(t('velero.fetchScheduleError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, veleroNS, message, t]);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      setCreating(true);
      await snapshotService.createSchedule(clusterId, {
        name: values.name,
        schedule: values.schedule,
        veleroNS,
        paused: values.paused ?? false,
        includedNamespaces: values.includedNamespaces
          ? values.includedNamespaces.split(',').map((s: string) => s.trim()).filter(Boolean)
          : undefined,
        storageLocation: values.storageLocation,
        ttl: values.ttl,
      });
      message.success(t('velero.scheduleCreateSuccess'));
      setCreateOpen(false);
      form.resetFields();
      load();
    } catch (e: unknown) {
      if (e && typeof e === 'object' && 'errorFields' in e) return;
      message.error(t('velero.scheduleCreateError'));
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (name: string) => {
    try {
      await snapshotService.deleteSchedule(clusterId, name, veleroNS);
      message.success(t('velero.scheduleDeleteSuccess'));
      load();
    } catch {
      message.error(t('velero.scheduleDeleteError'));
    }
  };

  const columns = [
    {
      title: t('velero.name'), dataIndex: 'name', key: 'name',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t('velero.schedule'), dataIndex: 'schedule', key: 'schedule',
      render: (v: string) => <Text code>{v}</Text>,
    },
    {
      title: t('velero.status'), key: 'status',
      render: (_: unknown, r: VeleroScheduleInfo) => (
        <Space>
          {r.paused
            ? <Tag color="orange">{t('velero.paused')}</Tag>
            : <Badge status="success" text={t('velero.active')} />}
        </Space>
      ),
    },
    {
      title: t('velero.lastBackup'), dataIndex: 'lastBackup', key: 'lastBackup',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
    {
      title: t('velero.storageLocation'), dataIndex: 'storageLocation', key: 'storageLocation',
      render: (v: string) => v || '—',
    },
    {
      title: t('velero.ttl'), dataIndex: 'ttl', key: 'ttl',
      render: (v: string) => v || '—',
    },
    {
      title: t('velero.actions'), key: 'actions', fixed: 'right' as const, width: 100,
      render: (_: unknown, r: VeleroScheduleInfo) => canWrite() ? (
        <Popconfirm
          title={t('velero.confirmDeleteSchedule')}
          description={t('velero.confirmDeleteScheduleDesc', { name: r.name })}
          onConfirm={() => handleDelete(r.name)}
          okText={t('velero.confirm')}
          cancelText={t('velero.cancel')}
        >
          <Button type="link" size="small" danger>{t('velero.delete')}</Button>
        </Popconfirm>
      ) : null,
    },
  ];

  return (
    <>
      <Space style={{ marginBottom: 12 }}>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={load}>{t('velero.refresh')}</Button>
        {canWrite() && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            {t('velero.createSchedule')}
          </Button>
        )}
      </Space>
      <Table
        rowKey="name" columns={columns} dataSource={schedules} loading={loading}
        size="small" scroll={{ x: 900 }}
        locale={{ emptyText: <EmptyState /> }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />
      <Modal
        title={t('velero.createScheduleTitle')}
        open={createOpen}
        onCancel={() => { setCreateOpen(false); form.resetFields(); }}
        onOk={handleCreate}
        confirmLoading={creating}
        okText={t('velero.confirm')}
        cancelText={t('velero.cancel')}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label={t('velero.name')} rules={[{ required: true }]}>
            <Input placeholder="daily-backup" />
          </Form.Item>
          <Form.Item name="schedule" label={t('velero.schedule')} rules={[{ required: true }]}
            help={t('velero.scheduleHelp')}>
            <Input placeholder="0 2 * * *" />
          </Form.Item>
          <Form.Item name="includedNamespaces" label={t('velero.includedNamespaces')}>
            <Input placeholder={t('velero.namespacesPlaceholder')} />
          </Form.Item>
          <Form.Item name="storageLocation" label={t('velero.storageLocation')}>
            <Input placeholder="default" />
          </Form.Item>
          <Form.Item name="ttl" label={t('velero.ttl')}>
            <Input placeholder="720h0m0s" />
          </Form.Item>
          <Form.Item name="paused" label={t('velero.paused')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

// ═══════════════════════════════════════════════════════════════════════════
// Main VeleroTab
// ═══════════════════════════════════════════════════════════════════════════

const VELERO_INSTALL_CMD = 'helm install velero vmware-tanzu/velero --namespace velero --create-namespace';

const VeleroTab: React.FC<VeleroTabProps> = ({ clusterId }) => {
  const { t } = useTranslation('storage');
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [installed, setInstalled] = useState<boolean | null>(null);
  const [veleroNS, setVeleroNS] = useState('velero');

  useEffect(() => {
    snapshotService.checkVelero(clusterId, veleroNS)
      .then((res: { data: { installed: boolean } }) => setInstalled(res.data.installed))
      .catch(() => message.error(t('velero.fetchError')))
      .finally(() => setLoading(false));
  }, [clusterId, veleroNS, message, t]);

  if (loading) return <Spin style={{ display: 'block', marginTop: 60 }} />;

  if (!installed) {
    return (
      <>
        <NotInstalledCard
          title={t('velero.notInstalled')}
          description={t('velero.installHint')}
          command={VELERO_INSTALL_CMD}
          docsUrl="https://velero.io/docs/main/basic-install/"
          onRecheck={() => {
            setInstalled(null);
            setLoading(true);
            snapshotService.checkVelero(clusterId, veleroNS)
              .then((res: { data: { installed: boolean } }) => setInstalled(res.data.installed))
              .catch(() => message.error(t('velero.fetchError')))
              .finally(() => setLoading(false));
          }}
          recheckLoading={loading}
        />
      </>
    );
  }

  const tabs = [
    {
      key: 'backups',
      label: t('velero.tabs.backup'),
      children: <BackupPanel clusterId={clusterId} veleroNS={veleroNS} />,
    },
    {
      key: 'restores',
      label: t('velero.tabs.restore'),
      children: <RestorePanel clusterId={clusterId} veleroNS={veleroNS} />,
    },
    {
      key: 'schedules',
      label: t('velero.tabs.schedule'),
      children: <SchedulePanel clusterId={clusterId} veleroNS={veleroNS} />,
    },
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={8}>
      <Space>
        <Text type="secondary">{t('velero.namespace')}:</Text>
        <Input
          value={veleroNS}
          onChange={e => setVeleroNS(e.target.value)}
          style={{ width: 140 }}
          size="small"
        />
      </Space>
      <Tabs items={tabs} size="small" />
    </Space>
  );
};

export default VeleroTab;

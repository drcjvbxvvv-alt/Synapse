import React, { useState, useEffect, useCallback } from 'react';
import {
  Table, Button, Space, Tag, Badge, Modal, Form, Input, Select,
  Popconfirm, Typography, Empty, Spin, App, Tooltip,
} from 'antd';
import { PlusOutlined, ReloadOutlined, CameraOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  snapshotService,
  type VolumeSnapshotInfo, type VolumeSnapshotClassInfo,
} from '../../services/snapshotService';

const { Text } = Typography;

interface VolumeSnapshotTabProps {
  clusterId: string;
}

const VolumeSnapshotTab: React.FC<VolumeSnapshotTabProps> = ({ clusterId }) => {
  const { t } = useTranslation('storage');
  const { message } = App.useApp();

  const [loading, setLoading] = useState(true);
  const [installed, setInstalled] = useState<boolean | null>(null);
  const [snapshots, setSnapshots] = useState<VolumeSnapshotInfo[]>([]);
  const [snapshotClasses, setSnapshotClasses] = useState<VolumeSnapshotClassInfo[]>([]);
  const [nsFilter, setNsFilter] = useState('');

  // Create modal
  const [createOpen, setCreateOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const statusRes = await snapshotService.checkVolumeSnapshotCRD(clusterId);
      const isInstalled = statusRes.data.installed;
      setInstalled(isInstalled);
      if (isInstalled) {
        const [snapRes, classRes] = await Promise.all([
          snapshotService.listVolumeSnapshots(clusterId, nsFilter || undefined),
          snapshotService.listVolumeSnapshotClasses(clusterId),
        ]);
        setSnapshots(snapRes.data.items ?? []);
        setSnapshotClasses(classRes.data.items ?? []);
      }
    } catch {
      message.error(t('snapshot.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, nsFilter, message, t]);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    try {
      const values = await form.validateFields();
      setCreating(true);
      await snapshotService.createVolumeSnapshot(clusterId, {
        name: values.name,
        namespace: values.namespace,
        pvcName: values.pvcName,
        snapshotClassName: values.snapshotClassName,
      });
      message.success(t('snapshot.createSuccess'));
      setCreateOpen(false);
      form.resetFields();
      load();
    } catch (e: unknown) {
      if (e && typeof e === 'object' && 'errorFields' in e) return; // form validation
      message.error(t('snapshot.createError'));
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (ns: string, name: string) => {
    try {
      await snapshotService.deleteVolumeSnapshot(clusterId, ns, name);
      message.success(t('snapshot.deleteSuccess'));
      load();
    } catch {
      message.error(t('snapshot.deleteError'));
    }
  };

  if (loading && installed === null) {
    return <Spin style={{ display: 'block', marginTop: 60 }} />;
  }

  if (!installed) {
    return (
      <Empty
        image={<CameraOutlined style={{ fontSize: 48, color: '#bfbfbf' }} />}
        description={
          <Space direction="vertical" size={4}>
            <Text strong>{t('snapshot.notInstalled')}</Text>
            <Text type="secondary">{t('snapshot.installHint')}</Text>
            <Text code>kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/master/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml</Text>
          </Space>
        }
        style={{ paddingTop: 60 }}
      />
    );
  }

  const columns = [
    {
      title: t('snapshot.name'), dataIndex: 'name', key: 'name',
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t('snapshot.namespace'), dataIndex: 'namespace', key: 'namespace',
      render: (v: string) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: t('snapshot.sourcePVC'), dataIndex: 'sourcePVC', key: 'sourcePVC',
      render: (v: string) => <Text code>{v}</Text>,
    },
    {
      title: t('snapshot.snapshotClass'), dataIndex: 'snapshotClassName', key: 'snapshotClassName',
      render: (v: string) => v ? <Tag>{v}</Tag> : '—',
    },
    {
      title: t('snapshot.size'), dataIndex: 'restoreSize', key: 'restoreSize',
      render: (v: string) => v || '—',
    },
    {
      title: t('snapshot.readyToUse'), dataIndex: 'readyToUse', key: 'readyToUse',
      render: (v: boolean, r: VolumeSnapshotInfo) => {
        if (r.error) return <Tooltip title={r.error}><Badge status="error" text={t('snapshot.error')} /></Tooltip>;
        return <Badge status={v ? 'success' : 'processing'} text={v ? t('snapshot.ready') : t('snapshot.pending')} />;
      },
    },
    {
      title: t('snapshot.createdAt'), dataIndex: 'createdAt', key: 'createdAt',
      render: (v: string) => v ? new Date(v).toLocaleString() : '—',
    },
    {
      title: t('snapshot.actions'), key: 'actions', fixed: 'right' as const, width: 100,
      render: (_: unknown, r: VolumeSnapshotInfo) => (
        <Popconfirm
          title={t('snapshot.confirmDelete')}
          description={t('snapshot.confirmDeleteDesc', { name: r.name })}
          onConfirm={() => handleDelete(r.namespace, r.name)}
          okText={t('snapshot.confirm')}
          cancelText={t('snapshot.cancel')}
        >
          <Button type="link" size="small" danger>{t('snapshot.delete')}</Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      <Space wrap>
        <Input
          allowClear
          placeholder={t('snapshot.nsFilter')}
          value={nsFilter}
          onChange={e => setNsFilter(e.target.value)}
          style={{ width: 200 }}
          onPressEnter={load}
        />
        <Button icon={<ReloadOutlined />} loading={loading} onClick={load}>{t('snapshot.refresh')}</Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
          {t('snapshot.create')}
        </Button>
      </Space>

      <Table
        rowKey={r => `${r.namespace}/${r.name}`}
        columns={columns}
        dataSource={snapshots}
        loading={loading}
        size="small"
        scroll={{ x: 900 }}
        pagination={{ pageSize: 20, showSizeChanger: true, showTotal: tot => t('snapshot.total', { total: tot }) }}
      />

      <Modal
        title={t('snapshot.createTitle')}
        open={createOpen}
        onCancel={() => { setCreateOpen(false); form.resetFields(); }}
        onOk={handleCreate}
        confirmLoading={creating}
        okText={t('snapshot.confirm')}
        cancelText={t('snapshot.cancel')}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="namespace" label={t('snapshot.namespace')} rules={[{ required: true }]}>
            <Input placeholder="default" />
          </Form.Item>
          <Form.Item name="name" label={t('snapshot.name')} rules={[{ required: true }]}>
            <Input placeholder="my-snapshot" />
          </Form.Item>
          <Form.Item name="pvcName" label={t('snapshot.sourcePVC')} rules={[{ required: true }]}>
            <Input placeholder="my-pvc" />
          </Form.Item>
          <Form.Item name="snapshotClassName" label={t('snapshot.snapshotClass')}>
            <Select
              allowClear
              placeholder={t('snapshot.snapshotClassPlaceholder')}
              options={snapshotClasses.map(sc => ({
                value: sc.name,
                label: `${sc.name}${sc.isDefault ? ' (default)' : ''}`,
              }))}
            />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
};

export default VolumeSnapshotTab;

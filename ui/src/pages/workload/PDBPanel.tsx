import React, { useState, useEffect, useCallback } from 'react';
import {
  App, Badge, Button, Card, Descriptions, Form, Input, Modal, Popconfirm, Space, Table, Tag, Tooltip,
} from 'antd';
import {
  DeleteOutlined, EditOutlined, InfoCircleOutlined, PlusOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { pdbService, type PDBInfo, type PDBRequest } from '../../services/pdbService';
import { usePermission } from '../../hooks/usePermission';

interface PDBPanelProps {
  clusterId: string;
  namespace: string;
  /** 若提供，只顯示相關 PDB；否則顯示命名空間所有 PDB */
  workloadLabels?: Record<string, string>;
}

const PDBPanel: React.FC<PDBPanelProps> = ({ clusterId, namespace, workloadLabels }) => {
  const { message } = App.useApp();
  const { canDelete } = usePermission();
  const [items, setItems] = useState<PDBInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<PDBInfo | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await pdbService.list(clusterId, namespace);
      setItems(res.data.items || []);
    } catch {
      message.error('載入 PDB 列表失敗');
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, message]);

  useEffect(() => { load(); }, [load]);

  const openCreate = () => {
    setEditTarget(null);
    form.resetFields();
    // 預填工作負載 labels
    if (workloadLabels) {
      form.setFieldsValue({
        selector: Object.entries(workloadLabels).map(([k, v]) => `${k}=${v}`).join(','),
      });
    }
    setModalOpen(true);
  };

  const openEdit = (pdb: PDBInfo) => {
    setEditTarget(pdb);
    form.setFieldsValue({
      name: pdb.name,
      namespace: pdb.namespace,
      selector: Object.entries(pdb.selector || {}).map(([k, v]) => `${k}=${v}`).join(','),
      minAvailable: pdb.minAvailable,
      maxUnavailable: pdb.maxUnavailable,
    });
    setModalOpen(true);
  };

  const parseSelectorString = (s: string): Record<string, string> => {
    const result: Record<string, string> = {};
    s.split(',').forEach(pair => {
      const [k, v] = pair.trim().split('=');
      if (k && v) result[k.trim()] = v.trim();
    });
    return result;
  };

  const handleSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const payload: PDBRequest = {
        name: values.name,
        namespace: values.namespace || namespace,
        selector: parseSelectorString(values.selector),
        minAvailable: values.minAvailable || undefined,
        maxUnavailable: values.maxUnavailable || undefined,
      };
      if (editTarget) {
        await pdbService.update(clusterId, editTarget.namespace, editTarget.name, payload);
        message.success('PDB 更新成功');
      } else {
        await pdbService.create(clusterId, payload);
        message.success('PDB 建立成功');
      }
      setModalOpen(false);
      load();
    } catch (e) {
      message.error('操作失敗：' + String(e));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (pdb: PDBInfo) => {
    try {
      await pdbService.delete(clusterId, pdb.namespace, pdb.name);
      message.success('PDB 刪除成功');
      load();
    } catch (e) {
      message.error('刪除失敗：' + String(e));
    }
  };

  const columns: ColumnsType<PDBInfo> = [
    { title: '名稱', dataIndex: 'name', ellipsis: true },
    {
      title: '保護規則',
      width: 160,
      render: (_, r) => (
        r.minAvailable
          ? <Tag color="green">minAvailable: {r.minAvailable}</Tag>
          : <Tag color="orange">maxUnavailable: {r.maxUnavailable}</Tag>
      ),
    },
    {
      title: '狀態',
      width: 160,
      render: (_, r) => (
        <Space>
          <Badge
            status={r.disruptionsAllowed > 0 ? 'success' : 'warning'}
            text={`可中斷 ${r.disruptionsAllowed} / ${r.expectedPods}`}
          />
        </Space>
      ),
    },
    { title: '健康 Pod', width: 100, render: (_, r) => `${r.currentHealthy} / ${r.desiredHealthy}` },
    {
      title: '操作',
      width: 110,
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>編輯</Button>
          {canDelete() && (
            <Popconfirm title="確定刪除此 PDB？" onConfirm={() => handleDelete(r)} okButtonProps={{ danger: true }}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>刪除</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <Card
      size="small"
      title="PodDisruptionBudget"
      extra={
        <Button size="small" type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          建立 PDB
        </Button>
      }
    >
      <Table
        scroll={{ x: 'max-content' }}
        rowKey="name"
        columns={columns}
        dataSource={items}
        loading={loading}
        pagination={false}
        size="small"
        locale={{ emptyText: '此命名空間無 PDB 設定' }}
        expandable={{
          expandedRowRender: (r) => (
            <Descriptions size="small" column={3} bordered>
              <Descriptions.Item label="Selector">
                {Object.entries(r.selector || {}).map(([k, v]) => (
                  <Tag key={k}>{k}={v}</Tag>
                ))}
              </Descriptions.Item>
              <Descriptions.Item label="建立時間">
                {new Date(r.createdAt).toLocaleString('zh-TW')}
              </Descriptions.Item>
            </Descriptions>
          ),
        }}
      />

      <Modal
        title={editTarget ? '編輯 PDB' : '建立 PDB'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        okText="儲存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 12 }}>
          <Form.Item name="name" label="PDB 名稱" rules={[{ required: true }]}>
            <Input disabled={!!editTarget} placeholder="例如 my-app-pdb" />
          </Form.Item>
          <Form.Item name="namespace" label="命名空間" initialValue={namespace}>
            <Input disabled />
          </Form.Item>
          <Form.Item
            name="selector"
            label={
              <Space>
                matchLabels
                <Tooltip title="格式：key1=value1,key2=value2">
                  <InfoCircleOutlined />
                </Tooltip>
              </Space>
            }
            rules={[{ required: true, message: '請輸入 selector labels' }]}
          >
            <Input placeholder="app=my-app,env=prod" />
          </Form.Item>
          <Form.Item name="minAvailable" label="minAvailable（優先）">
            <Input placeholder="數字（如 2）或百分比（如 50%）" />
          </Form.Item>
          <Form.Item name="maxUnavailable" label="maxUnavailable（與 minAvailable 二選一）">
            <Input placeholder="數字（如 1）或百分比（如 25%）" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
};

export default PDBPanel;

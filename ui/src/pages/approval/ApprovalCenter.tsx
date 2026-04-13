import React, { useState, useEffect, useCallback } from 'react';
import {
  App, Badge, Button, Card, Form, Input, Modal, Popconfirm,
  Select, Space, Table, Tooltip, Typography,
} from 'antd';
import {
  CheckCircleOutlined, CloseCircleOutlined, ClockCircleOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { approvalService, type ApprovalRequest } from '../../services/approvalService';

const { Text } = Typography;
const { Option } = Select;
const { TextArea } = Input;

const STATUS_CONFIG: Record<string, { color: string; label: string }> = {
  pending:  { color: 'processing', label: '待審批' },
  approved: { color: 'success',    label: '已核准' },
  rejected: { color: 'error',      label: '已拒絕' },
  expired:  { color: 'default',    label: '已逾期' },
};

const ACTION_LABELS: Record<string, string> = {
  scale: '縮放副本',
  delete: '刪除資源',
  update: '更新部署',
  apply: 'YAML Apply',
};

const ApprovalCenter: React.FC = () => {
  const { message } = App.useApp();
  const [items, setItems] = useState<ApprovalRequest[]>([]);
  const [loading, setLoading] = useState(false);
  const [statusFilter, setStatusFilter] = useState<string>('pending');
  const [rejectModalOpen, setRejectModalOpen] = useState(false);
  const [rejectTarget, setRejectTarget] = useState<number | null>(null);
  const [rejectForm] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await approvalService.listApprovals({ status: statusFilter || undefined });
      setItems(res.data.items || []);
    } catch {
      message.error('載入審批列表失敗');
    } finally {
      setLoading(false);
    }
  }, [statusFilter, message]);

  useEffect(() => { load(); }, [load]);

  const handleApprove = async (id: number) => {
    try {
      await approvalService.approve(id, '');
      message.success('已核准');
      load();
    } catch {
      message.error('操作失敗');
    }
  };

  const openReject = (id: number) => {
    setRejectTarget(id);
    rejectForm.resetFields();
    setRejectModalOpen(true);
  };

  const handleReject = async () => {
    try {
      const values = await rejectForm.validateFields();
      await approvalService.reject(rejectTarget!, values.reason);
      message.success('已拒絕');
      setRejectModalOpen(false);
      load();
    } catch {
      message.error('操作失敗');
    }
  };

  const columns: ColumnsType<ApprovalRequest> = [
    {
      title: '叢集',
      dataIndex: 'clusterName',
      width: 120,
    },
    {
      title: '命名空間',
      dataIndex: 'namespace',
      width: 140,
    },
    {
      title: '資源',
      width: 200,
      render: (_, r) => (
        <Space direction="vertical" size={0}>
          <Text>{r.resourceKind} / {r.resourceName}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {ACTION_LABELS[r.action] ?? r.action}
          </Text>
        </Space>
      ),
    },
    {
      title: '申請人',
      dataIndex: 'requesterName',
      width: 120,
    },
    {
      title: '狀態',
      dataIndex: 'status',
      width: 100,
      render: (status: string) => {
        const cfg = STATUS_CONFIG[status] ?? { color: 'default', label: status };
        return <Badge status={cfg.color as 'success' | 'processing' | 'error' | 'warning' | 'default'} text={cfg.label} />;
      },
    },
    {
      title: '逾期時間',
      dataIndex: 'expiresAt',
      width: 160,
      render: (v: string) => new Date(v).toLocaleString('zh-TW'),
    },
    {
      title: '申請時間',
      dataIndex: 'createdAt',
      width: 160,
      render: (v: string) => new Date(v).toLocaleString('zh-TW'),
    },
    {
      title: '審批理由',
      dataIndex: 'reason',
      ellipsis: true,
    },
    {
      title: '操作',
      width: 150,
      fixed: 'right',
      render: (_, r) => {
        if (r.status !== 'pending') return null;
        return (
          <Space>
            <Popconfirm title="確定核准此請求？" onConfirm={() => handleApprove(r.id)}>
              <Button type="link" size="small" icon={<CheckCircleOutlined />} style={{ color: '#52c41a' }}>
                核准
              </Button>
            </Popconfirm>
            <Button type="link" size="small" danger icon={<CloseCircleOutlined />} onClick={() => openReject(r.id)}>
              拒絕
            </Button>
          </Space>
        );
      },
    },
  ];

  return (
    <Card
      title={
        <Space>
          <ClockCircleOutlined />
          部署審批中心
        </Space>
      }
      extra={
        <Space>
          <Select value={statusFilter} onChange={setStatusFilter} style={{ width: 120 }}>
            <Option value="">全部</Option>
            <Option value="pending">待審批</Option>
            <Option value="approved">已核准</Option>
            <Option value="rejected">已拒絕</Option>
            <Option value="expired">已逾期</Option>
          </Select>
          <Tooltip title="重新整理">
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading} />
          </Tooltip>
        </Space>
      }
    >
      <Table
        rowKey="id"
        columns={columns}
        dataSource={items}
        loading={loading}
        scroll={{ x: 1200 }}
        pagination={{ pageSize: 20, showTotal: (t) => `共 ${t} 筆` }}
      />

      <Modal
        title="拒絕審批請求"
        open={rejectModalOpen}
        onOk={handleReject}
        onCancel={() => setRejectModalOpen(false)}
        okText="確定拒絕"
        okButtonProps={{ danger: true }}
      >
        <Form form={rejectForm} layout="vertical">
          <Form.Item
            name="reason"
            label="拒絕原因"
            rules={[{ required: true, message: '請填寫拒絕原因' }]}
          >
            <TextArea rows={4} placeholder="請說明拒絕原因..." />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
};

export default ApprovalCenter;

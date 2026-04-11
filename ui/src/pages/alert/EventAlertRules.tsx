import React, { useState, useCallback, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import EmptyState from '../../components/EmptyState';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Switch,
  Modal,
  Form,
  Input,
  Select,
  InputNumber,
  Tabs,
  App,
  Typography,
  Tooltip,
  Badge,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  ReloadOutlined,
  HistoryOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { TablePaginationConfig } from 'antd/es/table';
import { EventAlertService, type EventAlertRule, type EventAlertHistory } from '../../services/eventAlertService';

const { Text } = Typography;
const { Option } = Select;

// Common K8s event reasons for quick selection
const COMMON_REASONS = [
  'OOMKilling', 'BackOff', 'CrashLoopBackOff', 'Failed',
  'FailedScheduling', 'Unhealthy', 'Evicted', 'NodeNotReady',
  'ImagePullBackOff', 'ErrImagePull', 'FailedMount', 'FailedAttachVolume',
];

const EventAlertRules: React.FC = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const { t } = useTranslation(['alert', 'common']);
  const { message, modal } = App.useApp();

  const [activeTab, setActiveTab] = useState('rules');

  // Rules state
  const [rules, setRules] = useState<EventAlertRule[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);
  const [rulesTotal, setRulesTotal] = useState(0);
  const [rulesPage, setRulesPage] = useState(1);

  // History state
  const [history, setHistory] = useState<EventAlertHistory[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyPage, setHistoryPage] = useState(1);

  // Rule form
  const [formOpen, setFormOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<EventAlertRule | null>(null);
  const [form] = Form.useForm();
  const [saving, setSaving] = useState(false);

  const fetchRules = useCallback(async (page = rulesPage) => {
    if (!clusterId) return;
    setRulesLoading(true);
    try {
      const res = await EventAlertService.listRules(clusterId, page);
      setRules(res.items ?? []);
      setRulesTotal(res.total ?? 0);
    } finally {
      setRulesLoading(false);
    }
  }, [clusterId, rulesPage]);

  const fetchHistory = useCallback(async (page = historyPage) => {
    if (!clusterId) return;
    setHistoryLoading(true);
    try {
      const res = await EventAlertService.listHistory(clusterId, page);
      setHistory(res.items ?? []);
      setHistoryTotal(res.total ?? 0);
    } finally {
      setHistoryLoading(false);
    }
  }, [clusterId, historyPage]);

  useEffect(() => { fetchRules(1); setRulesPage(1); }, [clusterId]);
  useEffect(() => { if (activeTab === 'history') { fetchHistory(1); setHistoryPage(1); } }, [activeTab, clusterId]);

  const handleOpenCreate = () => {
    setEditingRule(null);
    form.resetFields();
    form.setFieldsValue({ minCount: 1, enabled: true, notifyType: 'webhook' });
    setFormOpen(true);
  };

  const handleOpenEdit = (rule: EventAlertRule) => {
    setEditingRule(rule);
    form.setFieldsValue(rule);
    setFormOpen(true);
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      if (editingRule) {
        await EventAlertService.updateRule(clusterId!, editingRule.id, { ...values, clusterId: Number(clusterId) });
        message.success(t('messages.saveSuccess'));
      } else {
        await EventAlertService.createRule(clusterId!, { ...values, clusterId: Number(clusterId) });
        message.success(t('messages.createSuccess'));
      }
      setFormOpen(false);
      fetchRules(1);
      setRulesPage(1);
    } catch {
      // validation error handled by form
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = (rule: EventAlertRule) => {
    modal.confirm({
      title: t('messages.confirmDelete'),
      content: rule.name,
      okType: 'danger',
      onOk: async () => {
        await EventAlertService.deleteRule(clusterId!, rule.id);
        message.success(t('messages.deleteSuccess'));
        fetchRules(1);
        setRulesPage(1);
      },
    });
  };

  const handleToggle = async (rule: EventAlertRule, enabled: boolean) => {
    try {
      await EventAlertService.toggleRule(clusterId!, rule.id, enabled);
      setRules(prev => prev.map(r => r.id === rule.id ? { ...r, enabled } : r));
    } catch {
      message.error(t('messages.error'));
    }
  };

  const ruleColumns = [
    { title: t('table.name'), dataIndex: 'name', key: 'name', render: (v: string) => <Text strong>{v}</Text> },
    {
      title: '事件型別', dataIndex: 'eventType', key: 'eventType',
      render: (v: string) => v ? <Tag color={v === 'Warning' ? 'orange' : 'green'}>{v}</Tag> : <Tag>全部</Tag>,
    },
    {
      title: '觸發原因', dataIndex: 'eventReason', key: 'eventReason',
      render: (v: string) => v ? v.split(',').map(r => <Tag key={r}>{r.trim()}</Tag>) : <Tag>全部</Tag>,
    },
    { title: '命名空間', dataIndex: 'namespace', key: 'namespace', render: (v: string) => v || '全叢集' },
    { title: '最小次數', dataIndex: 'minCount', key: 'minCount' },
    { title: '通知方式', dataIndex: 'notifyType', key: 'notifyType', render: (v: string) => <Tag>{v}</Tag> },
    {
      title: '狀態', dataIndex: 'enabled', key: 'enabled',
      render: (enabled: boolean, record: EventAlertRule) => (
        <Switch checked={enabled} onChange={(v) => handleToggle(record, v)} size="small" />
      ),
    },
    {
      title: t('table.actions'), key: 'actions', fixed: 'right' as const, width: 100,
      render: (_: unknown, record: EventAlertRule) => (
        <Space>
          <Tooltip title={t('actions.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpenEdit(record)} />
          </Tooltip>
          <Tooltip title={t('actions.delete')}>
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record)} />
          </Tooltip>
        </Space>
      ),
    },
  ];

  const historyColumns = [
    { title: '規則名稱', dataIndex: 'ruleName', key: 'ruleName' },
    { title: t('table.namespace'), dataIndex: 'namespace', key: 'namespace', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: '相關物件', dataIndex: 'involvedObj', key: 'involvedObj' },
    {
      title: '事件型別', dataIndex: 'eventType', key: 'eventType',
      render: (v: string) => <Tag color={v === 'Warning' ? 'orange' : 'green'}>{v}</Tag>,
    },
    { title: '原因', dataIndex: 'eventReason', key: 'eventReason' },
    { title: '訊息', dataIndex: 'message', key: 'message', ellipsis: true },
    {
      title: '通知結果', dataIndex: 'notifyResult', key: 'notifyResult',
      render: (v: string) => {
        const color = v === 'sent' ? 'green' : v === 'failed' ? 'red' : 'default';
        return <Badge color={color} text={v} />;
      },
    },
    { title: '觸發時間', dataIndex: 'triggeredAt', key: 'triggeredAt', render: (v: string) => new Date(v).toLocaleString() },
  ];

  const tabItems = [
    {
      key: 'rules',
      label: '告警規則',
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreate}>新增規則</Button>
            <Button icon={<ReloadOutlined />} onClick={() => fetchRules(1)}>重新整理</Button>
          </Space>
          <Table
            rowKey="id"
            columns={ruleColumns}
            dataSource={rules}
            loading={rulesLoading}
            scroll={{ x: 900 }}
            pagination={{
              current: rulesPage,
              total: rulesTotal,
              pageSize: 20,
              onChange: (p) => { setRulesPage(p); fetchRules(p); },
            }}
            locale={{ emptyText: <EmptyState description={t('noRules')} /> }}
          />
        </div>
      ),
    },
    {
      key: 'history',
      label: <><HistoryOutlined /> 告警歷史</>,
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={() => fetchHistory(1)}>重新整理</Button>
          </Space>
          <Table
            rowKey="id"
            columns={historyColumns}
            dataSource={history}
            loading={historyLoading}
            scroll={{ x: 1000 }}
            pagination={{
              current: historyPage,
              total: historyTotal,
              pageSize: 20,
              onChange: (p) => { setHistoryPage(p); fetchHistory(p); },
            }}
            locale={{ emptyText: <EmptyState description={t('noHistory')} /> }}
          />
        </div>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card bordered={false} title="Event 告警規則引擎">
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />
      </Card>

      <Modal
        title={editingRule ? '編輯規則' : '新增告警規則'}
        open={formOpen}
        onOk={handleSave}
        onCancel={() => setFormOpen(false)}
        confirmLoading={saving}
        width={600}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="規則名稱" rules={[{ required: true }]}>
            <Input placeholder="例如：OOMKill 告警" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="namespace" label="命名空間">
            <Input placeholder="留空表示監控全叢集" />
          </Form.Item>
          <Form.Item name="eventType" label="事件型別">
            <Select allowClear placeholder="全部">
              <Option value="Warning">Warning</Option>
              <Option value="Normal">Normal</Option>
            </Select>
          </Form.Item>
          <Form.Item name="eventReason" label="觸發原因" tooltip="多個原因用逗號分隔，留空匹配全部">
            <Select
              mode="tags"
              placeholder="選擇或輸入原因，例如 OOMKilling"
              tokenSeparators={[',']}
              options={COMMON_REASONS.map(r => ({ label: r, value: r }))}
            />
          </Form.Item>
          <Form.Item name="minCount" label="最小觸發次數" rules={[{ required: true }]}>
            <InputNumber min={1} style={{ width: 120 }} />
          </Form.Item>
          <Form.Item name="notifyType" label="通知方式" rules={[{ required: true }]}>
            <Select>
              <Option value="webhook">Webhook</Option>
              <Option value="telegram">Telegram</Option>
              <Option value="slack">Slack</Option>
              <Option value="teams">Microsoft Teams</Option>
            </Select>
          </Form.Item>
          <Form.Item name="notifyUrl" label="通知 URL" tooltip="Webhook / Telegram Bot API / Slack / Teams Incoming Webhook URL">
            <Input placeholder="https://..." />
          </Form.Item>
          <Form.Item name="enabled" label="啟用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default EventAlertRules;

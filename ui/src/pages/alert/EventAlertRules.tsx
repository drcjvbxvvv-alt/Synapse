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
  Popover,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  ReloadOutlined,
  HistoryOutlined,
  CopyOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { eventAlertService, type EventAlertRule, type EventAlertHistory } from '../../services/eventAlertService';
import { usePermission } from '../../hooks/usePermission';

const { Text } = Typography;
const { Option } = Select;

// 長文本懸停卡片
const TextPopover: React.FC<{ content: string; t: ReturnType<typeof useTranslation>[0] }> = ({ content, t }) => (
  <Popover
    content={
      <div style={{ maxWidth: 500, wordBreak: 'break-all', maxHeight: 300, overflow: 'auto' }}>
        <div style={{ marginBottom: 8 }}>{content}</div>
        <Button
          type="primary"
          size="small"
          icon={<CopyOutlined />}
          onClick={() => navigator.clipboard.writeText(content)}
        >
          {t('common:actions.copy')}
        </Button>
      </div>
    }
    title={t('alert:center.preview') || 'Preview'}
  >
    <Text ellipsis style={{ cursor: 'pointer', color: '#1890ff' }}>
      {content}
    </Text>
  </Popover>
);

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
  const { canWrite } = usePermission();

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
      const res = await eventAlertService.listRules(clusterId, page);
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
      const res = await eventAlertService.listHistory(clusterId, page);
      setHistory(res.items ?? []);
      setHistoryTotal(res.total ?? 0);
    } finally {
      setHistoryLoading(false);
    }
  }, [clusterId, historyPage]);

  useEffect(() => { fetchRules(1); setRulesPage(1); }, [clusterId, fetchRules]);
  useEffect(() => { if (activeTab === 'history') { fetchHistory(1); setHistoryPage(1); } }, [activeTab, clusterId, fetchHistory]);

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
        await eventAlertService.updateRule(clusterId!, editingRule.id, { ...values, clusterId: Number(clusterId) });
        message.success(t('alert:eventAlert.messages.saveSuccess'));
      } else {
        await eventAlertService.createRule(clusterId!, { ...values, clusterId: Number(clusterId) });
        message.success(t('alert:eventAlert.messages.createSuccess'));
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
      title: t('alert:eventAlert.messages.confirmDelete'),
      content: rule.name,
      okType: 'danger',
      onOk: async () => {
        await eventAlertService.deleteRule(clusterId!, rule.id);
        message.success(t('alert:eventAlert.messages.deleteSuccess'));
        fetchRules(1);
        setRulesPage(1);
      },
    });
  };

  const handleToggle = async (rule: EventAlertRule, enabled: boolean) => {
    try {
      await eventAlertService.toggleRule(clusterId!, rule.id, enabled);
      setRules(prev => prev.map(r => r.id === rule.id ? { ...r, enabled } : r));
    } catch {
      message.error(t('alert:eventAlert.messages.error'));
    }
  };

  const ruleColumns = [
    {
      title: t('alert:eventAlert.table.name'),
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (v: string) => <TextPopover content={v} t={t} />
    },
    {
      title: t('alert:eventAlert.table.eventType'), dataIndex: 'eventType', key: 'eventType',
      render: (v: string) => v ? <Tag color={v === 'Warning' ? 'orange' : 'green'}>{v}</Tag> : <Tag>{t('alert:eventAlert.tags.all')}</Tag>,
    },
    {
      title: t('alert:eventAlert.table.eventReason'), dataIndex: 'eventReason', key: 'eventReason',
      render: (v: string) => v ? v.split(',').map(r => <Tag key={r}>{r.trim()}</Tag>) : <Tag>{t('alert:eventAlert.tags.all')}</Tag>,
    },
    { title: t('alert:eventAlert.table.namespace'), dataIndex: 'namespace', key: 'namespace', render: (v: string) => v || t('alert:eventAlert.form.allNamespaces') },
    { title: t('alert:eventAlert.table.minCount'), dataIndex: 'minCount', key: 'minCount' },
    { title: t('alert:eventAlert.table.notifyType'), dataIndex: 'notifyType', key: 'notifyType', render: (v: string) => <Tag>{v}</Tag> },
    {
      title: t('alert:eventAlert.table.status'), dataIndex: 'enabled', key: 'enabled',
      render: (enabled: boolean, record: EventAlertRule) => (
        <Switch checked={enabled} onChange={(v) => handleToggle(record, v)} size="small" />
      ),
    },
    ...(canWrite() ? [{
      title: t('alert:eventAlert.table.actions'), key: 'actions', fixed: 'right' as const, width: 100,
      render: (_: unknown, record: EventAlertRule) => (
        <Space>
          <Tooltip title={t('common:actions.edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleOpenEdit(record)} />
          </Tooltip>
          <Tooltip title={t('common:actions.delete')}>
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record)} />
          </Tooltip>
        </Space>
      ),
    }] : []),
  ];

  const historyColumns = [
    {
      title: t('alert:eventAlert.history.ruleName'),
      dataIndex: 'ruleName',
      key: 'ruleName',
      width: 150,
      render: (v: string) => <TextPopover content={v} t={t} />
    },
    { title: t('alert:eventAlert.history.namespace'), dataIndex: 'namespace', key: 'namespace', render: (v: string) => <Tag color="blue">{v}</Tag> },
    {
      title: t('alert:eventAlert.history.involvedObject'),
      dataIndex: 'involvedObj',
      key: 'involvedObj',
      width: 200,
      render: (v: string) => <TextPopover content={v} t={t} />
    },
    {
      title: t('alert:eventAlert.history.eventType'), dataIndex: 'eventType', key: 'eventType',
      render: (v: string) => <Tag color={v === 'Warning' ? 'orange' : 'green'}>{v}</Tag>,
    },
    {
      title: t('alert:eventAlert.history.reason'),
      dataIndex: 'eventReason',
      key: 'eventReason',
      width: 120,
      render: (v: string) => <TextPopover content={v} t={t} />
    },
    {
      title: t('alert:eventAlert.history.message'),
      dataIndex: 'message',
      key: 'message',
      width: 200,
      render: (v: string) => <TextPopover content={v} t={t} />
    },
    {
      title: t('alert:eventAlert.history.notifyResult'), dataIndex: 'notifyResult', key: 'notifyResult',
      render: (v: string) => {
        const color = v === 'sent' ? 'green' : v === 'failed' ? 'red' : 'default';
        return <Badge color={color} text={v} />;
      },
    },
    { title: t('alert:eventAlert.history.triggeredAt'), dataIndex: 'triggeredAt', key: 'triggeredAt', render: (v: string) => new Date(v).toLocaleString() },
  ];

  const tabItems = [
    {
      key: 'rules',
      label: t('alert:eventAlert.tabRules'),
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreate}>{t('alert:eventAlert.createRule')}</Button>
            <Button icon={<ReloadOutlined />} onClick={() => fetchRules(1)}>{t('alert:eventAlert.refresh')}</Button>
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
            locale={{ emptyText: <EmptyState description={t('alert:noRules')} /> }}
          />
        </div>
      ),
    },
    {
      key: 'history',
      label: <><HistoryOutlined /> {t('alert:eventAlert.tabHistory')}</>,
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <Button icon={<ReloadOutlined />} onClick={() => fetchHistory(1)}>{t('alert:eventAlert.refresh')}</Button>
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
            locale={{ emptyText: <EmptyState description={t('alert:noHistory')} /> }}
          />
        </div>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card variant="borderless" title={t('alert:eventAlert.title')}>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />
      </Card>

      <Modal
        title={editingRule ? t('alert:eventAlert.editRule') : t('alert:eventAlert.addAlertRule')}
        open={formOpen}
        onOk={handleSave}
        onCancel={() => setFormOpen(false)}
        confirmLoading={saving}
        width={600}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('alert:eventAlert.form.ruleName')} rules={[{ required: true }]}>
            <Input placeholder={t('alert:eventAlert.form.rulePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('alert:eventAlert.form.description')}>
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="namespace" label={t('alert:eventAlert.form.namespace')}>
            <Input placeholder={t('alert:eventAlert.form.namespacePlaceholder')} />
          </Form.Item>
          <Form.Item name="eventType" label={t('alert:eventAlert.form.eventType')}>
            <Select allowClear placeholder={t('alert:eventAlert.form.eventTypeAll')}>
              <Option value="Warning">Warning</Option>
              <Option value="Normal">Normal</Option>
            </Select>
          </Form.Item>
          <Form.Item name="eventReason" label={t('alert:eventAlert.form.eventReason')} tooltip={t('alert:eventAlert.form.eventReasonTooltip')}>
            <Select
              mode="tags"
              placeholder={t('alert:eventAlert.form.eventReasonPlaceholder')}
              tokenSeparators={[',']}
              options={COMMON_REASONS.map(r => ({ label: r, value: r }))}
            />
          </Form.Item>
          <Form.Item name="minCount" label={t('alert:eventAlert.form.minTriggerCount')} rules={[{ required: true }]}>
            <InputNumber min={1} style={{ width: 120 }} />
          </Form.Item>
          <Form.Item name="notifyType" label={t('alert:eventAlert.form.notifyType')} rules={[{ required: true }]}>
            <Select>
              <Option value="webhook">{t('alert:eventAlert.tags.webhook')}</Option>
              <Option value="telegram">{t('alert:eventAlert.tags.telegram')}</Option>
              <Option value="slack">{t('alert:eventAlert.tags.slack')}</Option>
              <Option value="teams">{t('alert:eventAlert.tags.teams')}</Option>
            </Select>
          </Form.Item>
          <Form.Item name="notifyUrl" label={t('alert:eventAlert.form.notifyUrl')} tooltip={t('alert:eventAlert.form.notifyUrlTooltip')}>
            <Input placeholder="https://..." />
          </Form.Item>
          <Form.Item name="enabled" label={t('alert:eventAlert.form.enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default EventAlertRules;

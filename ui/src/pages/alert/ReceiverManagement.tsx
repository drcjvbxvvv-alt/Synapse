import React, { useState, useEffect, useCallback } from 'react';
import {
  Table,
  Button,
  Space,
  Modal,
  Form,
  Input,
  Select,
  Popconfirm,
  Tag,
  Tooltip,
  Tabs,
  Switch,
  InputNumber,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  SendOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { App } from 'antd';
import { usePermission } from '../../hooks/usePermission';
import {
  alertService,
  type ReceiverConfig,
  type EmailConfig,
  type SlackConfig,
  type WebhookConfig,
  type PagerdutyConfig,
  type TelegramConfig,
} from '../../services/alertService';

interface ReceiverManagementProps {
  clusterId: string;
}

type ReceiverType = 'email' | 'slack' | 'webhook' | 'pagerduty' | 'telegram';

const RECEIVER_TYPE_LABELS: Record<ReceiverType, string> = {
  email: 'Email',
  slack: 'Slack',
  webhook: 'Webhook',
  pagerduty: 'PagerDuty',
  telegram: 'Telegram',
};

const RECEIVER_TYPE_COLORS: Record<ReceiverType, string> = {
  email: 'blue',
  slack: 'purple',
  webhook: 'green',
  pagerduty: 'red',
  telegram: 'cyan',
};

function getReceiverTypes(r: ReceiverConfig): ReceiverType[] {
  const types: ReceiverType[] = [];
  if (r.emailConfigs?.length) types.push('email');
  if (r.slackConfigs?.length) types.push('slack');
  if (r.webhookConfigs?.length) types.push('webhook');
  if (r.pagerdutyConfigs?.length) types.push('pagerduty');
  if (r.telegramConfigs?.length) types.push('telegram');
  return types;
}

const ReceiverManagement: React.FC<ReceiverManagementProps> = ({ clusterId }) => {
  const { message: msg, modal: _modal } = App.useApp();
  const { canWrite, canDelete } = usePermission();
  const [loading, setLoading] = useState(false);
  const [receivers, setReceivers] = useState<ReceiverConfig[]>([]);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingReceiver, setEditingReceiver] = useState<ReceiverConfig | null>(null);
  const [testingName, setTestingName] = useState<string | null>(null);
  const [form] = Form.useForm();
  const [channelType, setChannelType] = useState<ReceiverType>('email');

  const loadReceivers = useCallback(async () => {
    try {
      setLoading(true);
      const data = await alertService.getFullReceivers(clusterId);
      setReceivers(data ?? []);
    } catch (err) {
      console.error('載入 Receiver 失敗', err);
      msg.error('載入 Receiver 列表失敗');
    } finally {
      setLoading(false);
    }
  }, [clusterId, msg]);

  useEffect(() => {
    loadReceivers();
  }, [loadReceivers]);

  const openCreate = () => {
    setEditingReceiver(null);
    form.resetFields();
    setChannelType('email');
    setModalOpen(true);
  };

  const openEdit = (r: ReceiverConfig) => {
    setEditingReceiver(r);
    const types = getReceiverTypes(r);
    const type: ReceiverType = types[0] ?? 'webhook';
    setChannelType(type);

    const baseValues: Record<string, unknown> = { name: r.name, channelType: type };
    if (type === 'email' && r.emailConfigs?.[0]) {
      const e = r.emailConfigs[0];
      Object.assign(baseValues, {
        emailTo: e.to,
        emailFrom: e.from,
        emailSmarthost: e.smarthost,
        emailAuthUsername: e.authUsername,
        emailAuthPassword: e.authPassword,
        emailRequireTls: e.requireTls ?? true,
      });
    } else if (type === 'slack' && r.slackConfigs?.[0]) {
      const s = r.slackConfigs[0];
      Object.assign(baseValues, {
        slackApiUrl: s.apiUrl,
        slackChannel: s.channel,
        slackUsername: s.username,
        slackText: s.text,
        slackTitle: s.title,
      });
    } else if (type === 'webhook' && r.webhookConfigs?.[0]) {
      const w = r.webhookConfigs[0];
      Object.assign(baseValues, {
        webhookUrl: w.url,
        webhookSendResolved: w.sendResolved ?? true,
        webhookMaxAlerts: w.maxAlerts,
      });
    } else if (type === 'pagerduty' && r.pagerdutyConfigs?.[0]) {
      const p = r.pagerdutyConfigs[0];
      Object.assign(baseValues, {
        pagerdutyRoutingKey: p.routingKey,
        pagerdutyServiceKey: p.serviceKey,
        pagerdutyDescription: p.description,
      });
    } else if (type === 'telegram' && r.telegramConfigs?.[0]) {
      const tg = r.telegramConfigs[0];
      Object.assign(baseValues, {
        telegramBotToken: tg.botToken,
        telegramChatId: tg.chatId,
        telegramParseMode: tg.parseMode ?? 'HTML',
        telegramDisableNotification: tg.disableNotification ?? false,
      });
    }
    form.setFieldsValue(baseValues);
    setModalOpen(true);
  };

  const handleDelete = async (name: string) => {
    try {
      await alertService.deleteReceiver(clusterId, name);
      msg.success(`已刪除 Receiver: ${name}`);
      loadReceivers();
    } catch (_err) {
      msg.error('刪除失敗');
    }
  };

  const handleTest = async (name: string) => {
    try {
      setTestingName(name);
      await alertService.testReceiver(clusterId, name);
      msg.success(`測試告警已傳送至 ${name}`);
    } catch (_err) {
      msg.error('測試失敗，請確認 Receiver 設定');
    } finally {
      setTestingName(null);
    }
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      const receiver = buildReceiverConfig(values);

      if (editingReceiver) {
        await alertService.updateReceiver(clusterId, editingReceiver.name, receiver);
        msg.success('Receiver 已更新');
      } else {
        await alertService.createReceiver(clusterId, receiver);
        msg.success('Receiver 已新增');
      }
      setModalOpen(false);
      loadReceivers();
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'errorFields' in err) return;
      msg.error('儲存失敗');
    }
  };

  function buildReceiverConfig(values: Record<string, unknown>): ReceiverConfig {
    const name = values.name as string;
    const type = values.channelType as ReceiverType;
    const receiver: ReceiverConfig = { name };

    if (type === 'email') {
      const emailCfg: EmailConfig = {
        to: values.emailTo as string,
        from: values.emailFrom as string | undefined,
        smarthost: values.emailSmarthost as string | undefined,
        authUsername: values.emailAuthUsername as string | undefined,
        authPassword: values.emailAuthPassword as string | undefined,
        requireTls: values.emailRequireTls as boolean | undefined,
      };
      receiver.emailConfigs = [emailCfg];
    } else if (type === 'slack') {
      const slackCfg: SlackConfig = {
        apiUrl: values.slackApiUrl as string,
        channel: values.slackChannel as string,
        username: values.slackUsername as string | undefined,
        text: values.slackText as string | undefined,
        title: values.slackTitle as string | undefined,
      };
      receiver.slackConfigs = [slackCfg];
    } else if (type === 'webhook') {
      const webhookCfg: WebhookConfig = {
        url: values.webhookUrl as string,
        sendResolved: values.webhookSendResolved as boolean | undefined,
        maxAlerts: values.webhookMaxAlerts as number | undefined,
      };
      receiver.webhookConfigs = [webhookCfg];
    } else if (type === 'pagerduty') {
      const pdCfg: PagerdutyConfig = {
        routingKey: values.pagerdutyRoutingKey as string,
        serviceKey: values.pagerdutyServiceKey as string | undefined,
        description: values.pagerdutyDescription as string | undefined,
      };
      receiver.pagerdutyConfigs = [pdCfg];
    } else if (type === 'telegram') {
      const tgCfg: TelegramConfig = {
        botToken: values.telegramBotToken as string,
        chatId: values.telegramChatId as string,
        parseMode: (values.telegramParseMode as string | undefined) ?? 'HTML',
        disableNotification: values.telegramDisableNotification as boolean | undefined,
      };
      receiver.telegramConfigs = [tgCfg];
    }
    return receiver;
  }

  const columns: ColumnsType<ReceiverConfig> = [
    {
      title: '名稱',
      dataIndex: 'name',
      key: 'name',
      width: 200,
    },
    {
      title: '告警渠道',
      key: 'types',
      render: (_, r) => (
        <Space wrap>
          {getReceiverTypes(r).map((t) => (
            <Tag key={t} color={RECEIVER_TYPE_COLORS[t]}>
              {RECEIVER_TYPE_LABELS[t]}
            </Tag>
          ))}
          {getReceiverTypes(r).length === 0 && <Tag>未設定</Tag>}
        </Space>
      ),
    },
    ...((canWrite() || canDelete()) ? [{
      title: '操作',
      key: 'actions',
      width: 180,
      render: (_: unknown, r: ReceiverConfig) => (
        <Space>
          {canWrite() && (
            <Tooltip title="測試推送">
              <Button
                size="small"
                icon={<SendOutlined />}
                loading={testingName === r.name}
                onClick={() => handleTest(r.name)}
              />
            </Tooltip>
          )}
          {canWrite() && (
            <Tooltip title="編輯">
              <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)} />
            </Tooltip>
          )}
          {canDelete() && (
            <Popconfirm
              title={`確定刪除 Receiver "${r.name}"？`}
              onConfirm={() => handleDelete(r.name)}
              okText="刪除"
              cancelText="取消"
              okButtonProps={{ danger: true }}
            >
              <Tooltip title="刪除">
                <Button size="small" danger icon={<DeleteOutlined />} />
              </Tooltip>
            </Popconfirm>
          )}
        </Space>
      ),
    }] : []),
  ];

  return (
    <>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新增 Receiver
        </Button>
        <Button icon={<ReloadOutlined />} onClick={loadReceivers} loading={loading}>
          重新整理
        </Button>
      </Space>

      <Table
        rowKey="name"
        columns={columns}
        dataSource={receivers}
        loading={loading}
        size="small"
        pagination={{ pageSize: 20, showSizeChanger: false }}
      />

      <Modal
        title={editingReceiver ? `編輯 Receiver: ${editingReceiver.name}` : '新增 Receiver'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        width={640}
        okText="儲存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={form} layout="vertical" size="small">
          <Form.Item
            name="name"
            label="Receiver 名稱"
            rules={[{ required: true, message: '請輸入名稱' }]}
          >
            <Input disabled={!!editingReceiver} placeholder="例：slack-alerts" />
          </Form.Item>

          <Form.Item
            name="channelType"
            label="告警渠道類型"
            initialValue="email"
            rules={[{ required: true }]}
          >
            <Select onChange={(v) => setChannelType(v as ReceiverType)}>
              {(Object.keys(RECEIVER_TYPE_LABELS) as ReceiverType[]).map((k) => (
                <Select.Option key={k} value={k}>
                  {RECEIVER_TYPE_LABELS[k]}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Tabs
            activeKey={channelType}
            items={[
              {
                key: 'email',
                label: 'Email',
                children: (
                  <>
                    <Form.Item name="emailTo" label="收件人" rules={channelType === 'email' ? [{ required: true, message: '請輸入收件人' }] : []}>
                      <Input placeholder="alert@example.com" />
                    </Form.Item>
                    <Form.Item name="emailFrom" label="發件人"><Input placeholder="alertmanager@example.com" /></Form.Item>
                    <Form.Item name="emailSmarthost" label="SMTP 伺服器"><Input placeholder="smtp.example.com:587" /></Form.Item>
                    <Form.Item name="emailAuthUsername" label="SMTP 帳號"><Input /></Form.Item>
                    <Form.Item name="emailAuthPassword" label="SMTP 密碼"><Input.Password /></Form.Item>
                    <Form.Item name="emailRequireTls" label="強制 TLS" valuePropName="checked" initialValue={true}><Switch /></Form.Item>
                  </>
                ),
              },
              {
                key: 'slack',
                label: 'Slack',
                children: (
                  <>
                    <Form.Item name="slackApiUrl" label="Webhook URL" rules={channelType === 'slack' ? [{ required: true, message: '請輸入 Webhook URL' }] : []}>
                      <Input placeholder="https://hooks.slack.com/services/..." />
                    </Form.Item>
                    <Form.Item name="slackChannel" label="頻道" rules={channelType === 'slack' ? [{ required: true, message: '請輸入頻道名稱' }] : []}>
                      <Input placeholder="#alerts" />
                    </Form.Item>
                    <Form.Item name="slackUsername" label="Bot 名稱"><Input placeholder="Alertmanager" /></Form.Item>
                    <Form.Item name="slackTitle" label="訊息標題"><Input /></Form.Item>
                    <Form.Item name="slackText" label="訊息內容"><Input.TextArea rows={3} /></Form.Item>
                  </>
                ),
              },
              {
                key: 'webhook',
                label: 'Webhook',
                children: (
                  <>
                    <Form.Item name="webhookUrl" label="Webhook URL" rules={channelType === 'webhook' ? [{ required: true, message: '請輸入 URL' }] : []}>
                      <Input placeholder="https://your-service.com/alerts" />
                    </Form.Item>
                    <Form.Item name="webhookSendResolved" label="傳送恢復通知" valuePropName="checked" initialValue={true}><Switch /></Form.Item>
                    <Form.Item name="webhookMaxAlerts" label="最大告警數（0=不限）">
                      <InputNumber min={0} style={{ width: '100%' }} />
                    </Form.Item>
                  </>
                ),
              },
              {
                key: 'pagerduty',
                label: 'PagerDuty',
                children: (
                  <>
                    <Form.Item name="pagerdutyRoutingKey" label="Routing Key" rules={channelType === 'pagerduty' ? [{ required: true, message: '請輸入 Routing Key' }] : []}>
                      <Input.Password />
                    </Form.Item>
                    <Form.Item name="pagerdutyServiceKey" label="Service Key"><Input.Password /></Form.Item>
                    <Form.Item name="pagerdutyDescription" label="描述"><Input /></Form.Item>
                  </>
                ),
              },
              {
                key: 'telegram',
                label: 'Telegram',
                children: (
                  <>
                    <Form.Item name="telegramBotToken" label="Bot Token" rules={channelType === 'telegram' ? [{ required: true, message: '請輸入 Bot Token' }] : []}>
                      <Input.Password placeholder="1234567890:AAxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" />
                    </Form.Item>
                    <Form.Item name="telegramChatId" label="Chat ID" rules={channelType === 'telegram' ? [{ required: true, message: '請輸入 Chat ID' }] : []} extra="群組 Chat ID 以負號開頭，例如 -1001234567890">
                      <Input placeholder="-1001234567890" />
                    </Form.Item>
                    <Form.Item name="telegramParseMode" label="訊息格式" initialValue="HTML">
                      <Select>
                        <Select.Option value="HTML">HTML</Select.Option>
                        <Select.Option value="Markdown">Markdown</Select.Option>
                        <Select.Option value="">純文字</Select.Option>
                      </Select>
                    </Form.Item>
                    <Form.Item name="telegramDisableNotification" label="靜音傳送" valuePropName="checked" initialValue={false}>
                      <Switch />
                    </Form.Item>
                  </>
                ),
              },
            ]}
          />
        </Form>
      </Modal>
    </>
  );
};

export default ReceiverManagement;

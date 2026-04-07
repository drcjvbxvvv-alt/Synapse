import React, { useEffect, useState } from 'react';
import {
  Button,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SendOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { ColumnsType } from 'antd/es/table';
import notifyChannelService from '../../services/notifyChannelService';
import type { NotifyChannel } from '../../services/notifyChannelService';

const { Title, Paragraph } = Typography;

const CHANNEL_TYPES = ['webhook', 'telegram', 'slack', 'teams'];

const NotificationSettings: React.FC = () => {
  const { t } = useTranslation(['settings']);
  const [channels, setChannels] = useState<NotifyChannel[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingChannel, setEditingChannel] = useState<NotifyChannel | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<number | null>(null);
  const [form] = Form.useForm();

  const fetchChannels = async () => {
    setLoading(true);
    try {
      const data = await notifyChannelService.list();
      setChannels(data);
    } catch {
      message.error(t('settings:notification.loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchChannels();
  }, []);

  const openCreate = () => {
    setEditingChannel(null);
    form.resetFields();
    form.setFieldsValue({ enabled: true, type: 'webhook' });
    setModalOpen(true);
  };

  const openEdit = (ch: NotifyChannel) => {
    setEditingChannel(ch);
    form.setFieldsValue({
      name: ch.name,
      type: ch.type,
      webhookUrl: ch.webhookUrl,
      telegramChatId: ch.telegramChatId,
      description: ch.description,
      enabled: ch.enabled,
    });
    setModalOpen(true);
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      if (editingChannel) {
        await notifyChannelService.update(editingChannel.id, values);
        message.success(t('settings:notification.updateSuccess'));
      } else {
        await notifyChannelService.create(values);
        message.success(t('settings:notification.createSuccess'));
      }
      setModalOpen(false);
      fetchChannels();
    } catch (err: unknown) {
      if (err && typeof err === 'object' && 'errorFields' in err) return; // form validation
      message.error(t('settings:notification.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await notifyChannelService.delete(id);
      message.success(t('settings:notification.deleteSuccess'));
      fetchChannels();
    } catch {
      message.error(t('settings:notification.deleteFailed'));
    }
  };

  const handleTest = async (ch: NotifyChannel) => {
    setTesting(ch.id);
    try {
      await notifyChannelService.test(ch.id);
      message.success(t('settings:notification.testSuccess'));
    } catch {
      message.error(t('settings:notification.testFailed'));
    } finally {
      setTesting(null);
    }
  };

  const typeTagColor: Record<string, string> = {
    webhook: 'blue',
    telegram: 'cyan',
    slack: 'green',
    teams: 'purple',
  };

  const columns: ColumnsType<NotifyChannel> = [
    {
      title: t('settings:notification.channelName'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: t('settings:notification.channelType'),
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => (
        <Tag color={typeTagColor[type] ?? 'default'}>{type.toUpperCase()}</Tag>
      ),
    },
    {
      title: t('settings:notification.webhookUrl'),
      dataIndex: 'webhookUrl',
      key: 'webhookUrl',
      ellipsis: true,
    },
    {
      title: t('settings:notification.description'),
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: t('settings:notification.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>
          {enabled ? t('settings:notification.enabledTag') : t('settings:notification.disabledTag')}
        </Tag>
      ),
    },
    {
      title: t('settings:notification.actions'),
      key: 'actions',
      render: (_: unknown, record: NotifyChannel) => (
        <Space>
          <Button
            size="small"
            icon={<SendOutlined />}
            loading={testing === record.id}
            onClick={() => handleTest(record)}
            disabled={!record.enabled}
          >
            {t('settings:notification.test')}
          </Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>
            {t('settings:notification.edit')}
          </Button>
          <Popconfirm
            title={t('settings:notification.deleteConfirm')}
            onConfirm={() => handleDelete(record.id)}
            okText={t('settings:notification.confirmOk')}
            cancelText={t('settings:notification.confirmCancel')}
          >
            <Button size="small" danger icon={<DeleteOutlined />}>
              {t('settings:notification.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: '0 8px' }}>
      <div style={{ marginBottom: 16 }}>
        <Title level={5} style={{ margin: 0 }}>
          {t('settings:notification.title')}
        </Title>
        <Paragraph type="secondary" style={{ margin: '4px 0 0' }}>
          {t('settings:notification.description')}
        </Paragraph>
      </div>

      <div style={{ marginBottom: 12 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          {t('settings:notification.addChannel')}
        </Button>
      </div>

      <Table
        scroll={{ x: 'max-content' }}
        rowKey="id"
        dataSource={channels}
        columns={columns}
        loading={loading}
        pagination={false}
        size="small"
      />

      <Modal
        title={editingChannel ? t('settings:notification.editChannel') : t('settings:notification.addChannel')}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label={t('settings:notification.channelName')}
            rules={[{ required: true, message: t('settings:notification.nameRequired') }]}
          >
            <Input placeholder={t('settings:notification.namePlaceholder')} />
          </Form.Item>

          <Form.Item
            name="type"
            label={t('settings:notification.channelType')}
            rules={[{ required: true }]}
          >
            <Select>
              {CHANNEL_TYPES.map(tp => (
                <Select.Option key={tp} value={tp}>
                  {tp.toUpperCase()}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="webhookUrl"
            label={t('settings:notification.webhookUrl')}
            rules={[{ required: true, message: t('settings:notification.webhookUrlRequired') }]}
          >
            <Input placeholder={t('settings:notification.webhookUrlPlaceholder')} />
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.type !== cur.type}
          >
            {({ getFieldValue }) =>
              getFieldValue('type') === 'telegram' ? (
                <Form.Item
                  name="telegramChatId"
                  label={t('settings:notification.telegramChatId')}
                  extra={t('settings:notification.telegramChatIdHint')}
                  rules={[{ required: true, message: t('settings:notification.telegramChatIdRequired') }]}
                >
                  <Input placeholder="-1001234567890" />
                </Form.Item>
              ) : null
            }
          </Form.Item>

          <Form.Item name="description" label={t('settings:notification.description')}>
            <Input placeholder={t('settings:notification.descriptionPlaceholder')} />
          </Form.Item>

          <Form.Item name="enabled" label={t('settings:notification.enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default NotificationSettings;

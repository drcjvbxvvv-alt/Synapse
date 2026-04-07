import React, { useState, useEffect } from 'react';
import {
  App, Button, Card, DatePicker, Divider, Form, Input, Space, Switch, Typography,
} from 'antd';
import {
  CloudUploadOutlined, DownloadOutlined, SaveOutlined, SendOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { siemService, type SIEMConfig } from '../../services/siemService';

const { Text } = Typography;
const { RangePicker } = DatePicker;

const SIEMConfigPage: React.FC = () => {
  const { message } = App.useApp();
  const { t } = useTranslation('settings');
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [exportRange, setExportRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);

  useEffect(() => {
    siemService.getConfig().then(res => {
      form.setFieldsValue(res.data);
    }).catch(() => {
      // 無設定時使用預設值
    });
  }, [form]);

  const handleSave = async () => {
    const values = await form.validateFields();
    setLoading(true);
    try {
      await siemService.updateConfig(values as SIEMConfig);
      message.success(t('security.siemSaved'));
    } catch {
      message.error(t('security.saveConfigFailed'));
    } finally {
      setLoading(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    try {
      const res = await siemService.testWebhook();
      message.success(`${res.data.message}（HTTP ${res.data.statusCode}）`);
    } catch {
      message.error(t('security.webhookTestFailed'));
    } finally {
      setTesting(false);
    }
  };

  const handleExport = () => {
    if (exportRange) {
      siemService.exportLogs({
        start: exportRange[0].format('YYYY-MM-DD'),
        end: exportRange[1].format('YYYY-MM-DD'),
      });
    } else {
      siemService.exportLogs();
    }
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      {/* Webhook 設定 */}
      <Card
        title={<Space><CloudUploadOutlined /> {t('security.siemWebhookTitle')}</Space>}
        extra={
          <Button type="primary" icon={<SaveOutlined />} onClick={handleSave} loading={loading}>
            {t('security.saveSettings')}
          </Button>
        }
      >
        <Form form={form} layout="vertical">
          <Form.Item name="enabled" label={t('security.enableSiem')} valuePropName="checked">
            <Switch
              checkedChildren={t('ai.enabled')}
              unCheckedChildren={t('ai.disabled')}
            />
          </Form.Item>
          <Form.Item
            name="webhookURL"
            label="Webhook URL"
            rules={[{ type: 'url', message: t('security.invalidUrl') }]}
          >
            <Input placeholder="https://siem.example.com/api/events" />
          </Form.Item>
          <Form.Item name="secretHeader" label={t('security.secretHeader')}>
            <Input placeholder={t('security.secretHeaderPlaceholder')} />
          </Form.Item>
          <Form.Item name="secretValue" label={t('security.secretValue')}>
            <Input.Password placeholder={t('security.secretValuePlaceholder')} />
          </Form.Item>
        </Form>

        <Button
          icon={<SendOutlined />}
          onClick={handleTest}
          loading={testing}
          style={{ marginTop: 8 }}
        >
          {t('security.testWebhook')}
        </Button>

        <Divider />
        <Text type="secondary" style={{ fontSize: 12 }}>
          {t('security.siemDesc')}
        </Text>
      </Card>

      {/* 批次匯出 */}
      <Card
        title={<Space><DownloadOutlined /> {t('security.auditExportTitle')}</Space>}
      >
        <Space wrap>
          <RangePicker
            onChange={(dates) => setExportRange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)}
            placeholder={[t('security.startDate'), t('security.endDate')]}
          />
          <Button icon={<DownloadOutlined />} type="primary" onClick={handleExport}>
            {t('security.exportJson')}
          </Button>
        </Space>
        <br />
        <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 8 }}>
          {t('security.auditExportDesc')}
        </Text>
      </Card>
    </Space>
  );
};

export default SIEMConfigPage;

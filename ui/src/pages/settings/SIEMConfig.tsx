import React, { useState, useEffect } from 'react';
import {
  App, Button, Card, DatePicker, Divider, Form, Input, Space, Switch, Typography,
} from 'antd';
import {
  CloudUploadOutlined, DownloadOutlined, SaveOutlined, SendOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { siemService, type SIEMConfig } from '../../services/siemService';

const { Text } = Typography;
const { RangePicker } = DatePicker;

const SIEMConfigPage: React.FC = () => {
  const { message } = App.useApp();
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
      message.success('SIEM 設定已儲存');
    } catch {
      message.error('儲存失敗');
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
      message.error('Webhook 測試失敗，請確認 URL 和設定是否正確');
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
        title={<Space><CloudUploadOutlined /> SIEM Webhook 即時推送</Space>}
        extra={
          <Button type="primary" icon={<SaveOutlined />} onClick={handleSave} loading={loading}>
            儲存設定
          </Button>
        }
      >
        <Form form={form} layout="vertical">
          <Form.Item name="enabled" label="啟用 SIEM 即時推送" valuePropName="checked">
            <Switch checkedChildren="啟用" unCheckedChildren="停用" />
          </Form.Item>
          <Form.Item
            name="webhookURL"
            label="Webhook URL"
            rules={[{ type: 'url', message: '請輸入有效的 URL' }]}
          >
            <Input placeholder="https://siem.example.com/api/events" />
          </Form.Item>
          <Form.Item name="secretHeader" label="認證 Header 名稱">
            <Input placeholder="例如 X-Auth-Token（可留空）" />
          </Form.Item>
          <Form.Item name="secretValue" label="認證 Header 值">
            <Input.Password placeholder="Header 值" />
          </Form.Item>
        </Form>

        <Button
          icon={<SendOutlined />}
          onClick={handleTest}
          loading={testing}
          style={{ marginTop: 8 }}
        >
          測試 Webhook 連線
        </Button>

        <Divider />
        <Text type="secondary" style={{ fontSize: 12 }}>
          啟用後，每筆寫入操作（POST/PUT/DELETE）的稽核記錄將即時推送到指定的 Webhook URL。
          推送失敗不影響正常操作，僅記錄警告日誌。
        </Text>
      </Card>

      {/* 批次匯出 */}
      <Card
        title={<Space><DownloadOutlined /> 稽核日誌批次匯出（JSON）</Space>}
      >
        <Space wrap>
          <RangePicker
            onChange={(dates) => setExportRange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)}
            placeholder={['開始日期', '結束日期']}
          />
          <Button icon={<DownloadOutlined />} type="primary" onClick={handleExport}>
            匯出 JSON
          </Button>
        </Space>
        <br />
        <Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 8 }}>
          不選日期範圍則匯出最近 10,000 筆記錄。檔案格式：JSON（每筆為一個 OperationLog 物件）。
          可直接匯入 Splunk、Elasticsearch 或 Datadog。
        </Text>
      </Card>
    </Space>
  );
};

export default SIEMConfigPage;

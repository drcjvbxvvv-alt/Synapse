import React, { useState, useEffect } from 'react';
import {
  Card,
  Form,
  Input,
  Button,
  Space,
  Typography,
  Divider,
  App,
  Switch,
  Select,
  AutoComplete,
  Tag,
  Alert,
  Row,
  Col,
} from 'antd';
import {
  RobotOutlined,
  SaveOutlined,
  ApiOutlined,
  LinkOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { aiService } from '../../services/aiService';
import type { AIConfig } from '../../types/ai';
import { useTranslation } from 'react-i18next';
import PageSkeleton from '../../components/PageSkeleton';

const { Title, Text } = Typography;

// 各 Provider 的預設值與說明
const PROVIDER_CONFIG: Record<string, {
  label: string;
  defaultEndpoint: string;
  defaultModel: string;
  needsApiKey: boolean;
  needsApiVersion: boolean;
  modelOptions: string[];
  endpointHint: string;
}> = {
  openai: {
    label: 'OpenAI / Compatible',
    defaultEndpoint: 'https://api.openai.com/v1',
    defaultModel: 'gpt-4o',
    needsApiKey: true,
    needsApiVersion: false,
    modelOptions: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'gpt-3.5-turbo', 'deepseek-chat', 'qwen-turbo'],
    endpointHint: 'OpenAI-compatible endpoint, e.g. https://api.openai.com/v1 or DeepSeek / Qwen proxy URL',
  },
  azure: {
    label: 'Azure OpenAI',
    defaultEndpoint: 'https://{resource-name}.openai.azure.com',
    defaultModel: 'gpt-4o',
    needsApiKey: true,
    needsApiVersion: true,
    modelOptions: ['gpt-4o', 'gpt-4o-mini', 'gpt-4', 'gpt-35-turbo'],
    endpointHint: 'Azure OpenAI resource endpoint, e.g. https://my-resource.openai.azure.com',
  },
  anthropic: {
    label: 'Anthropic Claude',
    defaultEndpoint: 'https://api.anthropic.com',
    defaultModel: 'claude-3-5-sonnet-20241022',
    needsApiKey: true,
    needsApiVersion: false,
    modelOptions: ['claude-opus-4-6', 'claude-sonnet-4-6', 'claude-3-5-sonnet-20241022', 'claude-3-5-haiku-20241022', 'claude-3-opus-20240229'],
    endpointHint: 'Anthropic API endpoint, default: https://api.anthropic.com',
  },
  ollama: {
    label: 'Ollama (Local)',
    defaultEndpoint: 'http://localhost:11434',
    defaultModel: 'llama3',
    needsApiKey: false,
    needsApiVersion: false,
    modelOptions: ['llama3', 'llama3.1', 'llama3.2', 'mistral', 'qwen2', 'gemma2', 'phi3'],
    endpointHint: 'Ollama server URL, default: http://localhost:11434',
  },
};

const AISettings: React.FC = () => {
  const { t } = useTranslation(['settings', 'common']);
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [hasApiKey, setHasApiKey] = useState(false);
  const [provider, setProvider] = useState<string>('openai');
  const { message } = App.useApp();

  const providerCfg = PROVIDER_CONFIG[provider] ?? PROVIDER_CONFIG.openai;

  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const response = await aiService.getConfig();
        const config = response as unknown as AIConfig;
        if (config.api_key === '******') {
          setHasApiKey(true);
          config.api_key = '';
        }
        setProvider(config.provider || 'openai');
        form.setFieldsValue(config);
      } catch (error) {
        message.error(t('settings:ai.loadConfigFailed'));
        console.error(error);
      } finally {
        setLoading(false);
      }
    };
    fetchConfig();
  }, [form, message, t]);

  const handleProviderChange = (val: string) => {
    setProvider(val);
    const cfg = PROVIDER_CONFIG[val] ?? PROVIDER_CONFIG.openai;
    form.setFieldsValue({
      endpoint: cfg.defaultEndpoint,
      model: cfg.defaultModel,
      api_version: '',
    });
  };

  const getSubmitData = (): AIConfig => {
    const values = form.getFieldsValue();
    const submitData = { ...values };
    if (!submitData.api_key && hasApiKey) {
      submitData.api_key = '******';
    }
    return submitData;
  };

  const handleSave = async () => {
    try {
      await form.validateFields();
      setSaving(true);
      const submitData = getSubmitData();
      await aiService.updateConfig(submitData);
      message.success(t('settings:ai.saveConfigSuccess'));
      if (form.getFieldValue('api_key')) {
        setHasApiKey(true);
        form.setFieldValue('api_key', '');
      }
    } catch (error) {
      message.error(t('settings:ai.saveConfigFailed'));
      console.error(error);
    } finally {
      setSaving(false);
    }
  };

  const handleTestConnection = async () => {
    try {
      const endpoint = form.getFieldValue('endpoint');
      if (!endpoint) {
        message.warning(t('settings:ai.endpointRequired'));
        return;
      }
      setTesting(true);
      const submitData = getSubmitData();
      const response = await aiService.testConnection(submitData);
      if ((response as unknown as { success: boolean })?.success) {
        message.success(t('settings:ai.testConnectionSuccess'));
      } else {
        message.error(t('settings:ai.testConnectionFailed'));
      }
    } catch (error) {
      message.error(t('settings:ai.testConnectionFailed'));
      console.error(error);
    } finally {
      setTesting(false);
    }
  };

  if (loading) return <PageSkeleton variant="detail" />;

  return (
    <div>
      <Card>
        <div style={{ marginBottom: 24 }}>
          <Title level={4} style={{ margin: 0 }}>
            <RobotOutlined style={{ marginRight: 8 }} />
            {t('settings:ai.title')}
          </Title>
          <Text type="secondary">
            {t('settings:ai.description')}
          </Text>
        </div>

        <Alert
          message={t('settings:ai.tip')}
          description={t('settings:ai.tipDesc')}
          type="info"
          showIcon
          style={{ marginBottom: 24 }}
        />

        <Form
          form={form}
          layout="vertical"
          initialValues={{
            provider: 'openai',
            endpoint: 'https://api.openai.com/v1',
            api_key: '',
            model: 'gpt-4o',
            api_version: '',
            enabled: false,
          }}
        >
          <Form.Item
            name="enabled"
            label={t('settings:ai.enableAI')}
            valuePropName="checked"
          >
            <Switch
              checkedChildren={t('settings:ai.enabled')}
              unCheckedChildren={t('settings:ai.disabled')}
            />
          </Form.Item>

          <Divider>{t('settings:ai.providerConfig')}</Divider>

          {/* Provider 選擇 */}
          <Form.Item name="provider" label={t('settings:ai.provider')}>
            <Select onChange={handleProviderChange} style={{ width: 280 }}>
              {Object.entries(PROVIDER_CONFIG).map(([key, cfg]) => (
                <Select.Option key={key} value={key}>
                  {cfg.label}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Row gutter={16}>
            <Col span={16}>
              <Form.Item
                name="endpoint"
                label={t('settings:ai.endpoint')}
                rules={[{ required: true, message: t('settings:ai.endpointRequired') }]}
                tooltip={providerCfg.endpointHint}
              >
                <Input prefix={<LinkOutlined />} placeholder={providerCfg.defaultEndpoint} />
              </Form.Item>
            </Col>
            <Col span={8}>
              {providerCfg.needsApiVersion && (
                <Form.Item
                  name="api_version"
                  label="API Version"
                  tooltip="Azure OpenAI api-version, e.g. 2024-05-01-preview"
                >
                  <Input placeholder="2024-05-01-preview" />
                </Form.Item>
              )}
            </Col>
          </Row>

          {/* API Key — Ollama 不需要 */}
          {providerCfg.needsApiKey && (
            <Form.Item
              name="api_key"
              label={
                <Space>
                  <span>{t('settings:ai.apiKey')}</span>
                  {hasApiKey && (
                    <Tag color="green" icon={<CheckCircleOutlined />}>
                      {t('settings:ai.configured')}
                    </Tag>
                  )}
                </Space>
              }
              tooltip={t('settings:ai.apiKeyTooltip')}
            >
              <Input.Password
                prefix={<ApiOutlined />}
                placeholder={
                  hasApiKey
                    ? t('settings:ai.apiKeyConfiguredPlaceholder')
                    : t('settings:ai.apiKeyPlaceholder')
                }
              />
            </Form.Item>
          )}

          {/* Model */}
          <Form.Item
            name="model"
            label={t('settings:ai.model')}
            rules={[{ required: true, message: t('settings:ai.modelRequired') }]}
            tooltip={t('settings:ai.modelTooltip')}
          >
            <AutoComplete
              placeholder={providerCfg.defaultModel}
              options={providerCfg.modelOptions.map(m => ({ label: m, value: m }))}
              filterOption={(input, option) =>
                (option?.value as string)?.toLowerCase().includes(input.toLowerCase())
              }
              dropdownRender={(menu) => (
                <>
                  {menu}
                  <Divider style={{ margin: '4px 0' }} />
                  <div style={{ padding: '4px 8px', color: '#999', fontSize: 12 }}>
                    {t('settings:ai.modelCustomHint')}
                  </div>
                </>
              )}
            />
          </Form.Item>

          <Divider />

          <Form.Item>
            <Space>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={saving}
                onClick={handleSave}
              >
                {t('settings:ai.saveConfig')}
              </Button>
              <Button
                icon={<ApiOutlined />}
                loading={testing}
                onClick={handleTestConnection}
              >
                {t('settings:ai.testConnection')}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default AISettings;

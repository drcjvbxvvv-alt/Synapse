import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Form,
  Input,
  Select,
  Button,
  message,
  Space,
  Divider,
  Row,
  Col,
  Modal,
  Alert,
  Typography,
  Collapse,
} from 'antd';
import { SaveOutlined, ExperimentOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import api, { parseApiError } from '../utils/api';

const { Option } = Select;
const { TextArea } = Input;
const { Text } = Typography;
const { Panel } = Collapse;

interface MonitoringConfig {
  type: 'disabled' | 'prometheus' | 'victoriametrics';
  endpoint: string;
  auth?: {
    type: 'none' | 'basic' | 'bearer' | 'mtls';
    username?: string;
    password?: string;
    token?: string;
    certFile?: string;
    keyFile?: string;
    caFile?: string;
  };
  labels?: Record<string, string>;
  options?: Record<string, unknown>;
}

interface MonitoringTemplates {
  disabled: MonitoringConfig;
  prometheus: MonitoringConfig;
  victoriametrics: MonitoringConfig;
}

interface MonitoringConfigFormProps {
  clusterId: string;
  onConfigChange?: () => void;
}

const MonitoringConfigForm: React.FC<MonitoringConfigFormProps> = ({
  clusterId,
  onConfigChange,
}) => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [templates, setTemplates] = useState<MonitoringTemplates | null>(null);
  const [configType, setConfigType] = useState<string>('disabled');
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [saveResult, setSaveResult] = useState<{ success: boolean; message: string } | null>(null);

  const loadTemplates = async () => {
    try {
      const response = await api.get('/monitoring/templates');
      setTemplates(response.data);
    } catch (error: unknown) {
      console.error('載入監控模板失敗:', error);
      message.error('載入監控模板失敗');
    }
  };

  const loadCurrentConfig = useCallback(async () => {
    try {
      const response = await api.get(`/clusters/${clusterId}/monitoring/config`);
      const config = response.data;
      setConfigType(config.type);
      form.setFieldsValue(config);
    } catch (error: unknown) {
      console.error('載入當前配置失敗:', error);
      message.error('載入當前配置失敗');
    }
  }, [clusterId, form]);

  useEffect(() => {
    loadTemplates();
    loadCurrentConfig();
  }, [clusterId, loadCurrentConfig]);

  const handleTypeChange = (type: string) => {
    setConfigType(type);
    if (templates && templates[type as keyof MonitoringTemplates]) {
      const template = templates[type as keyof MonitoringTemplates];
      form.setFieldsValue(template);
    }
  };

  const handleSave = async () => {
    try {
      setLoading(true);
      setSaveResult(null);
      
      // 表單驗證
      const values = await form.validateFields();
      
      // 傳送儲存請求
      const response = await api.put(`/clusters/${clusterId}/monitoring/config`, values);
      
      // 顯示成功訊息
      const successMsg = response.data?.message || '監控配置儲存成功';
      message.success(successMsg);
      setSaveResult({ success: true, message: successMsg });
      onConfigChange?.();
    } catch (error: unknown) {
      console.error('儲存監控配置失敗:', error);
      
      // 處理表單驗證錯誤
      if (error && typeof error === 'object' && 'errorFields' in error) {
        const errorMsg = '請檢查表單填寫是否正確';
        message.error(errorMsg);
        setSaveResult({ success: false, message: errorMsg });
        return;
      }
      
      const errorMsg = parseApiError(error);

      message.error(errorMsg);
      setSaveResult({ success: false, message: errorMsg });
    } finally {
      setLoading(false);
    }
  };

  const handleTest = async () => {
    try {
      setTesting(true);
      setTestResult(null);
      
      // 表單驗證
      const values = await form.validateFields();
      
      // 如果監控型別是禁用，不允許測試
      if (values.type === 'disabled') {
        message.warning('請先選擇監控型別');
        setTestResult({ success: false, message: '監控功能已禁用，無法測試連線' });
        return;
      }
      
      // 傳送測試請求
      const response = await api.post(`/clusters/${clusterId}/monitoring/test-connection`, values);
      
      // 顯示成功訊息
      const successMsg = response.data?.message || '連線測試成功';
      message.success(successMsg);
      setTestResult({ success: true, message: successMsg });
    } catch (error: unknown) {
      console.error('連線測試失敗:', error);
      
      // 處理表單驗證錯誤
      if (error && typeof error === 'object' && 'errorFields' in error) {
        const errorMsg = '請檢查表單填寫是否正確';
        message.error(errorMsg);
        setTestResult({ success: false, message: errorMsg });
        return;
      }
      
      const errorMsg = parseApiError(error);

      message.error(errorMsg);
      setTestResult({ success: false, message: errorMsg });
    } finally {
      setTesting(false);
    }
  };

  const renderAuthConfig = () => {
    const authType = form.getFieldValue(['auth', 'type']);
    
    return (
      <Card title="認證配置" size="small">
        <Form.Item
          name={['auth', 'type']}
          label="認證型別"
          rules={[{ required: configType !== 'disabled', message: '請選擇認證型別' }]}
          initialValue="none"
        >
          <Select placeholder="選擇認證型別">
            <Option value="none">無需認證</Option>
            <Option value="basic">Basic Auth</Option>
            <Option value="bearer">Bearer Token</Option>
            <Option value="mtls">mTLS</Option>
          </Select>
        </Form.Item>

        {authType === 'none' && (
          <Alert
            message="無需認證"
            description="將直接訪問監控端點，不進行任何身份驗證。"
            type="info"
            showIcon
            style={{ marginTop: 16 }}
          />
        )}

        {authType === 'basic' && (
          <>
            <Form.Item
              name={['auth', 'username']}
              label="使用者名稱"
              rules={[{ required: true, message: '請輸入使用者名稱' }]}
            >
              <Input placeholder="請輸入使用者名稱" />
            </Form.Item>
            <Form.Item
              name={['auth', 'password']}
              label="密碼"
              rules={[{ required: true, message: '請輸入密碼' }]}
            >
              <Input.Password placeholder="請輸入密碼" />
            </Form.Item>
          </>
        )}

        {authType === 'bearer' && (
          <Form.Item
            name={['auth', 'token']}
            label="Token"
            rules={[{ required: true, message: '請輸入Token' }]}
          >
            <Input.Password placeholder="請輸入Bearer Token" />
          </Form.Item>
        )}

        {authType === 'mtls' && (
          <>
            <Form.Item
              name={['auth', 'certFile']}
              label="證書檔案路徑"
              rules={[{ required: true, message: '請輸入證書檔案路徑' }]}
            >
              <Input placeholder="請輸入證書檔案路徑" />
            </Form.Item>
            <Form.Item
              name={['auth', 'keyFile']}
              label="金鑰檔案路徑"
              rules={[{ required: true, message: '請輸入金鑰檔案路徑' }]}
            >
              <Input placeholder="請輸入金鑰檔案路徑" />
            </Form.Item>
            <Form.Item
              name={['auth', 'caFile']}
              label="CA檔案路徑"
            >
              <Input placeholder="請輸入CA檔案路徑（可選）" />
            </Form.Item>
          </>
        )}
      </Card>
    );
  };

  const renderLabelsConfig = () => {
    return (
      <Card title="標籤配置" size="small">
        <Alert
          message="標籤配置說明"
          description="用於統一資料來源（如VictoriaMetrics）時區分不同叢集的監控資料。"
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <Form.Item
          name={['labels', 'cluster']}
          label="叢集標籤"
          tooltip="用於標識叢集的標籤鍵值對"
        >
          <Input placeholder="例如: cluster-name" />
        </Form.Item>
        <Text type="secondary">
          其他標籤可以透過高階配置新增
        </Text>
      </Card>
    );
  };

  return (
    <div>
      <Card title="監控配置" extra={
        <Space>
          <Button
            icon={<ExperimentOutlined />}
            onClick={handleTest}
            loading={testing}
            disabled={configType === 'disabled'}
          >
            測試連線
          </Button>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={handleSave}
            loading={loading}
          >
            儲存配置
          </Button>
        </Space>
      }>
        {/* 測試結果彈窗 */}
        <Modal
          open={testResult !== null}
          title={
            <Space>
              {testResult?.success ? (
                <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 20 }} />
              ) : (
                <CloseCircleOutlined style={{ color: '#ff4d4f', fontSize: 20 }} />
              )}
              <span>{testResult?.success ? '連線測試成功' : '連線測試失敗'}</span>
            </Space>
          }
          onCancel={() => setTestResult(null)}
          footer={[
            <Button key="ok" type="primary" onClick={() => setTestResult(null)}>
              確定
            </Button>
          ]}
        >
          <p>{testResult?.message}</p>
        </Modal>
        
        {/* 儲存結果彈窗 */}
        <Modal
          open={saveResult !== null}
          title={
            <Space>
              {saveResult?.success ? (
                <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 20 }} />
              ) : (
                <CloseCircleOutlined style={{ color: '#ff4d4f', fontSize: 20 }} />
              )}
              <span>{saveResult?.success ? '配置儲存成功' : '配置儲存失敗'}</span>
            </Space>
          }
          onCancel={() => setSaveResult(null)}
          footer={[
            <Button key="ok" type="primary" onClick={() => setSaveResult(null)}>
              確定
            </Button>
          ]}
        >
          <p>{saveResult?.message}</p>
        </Modal>
        
        <Form
          form={form}
          layout="vertical"
          initialValues={{ type: 'disabled' }}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="type"
                label="監控型別"
                rules={[{ required: true, message: '請選擇監控型別' }]}
              >
                <Select onChange={handleTypeChange}>
                  <Option value="disabled">禁用監控</Option>
                  <Option value="prometheus">Prometheus</Option>
                  <Option value="victoriametrics">VictoriaMetrics</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="endpoint"
                label="監控端點"
                rules={[
                  { required: configType !== 'disabled', message: '請輸入監控端點' },
                  { type: 'url', message: '請輸入有效的URL' }
                ]}
              >
                <Input placeholder="http://prometheus:9090" />
              </Form.Item>
            </Col>
          </Row>

          {configType !== 'disabled' && (
            <>
              <Divider />
              {renderAuthConfig()}
              <Divider />
              {renderLabelsConfig()}
            </>
          )}

          <Collapse>
            <Panel header="高階配置" key="advanced">
              <Form.Item
                name="options"
                label="額外選項"
                tooltip="JSON格式的額外配置選項"
              >
                <TextArea
                  rows={4}
                  placeholder='{"timeout": "30s", "maxPoints": 1000}'
                />
              </Form.Item>
            </Panel>
          </Collapse>
        </Form>
      </Card>

      <Card title="配置說明" style={{ marginTop: 16 }}>
        <Collapse>
          <Panel header="Prometheus 配置" key="prometheus">
            <div>
              <Text strong>直接連線 Prometheus：</Text>
              <ul>
                <li>端點：<Text code>http://prometheus-server:9090</Text></li>
                <li>認證：支援無需認證、Basic Auth、Bearer Token</li>
                <li>標籤：通常不需要額外標籤</li>
              </ul>
            </div>
          </Panel>
          <Panel header="VictoriaMetrics 配置" key="victoriametrics">
            <div>
              <Text strong>統一資料來源 VictoriaMetrics：</Text>
              <ul>
                <li>端點：<Text code>http://victoriametrics:8428</Text></li>
                <li>認證：支援無需認證、Basic Auth、Bearer Token</li>
                <li>標籤：<Text code>cluster="cluster-name"</Text> 用於區分叢集</li>
                <li>優勢：支援多叢集資料統一儲存和查詢</li>
              </ul>
            </div>
          </Panel>
          <Panel header="標籤說明" key="labels">
            <div>
              <Text strong>標籤配置說明：</Text>
              <ul>
                <li><Text code>cluster</Text>：叢集標識，用於區分不同叢集的監控資料</li>
                <li><Text code>environment</Text>：環境標識，如 prod、test、dev</li>
                <li><Text code>region</Text>：地域標識，如 us-east-1、ap-southeast-1</li>
                <li>其他自定義標籤可根據需要新增</li>
              </ul>
            </div>
          </Panel>
        </Collapse>
      </Card>
    </div>
  );
};

export default MonitoringConfigForm;

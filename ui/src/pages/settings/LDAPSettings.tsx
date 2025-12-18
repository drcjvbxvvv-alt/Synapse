import React, { useState, useEffect } from 'react';
import {
  Card,
  Form,
  Input,
  InputNumber,
  Switch,
  Button,
  Space,
  Typography,
  Divider,
  App,
  Modal,
  Alert,
  Spin,
  Row,
  Col,
} from 'antd';
import {
  CloudServerOutlined,
  SaveOutlined,
  ApiOutlined,
  UserOutlined,
  LockOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
} from '@ant-design/icons';
import { systemSettingService } from '../../services/authService';
import type { LDAPConfig } from '../../types';

const { Title, Text } = Typography;

const LDAPSettings: React.FC = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testAuthModalOpen, setTestAuthModalOpen] = useState(false);
  const [testingAuth, setTestingAuth] = useState(false);
  const [testAuthResult, setTestAuthResult] = useState<{
    success: boolean;
    error?: string;
    username?: string;
    email?: string;
    display_name?: string;
    groups?: string[];
  } | null>(null);
  const [testAuthForm] = Form.useForm();
  const { message } = App.useApp();

  // 加载LDAP配置
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const response = await systemSettingService.getLDAPConfig();
        if (response.code === 200) {
          form.setFieldsValue(response.data);
        }
      } catch (error) {
        message.error('加载LDAP配置失败');
        console.error(error);
      } finally {
        setLoading(false);
      }
    };

    fetchConfig();
  }, [form]);

  // 保存配置
  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      
      const response = await systemSettingService.updateLDAPConfig(values as LDAPConfig);
      if (response.code === 200) {
        message.success('LDAP配置保存成功');
      } else {
        message.error(response.message || '保存失败');
      }
    } catch (error) {
      message.error('保存LDAP配置失败');
      console.error(error);
    } finally {
      setSaving(false);
    }
  };

  // 测试连接
  const handleTestConnection = async () => {
    try {
      const values = await form.validateFields();
      setTesting(true);
      
      const response = await systemSettingService.testLDAPConnection(values as LDAPConfig);
      console.log('测试连接响应:', response);
      
      if (response.code === 200 && response.data?.success) {
        message.success('LDAP连接测试成功');
      } else {
        const errorMsg = response.data?.error || response.message || '连接测试失败';
        message.error(errorMsg);
      }
    } catch (error: any) {
      console.error('测试连接错误:', error);
      const errorMsg = error.response?.data?.message || error.message || 'LDAP连接测试失败';
      message.error(errorMsg);
    } finally {
      setTesting(false);
    }
  };

  // 测试用户认证
  const handleTestAuth = async () => {
    try {
      const values = await testAuthForm.validateFields();
      const ldapConfig = form.getFieldsValue();
      setTestingAuth(true);
      setTestAuthResult(null);
      
      const response = await systemSettingService.testLDAPAuth({
        username: values.username,
        password: values.password,
        server: ldapConfig.server,
        port: ldapConfig.port,
        use_tls: ldapConfig.use_tls,
        skip_tls_verify: ldapConfig.skip_tls_verify,
        bind_dn: ldapConfig.bind_dn,
        bind_password: ldapConfig.bind_password,
        base_dn: ldapConfig.base_dn,
        user_filter: ldapConfig.user_filter,
        username_attr: ldapConfig.username_attr,
        email_attr: ldapConfig.email_attr,
        display_name_attr: ldapConfig.display_name_attr,
        group_filter: ldapConfig.group_filter,
        group_attr: ldapConfig.group_attr,
      });
      
      console.log('测试认证响应:', response);
      
      if (response.code === 200 && response.data) {
        setTestAuthResult(response.data);
      } else {
        setTestAuthResult({
          success: false,
          error: response.message || '认证测试失败',
        });
      }
    } catch (error: any) {
      console.error('测试认证错误:', error);
      setTestAuthResult({
        success: false,
        error: error.response?.data?.message || error.message || '认证测试失败',
      });
    } finally {
      setTestingAuth(false);
    }
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 48 }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div>
      <Card>
        <div style={{ marginBottom: 24 }}>
          <Title level={4} style={{ margin: 0 }}>
            <CloudServerOutlined style={{ marginRight: 8 }} />
            LDAP 配置
          </Title>
          <Text type="secondary">
            配置 LDAP/Active Directory 以启用企业统一身份认证
          </Text>
        </div>

        <Form
          form={form}
          layout="vertical"
          initialValues={{
            enabled: false,
            port: 389,
            use_tls: false,
            skip_tls_verify: false,
            user_filter: '(uid=%s)',
            username_attr: 'uid',
            email_attr: 'mail',
            display_name_attr: 'cn',
            group_filter: '(memberUid=%s)',
            group_attr: 'cn',
          }}
        >
          <Form.Item
            name="enabled"
            label="启用 LDAP"
            valuePropName="checked"
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          <Divider>服务器配置</Divider>

          <Row gutter={16}>
            <Col span={16}>
              <Form.Item
                name="server"
                label="LDAP 服务器地址"
                rules={[{ required: true, message: '请输入服务器地址' }]}
              >
                <Input placeholder="例如: ldap.example.com" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="port"
                label="端口"
                rules={[{ required: true, message: '请输入端口' }]}
              >
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="use_tls"
                label="使用 TLS"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="skip_tls_verify"
                label="跳过 TLS 验证"
                valuePropName="checked"
                tooltip="不建议在生产环境启用"
              >
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Divider>绑定配置</Divider>

          <Form.Item
            name="bind_dn"
            label="绑定 DN"
            rules={[{ required: true, message: '请输入绑定 DN' }]}
            tooltip="用于搜索用户的管理员 DN"
          >
            <Input placeholder="例如: cn=admin,dc=example,dc=com" />
          </Form.Item>

          <Form.Item
            name="bind_password"
            label="绑定密码"
            rules={[{ required: true, message: '请输入绑定密码' }]}
          >
            <Input.Password placeholder="输入绑定密码" />
          </Form.Item>

          <Form.Item
            name="base_dn"
            label="Base DN"
            rules={[{ required: true, message: '请输入 Base DN' }]}
            tooltip="用户搜索的基础 DN"
          >
            <Input placeholder="例如: dc=example,dc=com" />
          </Form.Item>

          <Divider>用户属性映射</Divider>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="user_filter"
                label="用户过滤器"
                rules={[{ required: true, message: '请输入用户过滤器' }]}
                tooltip="%s 会被替换为用户名"
              >
                <Input placeholder="例如: (uid=%s)" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="username_attr"
                label="用户名属性"
                rules={[{ required: true, message: '请输入用户名属性' }]}
              >
                <Input placeholder="例如: uid 或 sAMAccountName" />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="email_attr"
                label="邮箱属性"
                rules={[{ required: true, message: '请输入邮箱属性' }]}
              >
                <Input placeholder="例如: mail" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="display_name_attr"
                label="显示名称属性"
                rules={[{ required: true, message: '请输入显示名称属性' }]}
              >
                <Input placeholder="例如: cn 或 displayName" />
              </Form.Item>
            </Col>
          </Row>

          <Divider>组配置（可选）</Divider>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="group_filter"
                label="组过滤器"
                tooltip="%s 会被替换为用户名"
              >
                <Input placeholder="例如: (memberUid=%s)" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="group_attr"
                label="组属性"
              >
                <Input placeholder="例如: cn" />
              </Form.Item>
            </Col>
          </Row>

          <Divider />

          <Form.Item>
            <Space>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={saving}
                onClick={handleSave}
              >
                保存配置
              </Button>
              <Button
                icon={<ApiOutlined />}
                loading={testing}
                onClick={handleTestConnection}
              >
                测试连接
              </Button>
              <Button
                icon={<UserOutlined />}
                onClick={() => {
                  setTestAuthModalOpen(true);
                  setTestAuthResult(null);
                  testAuthForm.resetFields();
                }}
              >
                测试用户认证
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>

      <Modal
        title="测试 LDAP 用户认证"
        open={testAuthModalOpen}
        onCancel={() => setTestAuthModalOpen(false)}
        footer={null}
        width={500}
      >
        <Form
          form={testAuthForm}
          layout="vertical"
          onFinish={handleTestAuth}
        >
          <Form.Item
            name="username"
            label="用户名"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input prefix={<UserOutlined />} placeholder="LDAP 用户名" />
          </Form.Item>

          <Form.Item
            name="password"
            label="密码"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="LDAP 密码" />
          </Form.Item>

          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              loading={testingAuth}
              block
            >
              测试认证
            </Button>
          </Form.Item>
        </Form>

        {testAuthResult && (
          <div style={{ marginTop: 16 }}>
            {testAuthResult.success ? (
              <Alert
                type="success"
                icon={<CheckCircleOutlined />}
                message="认证成功"
                description={
                  <div>
                    <p><strong>用户名:</strong> {testAuthResult.username}</p>
                    <p><strong>邮箱:</strong> {testAuthResult.email}</p>
                    <p><strong>显示名称:</strong> {testAuthResult.display_name}</p>
                    {testAuthResult.groups && testAuthResult.groups.length > 0 && (
                      <p><strong>用户组:</strong> {testAuthResult.groups.join(', ')}</p>
                    )}
                  </div>
                }
              />
            ) : (
              <Alert
                type="error"
                icon={<CloseCircleOutlined />}
                message="认证失败"
                description={testAuthResult.error}
              />
            )}
          </div>
        )}
      </Modal>
    </div>
  );
};

export default LDAPSettings;

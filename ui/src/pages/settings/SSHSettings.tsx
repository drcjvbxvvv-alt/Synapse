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
  Alert,
  Spin,
  Radio,
  Tag,
} from 'antd';
import {
  KeyOutlined,
  SaveOutlined,
  LockOutlined,
  UserOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { systemSettingService } from '../../services/authService';
import type { SSHConfig } from '../../types';

const { Title, Text } = Typography;

const SSHSettings: React.FC = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [hasPassword, setHasPassword] = useState(false);
  const [hasPrivateKey, setHasPrivateKey] = useState(false);
  const { message } = App.useApp();

  // 加载SSH配置
  useEffect(() => {
    const fetchConfig = async () => {
      try {
        const response = await systemSettingService.getSSHConfig();
        if (response.code === 200) {
          const config = response.data;
          
          // 检查是否已配置密码/私钥（后端返回 "******" 表示已配置）
          if (config.password === '******') {
            setHasPassword(true);
            config.password = ''; // 清空，不显示占位符
          }
          if (config.private_key === '******') {
            setHasPrivateKey(true);
            config.private_key = ''; // 清空，不显示占位符
          }
          
          form.setFieldsValue(config);
        }
      } catch (error) {
        message.error('加载SSH配置失败');
        console.error(error);
      } finally {
        setLoading(false);
      }
    };

    fetchConfig();
  }, [form, message]);

  // 保存配置
  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      
      // 如果密码/私钥为空且之前已配置，发送占位符表示保持原值
      const submitData = { ...values };
      if (!submitData.password && hasPassword) {
        submitData.password = '******';
      }
      if (!submitData.private_key && hasPrivateKey) {
        submitData.private_key = '******';
      }
      
      const response = await systemSettingService.updateSSHConfig(submitData as SSHConfig);
      if (response.code === 200) {
        message.success('SSH配置保存成功');
        // 更新状态
        if (values.password) {
          setHasPassword(true);
          form.setFieldValue('password', '');
        }
        if (values.private_key) {
          setHasPrivateKey(true);
          form.setFieldValue('private_key', '');
        }
      } else {
        message.error(response.message || '保存失败');
      }
    } catch (error) {
      message.error('保存SSH配置失败');
      console.error(error);
    } finally {
      setSaving(false);
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
            <KeyOutlined style={{ marginRight: 8 }} />
            全局 SSH 凭据
          </Title>
          <Text type="secondary">
            配置全局 SSH 凭据后，Node 终端点击"连接"按钮将自动使用此凭据连接
          </Text>
        </div>

        <Alert
          message="安全提示"
          description="SSH 凭据将加密存储在数据库中。建议使用 SSH 密钥认证方式，更加安全。"
          type="info"
          showIcon
          style={{ marginBottom: 24 }}
        />

        <Form
          form={form}
          layout="vertical"
          initialValues={{
            enabled: false,
            username: 'root',
            port: 22,
            auth_type: 'password',
          }}
        >
          <Form.Item
            name="enabled"
            label="启用全局 SSH 凭据"
            valuePropName="checked"
            tooltip="启用后，Node SSH 终端点击连接按钮将自动使用此凭据连接"
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          <Divider>连接配置</Divider>

          <Form.Item
            name="username"
            label="用户名"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input 
              prefix={<UserOutlined />} 
              placeholder="SSH 登录用户名，默认 root" 
            />
          </Form.Item>

          <Form.Item
            name="port"
            label="SSH 端口"
            rules={[{ required: true, message: '请输入端口号' }]}
          >
            <InputNumber 
              min={1} 
              max={65535} 
              style={{ width: '100%' }} 
              placeholder="默认 22"
            />
          </Form.Item>

          <Divider>认证方式</Divider>

          <Form.Item
            name="auth_type"
            label="认证方式"
            rules={[{ required: true, message: '请选择认证方式' }]}
          >
            <Radio.Group>
              <Radio.Button value="password">
                <LockOutlined /> 密码认证
              </Radio.Button>
              <Radio.Button value="key">
                <KeyOutlined /> 密钥认证
              </Radio.Button>
            </Radio.Group>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prevValues, currentValues) =>
              prevValues.auth_type !== currentValues.auth_type
            }
          >
            {({ getFieldValue }) => {
              const authType = getFieldValue('auth_type');
              
              if (authType === 'password') {
                return (
                  <Form.Item
                    name="password"
                    label={
                      <Space>
                        <span>密码</span>
                        {hasPassword && (
                          <Tag color="green" icon={<CheckCircleOutlined />}>
                            已配置
                          </Tag>
                        )}
                      </Space>
                    }
                    tooltip="留空则保持原密码不变"
                  >
                    <Input.Password 
                      placeholder={hasPassword ? "已配置密码，留空保持不变，输入新值则覆盖" : "请输入 SSH 登录密码"}
                    />
                  </Form.Item>
                );
              }
              
              if (authType === 'key') {
                return (
                  <Form.Item
                    name="private_key"
                    label={
                      <Space>
                        <span>私钥内容</span>
                        {hasPrivateKey && (
                          <Tag color="green" icon={<CheckCircleOutlined />}>
                            已配置
                          </Tag>
                        )}
                      </Space>
                    }
                    tooltip="留空则保持原私钥不变"
                  >
                    <Input.TextArea
                      rows={8}
                      placeholder={hasPrivateKey 
                        ? "已配置私钥，留空保持不变，输入新值则覆盖" 
                        : `请粘贴私钥内容（PEM格式），例如：
-----BEGIN OPENSSH PRIVATE KEY-----
...
-----END OPENSSH PRIVATE KEY-----`}
                      style={{ fontFamily: 'monospace' }}
                    />
                  </Form.Item>
                );
              }
              
              return null;
            }}
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
                保存配置
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default SSHSettings;

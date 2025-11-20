/** genAI_main_start */
import React from 'react';
import {
  Form,
  Input,
  InputNumber,
  Select,
  Switch,
  Button,
  Space,
  Row,
  Col,
  Card,
} from 'antd';
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';

const { Option } = Select;
const { TextArea } = Input;

export interface WorkloadFormData {
  name: string;
  namespace: string;
  replicas?: number;
  image: string;
  containerName: string;
  containerPort?: number;
  command?: string[];
  args?: string[];
  env?: Array<{ name: string; value: string }>;
  resources?: {
    limits?: {
      cpu?: string;
      memory?: string;
    };
    requests?: {
      cpu?: string;
      memory?: string;
    };
  };
  labels?: Array<{ key: string; value: string }>;
  annotations?: Array<{ key: string; value: string }>;
  // StatefulSet specific
  serviceName?: string;
  // CronJob specific
  schedule?: string;
  suspend?: boolean;
  // Job specific
  completions?: number;
  parallelism?: number;
  backoffLimit?: number;
}

interface WorkloadFormProps {
  workloadType: 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Rollout' | 'Job' | 'CronJob';
  initialData?: Partial<WorkloadFormData>;
  namespaces: string[];
  onValuesChange?: (values: Partial<WorkloadFormData>) => void;
  form?: any;
}

const WorkloadForm: React.FC<WorkloadFormProps> = ({
  workloadType,
  initialData,
  namespaces,
  onValuesChange,
  form: externalForm,
}) => {
  const [form] = Form.useForm(externalForm);

  // 设置初始值
  React.useEffect(() => {
    if (initialData) {
      form.setFieldsValue(initialData);
    } else {
      // 设置默认值
      form.setFieldsValue({
        namespace: 'default',
        replicas: workloadType === 'DaemonSet' ? undefined : 1,
        containerName: 'main',
        resources: {
          requests: {
            cpu: '100m',
            memory: '128Mi',
          },
          limits: {
            cpu: '500m',
            memory: '512Mi',
          },
        },
      });
    }
  }, [initialData, form, workloadType]);

  return (
    <Form
      form={form}
      layout="vertical"
      onValuesChange={onValuesChange}
    >
      {/* 基本信息 */}
      <Card title="基本信息" style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="name"
              label="名称"
              rules={[
                { required: true, message: '请输入名称' },
                {
                  pattern: /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/,
                  message: '名称只能包含小写字母、数字和连字符',
                },
              ]}
            >
              <Input placeholder="请输入名称" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="namespace"
              label="命名空间"
              rules={[{ required: true, message: '请选择命名空间' }]}
            >
              <Select placeholder="请选择命名空间" showSearch>
                {namespaces.map((ns) => (
                  <Option key={ns} value={ns}>
                    {ns}
                  </Option>
                ))}
              </Select>
            </Form.Item>
          </Col>
        </Row>

        {workloadType !== 'DaemonSet' && (
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="replicas"
                label="副本数"
                rules={[{ required: true, message: '请输入副本数' }]}
              >
                <InputNumber min={0} max={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
        )}

        {workloadType === 'StatefulSet' && (
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="serviceName"
                label="Headless Service"
                rules={[{ required: true, message: '请输入Service名称' }]}
              >
                <Input placeholder="请输入Headless Service名称" />
              </Form.Item>
            </Col>
          </Row>
        )}

        {workloadType === 'CronJob' && (
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="schedule"
                label="Cron表达式"
                rules={[{ required: true, message: '请输入Cron表达式' }]}
              >
                <Input placeholder="例如: 0 0 * * *" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="suspend" label="暂停" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>
        )}

        {workloadType === 'Job' && (
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="completions" label="完成次数">
                <InputNumber min={1} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="parallelism" label="并行度">
                <InputNumber min={1} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="backoffLimit" label="重试次数">
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
        )}
      </Card>

      {/* 容器配置 */}
      <Card title="容器配置" style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="containerName"
              label="容器名称"
              rules={[{ required: true, message: '请输入容器名称' }]}
            >
              <Input placeholder="请输入容器名称" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="image"
              label="镜像"
              rules={[{ required: true, message: '请输入镜像地址' }]}
            >
              <Input placeholder="例如: nginx:latest" />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="containerPort" label="容器端口">
              <InputNumber
                min={1}
                max={65535}
                style={{ width: '100%' }}
                placeholder="例如: 8080"
              />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item label="环境变量">
          <Form.List name="env">
            {(fields, { add, remove }) => (
              <>
                {fields.map((field) => (
                  <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                    <Form.Item
                      {...field}
                      name={[field.name, 'name']}
                      rules={[{ required: true, message: '请输入变量名' }]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="变量名" style={{ width: 200 }} />
                    </Form.Item>
                    <Form.Item
                      {...field}
                      name={[field.name, 'value']}
                      rules={[{ required: true, message: '请输入变量值' }]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="变量值" style={{ width: 300 }} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(field.name)} />
                  </Space>
                ))}
                <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
                  添加环境变量
                </Button>
              </>
            )}
          </Form.List>
        </Form.Item>
      </Card>

      {/* 资源配置 */}
      <Card title="资源配置" style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={12}>
            <h4 style={{ marginBottom: 16 }}>请求资源 (Requests)</h4>
            <Form.Item name={['resources', 'requests', 'cpu']} label="CPU">
              <Input placeholder="例如: 100m" />
            </Form.Item>
            <Form.Item name={['resources', 'requests', 'memory']} label="内存">
              <Input placeholder="例如: 128Mi" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <h4 style={{ marginBottom: 16 }}>限制资源 (Limits)</h4>
            <Form.Item name={['resources', 'limits', 'cpu']} label="CPU">
              <Input placeholder="例如: 500m" />
            </Form.Item>
            <Form.Item name={['resources', 'limits', 'memory']} label="内存">
              <Input placeholder="例如: 512Mi" />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      {/* 标签和注解 */}
      <Card title="标签和注解">
        <Form.Item label="标签 (Labels)">
          <Form.List name="labels">
            {(fields, { add, remove }) => (
              <>
                {fields.map((field) => (
                  <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                    <Form.Item
                      {...field}
                      name={[field.name, 'key']}
                      rules={[{ required: true, message: '请输入标签键' }]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="标签键" style={{ width: 200 }} />
                    </Form.Item>
                    <Form.Item
                      {...field}
                      name={[field.name, 'value']}
                      rules={[{ required: true, message: '请输入标签值' }]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="标签值" style={{ width: 300 }} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(field.name)} />
                  </Space>
                ))}
                <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
                  添加标签
                </Button>
              </>
            )}
          </Form.List>
        </Form.Item>

        <Form.Item label="注解 (Annotations)" style={{ marginTop: 16 }}>
          <Form.List name="annotations">
            {(fields, { add, remove }) => (
              <>
                {fields.map((field) => (
                  <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                    <Form.Item
                      {...field}
                      name={[field.name, 'key']}
                      rules={[{ required: true, message: '请输入注解键' }]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="注解键" style={{ width: 200 }} />
                    </Form.Item>
                    <Form.Item
                      {...field}
                      name={[field.name, 'value']}
                      rules={[{ required: true, message: '请输入注解值' }]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="注解值" style={{ width: 300 }} />
                    </Form.Item>
                    <MinusCircleOutlined onClick={() => remove(field.name)} />
                  </Space>
                ))}
                <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
                  添加注解
                </Button>
              </>
            )}
          </Form.List>
        </Form.Item>
      </Card>
    </Form>
  );
};

export default WorkloadForm;
export type { WorkloadFormProps };
/** genAI_main_end */


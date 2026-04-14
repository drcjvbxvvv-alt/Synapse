import React from 'react';
import { Form, Input, InputNumber, Select, Button, Space } from 'antd';
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';
import type { FormInstance } from 'antd';
import { useTranslation } from 'react-i18next';
import type { Service } from '../../types';

interface IngressFormContentProps {
  form: FormInstance;
  namespaces: string[];
  loadingNamespaces: boolean;
  services: Service[];
  loadingServices: boolean;
  onNamespaceChange: (ns: string) => void;
}

const IngressFormContent: React.FC<IngressFormContentProps> = ({
  form,
  namespaces,
  loadingNamespaces,
  services,
  loadingServices,
  onNamespaceChange,
}) => {
  const { t } = useTranslation(['network', 'common']);

  return (
    <Form
      form={form}
      layout="vertical"
      initialValues={{
        namespace: 'default',
        ingressClassName: 'nginx',
        rules: [{
          host: 'example.com',
          paths: [{ path: '/', pathType: 'Prefix', serviceName: 'my-service', servicePort: 80 }],
        }],
      }}
    >
      <Form.Item
        label={t('network:create.namespace')}
        name="namespace"
        rules={[{ required: true, message: t('network:create.namespaceRequired') }]}
      >
        <Select
          placeholder={t('network:create.namespacePlaceholder')}
          loading={loadingNamespaces}
          showSearch
          filterOption={(input, option) => {
            if (!option?.children) return false;
            return String(option.children).toLowerCase().includes(input.toLowerCase());
          }}
          onChange={(ns: string) => {
            onNamespaceChange(ns);
            // Clear service fields in all rule paths when namespace changes
            const rules = form.getFieldValue('rules') ?? [];
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const cleared = rules.map((rule: any) => ({
              ...rule,
              paths: (rule.paths ?? []).map((p: any) => ({ ...p, serviceName: undefined, servicePort: undefined })),
            }));
            form.setFieldValue('rules', cleared);
          }}
        >
          {namespaces.map((ns) => (
            <Select.Option key={ns} value={ns}>
              {ns}
            </Select.Option>
          ))}
        </Select>
      </Form.Item>

      <Form.Item
        label={t('network:create.ingressName')}
        name="name"
        rules={[
          { required: true, message: t('network:create.ingressNameRequired') },
          { pattern: /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/, message: t('network:create.ingressNamePattern') },
        ]}
      >
        <Input placeholder="my-ingress" />
      </Form.Item>

      <Form.Item label={t('network:create.ingressClassName')} name="ingressClassName">
        <Input placeholder="nginx" />
      </Form.Item>

      <Form.Item label={t('network:create.ruleConfig')} required>
        <Form.List name="rules">
          {(fields, { add, remove }) => (
            <>
              {fields.map((field, index) => (
                <div key={field.key} style={{ border: '1px solid #d9d9d9', padding: 16, marginBottom: 16, borderRadius: 4 }}>
                  <Space style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                    <strong>{t('network:create.rule')} {index + 1}</strong>
                    {fields.length > 1 && (
                      <Button type="link" danger onClick={() => remove(field.name)}>
                        {t('network:create.deleteRule')}
                      </Button>
                    )}
                  </Space>

                  <Form.Item
                    {...field}
                    label={t('network:create.host')}
                    name={[field.name, 'host']}
                    rules={[{ required: true, message: t('network:create.hostRequired') }]}
                  >
                    <Input placeholder="example.com" />
                  </Form.Item>

                  <Form.Item label={t('network:create.pathConfig')}>
                    <Form.List name={[field.name, 'paths']}>
                      {(pathFields, { add: addPath, remove: removePath }) => (
                        <>
                          {pathFields.map((pathField, pathIdx) => (
                            <Space key={pathField.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                              <Form.Item
                                {...pathField}
                                name={[pathField.name, 'path']}
                                rules={[{ required: true, message: t('network:create.required') }]}
                                noStyle
                              >
                                <Input placeholder="Path (/)" style={{ width: 100 }} />
                              </Form.Item>
                              <Form.Item
                                {...pathField}
                                name={[pathField.name, 'pathType']}
                                rules={[{ required: true, message: t('network:create.required') }]}
                                noStyle
                              >
                                <Select placeholder="PathType" style={{ width: 130 }}>
                                  <Select.Option value="Prefix">Prefix</Select.Option>
                                  <Select.Option value="Exact">Exact</Select.Option>
                                  <Select.Option value="ImplementationSpecific">ImplementationSpecific</Select.Option>
                                </Select>
                              </Form.Item>
                              <Form.Item
                                {...pathField}
                                name={[pathField.name, 'serviceName']}
                                rules={[{ required: true, message: t('network:create.required') }]}
                                noStyle
                              >
                                <Select
                                  showSearch
                                  allowClear
                                  placeholder={t('network:create.serviceNameField')}
                                  style={{ width: 180 }}
                                  loading={loadingServices}
                                  filterOption={(input, option) =>
                                    String(option?.value ?? '').toLowerCase().includes(input.toLowerCase())
                                  }
                                  options={services.map((s) => ({ label: s.name, value: s.name }))}
                                  notFoundContent={
                                    <span style={{ fontSize: 12, color: '#999' }}>
                                      {t('network:create.noServiceFound')}
                                    </span>
                                  }
                                  onChange={(svcName: string) => {
                                    const svc = services.find((s) => s.name === svcName);
                                    if (svc?.ports?.[0]) {
                                      form.setFieldValue(
                                        ['rules', field.name, 'paths', pathIdx, 'servicePort'],
                                        svc.ports[0].port,
                                      );
                                    }
                                  }}
                                />
                              </Form.Item>
                              <Form.Item noStyle shouldUpdate>
                                {({ getFieldValue }) => {
                                  const svcName = getFieldValue(['rules', field.name, 'paths', pathIdx, 'serviceName']);
                                  const svc = services.find((s) => s.name === svcName);
                                  const portOpts = svc?.ports.map((p) => ({
                                    label: p.name ? `${p.port} (${p.name})` : String(p.port),
                                    value: p.port,
                                  })) ?? [];
                                  return (
                                    <Form.Item
                                      name={[pathField.name, 'servicePort']}
                                      rules={[{ required: true, message: t('network:create.required') }]}
                                      noStyle
                                    >
                                      {portOpts.length > 0 ? (
                                        <Select
                                          showSearch
                                          placeholder="Port"
                                          style={{ width: 130 }}
                                          options={portOpts}
                                          filterOption={(input, option) =>
                                            String(option?.value ?? '').includes(input)
                                          }
                                        />
                                      ) : (
                                        <InputNumber
                                          placeholder="Port"
                                          style={{ width: 130 }}
                                          min={1}
                                          max={65535}
                                        />
                                      )}
                                    </Form.Item>
                                  );
                                }}
                              </Form.Item>
                              <MinusCircleOutlined onClick={() => removePath(pathField.name)} />
                            </Space>
                          ))}
                          <Button type="dashed" onClick={() => addPath()} size="small" icon={<PlusOutlined />}>
                            {t('network:create.addPath')}
                          </Button>
                        </>
                      )}
                    </Form.List>
                  </Form.Item>
                </div>
              ))}
              <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
                {t('network:create.addRule')}
              </Button>
            </>
          )}
        </Form.List>
      </Form.Item>

      <Form.Item label={t('network:create.tlsConfig')}>
        <Form.List name="tls">
          {(fields, { add, remove }) => (
            <>
              {fields.map((field) => (
                <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                  <Form.Item
                    {...field}
                    name={[field.name, 'hosts']}
                    rules={[{ required: true, message: t('network:create.hostsRequired') }]}
                    noStyle
                  >
                    <Input placeholder={t('network:create.hostsPlaceholder')} style={{ width: 300 }} />
                  </Form.Item>
                  <Form.Item
                    {...field}
                    name={[field.name, 'secretName']}
                    rules={[{ required: true, message: t('network:create.secretNameRequired') }]}
                    noStyle
                  >
                    <Input placeholder={t('network:create.secretName')} style={{ width: 200 }} />
                  </Form.Item>
                  <MinusCircleOutlined onClick={() => remove(field.name)} />
                </Space>
              ))}
              <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
                {t('network:create.addTLS')}
              </Button>
            </>
          )}
        </Form.List>
      </Form.Item>
    </Form>
  );
};

export default IngressFormContent;

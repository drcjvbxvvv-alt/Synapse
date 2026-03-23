import React from 'react';
import {
  Modal,
  Tabs,
  Form,
  Input,
  Select,
  Button,
  Space,
  InputNumber,
} from 'antd';
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import type { Ingress } from '../../types';
import type { FormInstance } from 'antd';
import { useTranslation } from 'react-i18next';

interface IngressFormProps {
  visible: boolean;
  editingIngress: Ingress | null;
  editMode: 'form' | 'yaml';
  editYaml: string;
  saveLoading: boolean;
  form: FormInstance;
  namespaces: { name: string; count: number }[];
  onEditModeChange: (mode: 'form' | 'yaml') => void;
  onEditYamlChange: (yaml: string) => void;
  onCancel: () => void;
  onSave: () => void;
}

const IngressForm: React.FC<IngressFormProps> = ({
  visible,
  editingIngress,
  editMode,
  editYaml,
  saveLoading,
  form,
  namespaces,
  onEditModeChange,
  onEditYamlChange,
  onCancel,
  onSave,
}) => {
  const { t } = useTranslation(['network', 'common']);

  return (
    <Modal
      title={t('network:ingress.edit.title', { name: editingIngress?.name })}
      open={visible}
      onCancel={onCancel}
      onOk={onSave}
      confirmLoading={saveLoading}
      width={1000}
      okText={t('common:actions.save')}
      cancelText={t('common:actions.cancel')}
    >
      <Tabs activeKey={editMode} onChange={(key) => onEditModeChange(key as 'form' | 'yaml')}>
        <Tabs.TabPane tab={t('network:ingress.edit.formTab')} key="form">
          <Form form={form} layout="vertical">
            <Form.Item label={t('network:ingress.edit.name')} name="name" rules={[{ required: true, message: t('network:ingress.edit.nameRequired') }]}>
              <Input disabled placeholder={t('network:ingress.edit.namePlaceholder')} />
            </Form.Item>
            
            <Form.Item label={t('network:ingress.edit.namespace')} name="namespace" rules={[{ required: true, message: t('network:ingress.edit.namespaceRequired') }]}>
              <Select disabled placeholder={t('network:ingress.edit.namespacePlaceholder')}>
                {namespaces.map((ns) => (
                  <Select.Option key={ns.name} value={ns.name}>
                    {ns.name}
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>
            
            <Form.Item label={t('network:ingress.edit.ingressClass')} name="ingressClass">
              <Input placeholder={t('network:ingress.edit.ingressClassPlaceholder')} />
            </Form.Item>
            
            <Form.Item label={t('network:ingress.edit.rules')}>
              <Form.List name="rules">
                {(fields, { add, remove }) => (
                  <>
                    {fields.map((field) => (
                      <div key={field.key} style={{ marginBottom: 16, padding: 16, border: '1px solid #d9d9d9', borderRadius: 4 }}>
                        <Form.Item {...field} name={[field.name, 'host']} label={t('network:ingress.edit.host')}>
                          <Input placeholder="example.com" />
                        </Form.Item>
                        
                        <Form.Item label={t('network:ingress.edit.paths')}>
                          <Form.List name={[field.name, 'paths']}>
                            {(pathFields, { add: addPath, remove: removePath }) => (
                              <>
                                {pathFields.map((pathField) => (
                                  <Space key={pathField.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                                    <Form.Item {...pathField} name={[pathField.name, 'path']} noStyle>
                                      <Input placeholder="/" style={{ width: 100 }} />
                                    </Form.Item>
                                    <Form.Item {...pathField} name={[pathField.name, 'pathType']} noStyle initialValue="Prefix">
                                      <Select style={{ width: 120 }}>
                                        <Select.Option value="Prefix">Prefix</Select.Option>
                                        <Select.Option value="Exact">Exact</Select.Option>
                                      </Select>
                                    </Form.Item>
                                    <Form.Item {...pathField} name={[pathField.name, 'serviceName']} noStyle>
                                      <Input placeholder={t('network:ingress.edit.serviceName')} style={{ width: 120 }} />
                                    </Form.Item>
                                    <Form.Item {...pathField} name={[pathField.name, 'servicePort']} noStyle>
                                      <InputNumber placeholder={t('network:ingress.edit.servicePort')} min={1} max={65535} style={{ width: 100 }} />
                                    </Form.Item>
                                    <MinusCircleOutlined onClick={() => removePath(pathField.name)} />
                                  </Space>
                                ))}
                                <Button type="dashed" onClick={() => addPath()} block icon={<PlusOutlined />}>
                                  {t('network:ingress.edit.addPath')}
                                </Button>
                              </>
                            )}
                          </Form.List>
                        </Form.Item>
                        
                        <Button type="link" danger onClick={() => remove(field.name)}>
                          {t('network:ingress.edit.deleteRule')}
                        </Button>
                      </div>
                    ))}
                    <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                      {t('network:ingress.edit.addRule')}
                    </Button>
                  </>
                )}
              </Form.List>
            </Form.Item>
            
            <Form.Item label={t('network:ingress.edit.labels')}>
              <Form.List name="labels">
                {(fields, { add, remove }) => (
                  <>
                    {fields.map((field) => (
                      <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }}>
                        <Form.Item {...field} name={[field.name, 'key']} noStyle>
                          <Input placeholder={t('network:ingress.edit.key')} style={{ width: 150 }} />
                        </Form.Item>
                        <Form.Item {...field} name={[field.name, 'value']} noStyle>
                          <Input placeholder={t('network:ingress.edit.value')} style={{ width: 150 }} />
                        </Form.Item>
                        <MinusCircleOutlined onClick={() => remove(field.name)} />
                      </Space>
                    ))}
                    <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                      {t('network:ingress.edit.addLabel')}
                    </Button>
                  </>
                )}
              </Form.List>
            </Form.Item>
            
            <Form.Item label={t('network:ingress.edit.annotations')}>
              <Form.List name="annotations">
                {(fields, { add, remove }) => (
                  <>
                    {fields.map((field) => (
                      <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }}>
                        <Form.Item {...field} name={[field.name, 'key']} noStyle>
                          <Input placeholder={t('network:ingress.edit.key')} style={{ width: 150 }} />
                        </Form.Item>
                        <Form.Item {...field} name={[field.name, 'value']} noStyle>
                          <Input placeholder={t('network:ingress.edit.value')} style={{ width: 150 }} />
                        </Form.Item>
                        <MinusCircleOutlined onClick={() => remove(field.name)} />
                      </Space>
                    ))}
                    <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                      {t('network:ingress.edit.addAnnotation')}
                    </Button>
                  </>
                )}
              </Form.List>
            </Form.Item>
          </Form>
        </Tabs.TabPane>
        
        <Tabs.TabPane tab={t('network:ingress.edit.yamlTab')} key="yaml">
          <MonacoEditor
            height="600px"
            language="yaml"
            value={editYaml}
            onChange={(value) => onEditYamlChange(value || '')}
            options={{
              minimap: { enabled: false },
              fontSize: 14,
              wordWrap: 'on',
              scrollBeyondLastLine: false,
            }}
          />
        </Tabs.TabPane>
      </Tabs>
    </Modal>
  );
};

export default IngressForm;

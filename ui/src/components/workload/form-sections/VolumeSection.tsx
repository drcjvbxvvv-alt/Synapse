import React from 'react';
import { Form, Input, Select, Button, Row, Col, Card } from 'antd';
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';
import type { FormSectionProps } from './types';

const { Option } = Select;

const VolumeSection: React.FC<FormSectionProps> = ({ form, t }) => {
  return (
    <Card title={t('workloadForm.volumeConfig')} style={{ marginBottom: 16 }}>
      <Form.List name="volumes">
        {(fields, { add, remove }) => (
          <>
            {fields.map((field) => (
              <Card key={field.key} size="small" style={{ marginBottom: 16 }}>
                <Row gutter={16}>
                  <Col span={6}>
                    <Form.Item
                      name={[field.name, 'name']}
                      label={t('workloadForm.volumeName')}
                      rules={[{ required: true, message: t('workloadForm.nameRequired') }]}
                    >
                      <Input placeholder="volume-name" />
                    </Form.Item>
                  </Col>
                  <Col span={6}>
                    <Form.Item
                      name={[field.name, 'type']}
                      label={t('workloadForm.volumeType')}
                      rules={[{ required: true, message: t('workloadForm.selectType') }]}
                    >
                      <Select placeholder={t('workloadForm.selectType')}>
                        <Option value="emptyDir">{t('workloadForm.emptyDir')}</Option>
                        <Option value="hostPath">{t('workloadForm.hostPath')}</Option>
                        <Option value="configMap">ConfigMap</Option>
                        <Option value="secret">Secret</Option>
                        <Option value="persistentVolumeClaim">PVC</Option>
                      </Select>
                    </Form.Item>
                  </Col>

                  <Form.Item noStyle shouldUpdate>
                    {() => {
                      const volumeType = form.getFieldValue(['volumes', field.name, 'type']);
                      return (
                        <>
                          {volumeType === 'hostPath' && (
                            <Col span={10}>
                              <Form.Item
                                name={[field.name, 'hostPath', 'path']}
                                label={t('workloadForm.hostPathLabel')}
                                rules={[{ required: true, message: t('workloadForm.pathRequired') }]}
                              >
                                <Input placeholder="/data/host-path" />
                              </Form.Item>
                            </Col>
                          )}
                          {volumeType === 'configMap' && (
                            <Col span={10}>
                              <Form.Item
                                name={[field.name, 'configMap', 'name']}
                                label={t('workloadForm.configMapName')}
                                rules={[{ required: true, message: t('workloadForm.nameRequired') }]}
                              >
                                <Input placeholder="configmap-name" />
                              </Form.Item>
                            </Col>
                          )}
                          {volumeType === 'secret' && (
                            <Col span={10}>
                              <Form.Item
                                name={[field.name, 'secret', 'secretName']}
                                label={t('workloadForm.secretName')}
                                rules={[{ required: true, message: t('workloadForm.nameRequired') }]}
                              >
                                <Input placeholder="secret-name" />
                              </Form.Item>
                            </Col>
                          )}
                          {volumeType === 'persistentVolumeClaim' && (
                            <Col span={10}>
                              <Form.Item
                                name={[field.name, 'persistentVolumeClaim', 'claimName']}
                                label={t('workloadForm.pvcName')}
                                rules={[{ required: true, message: t('workloadForm.nameRequired') }]}
                              >
                                <Input placeholder="pvc-name" />
                              </Form.Item>
                            </Col>
                          )}
                        </>
                      );
                    }}
                  </Form.Item>

                  <Col span={2}>
                    <Form.Item label=" ">
                      <Button
                        type="text"
                        danger
                        icon={<MinusCircleOutlined />}
                        onClick={() => remove(field.name)}
                      />
                    </Form.Item>
                  </Col>
                </Row>
              </Card>
            ))}
            <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
              {t('workloadForm.addVolume')}
            </Button>
          </>
        )}
      </Form.List>
    </Card>
  );
};

export default VolumeSection;

import React from 'react';
import { Form, Input, InputNumber, Select, Button, Row, Col, Card, Collapse } from 'antd';
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';
import type { FormSectionProps } from './types';

const { Option } = Select;
const { Panel } = Collapse;

const TolerationsSection: React.FC<Omit<FormSectionProps, 'form' | 'workloadType' | 'isEdit'>> = ({
  t,
}) => {
  return (
    <Panel header={t('workloadForm.tolerations')} key="tolerations">
      <Form.List name="tolerations">
        {(fields, { add, remove }) => (
          <>
            {fields.map((field) => (
              <Card key={field.key} size="small" style={{ marginBottom: 8 }}>
                <Row gutter={16}>
                  <Col span={5}>
                    <Form.Item name={[field.name, 'key']} label={t('workloadForm.key')}>
                      <Input placeholder="node.kubernetes.io/not-ready" />
                    </Form.Item>
                  </Col>
                  <Col span={4}>
                    <Form.Item name={[field.name, 'operator']} label={t('workloadForm.operator')}>
                      <Select defaultValue="Equal">
                        <Option value="Equal">Equal</Option>
                        <Option value="Exists">Exists</Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col span={5}>
                    <Form.Item name={[field.name, 'value']} label={t('workloadForm.value')}>
                      <Input placeholder={t('workloadForm.value')} />
                    </Form.Item>
                  </Col>
                  <Col span={4}>
                    <Form.Item name={[field.name, 'effect']} label={t('workloadForm.effect')}>
                      <Select>
                        <Option value="">{t('workloadForm.all')}</Option>
                        <Option value="NoSchedule">NoSchedule</Option>
                        <Option value="PreferNoSchedule">PreferNoSchedule</Option>
                        <Option value="NoExecute">NoExecute</Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col span={4}>
                    <Form.Item
                      name={[field.name, 'tolerationSeconds']}
                      label={t('workloadForm.tolerationSeconds')}
                    >
                      <InputNumber min={0} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col span={2}>
                    <Form.Item label=" ">
                      <MinusCircleOutlined onClick={() => remove(field.name)} />
                    </Form.Item>
                  </Col>
                </Row>
              </Card>
            ))}
            <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
              {t('workloadForm.addToleration')}
            </Button>
          </>
        )}
      </Form.List>
    </Panel>
  );
};

export default TolerationsSection;

import React from 'react';
import { Form, Input, Button, Row, Col, Divider, Collapse } from 'antd';
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons';
import type { FormSectionProps } from './types';

const { Panel } = Collapse;

const LabelsAnnotationsSection: React.FC<
  Omit<FormSectionProps, 'form' | 'workloadType' | 'isEdit'>
> = ({ t }) => {
  return (
    <Panel header={t('workloadForm.labelsAnnotations')} key="labels">
      <Divider orientation="left">{t('workloadForm.labels')}</Divider>
      <Form.List name="labels">
        {(fields, { add, remove }) => (
          <>
            {fields.map((field) => (
              <Row key={field.key} gutter={16} style={{ marginBottom: 8 }}>
                <Col span={10}>
                  <Form.Item name={[field.name, 'key']} noStyle>
                    <Input placeholder={t('workloadForm.key')} />
                  </Form.Item>
                </Col>
                <Col span={10}>
                  <Form.Item name={[field.name, 'value']} noStyle>
                    <Input placeholder={t('workloadForm.value')} />
                  </Form.Item>
                </Col>
                <Col span={4}>
                  <MinusCircleOutlined onClick={() => remove(field.name)} />
                </Col>
              </Row>
            ))}
            <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
              {t('workloadForm.addLabel')}
            </Button>
          </>
        )}
      </Form.List>

      <Divider orientation="left">{t('workloadForm.annotations')}</Divider>
      <Form.List name="annotations">
        {(fields, { add, remove }) => (
          <>
            {fields.map((field) => (
              <Row key={field.key} gutter={16} style={{ marginBottom: 8 }}>
                <Col span={10}>
                  <Form.Item name={[field.name, 'key']} noStyle>
                    <Input placeholder={t('workloadForm.key')} />
                  </Form.Item>
                </Col>
                <Col span={10}>
                  <Form.Item name={[field.name, 'value']} noStyle>
                    <Input placeholder={t('workloadForm.value')} />
                  </Form.Item>
                </Col>
                <Col span={4}>
                  <MinusCircleOutlined onClick={() => remove(field.name)} />
                </Col>
              </Row>
            ))}
            <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
              {t('workloadForm.addAnnotation')}
            </Button>
          </>
        )}
      </Form.List>
    </Panel>
  );
};

export default LabelsAnnotationsSection;

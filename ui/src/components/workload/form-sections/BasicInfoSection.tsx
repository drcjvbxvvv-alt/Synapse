import React from 'react';
import { Form, Input, InputNumber, Select, Switch, Row, Col, Card } from 'antd';
import type { BasicInfoSectionProps } from './types';

const { Option } = Select;
const { TextArea } = Input;

const BasicInfoSection: React.FC<BasicInfoSectionProps> = ({
  t,
  workloadType,
  namespaces,
  isEdit = false,
}) => {
  return (
    <Card title={t('workloadForm.basicInfo')} style={{ marginBottom: 16 }}>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item
            name="name"
            label={t('workloadForm.name')}
            rules={[
              { required: true, message: t('workloadForm.nameRequired') },
              {
                pattern: /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/,
                message: t('workloadForm.namePattern'),
              },
            ]}
            tooltip={isEdit ? t('workloadForm.nameEditTooltip') : undefined}
          >
            <Input placeholder={t('workloadForm.namePlaceholder')} disabled={isEdit} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item
            name="namespace"
            label={t('workloadForm.namespace')}
            rules={[{ required: true, message: t('workloadForm.namespaceRequired') }]}
            tooltip={isEdit ? t('workloadForm.namespaceEditTooltip') : undefined}
          >
            <Select placeholder={t('workloadForm.namespacePlaceholder')} showSearch disabled={isEdit}>
              {namespaces.map((ns) => (
                <Option key={ns} value={ns}>
                  {ns}
                </Option>
              ))}
            </Select>
          </Form.Item>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={24}>
          <Form.Item name="description" label={t('workloadForm.description')}>
            <TextArea
              rows={2}
              placeholder={t('workloadForm.descriptionPlaceholder')}
              maxLength={200}
              showCount
            />
          </Form.Item>
        </Col>
      </Row>

      {workloadType !== 'DaemonSet' && workloadType !== 'Job' && workloadType !== 'CronJob' && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="replicas"
              label={t('workloadForm.replicas')}
              rules={[{ required: true, message: t('workloadForm.replicasRequired') }]}
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
              label={t('workloadForm.headlessService')}
              rules={[{ required: true, message: t('workloadForm.headlessServiceRequired') }]}
            >
              <Input placeholder={t('workloadForm.headlessServicePlaceholder')} />
            </Form.Item>
          </Col>
        </Row>
      )}

      {workloadType === 'CronJob' && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="schedule"
              label={t('workloadForm.cronExpression')}
              rules={[{ required: true, message: t('workloadForm.cronRequired') }]}
            >
              <Input placeholder="0 0 * * *" />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="suspend" label={t('workloadForm.suspend')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="concurrencyPolicy" label={t('workloadForm.concurrencyPolicy')}>
              <Select defaultValue="Allow">
                <Option value="Allow">{t('workloadForm.allowConcurrent')}</Option>
                <Option value="Forbid">{t('workloadForm.forbidConcurrent')}</Option>
                <Option value="Replace">{t('workloadForm.replaceConcurrent')}</Option>
              </Select>
            </Form.Item>
          </Col>
        </Row>
      )}

      {workloadType === 'Job' && (
        <Row gutter={16}>
          <Col span={6}>
            <Form.Item name="completions" label={t('workloadForm.completions')}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="parallelism" label={t('workloadForm.parallelism')}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="backoffLimit" label={t('workloadForm.backoffLimit')}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="activeDeadlineSeconds" label={t('workloadForm.activeDeadlineSeconds')}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
        </Row>
      )}
    </Card>
  );
};

export default BasicInfoSection;

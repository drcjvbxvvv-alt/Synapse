import React from 'react';
import { Form, Input, InputNumber, Select, Row, Col } from 'antd';
import { Collapse } from 'antd';
import type { FormSectionProps } from './types';

const { Option } = Select;
const { Panel } = Collapse;

const DeploymentStrategySection: React.FC<FormSectionProps> = ({ form, t }) => {
  return (
    <Panel header={t('workloadForm.upgradeStrategy')} key="strategy">
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name={['strategy', 'type']} label={t('workloadForm.strategyType')}>
            <Select defaultValue="RollingUpdate">
              <Option value="RollingUpdate">{t('workloadForm.rollingUpdate')}</Option>
              <Option value="Recreate">{t('workloadForm.recreate')}</Option>
            </Select>
          </Form.Item>
        </Col>
        <Form.Item noStyle shouldUpdate>
          {() => {
            const strategyType = form.getFieldValue(['strategy', 'type']);
            if (strategyType !== 'RollingUpdate') return null;
            return (
              <>
                <Col span={8}>
                  <Form.Item
                    name={['strategy', 'rollingUpdate', 'maxUnavailable']}
                    label={t('workloadForm.maxUnavailable')}
                  >
                    <Input placeholder={t('workloadForm.maxUnavailablePlaceholder')} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item
                    name={['strategy', 'rollingUpdate', 'maxSurge']}
                    label={t('workloadForm.maxSurge')}
                  >
                    <Input placeholder={t('workloadForm.maxSurgePlaceholder')} />
                  </Form.Item>
                </Col>
              </>
            );
          }}
        </Form.Item>
      </Row>
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="minReadySeconds" label={t('workloadForm.minReadySeconds')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="revisionHistoryLimit" label={t('workloadForm.revisionHistoryLimit')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="progressDeadlineSeconds" label={t('workloadForm.progressDeadlineSeconds')}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
    </Panel>
  );
};

export default DeploymentStrategySection;

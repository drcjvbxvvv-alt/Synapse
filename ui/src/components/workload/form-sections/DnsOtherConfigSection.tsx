import React from 'react';
import { Form, Input, InputNumber, Select, Switch, Row, Col, Collapse } from 'antd';
import type { FormSectionProps } from './types';

const { Option } = Select;
const { Panel } = Collapse;

// DNS configuration panel
export const DnsConfigSection: React.FC<
  Omit<FormSectionProps, 'form' | 'workloadType' | 'isEdit'>
> = ({ t }) => {
  return (
    <Panel header={t('workloadForm.dnsConfig')} key="dns">
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="dnsPolicy" label={t('workloadForm.dnsPolicy')}>
            <Select defaultValue="ClusterFirst">
              <Option value="ClusterFirst">ClusterFirst</Option>
              <Option value="ClusterFirstWithHostNet">ClusterFirstWithHostNet</Option>
              <Option value="Default">Default</Option>
              <Option value="None">None</Option>
            </Select>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name={['dnsConfig', 'nameservers']} label={t('workloadForm.dnsServers')}>
            <Input placeholder="8.8.8.8, 8.8.4.4" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name={['dnsConfig', 'searches']} label={t('workloadForm.dnsSearchDomains')}>
            <Input placeholder="ns1.svc.cluster.local, svc.cluster.local" />
          </Form.Item>
        </Col>
      </Row>
    </Panel>
  );
};

// Other configuration panel
export const OtherConfigSection: React.FC<
  Omit<FormSectionProps, 'form' | 'workloadType' | 'isEdit'>
> = ({ t }) => {
  return (
    <Panel header={t('workloadForm.otherConfig')} key="other">
      <Row gutter={16}>
        <Col span={8}>
          <Form.Item name="terminationGracePeriodSeconds" label={t('workloadForm.terminationGracePeriod')}>
            <InputNumber min={0} style={{ width: '100%' }} placeholder="30" />
          </Form.Item>
        </Col>
        <Col span={8}>
          <Form.Item name="hostNetwork" label={t('workloadForm.hostNetwork')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Col>
      </Row>
    </Panel>
  );
};

export default DnsConfigSection;

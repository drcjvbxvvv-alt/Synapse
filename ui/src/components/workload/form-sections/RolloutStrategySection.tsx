import React from 'react';
import {
  Form,
  Input,
  InputNumber,
  Select,
  Switch,
  Button,
  Row,
  Col,
  Card,
  Collapse,
  Divider,
  Typography,
  Alert,
  Tooltip,
  Space,
} from 'antd';
import { PlusOutlined, MinusCircleOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import type { FormSectionProps } from './types';

const { Option } = Select;
const { Panel } = Collapse;
const { Text } = Typography;

// Canary strategy configuration sub-component
const CanaryConfig: React.FC<FormSectionProps> = ({ form: _form, t }) => (
  <>
    <Divider orientation="left">{t('workloadForm.canaryConfig')}</Divider>

    {/* Service configuration */}
    <Row gutter={16}>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'canary', 'stableService']}
          label={
            <Space>
              {t('workloadForm.stableService')}
              <Tooltip title={t('workloadForm.stableServiceTooltip')}>
                <QuestionCircleOutlined />
              </Tooltip>
            </Space>
          }
        >
          <Input placeholder="my-app-stable" />
        </Form.Item>
      </Col>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'canary', 'canaryService']}
          label={
            <Space>
              {t('workloadForm.canaryService')}
              <Tooltip title={t('workloadForm.canaryServiceTooltip')}>
                <QuestionCircleOutlined />
              </Tooltip>
            </Space>
          }
        >
          <Input placeholder="my-app-canary" />
        </Form.Item>
      </Col>
    </Row>

    {/* Basic configuration */}
    <Row gutter={16}>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'canary', 'maxSurge']}
          label={t('workloadForm.maxSurge')}
        >
          <Input placeholder={t('workloadForm.maxSurgePlaceholder')} />
        </Form.Item>
      </Col>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'canary', 'maxUnavailable']}
          label={t('workloadForm.maxUnavailable')}
        >
          <Input placeholder={t('workloadForm.maxUnavailablePlaceholder')} />
        </Form.Item>
      </Col>
    </Row>

    {/* Release steps */}
    <Divider orientation="left">
      <Space>
        {t('workloadForm.releaseSteps')}
        <Tooltip title={t('workloadForm.releaseStepsTooltip')}>
          <QuestionCircleOutlined />
        </Tooltip>
      </Space>
    </Divider>

    <Form.List name={['rolloutStrategy', 'canary', 'steps']}>
      {(fields, { add, remove }) => (
        <>
          {fields.map((field, index) => (
            <Card
              key={field.key}
              size="small"
              style={{ marginBottom: 8 }}
              title={t('workloadForm.stepIndex', { index: index + 1 })}
              extra={
                <Button
                  type="text"
                  danger
                  icon={<MinusCircleOutlined />}
                  onClick={() => remove(field.name)}
                />
              }
            >
              <Row gutter={16}>
                <Col span={8}>
                  <Form.Item
                    name={[field.name, 'setWeight']}
                    label={t('workloadForm.trafficWeight')}
                  >
                    <InputNumber
                      min={0}
                      max={100}
                      style={{ width: '100%' }}
                      placeholder={t('workloadForm.trafficWeightPlaceholder')}
                    />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item
                    name={[field.name, 'pause', 'duration']}
                    label={
                      <Space>
                        {t('workloadForm.pauseDuration')}
                        <Tooltip title={t('workloadForm.pauseDurationTooltip')}>
                          <QuestionCircleOutlined />
                        </Tooltip>
                      </Space>
                    }
                  >
                    <Input placeholder={t('workloadForm.pauseDurationPlaceholder')} />
                  </Form.Item>
                </Col>
              </Row>
            </Card>
          ))}
          <Button
            type="dashed"
            onClick={() => add({ setWeight: 20 })}
            icon={<PlusOutlined />}
            style={{ marginBottom: 16 }}
          >
            {t('workloadForm.addReleaseStep')}
          </Button>
          {fields.length === 0 && (
            <Alert
              message={t('workloadForm.addReleaseStepSuggestion')}
              description={t('workloadForm.addReleaseStepExample')}
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
            />
          )}
        </>
      )}
    </Form.List>

    {/* Traffic routing */}
    <Collapse ghost>
      <Panel header={t('workloadForm.trafficRouting')} key="trafficRouting">
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          {t('workloadForm.trafficRoutingDesc')}
        </Text>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name={['rolloutStrategy', 'canary', 'trafficRouting', 'nginx', 'stableIngress']}
              label={t('workloadForm.nginxIngressName')}
            >
              <Input placeholder="my-app-ingress" />
            </Form.Item>
          </Col>
        </Row>
      </Panel>
    </Collapse>
  </>
);

// BlueGreen strategy configuration sub-component
const BlueGreenConfig: React.FC<FormSectionProps> = ({ form, t }) => (
  <>
    <Divider orientation="left">{t('workloadForm.blueGreenConfig')}</Divider>

    {/* Service configuration */}
    <Row gutter={16}>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'blueGreen', 'activeService']}
          label={
            <Space>
              {t('workloadForm.activeService')}
              <Tooltip title={t('workloadForm.activeServiceTooltip')}>
                <QuestionCircleOutlined />
              </Tooltip>
            </Space>
          }
          rules={[{ required: true, message: t('workloadForm.activeServiceRequired') }]}
        >
          <Input placeholder="my-app-active" />
        </Form.Item>
      </Col>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'blueGreen', 'previewService']}
          label={
            <Space>
              {t('workloadForm.previewService')}
              <Tooltip title={t('workloadForm.previewServiceTooltip')}>
                <QuestionCircleOutlined />
              </Tooltip>
            </Space>
          }
        >
          <Input placeholder="my-app-preview" />
        </Form.Item>
      </Col>
    </Row>

    {/* Promotion configuration */}
    <Row gutter={16}>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'blueGreen', 'autoPromotionEnabled']}
          label={
            <Space>
              {t('workloadForm.autoPromotion')}
              <Tooltip title={t('workloadForm.autoPromotionTooltip')}>
                <QuestionCircleOutlined />
              </Tooltip>
            </Space>
          }
          valuePropName="checked"
        >
          <Switch />
        </Form.Item>
      </Col>
      <Form.Item noStyle shouldUpdate>
        {() => {
          const autoPromotion = form.getFieldValue([
            'rolloutStrategy',
            'blueGreen',
            'autoPromotionEnabled',
          ]);
          if (!autoPromotion) return null;
          return (
            <Col span={8}>
              <Form.Item
                name={['rolloutStrategy', 'blueGreen', 'autoPromotionSeconds']}
                label={t('workloadForm.autoPromotionDelay')}
              >
                <InputNumber min={0} style={{ width: '100%' }} placeholder="30" />
              </Form.Item>
            </Col>
          );
        }}
      </Form.Item>
    </Row>

    {/* Scale down configuration */}
    <Row gutter={16}>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'blueGreen', 'scaleDownDelaySeconds']}
          label={
            <Space>
              {t('workloadForm.scaleDownDelay')}
              <Tooltip title={t('workloadForm.scaleDownDelayTooltip')}>
                <QuestionCircleOutlined />
              </Tooltip>
            </Space>
          }
        >
          <InputNumber min={0} style={{ width: '100%' }} placeholder="30" />
        </Form.Item>
      </Col>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'blueGreen', 'scaleDownDelayRevisionLimit']}
          label={t('workloadForm.keepOldVersions')}
        >
          <InputNumber min={0} style={{ width: '100%' }} placeholder="2" />
        </Form.Item>
      </Col>
      <Col span={8}>
        <Form.Item
          name={['rolloutStrategy', 'blueGreen', 'previewReplicaCount']}
          label={t('workloadForm.previewReplicaCount')}
        >
          <InputNumber min={1} style={{ width: '100%' }} placeholder="1" />
        </Form.Item>
      </Col>
    </Row>
  </>
);

// General configuration for both strategies
const GeneralRolloutConfig: React.FC<FormSectionProps> = ({ t }) => (
  <>
    <Divider orientation="left">{t('workloadForm.generalConfig')}</Divider>
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
  </>
);

const RolloutStrategySection: React.FC<FormSectionProps> = ({ form, t, workloadType }) => {
  return (
    <Panel
      header={
        <Space>
          <span>{t('workloadForm.rolloutStrategy')}</span>
          <Tooltip title={t('workloadForm.rolloutStrategyTooltip')}>
            <QuestionCircleOutlined />
          </Tooltip>
        </Space>
      }
      key="rolloutStrategy"
    >
      <Alert
        message={t('workloadForm.rolloutStrategyDesc')}
        description={
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            <li>
              <strong>{t('workloadForm.canaryLabel')}</strong>: {t('workloadForm.canaryDesc')}
            </li>
            <li>
              <strong>{t('workloadForm.blueGreenLabel')}</strong>: {t('workloadForm.blueGreenDesc')}
            </li>
          </ul>
        }
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Row gutter={16}>
        <Col span={8}>
          <Form.Item
            name={['rolloutStrategy', 'type']}
            label={t('workloadForm.rolloutStrategyType')}
            rules={[{ required: true, message: t('workloadForm.rolloutStrategyTypeRequired') }]}
            initialValue="Canary"
          >
            <Select>
              <Option value="Canary">
                <Space>{t('workloadForm.canaryOption')}</Space>
              </Option>
              <Option value="BlueGreen">
                <Space>{t('workloadForm.blueGreenOption')}</Space>
              </Option>
            </Select>
          </Form.Item>
        </Col>
      </Row>

      {/* Canary configuration */}
      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) => prev?.rolloutStrategy?.type !== curr?.rolloutStrategy?.type}
      >
        {() => {
          const strategyType = form.getFieldValue(['rolloutStrategy', 'type']);
          if (strategyType !== 'Canary') return null;
          return <CanaryConfig form={form} t={t} workloadType={workloadType} />;
        }}
      </Form.Item>

      {/* BlueGreen configuration */}
      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) => prev?.rolloutStrategy?.type !== curr?.rolloutStrategy?.type}
      >
        {() => {
          const strategyType = form.getFieldValue(['rolloutStrategy', 'type']);
          if (strategyType !== 'BlueGreen') return null;
          return <BlueGreenConfig form={form} t={t} workloadType={workloadType} />;
        }}
      </Form.Item>

      <GeneralRolloutConfig form={form} t={t} workloadType={workloadType} />
    </Panel>
  );
};

export default RolloutStrategySection;

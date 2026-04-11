import React from 'react';
import {
  Card,
  Typography,
  Radio,
  Form,
  Input,
  Checkbox,
  InputNumber,
  Progress,
  List,
  Space,
  Row,
  Col,
  Alert,
  Tag,
  Badge,
  Statistic,
} from 'antd';
import type { RadioChangeEvent } from 'antd/es/radio';
import {
  PauseCircleOutlined,
  PlayCircleOutlined,
  ExportOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  LoadingOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { Node } from '../../../types';
import type {
  NodeOperationStatusItem,
  OperationResults,
  DrainOptions,
  ConfirmChecks,
} from '../hooks/useNodeOperations';

const { Title, Text } = Typography;

// ─── Step 0: Select operation type ────────────────────────────────────────

interface OperationSelectStepProps {
  operationType: string;
  onOperationTypeChange: (value: 'cordon' | 'uncordon' | 'drain') => void;
  operationReason: string;
  onReasonChange: (v: string) => void;
  executionStrategy: string;
  onExecutionStrategyChange: (v: string) => void;
  failureHandling: string;
  onFailureHandlingChange: (v: string) => void;
  selectedNodes: Node[];
  operationTitle: string;
  operationDescription: string;
}

export function OperationSelectStep({
  operationType,
  onOperationTypeChange,
  operationReason,
  onReasonChange,
  executionStrategy,
  onExecutionStrategyChange,
  failureHandling,
  onFailureHandlingChange,
  selectedNodes,
  operationTitle,
  operationDescription,
}: OperationSelectStepProps) {
  const { t } = useTranslation(['nodeOps', 'common']);

  const handleRadioChange = (e: RadioChangeEvent) => {
    onOperationTypeChange(e.target.value as 'cordon' | 'uncordon' | 'drain');
  };

  return (
    <Card title={t('nodeOps:operationType.title')}>
      <Radio.Group
        value={operationType}
        onChange={handleRadioChange}
        buttonStyle="solid"
        size="large"
        style={{ marginBottom: 24 }}
      >
        <Radio.Button value="cordon">
          <PauseCircleOutlined /> {t('nodeOps:operationType.cordon')}
        </Radio.Button>
        <Radio.Button value="uncordon">
          <PlayCircleOutlined /> {t('nodeOps:operationType.uncordon')}
        </Radio.Button>
        <Radio.Button value="drain">
          <ExportOutlined /> {t('nodeOps:operationType.drain')}
        </Radio.Button>
      </Radio.Group>

      <Alert
        message={operationTitle}
        description={operationDescription}
        type="info"
        showIcon
        style={{ marginBottom: 24 }}
      />

      <div>
        <Title level={5}>{t('nodeOps:common.selectedNodes', { count: selectedNodes.length })}</Title>
        <List
          size="small"
          bordered
          dataSource={selectedNodes}
          renderItem={(node: Node) => (
            <List.Item>
              <Space>
                {node.status === 'Ready' ? (
                  <Badge status="success" />
                ) : (
                  <Badge status="error" />
                )}
                <Text>{node.name}</Text>
                <Tag color="blue">{node.roles.join(', ')}</Tag>
                {node.taints?.some(taint => taint.effect === 'NoSchedule') && (
                  <Tag color="orange">{t('nodeOps:common.schedulingDisabled')}</Tag>
                )}
              </Space>
            </List.Item>
          )}
          style={{ marginBottom: 24 }}
        />
      </div>

      <Form layout="vertical">
        <Form.Item label={t('nodeOps:common.executionStrategy')}>
          <Radio.Group value={executionStrategy} onChange={e => onExecutionStrategyChange(e.target.value)}>
            <Radio value="parallel">{t('nodeOps:common.parallel')}</Radio>
            <Radio value="serial">{t('nodeOps:common.serial')}</Radio>
          </Radio.Group>
        </Form.Item>

        <Form.Item label={t('nodeOps:common.failureHandling')}>
          <Radio.Group value={failureHandling} onChange={e => onFailureHandlingChange(e.target.value)}>
            <Radio value="stop">{t('nodeOps:common.stopOnError')}</Radio>
            <Radio value="continue">{t('nodeOps:common.continueOnError')}</Radio>
          </Radio.Group>
        </Form.Item>

        <Form.Item label={t('nodeOps:common.operationReason')}>
          <Input.TextArea
            rows={3}
            placeholder={t('nodeOps:common.reasonPlaceholder')}
            value={operationReason}
            onChange={e => onReasonChange(e.target.value)}
          />
        </Form.Item>
      </Form>
    </Card>
  );
}

// ─── Step 1: Configure operation ──────────────────────────────────────────

interface OperationConfigStepProps {
  operationType: string;
  operationReason: string;
  onReasonChange: (v: string) => void;
  drainOptions: DrainOptions;
  onDrainOptionsChange: (v: string[]) => void;
  onDrainOptionNumberChange: (key: 'gracePeriodSeconds' | 'timeoutSeconds', v: number) => void;
  confirmChecks: ConfirmChecks;
  onConfirmChecksChange: (v: string[]) => void;
  selectedNodes: Node[];
}

export function OperationConfigStep({
  operationType,
  operationReason,
  onReasonChange,
  drainOptions,
  onDrainOptionsChange,
  onDrainOptionNumberChange,
  confirmChecks: _confirmChecks,
  onConfirmChecksChange,
  selectedNodes,
}: OperationConfigStepProps) {
  const { t } = useTranslation(['nodeOps', 'common']);

  if (operationType === 'cordon') {
    return (
      <Card title={t('nodeOps:cordon.title')}>
        <Alert
          message={t('nodeOps:cordon.instructions')}
          description={
            <ul>
              <li>{t('nodeOps:cordon.rule1')}</li>
              <li>{t('nodeOps:cordon.rule2')}</li>
              <li>{t('nodeOps:cordon.rule3')}</li>
              <li>{t('nodeOps:cordon.rule4')}</li>
            </ul>
          }
          type="info"
          showIcon
          style={{ marginBottom: 24 }}
        />
        <div style={{ marginBottom: 24 }}>
          <Title level={5}>{t('nodeOps:common.targetNodes')}</Title>
          <List
            size="small"
            bordered
            dataSource={selectedNodes}
            renderItem={(node: Node) => (
              <List.Item>
                <Space>
                  {node.taints?.some(taint => taint.effect === 'NoSchedule') ? (
                    <Badge status="warning" text={`${node.name} (${t('nodeOps:common.schedulingDisabled')})`} />
                  ) : (
                    <Badge status="success" text={`${node.name} (${t('nodeOps:common.schedulable')})`} />
                  )}
                </Space>
              </List.Item>
            )}
          />
        </div>
        <Form layout="vertical">
          <Form.Item label={t('nodeOps:common.operationReason')}>
            <Input.TextArea
              rows={3}
              placeholder={t('nodeOps:common.reasonPlaceholder')}
              value={operationReason}
              onChange={e => onReasonChange(e.target.value)}
            />
          </Form.Item>
          <Form.Item>
            <Checkbox.Group>
              <Checkbox value="send-notification">{t('nodeOps:common.sendNotification')}</Checkbox>
              <Checkbox value="record-log" defaultChecked disabled>{t('nodeOps:common.recordLog')}</Checkbox>
            </Checkbox.Group>
          </Form.Item>
        </Form>
      </Card>
    );
  }

  if (operationType === 'uncordon') {
    return (
      <Card title={t('nodeOps:uncordon.title')}>
        <Alert
          message={t('nodeOps:uncordon.instructions')}
          description={
            <ul>
              <li>{t('nodeOps:uncordon.rule1')}</li>
              <li>{t('nodeOps:uncordon.rule2')}</li>
              <li>{t('nodeOps:uncordon.rule3')}</li>
            </ul>
          }
          type="info"
          showIcon
          style={{ marginBottom: 24 }}
        />
        <div style={{ marginBottom: 24 }}>
          <Title level={5}>{t('nodeOps:common.targetNodes')}</Title>
          <List
            size="small"
            bordered
            dataSource={selectedNodes}
            renderItem={(node: Node) => (
              <List.Item>
                <Space>
                  {node.taints?.some(taint => taint.effect === 'NoSchedule') ? (
                    <Badge status="warning" text={`${node.name} (${t('nodeOps:common.schedulingDisabled')})`} />
                  ) : (
                    <Badge status="success" text={`${node.name} (${t('nodeOps:common.schedulable')})`} />
                  )}
                </Space>
              </List.Item>
            )}
          />
        </div>
        <Form layout="vertical">
          <Form.Item label={t('nodeOps:common.operationReason')}>
            <Input.TextArea
              rows={3}
              placeholder={t('nodeOps:common.reasonPlaceholderRestore')}
              value={operationReason}
              onChange={e => onReasonChange(e.target.value)}
            />
          </Form.Item>
          <Form.Item>
            <Checkbox.Group>
              <Checkbox value="check-status">{t('nodeOps:common.checkStatusAfter')}</Checkbox>
              <Checkbox value="send-notification">{t('nodeOps:common.sendRecoveryNotification')}</Checkbox>
              <Checkbox value="record-log" defaultChecked disabled>{t('nodeOps:common.recordLog')}</Checkbox>
            </Checkbox.Group>
          </Form.Item>
        </Form>
      </Card>
    );
  }

  // drain
  return (
    <Card title={t('nodeOps:drain.title')}>
      <Alert
        message={t('nodeOps:drain.warning')}
        description={t('nodeOps:drain.warningDesc')}
        type="warning"
        showIcon
        style={{ marginBottom: 24 }}
      />
      <div style={{ marginBottom: 24 }}>
        <Title level={5}>{t('nodeOps:common.targetNodes')}</Title>
        <List
          size="small"
          bordered
          dataSource={selectedNodes}
          renderItem={(node: Node) => (
            <List.Item>
              <Space>
                <Badge status="success" />
                <Text>{node.name}</Text>
                <Tag color="blue">{node.roles.join(', ')}</Tag>
                <Tag color="green">{t('nodeOps:drain.podCount', { count: node.podCount })}</Tag>
              </Space>
            </List.Item>
          )}
        />
      </div>
      <Form layout="vertical">
        <Form.Item label={t('nodeOps:drain.advancedOptions')}>
          <Checkbox.Group onChange={onDrainOptionsChange} defaultValue={['ignore-daemonsets']}>
            <Checkbox value="ignore-daemonsets">{t('nodeOps:drain.ignoreDaemonSets')}</Checkbox>
            <Checkbox value="delete-emptydir-data">{t('nodeOps:drain.deleteLocalData')}</Checkbox>
            <Checkbox value="force">{t('nodeOps:drain.forceDelete')}</Checkbox>
          </Checkbox.Group>
        </Form.Item>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t('nodeOps:drain.gracePeriod')}>
              <InputNumber
                min={0}
                max={300}
                value={drainOptions.gracePeriodSeconds}
                onChange={value => onDrainOptionNumberChange('gracePeriodSeconds', value as number)}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t('nodeOps:drain.timeout')}>
              <InputNumber
                min={60}
                max={1800}
                value={drainOptions.timeoutSeconds}
                onChange={value => onDrainOptionNumberChange('timeoutSeconds', value as number)}
              />
            </Form.Item>
          </Col>
        </Row>
        <Form.Item label={t('nodeOps:drain.confirmRisk')}>
          <Checkbox.Group onChange={onConfirmChecksChange}>
            <div style={{ marginBottom: 8 }}>
              <Checkbox value="service-interruption">{t('nodeOps:drain.riskServiceInterruption')}</Checkbox>
            </div>
            <div style={{ marginBottom: 8 }}>
              <Checkbox value="replica-confirmed">{t('nodeOps:drain.riskReplicaConfirmed')}</Checkbox>
            </div>
            <div>
              <Checkbox value="team-notified">{t('nodeOps:drain.riskTeamNotified')}</Checkbox>
            </div>
          </Checkbox.Group>
        </Form.Item>
        <Form.Item label={t('nodeOps:common.operationReason')}>
          <Input.TextArea
            rows={3}
            placeholder={t('nodeOps:common.reasonPlaceholderDrain')}
            value={operationReason}
            onChange={e => onReasonChange(e.target.value)}
          />
        </Form.Item>
      </Form>
    </Card>
  );
}

// ─── Step 2: Execution progress ───────────────────────────────────────────

function getStatusIcon(status: string) {
  switch (status) {
    case 'success': return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
    case 'failed':  return <CloseCircleOutlined style={{ color: '#f5222d' }} />;
    case 'running': return <LoadingOutlined style={{ color: '#1890ff' }} />;
    case 'waiting': return <InfoCircleOutlined style={{ color: '#faad14' }} />;
    default:        return <InfoCircleOutlined style={{ color: '#d9d9d9' }} />;
  }
}

interface ExecutionProgressStepProps {
  operationTitle: string;
  operationProgress: number;
  nodeOperationStatus: NodeOperationStatusItem[];
}

export function ExecutionProgressStep({
  operationTitle,
  operationProgress,
  nodeOperationStatus,
}: ExecutionProgressStepProps) {
  const { t } = useTranslation(['nodeOps', 'common']);

  return (
    <Card title={t('nodeOps:execution.executing', { title: operationTitle })}>
      <div style={{ marginBottom: 16 }}>
        <Text>{t('nodeOps:execution.overallProgress')}</Text>
        <Progress percent={operationProgress} status="active" />
      </div>

      <List
        itemLayout="horizontal"
        dataSource={nodeOperationStatus}
        renderItem={(item: NodeOperationStatusItem) => (
          <List.Item>
            <List.Item.Meta
              avatar={getStatusIcon(item.status)}
              title={item.nodeName}
              description={item.description}
            />
            <Progress
              percent={item.progress}
              size="small"
              status={
                item.status === 'failed' ? 'exception' :
                item.status === 'success' ? 'success' : 'active'
              }
            />
          </List.Item>
        )}
        style={{ marginBottom: 24 }}
      />

      <div>
        <Title level={5}>{t('nodeOps:execution.realtimeLog')}</Title>
        <div
          style={{
            height: 150,
            overflow: 'auto',
            padding: 16,
            backgroundColor: '#f5f5f5',
            borderRadius: 4,
          }}
        >
          {nodeOperationStatus.map((item, index) => (
            <div key={index}>
              <Text code>[{new Date().toLocaleTimeString()}] {item.description} - {item.nodeName}</Text>
            </div>
          ))}
        </div>
      </div>
    </Card>
  );
}

// ─── Step 3: Results ──────────────────────────────────────────────────────

interface OperationResultStepProps {
  operationTitle: string;
  operationResults: OperationResults;
}

export function OperationResultStep({
  operationTitle,
  operationResults,
}: OperationResultStepProps) {
  const { t } = useTranslation(['nodeOps', 'common']);

  return (
    <Card title={t('nodeOps:result.title')}>
      <Alert
        message={t('nodeOps:result.completed')}
        description={t('nodeOps:result.operationType', { title: operationTitle })}
        type="success"
        showIcon
        style={{ marginBottom: 24 }}
      />

      <div style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={8}>
            <Statistic
              title={t('nodeOps:result.startTime')}
              value={operationResults.startTime}
              formatter={value => <span>{value}</span>}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title={t('nodeOps:result.endTime')}
              value={operationResults.endTime}
              formatter={value => <span>{value}</span>}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title={t('nodeOps:result.totalDuration')}
              value={operationResults.duration}
            />
          </Col>
        </Row>
      </div>

      <div style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={8}>
            <Statistic
              title={t('nodeOps:result.success')}
              value={operationResults.success}
              valueStyle={{ color: '#3f8600' }}
              prefix={<CheckCircleOutlined />}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title={t('nodeOps:result.failed')}
              value={operationResults.failed}
              valueStyle={{ color: '#cf1322' }}
              prefix={<CloseCircleOutlined />}
            />
          </Col>
          <Col span={8}>
            <Statistic
              title={t('nodeOps:result.skipped')}
              value={operationResults.skipped}
              valueStyle={{ color: '#faad14' }}
              prefix={<ExclamationCircleOutlined />}
            />
          </Col>
        </Row>
      </div>

      <div>
        <Title level={5}>{t('nodeOps:result.detailResult')}</Title>
        <List
          size="small"
          bordered
          dataSource={operationResults.details}
          renderItem={(item: { nodeName: string; status: string; message: string }) => (
            <List.Item>
              <List.Item.Meta
                avatar={
                  item.status === 'success' ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> :
                  item.status === 'failed'  ? <CloseCircleOutlined style={{ color: '#f5222d' }} /> :
                  <ExclamationCircleOutlined style={{ color: '#faad14' }} />
                }
                title={item.nodeName}
                description={item.message}
              />
            </List.Item>
          )}
          style={{ marginBottom: 24 }}
        />
      </div>

      {operationResults.failed > 0 && (
        <Alert
          message={t('nodeOps:result.suggestion')}
          description={t('nodeOps:result.suggestionDesc')}
          type="warning"
          showIcon
          style={{ marginBottom: 24 }}
        />
      )}
    </Card>
  );
}

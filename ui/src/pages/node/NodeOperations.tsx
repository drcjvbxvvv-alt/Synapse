import React, { useState, useEffect } from 'react';
import {
  Card,
  Typography,
  Steps,
  Radio,
  Button,
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
  Modal,
  message,
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
import { nodeService } from '../../services/nodeService';
import type { Node } from '../../types';
import { useTranslation } from 'react-i18next';

const { Title, Text } = Typography;
const { Step } = Steps;

interface NodeOperationProps {
  clusterId: string;
  selectedNodes: Node[];
  onClose: () => void;
  onSuccess: () => void;
}

interface OperationResults {
  success: number;
  failed: number;
  skipped: number;
  details: Array<{
    nodeName: string;
    status: string;
    message: string;
  }>;
  startTime: string;
  endTime: string;
  duration: string;
}

const NodeOperations: React.FC<NodeOperationProps> = ({
  clusterId,
  selectedNodes,
  onClose,
  onSuccess,
}) => {
const { t } = useTranslation(['nodeOps', 'common']);
const [currentStep, setCurrentStep] = useState(0);
  const [operationType, setOperationType] = useState<'cordon' | 'uncordon' | 'drain'>('cordon');
  const [operationReason, setOperationReason] = useState('');
  const [drainOptions, setDrainOptions] = useState({
    ignoreDaemonSets: true,
    deleteLocalData: false,
    force: false,
    gracePeriodSeconds: 30,
    timeoutSeconds: 300,
  });
  const [confirmChecks, setConfirmChecks] = useState({
    serviceInterruption: false,
    replicaConfirmed: false,
    teamNotified: false,
  });
  const [operationProgress, setOperationProgress] = useState(0);
  interface NodeOperationStatusItem {
    nodeName: string;
    status: 'pending' | 'running' | 'success' | 'failed' | 'skipped' | 'waiting';
    message?: string;
    description?: string;
    progress?: number;
  }
  const [nodeOperationStatus, setNodeOperationStatus] = useState<NodeOperationStatusItem[]>([]);
  const [operationResults, setOperationResults] = useState<OperationResults>({
    success: 0,
    failed: 0,
    skipped: 0,
    details: [],
    startTime: '',
    endTime: '',
    duration: '',
  });
  const [executionStrategy, setExecutionStrategy] = useState('serial');
  const [failureHandling, setFailureHandling] = useState('stop');
  const [loading, setLoading] = useState(false);
  // const [form] = Form.useForm(); // 未使用

  // 初始化节点操作状态
  useEffect(() => {
    if (selectedNodes.length > 0) {
      const initialStatus: NodeOperationStatusItem[] = selectedNodes.map(node => ({
        nodeName: node.name,
        status: 'pending' as const,
        description: t('nodeOps:execution.waitingOperation'),
        progress: 0,
      }));
      setNodeOperationStatus(initialStatus);
    }
  }, [selectedNodes]);

  // 处理操作类型变更
  const handleOperationTypeChange = (e: RadioChangeEvent) => {
    setOperationType(e.target.value as 'cordon' | 'uncordon' | 'drain');
  };

  // 处理下一步
  const handleNext = () => {
    if (currentStep === 0) {
      // 验证操作类型选择
      if (!operationType) {
        message.error(t('nodeOps:operationType.selectRequired'));
        return;
      }
      setCurrentStep(currentStep + 1);
    } else if (currentStep === 1) {
      // 验证操作配置
      if (operationType === 'drain') {
        // 验证Drain操作的确认项
        if (!confirmChecks.serviceInterruption || !confirmChecks.replicaConfirmed || !confirmChecks.teamNotified) {
          message.error(t('nodeOps:drain.confirmAllRisks'));
          return;
        }
      }
      setCurrentStep(currentStep + 1);
    } else if (currentStep === 2) {
      // 开始执行操作
      executeOperation();
    }
  };

  // 处理上一步
  const handlePrevious = () => {
    setCurrentStep(currentStep - 1);
  };

  // 处理取消
  const handleCancel = () => {
    Modal.confirm({
      title: t('nodeOps:cancel.title'),
      content: t('nodeOps:cancel.content'),
      onOk: () => {
        onClose();
      },
    });
  };

  // 处理确认项变更
  const handleConfirmChecksChange = (checkedValues: string[]) => {
    setConfirmChecks({
      serviceInterruption: checkedValues.includes('service-interruption'),
      replicaConfirmed: checkedValues.includes('replica-confirmed'),
      teamNotified: checkedValues.includes('team-notified'),
    });
  };

  // 处理Drain选项变更
  const handleDrainOptionsChange = (checkedValues: string[]) => {
    setDrainOptions({
      ...drainOptions,
      ignoreDaemonSets: checkedValues.includes('ignore-daemonsets'),
      deleteLocalData: checkedValues.includes('delete-emptydir-data'),
      force: checkedValues.includes('force'),
    });
  };

  // 执行节点操作
  const executeOperation = async () => {
    setLoading(true);
    const startTime = new Date();
    setOperationResults({
      ...operationResults,
      startTime: startTime.toLocaleString(),
    });

    try {
      // 更新所有节点状态为等待中
      const updatedStatus: NodeOperationStatusItem[] = nodeOperationStatus.map(node => ({
        ...node,
        status: 'waiting' as const,
        description: t('nodeOps:execution.waitingOperation'),
      }));
      setNodeOperationStatus(updatedStatus);

      // 根据执行策略处理节点
      if (executionStrategy === 'parallel') {
        // 并行执行
        await executeParallel();
      } else {
        // 串行执行
        await executeSerial();
      }

      const endTime = new Date();
      const duration = Math.round((endTime.getTime() - startTime.getTime()) / 1000);
      const minutes = Math.floor(duration / 60);
      const seconds = duration % 60;

      setOperationResults({
        ...operationResults,
        endTime: endTime.toLocaleString(),
        duration: `${minutes}分${seconds}秒`,
      });

      setCurrentStep(currentStep + 1);
    } catch (error) {
      console.error('执行节点操作失败:', error);
      message.error(t('nodeOps:execution.executeFailed'));
    } finally {
      setLoading(false);
    }
  };

  // 并行执行操作
  const executeParallel = async () => {
    const operations = selectedNodes.map(async (node, index) => {
      try {
        // 更新当前节点状态为执行中
        updateNodeStatus(index, 'running', t('nodeOps:execution.executingOperation'), 10);

        // 根据操作类型执行不同的操作
        await executeNodeOperation(node.name, index);

        // 更新成功结果
        updateNodeStatus(index, 'success', t('nodeOps:execution.operationSuccess'), 100);
        setOperationResults((prev: OperationResults) => ({
          ...prev,
          success: prev.success + 1,
          details: [...prev.details, {
            nodeName: node.name,
            status: 'success',
            message: `${operationType}操作成功`,
          }],
        }));
      } catch (error) {
        // 更新失败结果
        updateNodeStatus(index, 'failed', `操作失败: ${error}`, 100);
        setOperationResults((prev: OperationResults) => ({
          ...prev,
          failed: prev.failed + 1,
          details: [...prev.details, {
            nodeName: node.name,
            status: 'failed',
            message: `${operationType}操作失败: ${error}`,
          }],
        }));

        // 如果设置了失败停止，则抛出异常中断后续操作
        if (failureHandling === 'stop') {
          throw error;
        }
      }
    });

    await Promise.all(operations);
    
    // 更新总体进度
    setOperationProgress(100);
  };

  // 串行执行操作
  const executeSerial = async () => {
    for (let i = 0; i < selectedNodes.length; i++) {
      const node = selectedNodes[i];
      try {
        // 更新当前节点状态为执行中
        updateNodeStatus(i, 'running', t('nodeOps:execution.executingOperation'), 10);

        // 根据操作类型执行不同的操作
        await executeNodeOperation(node.name, i);

        // 更新成功结果
        updateNodeStatus(i, 'success', t('nodeOps:execution.operationSuccess'), 100);
        setOperationResults((prev: OperationResults) => ({
          ...prev,
          success: prev.success + 1,
          details: [...prev.details, {
            nodeName: node.name,
            status: 'success',
            message: `${operationType}操作成功`,
          }],
        }));

        // 更新总体进度
        setOperationProgress(Math.round(((i + 1) / selectedNodes.length) * 100));

        // 如果不是最后一个节点，等待一段时间再执行下一个
        if (i < selectedNodes.length - 1) {
          await new Promise(resolve => setTimeout(resolve, 1000));
        }
      } catch (error) {
        // 更新失败结果
        updateNodeStatus(i, 'failed', `操作失败: ${error}`, 100);
        setOperationResults((prev: OperationResults) => ({
          ...prev,
          failed: prev.failed + 1,
          details: [...prev.details, {
            nodeName: node.name,
            status: 'failed',
            message: `${operationType}操作失败: ${error}`,
          }],
        }));

        // 如果设置了失败停止，则中断后续操作
        if (failureHandling === 'stop') {
          break;
        }
      }
    }
  };

  // 执行单个节点操作
  const executeNodeOperation = async (nodeName: string, index: number) => {
    switch (operationType) {
      case 'cordon':
        updateNodeStatus(index, 'running', t('nodeOps:execution.cordoning'), 30);
        await nodeService.cordonNode(clusterId, nodeName);
        updateNodeStatus(index, 'running', t('nodeOps:execution.cordonSuccess'), 90);
        break;
      case 'uncordon':
        updateNodeStatus(index, 'running', t('nodeOps:execution.uncordoning'), 30);
        await nodeService.uncordonNode(clusterId, nodeName);
        updateNodeStatus(index, 'running', t('nodeOps:execution.uncordonSuccess'), 90);
        break;
      case 'drain':
        updateNodeStatus(index, 'running', t('nodeOps:execution.draining'), 30);
        await nodeService.drainNode(clusterId, nodeName, {
          ignoreDaemonSets: drainOptions.ignoreDaemonSets,
          deleteLocalData: drainOptions.deleteLocalData,
          force: drainOptions.force,
          gracePeriodSeconds: drainOptions.gracePeriodSeconds,
        });
        updateNodeStatus(index, 'running', t('nodeOps:execution.drainSuccess'), 90);
        break;
      default:
        throw new Error(t('nodeOps:execution.unsupportedType'));
    }

    // 模拟操作完成
    await new Promise(resolve => setTimeout(resolve, 500));
  };

  // 更新节点状态
  const updateNodeStatus = (index: number, status: NodeOperationStatusItem['status'], description: string, progress: number) => {
    setNodeOperationStatus(prev => {
      const updated = [...prev];
      updated[index] = {
        ...updated[index],
        status,
        description,
        progress,
      };
      return updated;
    });
  };

  // 处理完成
  const handleFinish = () => {
    message.success(t('nodeOps:result.nodeOperationComplete'));
    onSuccess();
    onClose();
  };

  // 获取节点状态图标
  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'failed':
        return <CloseCircleOutlined style={{ color: '#f5222d' }} />;
      case 'running':
        return <LoadingOutlined style={{ color: '#1890ff' }} />;
      case 'waiting':
        return <InfoCircleOutlined style={{ color: '#faad14' }} />;
      default:
        return <InfoCircleOutlined style={{ color: '#d9d9d9' }} />;
    }
  };

  // 获取操作类型标题
  const getOperationTitle = () => {
    switch (operationType) {
      case 'cordon':
        return t('nodeOps:cordon.titleShort');
      case 'uncordon':
        return t('nodeOps:uncordon.titleShort');
      case 'drain':
        return t('nodeOps:drain.titleShort');
      default:
        return t('nodeOps:operationType.nodeOperation');
    }
  };

  // 获取操作类型描述
  const getOperationDescription = () => {
    switch (operationType) {
      case 'cordon':
        return t('nodeOps:cordon.description');
      case 'uncordon':
        return t('nodeOps:uncordon.description');
      case 'drain':
        return t('nodeOps:drain.description');
      default:
        return '';
    }
  };

  // 渲染步骤内容
  const renderStepContent = () => {
    switch (currentStep) {
      case 0:
        return renderSelectOperationType();
      case 1:
        return renderConfigureOperation();
      case 2:
        return renderConfirmOperation();
      case 3:
        return renderOperationResult();
      default:
        return null;
    }
  };

  // 渲染选择操作类型
  const renderSelectOperationType = () => {
    return (
      <Card title={t('nodeOps:operationType.title')}>
        <Radio.Group
          value={operationType}
          onChange={handleOperationTypeChange}
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
          message={getOperationTitle()}
          description={getOperationDescription()}
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
                  {node.taints?.some(t => t.effect === 'NoSchedule') && (
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
            <Radio.Group value={executionStrategy} onChange={e => setExecutionStrategy(e.target.value)}>
              <Radio value="parallel">{t('nodeOps:common.parallel')}</Radio>
              <Radio value="serial">{t('nodeOps:common.serial')}</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item label={t('nodeOps:common.failureHandling')}>
            <Radio.Group value={failureHandling} onChange={e => setFailureHandling(e.target.value)}>
              <Radio value="stop">{t('nodeOps:common.stopOnError')}</Radio>
              <Radio value="continue">{t('nodeOps:common.continueOnError')}</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item label={t('nodeOps:common.operationReason')}>
            <Input.TextArea
              rows={3}
              placeholder={t('nodeOps:common.reasonPlaceholder')}
              value={operationReason}
              onChange={e => setOperationReason(e.target.value)}
            />
          </Form.Item>
        </Form>
      </Card>
    );
  };

  // 渲染配置操作
  const renderConfigureOperation = () => {
    switch (operationType) {
      case 'cordon':
        return renderCordonConfig();
      case 'uncordon':
        return renderUncordonConfig();
      case 'drain':
        return renderDrainConfig();
      default:
        return null;
    }
  };

  // 渲染Cordon配置
  const renderCordonConfig = () => {
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
                  {node.taints?.some(t => t.effect === 'NoSchedule') ? (
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
              onChange={e => setOperationReason(e.target.value)}
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
  };

  // 渲染Uncordon配置
  const renderUncordonConfig = () => {
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
                  {node.taints?.some(t => t.effect === 'NoSchedule') ? (
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
              onChange={e => setOperationReason(e.target.value)}
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
  };

  // 渲染Drain配置
  const renderDrainConfig = () => {
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
            <Checkbox.Group onChange={handleDrainOptionsChange} defaultValue={['ignore-daemonsets']}>
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
                  onChange={value => setDrainOptions({ ...drainOptions, gracePeriodSeconds: value as number })}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label={t('nodeOps:drain.timeout')}>
                <InputNumber
                  min={60}
                  max={1800}
                  value={drainOptions.timeoutSeconds}
                  onChange={value => setDrainOptions({ ...drainOptions, timeoutSeconds: value as number })}
                />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item label={t('nodeOps:drain.confirmRisk')}>
            <Checkbox.Group onChange={handleConfirmChecksChange}>
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
              onChange={e => setOperationReason(e.target.value)}
            />
          </Form.Item>
        </Form>
      </Card>
    );
  };

  // 渲染确认操作
  const renderConfirmOperation = () => {
    return (
      <Card title={t('nodeOps:execution.executing', { title: getOperationTitle() })}>
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
              <Progress percent={item.progress} size="small" status={
                item.status === 'failed' ? 'exception' :
                item.status === 'success' ? 'success' : 'active'
              } />
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
  };

  // 渲染操作结果
  const renderOperationResult = () => {
    return (
      <Card title={t('nodeOps:result.title')}>
        <Alert
          message={t('nodeOps:result.completed')}
          description={t('nodeOps:result.operationType', { title: getOperationTitle() })}
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
                    item.status === 'failed' ? <CloseCircleOutlined style={{ color: '#f5222d' }} /> :
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

        <div style={{ marginTop: 24, textAlign: 'right' }}>
          <Button type="primary" onClick={handleFinish}>
            {t('nodeOps:buttons.finish')}
          </Button>
        </div>
      </Card>
    );
  };

  return (
    <div className="node-operations">
      <Card
        title={<Title level={4}>{t('nodeOps:title')}</Title>}
        extra={
          <Button onClick={handleCancel}>
            {t('nodeOps:buttons.cancel')}
          </Button>
        }
      >
        <Steps current={currentStep} style={{ marginBottom: 24 }}>
          <Step title={t('nodeOps:steps.selectOperation')} />
          <Step title={t('nodeOps:steps.configParams')} />
          <Step title={t('nodeOps:steps.executeOperation')} />
          <Step title={t('nodeOps:steps.complete')} />
        </Steps>

        {renderStepContent()}

        <div style={{ marginTop: 24, textAlign: 'right' }}>
          {currentStep > 0 && currentStep < 3 && (
            <Button style={{ marginRight: 8 }} onClick={handlePrevious}>
              {t('nodeOps:buttons.previous')}
            </Button>
          )}
          {currentStep < 2 && (
            <Button type="primary" onClick={handleNext}>
              {t('nodeOps:buttons.next')}
            </Button>
          )}
          {currentStep === 2 && (
            <Button type="primary" onClick={handleNext} loading={loading}>
              {t('nodeOps:buttons.startExecution')}
            </Button>
          )}
          {currentStep === 3 && (
            <Button type="primary" onClick={handleFinish}>
              {t('nodeOps:buttons.finish')}
            </Button>
          )}
        </div>
      </Card>
    </div>
  );
};

export default NodeOperations;

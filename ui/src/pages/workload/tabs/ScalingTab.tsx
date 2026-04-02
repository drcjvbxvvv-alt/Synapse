import React, { useState, useEffect, useCallback } from 'react';
import { App, Button, Card, Descriptions, Form, InputNumber, Modal, Popconfirm, Spin, Empty, Space, Tag } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { WorkloadService } from '../../../services/workloadService';
import { useTranslation } from 'react-i18next';

interface HPAInfo {
  name: string;
  namespace: string;
  targetKind: string;
  targetName: string;
  minReplicas: number;
  maxReplicas: number;
  currentReplicas: number;
  desiredReplicas: number;
  metrics?: Array<{
    type: string;
    resource?: {
      name: string;
      target: {
        type: string;
        averageUtilization?: number;
        averageValue?: string;
      };
    };
  }>;
  conditions?: Array<{
    type: string;
    status: string;
    reason?: string;
    message?: string;
  }>;
}

interface ScalingTabProps {
  clusterId: string;
  namespace: string;
  deploymentName?: string;
  rolloutName?: string;
  statefulSetName?: string;
  daemonSetName?: string;
  jobName?: string;
  cronJobName?: string;
}

interface HPAFormValues {
  minReplicas: number;
  maxReplicas: number;
  cpuTargetUtilization?: number;
  memTargetUtilization?: number;
}

const ScalingTab: React.FC<ScalingTabProps> = ({
  clusterId,
  namespace,
  deploymentName,
  rolloutName,
  statefulSetName,
  daemonSetName,
  jobName,
  cronJobName
}) => {
  const { t } = useTranslation(['workload', 'common']);
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [hpa, setHpa] = useState<HPAInfo | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<HPAFormValues>();

  const workloadName = deploymentName || rolloutName || statefulSetName || daemonSetName || jobName || cronJobName;
  const workloadType = deploymentName ? 'Deployment'
    : rolloutName ? 'Rollout'
    : statefulSetName ? 'StatefulSet'
    : daemonSetName ? 'DaemonSet'
    : jobName ? 'Job'
    : cronJobName ? 'CronJob'
    : '';

  const loadHPA = useCallback(async () => {
    if (!clusterId || !namespace || !workloadName || !workloadType) return;
    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloadHPA(clusterId, namespace, workloadType, workloadName);
      setHpa(response ? (response as HPAInfo) : null);
    } catch {
      setHpa(null);
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, workloadName, workloadType]);

  useEffect(() => { loadHPA(); }, [loadHPA]);

  const openCreate = () => {
    form.resetFields();
    form.setFieldsValue({ minReplicas: 1, maxReplicas: 10 });
    setModalOpen(true);
  };

  const openEdit = () => {
    if (!hpa) return;
    const cpuMetric = hpa.metrics?.find(m => m.resource?.name === 'cpu');
    const memMetric = hpa.metrics?.find(m => m.resource?.name === 'memory');
    form.setFieldsValue({
      minReplicas: hpa.minReplicas,
      maxReplicas: hpa.maxReplicas,
      cpuTargetUtilization: cpuMetric?.resource?.target.averageUtilization,
      memTargetUtilization: memMetric?.resource?.target.averageUtilization,
    });
    setModalOpen(true);
  };

  const handleSave = async () => {
    const values = await form.validateFields();
    if (!workloadName) return;
    setSaving(true);
    try {
      const payload = {
        name: hpa?.name ?? `${workloadName}-hpa`,
        namespace,
        targetKind: workloadType,
        targetName: workloadName,
        minReplicas: values.minReplicas,
        maxReplicas: values.maxReplicas,
        cpuTargetUtilization: values.cpuTargetUtilization,
        memTargetUtilization: values.memTargetUtilization,
      };
      if (hpa) {
        await WorkloadService.updateHPA(clusterId, namespace, hpa.name, payload);
        message.success('HPA 更新成功');
      } else {
        await WorkloadService.createHPA(clusterId, payload);
        message.success('HPA 建立成功');
      }
      setModalOpen(false);
      loadHPA();
    } catch (e) {
      message.error('操作失敗：' + String(e));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!hpa) return;
    try {
      await WorkloadService.deleteHPA(clusterId, namespace, hpa.name);
      message.success('HPA 刪除成功');
      setHpa(null);
    } catch (e) {
      message.error('刪除失敗：' + String(e));
    }
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '50px 0' }}>
        <Spin tip={t('scaling.loading')} />
      </div>
    );
  }

  const isHPASupported = ['Deployment', 'StatefulSet', 'Rollout'].includes(workloadType);

  return (
    <div>
      {!hpa ? (
        <Empty description={t('scaling.noHpa')} style={{ padding: '50px 0' }}>
          {isHPASupported && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              建立 HPA
            </Button>
          )}
        </Empty>
      ) : (
        <>
          <Card
            title={t('scaling.hpaConfig')}
            size="small"
            style={{ marginBottom: 16 }}
            extra={
              <Space>
                <Button size="small" icon={<EditOutlined />} onClick={openEdit}>編輯</Button>
                <Popconfirm title="確定刪除此 HPA？" onConfirm={handleDelete} okText="刪除" cancelText="取消" okButtonProps={{ danger: true }}>
                  <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
                </Popconfirm>
              </Space>
            }
          >
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label={t('scaling.hpaName')}>{hpa.name}</Descriptions.Item>
              <Descriptions.Item label={t('scaling.namespace')}>{hpa.namespace}</Descriptions.Item>
              <Descriptions.Item label={t('scaling.minReplicas')}>{hpa.minReplicas}</Descriptions.Item>
              <Descriptions.Item label={t('scaling.maxReplicas')}>{hpa.maxReplicas}</Descriptions.Item>
              <Descriptions.Item label={t('scaling.currentReplicas')}>{hpa.currentReplicas}</Descriptions.Item>
              <Descriptions.Item label={t('scaling.desiredReplicas')}>{hpa.desiredReplicas}</Descriptions.Item>
            </Descriptions>
          </Card>

          {hpa.metrics && hpa.metrics.length > 0 && (
            <Card title={t('scaling.metrics')} size="small" style={{ marginBottom: 16 }}>
              <Descriptions column={1} bordered size="small">
                {hpa.metrics.map((metric, index) => (
                  <Descriptions.Item key={index} label={`指標 ${index + 1}`}>
                    <div>
                      <div>{t('scaling.metricType')}: <Tag>{metric.type}</Tag></div>
                      {metric.resource && (
                        <>
                          <div>{t('scaling.resource')}: {metric.resource.name}</div>
                          <div>{t('scaling.targetType')}: {metric.resource.target.type}</div>
                          {metric.resource.target.averageUtilization !== undefined && (
                            <div>{t('scaling.avgUtilization')}: {metric.resource.target.averageUtilization}%</div>
                          )}
                          {metric.resource.target.averageValue && (
                            <div>{t('scaling.avgValue')}: {metric.resource.target.averageValue}</div>
                          )}
                        </>
                      )}
                    </div>
                  </Descriptions.Item>
                ))}
              </Descriptions>
            </Card>
          )}

          {hpa.conditions && hpa.conditions.length > 0 && (
            <Card title={t('scaling.scalingStatus')} size="small">
              <Descriptions column={1} bordered size="small">
                {hpa.conditions.map((condition, index) => (
                  <Descriptions.Item key={index} label={condition.type}>
                    <div>
                      <div>
                        {t('scaling.conditionStatus')}: <Tag color={condition.status === 'True' ? 'success' : 'default'}>
                          {condition.status}
                        </Tag>
                      </div>
                      {condition.reason && <div>{t('scaling.conditionReason')}: {condition.reason}</div>}
                      {condition.message && <div>{t('scaling.conditionMessage')}: {condition.message}</div>}
                    </div>
                  </Descriptions.Item>
                ))}
              </Descriptions>
            </Card>
          )}
        </>
      )}

      <Modal
        open={modalOpen}
        title={hpa ? '編輯 HPA' : '建立 HPA'}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        okText="儲存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="minReplicas"
            label="最小副本數"
            rules={[{ required: true, message: '請輸入最小副本數' }]}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="maxReplicas"
            label="最大副本數"
            rules={[{ required: true, message: '請輸入最大副本數' }]}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="cpuTargetUtilization" label="CPU 目標使用率 (%)">
            <InputNumber min={1} max={100} placeholder="例如：80（不填則不設 CPU 指標）" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="memTargetUtilization" label="記憶體目標使用率 (%)">
            <InputNumber min={1} max={100} placeholder="例如：80（不填則不設記憶體指標）" style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ScalingTab;

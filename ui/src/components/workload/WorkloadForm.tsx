import React from 'react';
import { Form, Button, Card, Collapse, Divider, Typography, Space } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import ContainerConfigForm from './ContainerConfigForm';
import SchedulingConfigForm from './SchedulingConfigForm';
import {
  BasicInfoSection,
  VolumeSection,
  ImagePullSecretsSection,
  DeploymentStrategySection,
  RolloutStrategySection,
  TolerationsSection,
  LabelsAnnotationsSection,
  DnsConfigSection,
  OtherConfigSection,
} from './form-sections';
import type { WorkloadFormData } from '../../types/workload';

const { Panel } = Collapse;
const { Text } = Typography;

interface WorkloadFormProps {
  workloadType: 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Rollout' | 'ArgoRollout' | 'Job' | 'CronJob';
  initialData?: Partial<WorkloadFormData>;
  namespaces: string[];
  imagePullSecretsList?: string[];
  onValuesChange?: (changedValues: Partial<WorkloadFormData>, allValues: WorkloadFormData) => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  form?: ReturnType<typeof Form.useForm<any>>[0];
  isEdit?: boolean;
}

const WorkloadForm: React.FC<WorkloadFormProps> = ({
  workloadType,
  initialData,
  namespaces,
  imagePullSecretsList = [],
  onValuesChange,
  form: externalForm,
  isEdit = false,
}) => {
  const { t } = useTranslation('components');
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [form] = Form.useForm<WorkloadFormData>(externalForm as any);
  const [initialized, setInitialized] = React.useState(false);

  // Set initial values
  React.useEffect(() => {
    if (initialData) {
      console.log('Setting edit mode data:', initialData);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      form.setFieldsValue(initialData as any);
      setInitialized(true);
    } else if (!initialized) {
      const defaultValues: Record<string, unknown> = {
        namespace: 'default',
        replicas: workloadType === 'DaemonSet' ? undefined : 1,
        containers: [
          {
            name: 'main',
            image: '',
            imagePullPolicy: 'IfNotPresent',
            resources: {
              requests: { cpu: '100m', memory: '128Mi' },
              limits: { cpu: '500m', memory: '512Mi' },
            },
          },
        ],
      };

      if (workloadType === 'Rollout') {
        defaultValues.rolloutStrategy = {
          type: 'Canary',
          canary: {
            steps: [
              { setWeight: 20, pause: { duration: '10m' } },
              { setWeight: 50, pause: { duration: '10m' } },
              { setWeight: 80, pause: { duration: '10m' } },
            ],
          },
        };
      }

      form.setFieldsValue(defaultValues);
      setInitialized(true);
    }
  }, [initialData, form, workloadType, initialized]);

  return (
    <Form form={form} layout="vertical" onValuesChange={onValuesChange}>
      {/* Basic Info */}
      <BasicInfoSection
        form={form}
        t={t}
        workloadType={workloadType}
        namespaces={namespaces}
        isEdit={isEdit}
      />

      {/* Container Configuration */}
      <Card
        title={
          <Space>
            <span>{t('workloadForm.containerConfigMulti')}</span>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {t('workloadForm.multiContainerHint')}
            </Text>
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        {/* Business containers */}
        <Form.List name="containers">
          {(fields, { add, remove }) => (
            <>
              {fields.map((field) => (
                <ContainerConfigForm
                  key={field.key}
                  field={field}
                  remove={remove}
                  isInitContainer={false}
                />
              ))}
              <Button
                type="dashed"
                onClick={() =>
                  add({
                    name: `container-${fields.length + 1}`,
                    image: '',
                    imagePullPolicy: 'IfNotPresent',
                  })
                }
                icon={<PlusOutlined />}
                style={{ marginBottom: 16 }}
              >
                {t('workloadForm.addContainer')}
              </Button>
            </>
          )}
        </Form.List>

        <Divider orientation="left">
          <Text type="secondary">{t('workloadForm.initContainerOptional')}</Text>
        </Divider>

        {/* Init containers */}
        <Form.List name="initContainers">
          {(fields, { add, remove }) => (
            <>
              {fields.map((field) => (
                <ContainerConfigForm
                  key={field.key}
                  field={field}
                  remove={remove}
                  isInitContainer={true}
                />
              ))}
              <Button
                type="dashed"
                onClick={() =>
                  add({
                    name: `init-${fields.length + 1}`,
                    image: '',
                  })
                }
                icon={<PlusOutlined />}
              >
                {t('workloadForm.addInitContainer')}
              </Button>
            </>
          )}
        </Form.List>
      </Card>

      {/* Volume Configuration */}
      <VolumeSection form={form} t={t} workloadType={workloadType} />

      {/* Image Pull Secrets */}
      <ImagePullSecretsSection form={form} t={t} imagePullSecretsList={imagePullSecretsList} />

      {/* Advanced Configuration */}
      <Card title={t('workloadForm.advancedConfig')} style={{ marginBottom: 16 }}>
        <Collapse defaultActiveKey={workloadType === 'Rollout' ? ['rolloutStrategy'] : []} ghost>
          {/* Deployment upgrade strategy */}
          {workloadType === 'Deployment' && (
            <DeploymentStrategySection form={form} t={t} workloadType={workloadType} />
          )}

          {/* Argo Rollout strategy */}
          {workloadType === 'Rollout' && (
            <RolloutStrategySection form={form} t={t} workloadType={workloadType} />
          )}

          {/* Scheduling config */}
          <Panel header={t('workloadForm.scheduling')} key="scheduling">
            <SchedulingConfigForm />
          </Panel>

          {/* Tolerations */}
          <TolerationsSection t={t} />

          {/* Labels and Annotations */}
          <LabelsAnnotationsSection t={t} />

          {/* DNS config */}
          <DnsConfigSection t={t} />

          {/* Other config */}
          <OtherConfigSection t={t} />
        </Collapse>
      </Card>
    </Form>
  );
};

export default WorkloadForm;
export type { WorkloadFormProps };

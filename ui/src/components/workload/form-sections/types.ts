import type { FormInstance } from 'antd';
import type { TFunction } from 'i18next';
import type { WorkloadFormData } from '../../../types/workload';

export type WorkloadType = 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Rollout' | 'ArgoRollout' | 'Job' | 'CronJob';

export interface FormSectionProps {
  form: FormInstance<WorkloadFormData>;
  t: TFunction;
  workloadType: WorkloadType;
  isEdit?: boolean;
}

export interface BasicInfoSectionProps extends FormSectionProps {
  namespaces: string[];
}

export interface ImagePullSecretsSectionProps extends Omit<FormSectionProps, 'workloadType' | 'isEdit'> {
  imagePullSecretsList: string[];
}

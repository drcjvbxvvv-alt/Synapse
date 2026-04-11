// Barrel re-export — preserves all existing import paths.
// Sub-files live in ./workloadService/

export type {
  WorkloadInfo,
  WorkloadListResponse,
  WorkloadDetailResponse,
  ScaleWorkloadRequest,
  YAMLApplyRequest,
} from './workloadService/types';

export { WorkloadService } from './workloadService/WorkloadService';
export { formDataToYAML } from './workloadService/formDataToYAML';

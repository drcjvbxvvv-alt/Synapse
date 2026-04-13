import api from '../utils/api';

const base = (clusterId: string) => `/clusters/${clusterId}`;

// ─── VolumeSnapshot ────────────────────────────────────────────────────────

export interface VolumeSnapshotInfo {
  name: string;
  namespace: string;
  sourcePVC: string;
  snapshotClassName: string;
  readyToUse: boolean;
  restoreSize: string;
  boundContentName: string;
  error: string;
  createdAt: string;
}

export interface VolumeSnapshotClassInfo {
  name: string;
  driver: string;
  deletionPolicy: string;
  isDefault: boolean;
  createdAt: string;
}

// ─── Velero ────────────────────────────────────────────────────────────────

export interface VeleroBackupInfo {
  name: string;
  namespace: string;
  phase: string;        // New | InProgress | Completed | Failed | Deleting
  includedNamespaces: string[];
  storageLocation: string;
  ttl: string;
  startTimestamp: string;
  completionTimestamp: string;
  expiration: string;
  progress?: { totalItems?: number; itemsBackedUp?: number };
  createdAt: string;
}

export interface VeleroRestoreInfo {
  name: string;
  namespace: string;
  backupName: string;
  phase: string;        // New | InProgress | Completed | Failed | PartiallyFailed
  warnings: number;
  errors: number;
  createdAt: string;
}

export interface VeleroScheduleInfo {
  name: string;
  namespace: string;
  schedule: string;
  paused: boolean;
  phase: string;
  lastBackup: string;
  storageLocation: string;
  ttl: string;
  createdAt: string;
}

export interface CreateSnapshotRequest {
  name: string;
  namespace: string;
  pvcName: string;
  snapshotClassName?: string;
}

export interface TriggerRestoreRequest {
  backupName: string;
  restoreName?: string;
  veleroNS?: string;
  includedNamespaces?: string[];
  excludedNamespaces?: string[];
}

export interface CreateScheduleRequest {
  name: string;
  schedule: string;
  veleroNS?: string;
  paused?: boolean;
  includedNamespaces?: string[];
  storageLocation?: string;
  ttl?: string;
}

// ─── API ───────────────────────────────────────────────────────────────────

export const snapshotService = {
  // VolumeSnapshot CRD status
  checkVolumeSnapshotCRD: (clusterId: string) =>
    api.get<{ installed: boolean }>(`${base(clusterId)}/volume-snapshots/status`),

  listVolumeSnapshots: (clusterId: string, namespace?: string, pvc?: string) =>
    api.get<{ items: VolumeSnapshotInfo[]; total: number }>(
      `${base(clusterId)}/volume-snapshots`,
      { params: { namespace, pvc } },
    ),

  createVolumeSnapshot: (clusterId: string, req: CreateSnapshotRequest) =>
    api.post<VolumeSnapshotInfo>(`${base(clusterId)}/volume-snapshots`, req),

  deleteVolumeSnapshot: (clusterId: string, namespace: string, name: string) =>
    api.delete(`${base(clusterId)}/volume-snapshots/${namespace}/${name}`),

  listVolumeSnapshotClasses: (clusterId: string) =>
    api.get<{ items: VolumeSnapshotClassInfo[]; total: number }>(
      `${base(clusterId)}/volume-snapshot-classes`,
    ),

  // Velero
  checkVelero: (clusterId: string, veleroNS = 'velero') =>
    api.get<{ installed: boolean; namespace: string }>(
      `${base(clusterId)}/velero/status`,
      { params: { veleroNS } },
    ),

  listBackups: (clusterId: string, veleroNS = 'velero') =>
    api.get<{ items: VeleroBackupInfo[]; total: number }>(
      `${base(clusterId)}/velero/backups`,
      { params: { veleroNS } },
    ),

  listRestores: (clusterId: string, veleroNS = 'velero') =>
    api.get<{ items: VeleroRestoreInfo[]; total: number }>(
      `${base(clusterId)}/velero/restores`,
      { params: { veleroNS } },
    ),

  triggerRestore: (clusterId: string, req: TriggerRestoreRequest) =>
    api.post<VeleroRestoreInfo>(`${base(clusterId)}/velero/restores`, req),

  listSchedules: (clusterId: string, veleroNS = 'velero') =>
    api.get<{ items: VeleroScheduleInfo[]; total: number }>(
      `${base(clusterId)}/velero/schedules`,
      { params: { veleroNS } },
    ),

  createSchedule: (clusterId: string, req: CreateScheduleRequest) =>
    api.post<VeleroScheduleInfo>(`${base(clusterId)}/velero/schedules`, req),

  deleteSchedule: (clusterId: string, name: string, veleroNS = 'velero') =>
    api.delete(`${base(clusterId)}/velero/schedules/${name}`, { params: { veleroNS } }),
};

// ─── Helpers ───────────────────────────────────────────────────────────────

export function backupPhaseColor(phase: string): string {
  switch (phase) {
    case 'Completed': return 'success';
    case 'Failed': return 'error';
    case 'InProgress': return 'processing';
    case 'Deleting': return 'warning';
    default: return 'default';
  }
}

export function restorePhaseColor(phase: string): string {
  switch (phase) {
    case 'Completed': return 'success';
    case 'Failed': return 'error';
    case 'PartiallyFailed': return 'warning';
    case 'InProgress': return 'processing';
    default: return 'default';
  }
}

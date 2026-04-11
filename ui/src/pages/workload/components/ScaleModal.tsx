import React from 'react';
import { Modal, Space, InputNumber, Typography } from 'antd';
import {
  AppstoreOutlined,
  DeploymentUnitOutlined,
  ArrowRightOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import type { WorkloadInfo } from '../../../services/workloadService';
import type { TFunction } from 'i18next';
import type { WorkloadType } from '../hooks/useWorkloadTab';

const { Text } = Typography;

interface ScaleModalProps {
  open: boolean;
  workload: WorkloadInfo | null;
  replicas: number;
  workloadType: WorkloadType;
  onReplicasChange: (value: number) => void;
  onOk: () => void;
  onCancel: () => void;
  t: TFunction;
}

export const ScaleModal: React.FC<ScaleModalProps> = ({
  open,
  workload,
  replicas,
  workloadType,
  onReplicasChange,
  onOk,
  onCancel,
  t,
}) => {
  if (!workload) return null;

  return (
    <Modal
      title={
        <Space>
          <DeploymentUnitOutlined style={{ color: '#3b82f6' }} />
          <span>{t('scale.title', { type: workloadType })}</span>
        </Space>
      }
      open={open}
      onOk={onOk}
      onCancel={onCancel}
      okText={t('common:actions.confirm')}
      cancelText={t('common:actions.cancel')}
      width={420}
    >
      <div style={{ paddingTop: 8 }}>
        {/* Workload Info Card */}
        <div style={{
          background: 'linear-gradient(135deg, #eff6ff 0%, #f8fafc 100%)',
          border: '1px solid #dbeafe',
          borderRadius: 10,
          padding: '14px 16px',
          marginBottom: 24,
        }}>
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <AppstoreOutlined style={{ color: '#3b82f6', fontSize: 14 }} />
              <Text type="secondary" style={{ fontSize: 12, width: 80, flexShrink: 0 }}>{workloadType}</Text>
              <Text strong style={{ fontSize: 13, color: '#111827' }}>{workload.name}</Text>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <DeploymentUnitOutlined style={{ color: '#6b7280', fontSize: 14 }} />
              <Text type="secondary" style={{ fontSize: 12, width: 80, flexShrink: 0 }}>{t('scale.namespace')}</Text>
              <Text style={{ fontSize: 13 }}>{workload.namespace}</Text>
            </div>
          </Space>
        </div>

        {/* Replicas Adjustment */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 28,
          padding: '8px 0 20px',
        }}>
          {/* Current Replicas */}
          <div style={{ textAlign: 'center', minWidth: 80 }}>
            <div style={{ fontSize: 11, color: '#9ca3af', marginBottom: 6, fontWeight: 500, letterSpacing: '0.03em' }}>
              {t('scale.currentReplicas')}
            </div>
            <div style={{
              fontSize: 42,
              fontWeight: 800,
              color: '#374151',
              lineHeight: 1,
              fontFamily: '"SF Mono", "Fira Code", monospace',
            }}>
              {workload.replicas || 0}
            </div>
          </div>

          {/* Arrow */}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4, paddingTop: 20 }}>
            <ArrowRightOutlined style={{
              fontSize: 20,
              color: replicas !== (workload.replicas || 0) ? '#3b82f6' : '#d1d5db',
              transition: 'color 0.2s',
            }} />
          </div>

          {/* Target Replicas */}
          <div style={{ textAlign: 'center', minWidth: 80 }}>
            <div style={{ fontSize: 11, color: '#9ca3af', marginBottom: 6, fontWeight: 500, letterSpacing: '0.03em' }}>
              {t('scale.targetReplicas')}
            </div>
            <InputNumber
              min={0}
              max={100}
              value={replicas}
              onChange={(value) => onReplicasChange(value ?? 0)}
              controls
              className="scale-replicas-input"
              style={{
                width: 96,
                borderRadius: 8,
                borderColor: replicas !== (workload.replicas || 0) ? '#3b82f6' : undefined,
              }}
            />
          </div>
        </div>

        {/* Zero Replicas Warning */}
        {replicas === 0 && (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            background: '#fffbeb',
            border: '1px solid #fde68a',
            borderRadius: 8,
            padding: '10px 14px',
            marginTop: 4,
          }}>
            <ExclamationCircleOutlined style={{ color: '#f59e0b', flexShrink: 0 }} />
            <Text style={{ fontSize: 13, color: '#92400e' }}>
              副本數設為 0 將暫停所有 Pod，服務將停止對外提供。
            </Text>
          </div>
        )}
      </div>
    </Modal>
  );
};

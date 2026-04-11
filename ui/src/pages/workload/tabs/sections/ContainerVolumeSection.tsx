import React from 'react';
import { Card, Descriptions, Tag, Empty } from 'antd';
import type { TFunction } from 'i18next';
import type { ContainerInfo, VolumeConfig } from '../containerTypes';

interface ContainerVolumeSectionProps {
  container: ContainerInfo;
  volumes?: VolumeConfig[];
  t: TFunction;
}

function renderVolumeType(volume: VolumeConfig | null | undefined, t: TFunction): React.ReactNode {
  if (!volume) return <Tag>{t('container.volume.unknown')}</Tag>;
  if (volume.configMap) return <Tag color="blue">ConfigMap: {volume.configMap.name}</Tag>;
  if (volume.secret) return <Tag color="orange">Secret: {volume.secret.secretName}</Tag>;
  if (volume.emptyDir) return <Tag color="green">EmptyDir</Tag>;
  if (volume.hostPath) return <Tag color="red">HostPath: {volume.hostPath.path}</Tag>;
  if (volume.persistentVolumeClaim) return <Tag color="purple">PVC: {volume.persistentVolumeClaim.claimName}</Tag>;
  if (volume.downwardAPI) return <Tag color="cyan">DownwardAPI</Tag>;
  if (volume.projected) return <Tag color="geekblue">Projected</Tag>;
  if (volume.nfs) return <Tag color="volcano">NFS: {volume.nfs.server}:{volume.nfs.path}</Tag>;
  return <Tag>{t('container.volume.other')}</Tag>;
}

export const ContainerVolumeSection: React.FC<ContainerVolumeSectionProps> = ({ container, volumes, t }) => {
  if (!container.volumeMounts || container.volumeMounts.length === 0) {
    return <Empty description={t('container.volume.noMounts')} />;
  }

  const getVolumeInfo = (volumeName: string): VolumeConfig | undefined => {
    return volumes?.find(v => v.name === volumeName);
  };

  return (
    <div>
      <Card title={t('container.volume.mounts')} size="small" style={{ marginBottom: 16 }}>
        {container.volumeMounts.map((mount, index) => {
          const volumeInfo = getVolumeInfo(mount.name);
          return (
            <Card
              key={index}
              size="small"
              title={mount.name}
              extra={renderVolumeType(volumeInfo, t)}
              style={{ marginBottom: 8 }}
              type="inner"
            >
              <Descriptions column={2} size="small">
                <Descriptions.Item label={t('container.volume.mountPath')}>
                  <code>{mount.mountPath}</code>
                </Descriptions.Item>
                <Descriptions.Item label={t('container.volume.readOnly')}>
                  <Tag color={mount.readOnly ? 'orange' : 'green'}>
                    {mount.readOnly ? t('container.volume.yes') : t('container.volume.no')}
                  </Tag>
                </Descriptions.Item>
                {mount.subPath && (
                  <Descriptions.Item label={t('container.volume.subPath')}>{mount.subPath}</Descriptions.Item>
                )}
                {mount.subPathExpr && (
                  <Descriptions.Item label={t('container.volume.subPathExpr')}>{mount.subPathExpr}</Descriptions.Item>
                )}
              </Descriptions>
            </Card>
          );
        })}
      </Card>

      {volumes && volumes.length > 0 && (
        <Card title={t('container.volume.definitions')} size="small">
          {volumes.map((volume, index) => (
            <Card
              key={index}
              size="small"
              title={volume.name}
              extra={renderVolumeType(volume, t)}
              style={{ marginBottom: 8 }}
              type="inner"
            >
              <Descriptions column={1} size="small">
                {volume.configMap && (
                  <>
                    <Descriptions.Item label={t('container.volume.configMapName')}>{volume.configMap.name}</Descriptions.Item>
                    {volume.configMap.defaultMode !== undefined && (
                      <Descriptions.Item label={t('container.volume.defaultMode')}>{volume.configMap.defaultMode.toString(8)}</Descriptions.Item>
                    )}
                    {volume.configMap.items && (
                      <Descriptions.Item label={t('container.volume.specifiedKeys')}>
                        {volume.configMap.items.map((item: { key: string; path: string; mode?: number }, idx: number) => (
                          <Tag key={idx}>{item.key} → {item.path}</Tag>
                        ))}
                      </Descriptions.Item>
                    )}
                  </>
                )}
                {volume.secret && (
                  <>
                    <Descriptions.Item label={t('container.volume.secretName')}>{volume.secret.secretName}</Descriptions.Item>
                    {volume.secret.defaultMode !== undefined && (
                      <Descriptions.Item label={t('container.volume.defaultMode')}>{volume.secret.defaultMode.toString(8)}</Descriptions.Item>
                    )}
                  </>
                )}
                {volume.emptyDir && (
                  <>
                    {volume.emptyDir.medium && (
                      <Descriptions.Item label={t('container.volume.storageMedium')}>{volume.emptyDir.medium}</Descriptions.Item>
                    )}
                    {volume.emptyDir.sizeLimit && (
                      <Descriptions.Item label={t('container.volume.sizeLimit')}>{volume.emptyDir.sizeLimit}</Descriptions.Item>
                    )}
                  </>
                )}
                {volume.hostPath && (
                  <>
                    <Descriptions.Item label={t('container.volume.hostPath')}>{volume.hostPath.path}</Descriptions.Item>
                    {volume.hostPath.type && (
                      <Descriptions.Item label={t('container.volume.type')}>{volume.hostPath.type}</Descriptions.Item>
                    )}
                  </>
                )}
                {volume.persistentVolumeClaim && (
                  <>
                    <Descriptions.Item label={t('container.volume.pvcName')}>{volume.persistentVolumeClaim.claimName}</Descriptions.Item>
                    {volume.persistentVolumeClaim.readOnly !== undefined && (
                      <Descriptions.Item label={t('container.volume.readOnly')}>
                        {volume.persistentVolumeClaim.readOnly ? t('container.volume.yes') : t('container.volume.no')}
                      </Descriptions.Item>
                    )}
                  </>
                )}
                {volume.nfs && (
                  <>
                    <Descriptions.Item label={t('container.volume.nfsServer')}>{volume.nfs.server}</Descriptions.Item>
                    <Descriptions.Item label={t('container.volume.nfsPath')}>{volume.nfs.path}</Descriptions.Item>
                    {volume.nfs.readOnly !== undefined && (
                      <Descriptions.Item label={t('container.volume.readOnly')}>
                        {volume.nfs.readOnly ? t('container.volume.yes') : t('container.volume.no')}
                      </Descriptions.Item>
                    )}
                  </>
                )}
              </Descriptions>
            </Card>
          ))}
        </Card>
      )}
    </div>
  );
};

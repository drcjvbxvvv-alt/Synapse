import EmptyState from '@/components/EmptyState';
import React from 'react';
import {
  Modal, Space, Select, Input, Spin, Checkbox, Tag, Alert, Typography,
} from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { List as VirtualList } from 'react-window';
import { useTranslation } from 'react-i18next';
import type { LogPodInfo, LogStreamTarget } from '../../../services/logService';

const { Text } = Typography;

interface PodSelectorModalProps {
  visible: boolean;
  onOk: () => void;
  onCancel: () => void;
  namespaces: string[];
  selectedNamespace: string;
  setSelectedNamespace: (v: string) => void;
  pods: LogPodInfo[];
  podsLoading: boolean;
  selectedPods: LogStreamTarget[];
  setSelectedPods: (pods: LogStreamTarget[]) => void;
  podSearchKeyword: string;
  setPodSearchKeyword: (v: string) => void;
  filteredPods: LogPodInfo[];
  selectedPodsSet: Set<string>;
  fetchPods: (namespace?: string) => void;
}

export const PodSelectorModal: React.FC<PodSelectorModalProps> = ({
  visible,
  onOk,
  onCancel,
  namespaces,
  selectedNamespace,
  setSelectedNamespace,
  pods,
  podsLoading,
  selectedPods,
  setSelectedPods,
  podSearchKeyword,
  setPodSearchKeyword,
  filteredPods,
  selectedPodsSet,
  fetchPods,
}) => {
  const { t } = useTranslation(['logs', 'common']);

  const handleTogglePod = (pod: LogPodInfo) => {
    const isSelected = selectedPodsSet.has(`${pod.namespace}/${pod.name}`);
    if (isSelected) {
      setSelectedPods(selectedPods.filter((p) => !(p.namespace === pod.namespace && p.pod === pod.name)));
    } else {
      setSelectedPods([
        ...selectedPods,
        { namespace: pod.namespace, pod: pod.name, container: pod.containers[0] },
      ]);
    }
  };

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      const newTargets = filteredPods
        .filter((p) => !selectedPodsSet.has(`${p.namespace}/${p.name}`))
        .map((p) => ({
          namespace: p.namespace,
          pod: p.name,
          container: p.containers[0],
        }));
      setSelectedPods([...selectedPods, ...newTargets]);
    } else {
      const filteredSet = new Set(filteredPods.map((p) => `${p.namespace}/${p.name}`));
      setSelectedPods(selectedPods.filter((p) => !filteredSet.has(`${p.namespace}/${p.pod}`)));
    }
  };

  return (
    <Modal
      title={t('logs:center.selectPod')}
      open={visible}
      onOk={onOk}
      onCancel={() => {
        onCancel();
        setPodSearchKeyword('');
      }}
      width={700}
      okText={t('logs:center.confirmAdd')}
      cancelText={t('common:actions.cancel')}
    >
      <Space direction="vertical" style={{ width: '100%' }}>
        <Select
          placeholder={t('logs:center.selectNamespace')}
          style={{ width: '100%' }}
          value={selectedNamespace || undefined}
          onChange={(v) => {
            setSelectedNamespace(v);
            setPodSearchKeyword('');
            fetchPods(v);
          }}
          showSearch
          options={namespaces.map((ns) => ({ label: ns, value: ns }))}
        />

        {pods.length > 0 && (
          <Input
            placeholder={t('logs:center.searchPodPlaceholder')}
            prefix={<SearchOutlined />}
            allowClear
            value={podSearchKeyword}
            onChange={(e) => setPodSearchKeyword(e.target.value)}
            style={{ marginBottom: 8 }}
          />
        )}

        <Spin spinning={podsLoading}>
          {pods.length === 0 ? (
            <EmptyState description={t('logs:center.selectNamespaceFirst')} />
          ) : filteredPods.length === 0 ? (
            <EmptyState description={t('logs:center.noMatchingPods')} />
          ) : (
            <>
              <div style={{ marginBottom: 8, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ color: '#888' }}>
                  {t('logs:center.totalPods', { total: pods.length })}
                  {podSearchKeyword && `, ${t('logs:center.matchingPods', { filtered: filteredPods.length })}`}
                  {t('logs:center.selectedPods', { count: selectedPods.length })}
                </span>
                <Checkbox
                  indeterminate={
                    filteredPods.some((p) => selectedPodsSet.has(`${p.namespace}/${p.name}`)) &&
                    !filteredPods.every((p) => selectedPodsSet.has(`${p.namespace}/${p.name}`))
                  }
                  checked={
                    filteredPods.length > 0 &&
                    filteredPods.every((p) => selectedPodsSet.has(`${p.namespace}/${p.name}`))
                  }
                  onChange={(e) => handleSelectAll(e.target.checked)}
                >
                  {podSearchKeyword ? t('logs:center.selectAllMatching') : t('logs:center.selectAll')}
                </Checkbox>
              </div>

              <div style={{ border: '1px solid #d9d9d9', borderRadius: 8, overflow: 'hidden' }}>
                <VirtualList<{ pods: LogPodInfo[]; selectedPodsSet: Set<string>; onToggle: (pod: LogPodInfo) => void }>
                  style={{ height: 360 }}
                  rowCount={filteredPods.length}
                  rowHeight={60}
                  rowProps={{
                    pods: filteredPods,
                    selectedPodsSet,
                    onToggle: handleTogglePod,
                  }}
                  rowComponent={({ index, style, pods: podList, selectedPodsSet: selSet, onToggle }) => {
                    const pod = podList[index];
                    if (!pod) return <div style={style} />;
                    const isSelected = selSet.has(`${pod.namespace}/${pod.name}`);
                    return (
                      <div
                        style={{
                          ...style,
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                          padding: '8px 12px',
                          borderBottom: '1px solid #f0f0f0',
                          cursor: 'pointer',
                          backgroundColor: isSelected ? '#e6f7ff' : '#fff',
                          boxSizing: 'border-box',
                        }}
                        onClick={() => onToggle(pod)}
                      >
                        <div style={{ flex: 1, minWidth: 0, overflow: 'hidden' }}>
                          <Text strong style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {pod.name}
                          </Text>
                          <Text type="secondary" style={{ fontSize: 12, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {t('logs:center.container')}: {pod.containers.join(', ')}
                          </Text>
                        </div>
                        <Space style={{ flexShrink: 0 }}>
                          <Tag color={pod.status === 'Running' ? 'green' : 'orange'}>{pod.status}</Tag>
                          <Checkbox checked={isSelected} />
                        </Space>
                      </div>
                    );
                  }}
                />
              </div>
            </>
          )}
        </Spin>

        {selectedPods.length > 0 && (
          <Alert
            message={t('logs:center.selectedPodsCount', { count: selectedPods.length })}
            type="info"
            showIcon
          />
        )}
      </Space>
    </Modal>
  );
};

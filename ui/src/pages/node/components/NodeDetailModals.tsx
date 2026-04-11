import React from 'react';
import {
  Modal,
  Button,
  Input,
  Select,
  Typography,
  Divider,
  Space,
  Tag,
  Alert,
  Checkbox,
  InputNumber,
} from 'antd';
import type { Node } from '../../../types';
import type { TFunction } from 'i18next';

const { Text } = Typography;

interface DrainOptions {
  ignoreDaemonSets: boolean;
  deleteLocalData: boolean;
  force: boolean;
  gracePeriodSeconds: number;
}

interface LabelModalProps {
  visible: boolean;
  onCancel: () => void;
  node: Node | null;
  newLabelKey: string;
  newLabelValue: string;
  setNewLabelKey: (key: string) => void;
  setNewLabelValue: (value: string) => void;
  handleAddLabel: () => void;
  handleRemoveLabel: (key: string) => void;
  t: TFunction;
  tc: TFunction;
}

export const LabelModal: React.FC<LabelModalProps> = ({
  visible,
  onCancel,
  node,
  newLabelKey,
  newLabelValue,
  setNewLabelKey,
  setNewLabelValue,
  handleAddLabel,
  handleRemoveLabel,
  t,
  tc,
}) => (
  <Modal
    title={t('detail.editLabels')}
    open={visible}
    onCancel={onCancel}
    footer={[
      <Button key="cancel" onClick={onCancel}>
        {tc('actions.cancel')}
      </Button>,
      <Button key="submit" type="primary" onClick={handleAddLabel}>
        {t('detail.addLabel')}
      </Button>,
    ]}
  >
    <div style={{ marginBottom: 16 }}>
      <Text>{t('detail.systemLabelsReadOnly')}:</Text>
      <div style={{ marginTop: 8 }}>
        <Space wrap>
          {node?.labels && Array.isArray(node.labels) && node.labels
            .filter(label => label.key.startsWith('kubernetes.io/') || label.key.startsWith('node.kubernetes.io/') || label.key.startsWith('topology.kubernetes.io/'))
            .map((label: { key: string; value: string }, index: number) => (
              <Tag key={index} color="blue">
                {label.key}={label.value}
              </Tag>
            ))}
        </Space>
      </div>
    </div>

    <div style={{ marginBottom: 16 }}>
      <Text>{t('detail.customLabels')}:</Text>
      <div style={{ marginTop: 8 }}>
        <Space wrap>
          {node?.labels && Array.isArray(node.labels) && node.labels
            .filter(label => !label.key.startsWith('kubernetes.io/') && !label.key.startsWith('node.kubernetes.io/') && !label.key.startsWith('topology.kubernetes.io/'))
            .map((label: { key: string; value: string }, index: number) => (
              <Tag
                key={index}
                closable
                onClose={() => handleRemoveLabel(label.key)}
              >
                {label.key}={label.value}
              </Tag>
            ))}
        </Space>
      </div>
    </div>

    <Divider />

    <div>
      <Text>{t('detail.addNewLabel')}:</Text>
      <div style={{ marginTop: 8 }}>
        <Input
          placeholder={t('detail.taintKey')}
          value={newLabelKey}
          onChange={(e) => setNewLabelKey(e.target.value)}
          style={{ width: '45%', marginRight: '5%' }}
        />
        <Input
          placeholder={t('detail.taintValue')}
          value={newLabelValue}
          onChange={(e) => setNewLabelValue(e.target.value)}
          style={{ width: '45%' }}
        />
      </div>
    </div>
  </Modal>
);

interface TaintModalProps {
  visible: boolean;
  onCancel: () => void;
  newTaintKey: string;
  newTaintValue: string;
  newTaintEffect: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute';
  setNewTaintKey: (key: string) => void;
  setNewTaintValue: (value: string) => void;
  setNewTaintEffect: (effect: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute') => void;
  handleAddTaint: () => void;
  t: TFunction;
  tc: TFunction;
}

export const TaintModal: React.FC<TaintModalProps> = ({
  visible,
  onCancel,
  newTaintKey,
  newTaintValue,
  newTaintEffect,
  setNewTaintKey,
  setNewTaintValue,
  setNewTaintEffect,
  handleAddTaint,
  t,
  tc,
}) => (
  <Modal
    title={t('detail.manageTaints')}
    open={visible}
    onCancel={onCancel}
    footer={[
      <Button key="cancel" onClick={onCancel}>
        {tc('actions.cancel')}
      </Button>,
      <Button key="submit" type="primary" onClick={handleAddTaint}>
        {t('detail.addTaint')}
      </Button>,
    ]}
  >
    <div style={{ marginBottom: 16 }}>
      <Text>{t('detail.addNewTaint')}:</Text>
      <div style={{ marginTop: 8 }}>
        <Input
          placeholder={t('detail.taintKey')}
          value={newTaintKey}
          onChange={(e) => setNewTaintKey(e.target.value)}
          style={{ width: '100%', marginBottom: 8 }}
        />
        <Input
          placeholder={t('detail.taintValueOptional')}
          value={newTaintValue}
          onChange={(e) => setNewTaintValue(e.target.value)}
          style={{ width: '100%', marginBottom: 8 }}
        />
        <Select
          placeholder={t('detail.taintEffect')}
          value={newTaintEffect}
          onChange={(value) => setNewTaintEffect(value)}
          style={{ width: '100%' }}
        >
          <Select.Option value="NoSchedule">{t('detail.noScheduleOption')}</Select.Option>
          <Select.Option value="PreferNoSchedule">{t('detail.preferNoScheduleOption')}</Select.Option>
          <Select.Option value="NoExecute">{t('detail.noExecuteOption')}</Select.Option>
        </Select>
      </div>
    </div>

    <Divider />

    <div>
      <Text>{t('detail.taintEffectInfo')}:</Text>
      <ul>
        <li><Text strong>NoSchedule:</Text> {t('detail.noScheduleDesc')}</li>
        <li><Text strong>PreferNoSchedule:</Text> {t('detail.preferNoScheduleDesc')}</li>
        <li><Text strong>NoExecute:</Text> {t('detail.noExecuteDesc')}</li>
      </ul>
    </div>
  </Modal>
);

interface DrainModalProps {
  visible: boolean;
  onCancel: () => void;
  nodeName: string;
  drainOptions: DrainOptions;
  setDrainOptions: (options: DrainOptions) => void;
  handleDrain: () => void;
  t: TFunction;
  tc: TFunction;
}

export const DrainModal: React.FC<DrainModalProps> = ({
  visible,
  onCancel,
  nodeName,
  drainOptions,
  setDrainOptions,
  handleDrain,
  t,
  tc,
}) => (
  <Modal
    title={t('actions.drain')}
    open={visible}
    onCancel={onCancel}
    footer={[
      <Button key="cancel" onClick={onCancel}>
        {tc('actions.cancel')}
      </Button>,
      <Button key="submit" type="primary" danger onClick={handleDrain}>
        {tc('actions.confirm')}
      </Button>,
    ]}
  >
    <Alert
      message={t('detail.drainWarningTitle')}
      description={t('detail.drainWarningDesc', { name: nodeName })}
      type="warning"
      showIcon
      style={{ marginBottom: 16 }}
    />

    <div style={{ marginBottom: 16 }}>
      <Text strong>{t('detail.advancedOptions')}:</Text>
      <div style={{ marginTop: 8 }}>
        <Checkbox
          checked={drainOptions.ignoreDaemonSets}
          onChange={(e) => setDrainOptions({ ...drainOptions, ignoreDaemonSets: e.target.checked })}
        >
          {t('detail.ignoreDaemonSets')}
        </Checkbox>
      </div>
      <div style={{ marginTop: 8 }}>
        <Checkbox
          checked={drainOptions.deleteLocalData}
          onChange={(e) => setDrainOptions({ ...drainOptions, deleteLocalData: e.target.checked })}
        >
          {t('detail.deleteLocalData')}
        </Checkbox>
      </div>
      <div style={{ marginTop: 8 }}>
        <Checkbox
          checked={drainOptions.force}
          onChange={(e) => setDrainOptions({ ...drainOptions, force: e.target.checked })}
        >
          {t('detail.forceDelete')}
        </Checkbox>
      </div>
      <div style={{ marginTop: 8 }}>
        <Text>{t('detail.gracePeriod')}:</Text>
        <InputNumber
          min={0}
          max={3600}
          value={drainOptions.gracePeriodSeconds}
          onChange={(value) => setDrainOptions({ ...drainOptions, gracePeriodSeconds: value as number })}
          style={{ marginLeft: 8 }}
        />
        <Text style={{ marginLeft: 8 }}>{tc('time.seconds')}</Text>
      </div>
    </div>
  </Modal>
);

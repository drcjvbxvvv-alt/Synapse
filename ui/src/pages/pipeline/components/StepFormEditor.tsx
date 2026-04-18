/**
 * StepFormEditor — 視覺化 Pipeline Step 編輯器
 *
 * 以卡片式呈現每個 Step，與 Run 詳情頁的流程卡片視覺語言一致。
 * 支援：新增/刪除/排序/展開編輯，雙向同步 JSON。
 */
import React, { useState } from 'react';
import {
  Button,
  Card,
  Collapse,
  Flex,
  Input,
  Select,
  Space,
  Tag,
  Tooltip,
  Typography,
  theme,
  Popconfirm,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  ContainerOutlined,
  RocketOutlined,
  SecurityScanOutlined,
  CloudUploadOutlined,
  CodeOutlined,
  BranchesOutlined,
  ThunderboltOutlined,
  CheckCircleOutlined,
  BellOutlined,
  PauseCircleOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

import StepConfigForm from './StepConfigForm';

const { Text } = Typography;

// ─── Step type metadata ────────────────────────────────────────────────────

interface StepTypeMeta {
  labelKey: string;       // i18n key under pipeline:stepType.*
  descriptionKey: string; // i18n key under pipeline:stepType.*
  icon: React.ReactNode;
  group: string;
}

const STEP_TYPE_MAP: Record<string, StepTypeMeta> = {
  'build-image':        { labelKey: 'buildImage',       descriptionKey: 'buildImageDesc',       icon: <ContainerOutlined />,      group: 'ci' },
  'build-jar':          { labelKey: 'buildJar',         descriptionKey: 'buildJarDesc',         icon: <CodeOutlined />,           group: 'ci' },
  'push-image':         { labelKey: 'pushImage',        descriptionKey: 'pushImageDesc',        icon: <CloudUploadOutlined />,    group: 'ci' },
  'trivy-scan':         { labelKey: 'trivyScan',        descriptionKey: 'trivyScanDesc',        icon: <SecurityScanOutlined />,   group: 'security' },
  'deploy':             { labelKey: 'deploy',           descriptionKey: 'deployDesc',           icon: <RocketOutlined />,         group: 'cd' },
  'deploy-helm':        { labelKey: 'deployHelm',       descriptionKey: 'deployHelmDesc',       icon: <RocketOutlined />,         group: 'cd' },
  'deploy-argocd-sync': { labelKey: 'deployArgocd',     descriptionKey: 'deployArgocdDesc',     icon: <RocketOutlined />,         group: 'cd' },
  'deploy-rollout':     { labelKey: 'deployRollout',    descriptionKey: 'deployRolloutDesc',    icon: <RocketOutlined />,         group: 'cd' },
  'approval':           { labelKey: 'approval',         descriptionKey: 'approvalDesc',         icon: <PauseCircleOutlined />,    group: 'gate' },
  'shell':              { labelKey: 'shell',            descriptionKey: 'shellDesc',            icon: <CodeOutlined />,           group: 'script' },
  'run-script':         { labelKey: 'runScript',        descriptionKey: 'runScriptDesc',        icon: <CodeOutlined />,           group: 'script' },
  'smoke-test':         { labelKey: 'smokeTest',        descriptionKey: 'smokeTestDesc',        icon: <ThunderboltOutlined />,    group: 'test' },
  'notify':             { labelKey: 'notify',           descriptionKey: 'notifyDesc',           icon: <BellOutlined />,           group: 'notify' },
  'gitops-sync':        { labelKey: 'gitopsSync',       descriptionKey: 'gitopsSyncDesc',       icon: <BranchesOutlined />,       group: 'cd' },
  'custom':             { labelKey: 'custom',           descriptionKey: 'customDesc',           icon: <SettingOutlined />,        group: 'script' },
};

const GROUP_KEYS: Record<string, string> = {
  ci:       'groupCi',
  security: 'groupSecurity',
  cd:       'groupCd',
  gate:     'groupGate',
  script:   'groupScript',
  test:     'groupTest',
  notify:   'groupNotify',
};

// ─── Step data model ───────────────────────────────────────────────────────

export interface StepFormData {
  name: string;
  type: string;
  image?: string;
  command?: string;
  depends_on?: string[];
  config: Record<string, unknown>;
  max_retries?: number;
}

interface StepFormEditorProps {
  steps: StepFormData[];
  onChange: (steps: StepFormData[]) => void;
  registryOptions?: { label: string; value: string }[];
}

// ─── Component ─────────────────────────────────────────────────────────────

const StepFormEditor: React.FC<StepFormEditorProps> = ({ steps, onChange, registryOptions }) => {
  const { token } = theme.useToken();
  const { t } = useTranslation(['pipeline', 'common']);
  const [activeKeys, setActiveKeys] = useState<string[]>([]);

  const getTypeLabel = (type: string): string => {
    const meta = STEP_TYPE_MAP[type];
    return meta ? t(`pipeline:stepType.${meta.labelKey}`) : type;
  };

  const getTypeDesc = (type: string): string => {
    const meta = STEP_TYPE_MAP[type];
    return meta ? t(`pipeline:stepType.${meta.descriptionKey}`) : '';
  };

  const getTypeIcon = (type: string): React.ReactNode => {
    return STEP_TYPE_MAP[type]?.icon ?? <CodeOutlined />;
  };

  // ─── Actions ──────────────────────────────────────────────────────────

  const addStep = (type: string) => {
    const newStep: StepFormData = {
      name: `${type}-${steps.length + 1}`,
      type,
      depends_on: steps.length > 0 ? [steps[steps.length - 1].name] : [],
      config: {},
    };
    const newSteps = [...steps, newStep];
    onChange(newSteps);
    setActiveKeys([`step-${newSteps.length - 1}`]);
  };

  const removeStep = (index: number) => {
    const removedName = steps[index].name;
    const newSteps = steps.filter((_, i) => i !== index).map((s) => ({
      ...s,
      depends_on: (s.depends_on ?? []).filter((d) => d !== removedName),
    }));
    onChange(newSteps);
  };

  const moveStep = (index: number, direction: -1 | 1) => {
    const target = index + direction;
    if (target < 0 || target >= steps.length) return;
    const newSteps = [...steps];
    [newSteps[index], newSteps[target]] = [newSteps[target], newSteps[index]];
    onChange(newSteps);
  };

  const updateStep = (index: number, patch: Partial<StepFormData>) => {
    const newSteps = [...steps];
    newSteps[index] = { ...newSteps[index], ...patch };
    onChange(newSteps);
  };

  // ─── Type selector options ────────────────────────────────────────────

  // Group options for Select
  const groupedOptions = Object.entries(GROUP_KEYS).map(([group, key]) => ({
    label: t(`pipeline:stepType.${key}`),
    options: Object.entries(STEP_TYPE_MAP)
      .filter(([, meta]) => meta.group === group)
      .map(([type, meta]) => ({
        label: (
          <Flex align="center" gap={8}>
            {meta.icon}
            <span>{t(`pipeline:stepType.${meta.labelKey}`)}</span>
            <Text type="secondary" style={{ fontSize: 12 }}>— {t(`pipeline:stepType.${meta.descriptionKey}`)}</Text>
          </Flex>
        ),
        value: type,
      })),
  }));

  // ─── Dependency options for current step ──────────────────────────────

  const getDependencyOptions = (currentIndex: number) =>
    steps
      .filter((_, i) => i !== currentIndex)
      .map((s) => ({ label: s.name, value: s.name }));

  // ─── Render ───────────────────────────────────────────────────────────

  const collapseItems = steps.map((step, index) => {
    return {
      key: `step-${index}`,
      label: (
        <Flex align="center" justify="space-between" style={{ width: '100%', paddingRight: 8 }}>
          <Flex align="center" gap={token.marginSM}>
            <span style={{ fontSize: 18, color: token.colorPrimary }}>{getTypeIcon(step.type)}</span>
            <Text strong>{step.name}</Text>
            <Tag>{getTypeLabel(step.type)}</Tag>
            {(step.depends_on ?? []).length > 0 && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                ← {(step.depends_on ?? []).join(', ')}
              </Text>
            )}
          </Flex>
          <Space size={0} onClick={(e) => e.stopPropagation()}>
            <Tooltip title={t('common:actions.moveUp', { defaultValue: '上移' })}>
              <Button
                type="text"
                size="small"
                icon={<ArrowUpOutlined />}
                disabled={index === 0}
                onClick={() => moveStep(index, -1)}
              />
            </Tooltip>
            <Tooltip title={t('common:actions.moveDown', { defaultValue: '下移' })}>
              <Button
                type="text"
                size="small"
                icon={<ArrowDownOutlined />}
                disabled={index === steps.length - 1}
                onClick={() => moveStep(index, 1)}
              />
            </Tooltip>
            <Popconfirm
              title={t('common:confirm.deleteTitle')}
              onConfirm={() => removeStep(index)}
              okText={t('common:actions.delete')}
              okButtonProps={{ danger: true }}
              cancelText={t('common:actions.cancel')}
            >
              <Tooltip title={t('common:actions.delete')}>
                <Button type="text" size="small" danger icon={<DeleteOutlined />} />
              </Tooltip>
            </Popconfirm>
          </Space>
        </Flex>
      ),
      children: (
        <div style={{ padding: `0 ${token.paddingSM}px` }}>
          {/* 基本欄位 */}
          <Flex gap={token.marginMD} style={{ marginBottom: token.marginMD }}>
            <div style={{ flex: 1 }}>
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
                {t('pipeline:stepForm.name', { defaultValue: '步驟名稱' })}
              </Text>
              <Input
                value={step.name}
                onChange={(e) => updateStep(index, { name: e.target.value })}
                placeholder="e.g. build, deploy-prod"
              />
            </div>
            <div style={{ flex: 1 }}>
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
                {t('pipeline:stepForm.type', { defaultValue: '步驟類型' })}
              </Text>
              <Select
                value={step.type}
                onChange={(type) => updateStep(index, { type, config: {} })}
                options={groupedOptions}
                style={{ width: '100%' }}
                popupMatchSelectWidth={false}
              />
            </div>
            <div style={{ flex: 1 }}>
              <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
                {t('pipeline:stepForm.dependsOn', { defaultValue: '依賴步驟' })}
              </Text>
              <Select
                mode="multiple"
                value={step.depends_on ?? []}
                onChange={(deps) => updateStep(index, { depends_on: deps })}
                options={getDependencyOptions(index)}
                style={{ width: '100%' }}
                placeholder={t('pipeline:stepForm.noDeps', { defaultValue: '（無依賴）' })}
                allowClear
              />
            </div>
          </Flex>

          {/* 類型特定欄位 */}
          <StepConfigForm
            stepType={step.type}
            config={step.config}
            command={step.command}
            image={step.image}
            onChange={(patch) => updateStep(index, patch)}
            registryOptions={registryOptions}
          />
        </div>
      ),
    };
  });

  return (
    <div>
      {steps.length > 0 && (
        <Collapse
          activeKey={activeKeys}
          onChange={(keys) => setActiveKeys(keys as string[])}
          items={collapseItems}
          style={{ marginBottom: token.marginMD }}
        />
      )}

      {/* 連接箭頭視覺提示（在卡片之間） */}

      {/* 新增步驟 */}
      <Select
        placeholder={
          <Space>
            <PlusOutlined />
            {t('pipeline:stepForm.addStep', { defaultValue: '新增步驟' })}
          </Space>
        }
        options={groupedOptions}
        onSelect={(type) => addStep(type as string)}
        value={undefined}
        style={{ width: '100%' }}
        popupMatchSelectWidth={false}
        size="large"
      />
    </div>
  );
};

export default StepFormEditor;

import React, { useEffect, useState, useCallback } from 'react';
import {
  Card,
  Button,
  Space,
  message,
  Tag,
  Alert,
  Modal,
  Typography,
  App,
  Segmented,
} from 'antd';
import {
  ArrowLeftOutlined,
  SaveOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  DiffOutlined,
  FormOutlined,
  CodeOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { secretService, type SecretDetail } from '../../services/configService';
import { ResourceService } from '../../services/resourceService';
import MonacoEditor, { DiffEditor } from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { parseApiError, showApiError } from '../../utils/api';
import PageSkeleton from '../../components/PageSkeleton';
import SecretEditFormPanel from './components/SecretEditFormPanel';

const { Text, Title } = Typography;

const SecretEdit: React.FC = () => {
  const navigate = useNavigate();
  const { modal } = App.useApp();
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();

const { t } = useTranslation(['config', 'common']);
const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [secret, setSecret] = useState<SecretDetail | null>(null);
  const [yamlContent, setYamlContent] = useState('');
  const [originalYaml, setOriginalYaml] = useState('');

  // 編輯模式
  const [editMode, setEditMode] = useState<'form' | 'yaml'>('form');

  // 表單狀態
  const [formLabels, setFormLabels] = useState<{ key: string; value: string }[]>([]);
  const [formAnnotations, setFormAnnotations] = useState<{ key: string; value: string }[]>([]);
  const [formData, setFormData] = useState<{ key: string; value: string }[]>([]);

  // 預檢相關狀態
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{
    success: boolean;
    message: string;
  } | null>(null);

  // Diff 對比相關狀態
  const [diffModalVisible, setDiffModalVisible] = useState(false);
  const [pendingYaml, setPendingYaml] = useState<string>('');

  // 從表單狀態同步到 YAML
  const syncFormToYaml = useCallback(() => {
    const labelsObj = Object.fromEntries(formLabels.filter(l => l.key).map(l => [l.key, l.value]));
    const annotationsObj = Object.fromEntries(formAnnotations.filter(l => l.key).map(l => [l.key, l.value]));
    const dataObj = Object.fromEntries(formData.filter(d => d.key).map(d => [d.key, d.value]));
    const yamlObj = {
      apiVersion: 'v1',
      kind: 'Secret',
      type: secret?.type,
      metadata: { name, namespace, labels: labelsObj, annotations: annotationsObj },
      data: dataObj,
    };
    const yamlStr = YAML.stringify(yamlObj);
    setYamlContent(yamlStr);
    return yamlStr;
  }, [formLabels, formAnnotations, formData, name, namespace, secret?.type]);

  // 載入 Secret 詳情
  const loadSecret = useCallback(async () => {
    if (!clusterId || !namespace || !name) return;
    setLoading(true);
    try {
      const data = await secretService.getSecret(
        Number(clusterId),
        namespace,
        name
      );
      setSecret(data);

      // 生成 YAML 內容
      const yamlObj = {
        apiVersion: 'v1',
        kind: 'Secret',
        type: data.type,
        metadata: {
          name: data.name,
          namespace: data.namespace,
          labels: data.labels || {},
          annotations: data.annotations || {},
        },
        data: data.data || {},
      };
      const yamlStr = YAML.stringify(yamlObj);
      setYamlContent(yamlStr);
      setOriginalYaml(yamlStr);

      // 初始化表單狀態
      setFormLabels(Object.entries(data.labels || {}).map(([key, value]) => ({ key, value })));
      setFormAnnotations(Object.entries(data.annotations || {}).map(([key, value]) => ({ key, value })));
      setFormData(Object.entries(data.data || {}).map(([key, value]) => ({ key, value: String(value) })));
    } catch (error: unknown) {
      showApiError(error, t('config:edit.messages.loadSecretError'));
      navigate(`/clusters/${clusterId}/configs`);
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, name, navigate, t]);

  useEffect(() => {
    loadSecret();
  }, [loadSecret]);

  // 模式切換
  const handleModeChange = (mode: string) => {
    if (mode === 'yaml') {
      syncFormToYaml();
    } else {
      try {
        const parsed = YAML.parse(yamlContent) as unknown;
        const parsedObj = parsed as { metadata?: { labels?: Record<string, unknown>; annotations?: Record<string, unknown> }; data?: Record<string, unknown> };
        setFormLabels(Object.entries(parsedObj?.metadata?.labels || {}).map(([k, v]) => ({ key: k, value: String(v) })));
        setFormAnnotations(Object.entries(parsedObj?.metadata?.annotations || {}).map(([k, v]) => ({ key: k, value: String(v) })));
        setFormData(Object.entries(parsedObj?.data || {}).map(([k, v]) => ({ key: k, value: String(v) })));
      } catch {
        // intentional: parse error handled by YAML validator
      }
    }
    setEditMode(mode as 'form' | 'yaml');
  };

  // 預檢（Dry Run）
  const handleDryRun = async () => {
    if (!clusterId) return;

    let currentYaml = yamlContent;
    if (editMode === 'form') {
      currentYaml = syncFormToYaml();
    }

    // 驗證 YAML 格式
    try {
      YAML.parse(currentYaml);
    } catch (error) {
      message.error(t('config:edit.messages.yamlFormatError', { error: error instanceof Error ? error.message : t('config:edit.messages.unknownError') }));
      return;
    }

    setDryRunning(true);
    setDryRunResult(null);

    try {
      await ResourceService.applyYAML(clusterId, 'Secret', currentYaml, true);
      setDryRunResult({
        success: true,
        message: t('config:edit.messages.dryRunPassed'),
      });
    } catch (error: unknown) {
      setDryRunResult({
        success: false,
        message: parseApiError(error) || t('config:edit.messages.dryRunFailed'),
      });
    } finally {
      setDryRunning(false);
    }
  };

  // 確認 Diff 後提交
  const handleConfirmDiff = async () => {
    if (!clusterId || !pendingYaml) return;

    setSubmitting(true);
    try {
      await ResourceService.applyYAML(clusterId, 'Secret', pendingYaml, false);
      message.success(t('config:edit.messages.secretUpdateSuccess'));
      setDiffModalVisible(false);
      navigate(`/clusters/${clusterId}/configs/secret/${namespace}/${name}`);
    } catch (error: unknown) {
      showApiError(error, t('config:edit.messages.updateError'));
    } finally {
      setSubmitting(false);
    }
  };

  // 提交 - 先預檢，再展示 diff
  const handleSubmit = async () => {
    if (!clusterId || !namespace || !name) return;

    let currentYaml = yamlContent;
    if (editMode === 'form') {
      const labelsObj = Object.fromEntries(formLabels.filter(l => l.key).map(l => [l.key, l.value]));
      const annotationsObj = Object.fromEntries(formAnnotations.filter(l => l.key).map(l => [l.key, l.value]));
      const dataObj = Object.fromEntries(formData.filter(d => d.key).map(d => [d.key, d.value]));
      const yamlObj = {
        apiVersion: 'v1',
        kind: 'Secret',
        type: secret?.type,
        metadata: { name, namespace, labels: labelsObj, annotations: annotationsObj },
        data: dataObj,
      };
      currentYaml = YAML.stringify(yamlObj);
      setYamlContent(currentYaml);
    }

    // 驗證 YAML 格式
    try {
      YAML.parse(currentYaml);
    } catch (error) {
      message.error(t('config:edit.messages.yamlFormatError', { error: error instanceof Error ? error.message : t('config:edit.messages.unknownError') }));
      return;
    }

    // 執行預檢
    setSubmitting(true);
    try {
      await ResourceService.applyYAML(clusterId, 'Secret', currentYaml, true);
      // 預檢透過，展示 diff 對比
      setPendingYaml(currentYaml);
      setDiffModalVisible(true);
    } catch (error: unknown) {
      message.error(t('config:edit.messages.dryRunFailedWithError', { error: parseApiError(error) || t('config:edit.messages.unknownError') }));
    } finally {
      setSubmitting(false);
    }
  };

  // 返回上一頁
  const handleBack = () => {
    if (yamlContent !== originalYaml) {
      modal.confirm({
        title: t('config:edit.confirmLeaveTitle'),
        content: t('config:edit.confirmLeaveContent'),
        okText: t('common:actions.confirm'),
        cancelText: t('common:actions.cancel'),
        onOk: () => navigate(`/clusters/${clusterId}/configs/secret/${namespace}/${name}`),
      });
    } else {
      navigate(`/clusters/${clusterId}/configs/secret/${namespace}/${name}`);
    }
  };

  if (loading) return <PageSkeleton variant="detail" />;

  if (!secret) {
    return null;
  }

  const hasChanges = yamlContent !== originalYaml;

  return (
    <div style={{ padding: '24px' }}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        {/* 頭部 */}
        <Card>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <Button icon={<ArrowLeftOutlined />} onClick={handleBack}>
                {t('common:actions.back')}
              </Button>
              <Title level={4} style={{ margin: 0 }}>
                {t('config:edit.editSecret', { name: secret.name })}
              </Title>
              <Tag color="orange">{t('config:edit.sensitiveData')}</Tag>
              {hasChanges && (
                <Text type="warning">{t('config:edit.unsavedChanges')}</Text>
              )}
            </Space>
            <Space>
              <Segmented
                value={editMode}
                onChange={handleModeChange}
                options={[
                  { value: 'form', icon: <FormOutlined />, label: '表單模式' },
                  { value: 'yaml', icon: <CodeOutlined />, label: 'YAML 模式' },
                ]}
              />
              <Button
                icon={<CheckCircleOutlined />}
                loading={dryRunning}
                onClick={handleDryRun}
              >
                {t('config:edit.dryRun')}
              </Button>
              <Button onClick={handleBack}>
                {t('common:actions.cancel')}
              </Button>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={submitting}
                onClick={handleSubmit}
                disabled={!hasChanges && editMode === 'yaml'}
              >
                {t('common:actions.save')}
              </Button>
            </Space>
          </Space>
        </Card>

        {/* 預檢結果提示 */}
        {dryRunResult && (
          <Alert
            message={dryRunResult.success ? t('config:edit.dryRunPassedTitle') : t('config:edit.dryRunFailedTitle')}
            description={dryRunResult.message}
            type={dryRunResult.success ? 'success' : 'error'}
            showIcon
            icon={dryRunResult.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
            closable
            onClose={() => setDryRunResult(null)}
          />
        )}

        {/* 敏感資料警告 */}
        <Alert
          message={t('config:edit.sensitiveWarningTitle')}
          description={t('config:edit.sensitiveWarningDesc')}
          type="warning"
          showIcon
        />

        {/* 表單模式 */}
        {editMode === 'form' && (
          <SecretEditFormPanel
            secret={secret}
            name={name}
            namespace={namespace}
            formLabels={formLabels}
            formAnnotations={formAnnotations}
            formData={formData}
            onLabelsChange={setFormLabels}
            onAnnotationsChange={setFormAnnotations}
            onDataChange={setFormData}
          />
        )}

        {/* YAML 編輯器 */}
        {editMode === 'yaml' && (
          <Card title={t('config:edit.yamlEditor')}>
            <div style={{ border: '1px solid #d9d9d9', borderRadius: '4px' }}>
              <MonacoEditor
                height="600px"
                language="yaml"
                value={yamlContent}
                onChange={(value) => {
                  setYamlContent(value || '');
                  setDryRunResult(null);
                }}
                options={{
                  minimap: { enabled: true },
                  fontSize: 14,
                  lineNumbers: 'on',
                  scrollBeyondLastLine: false,
                  automaticLayout: true,
                  tabSize: 2,
                  insertSpaces: true,
                  wordWrap: 'on',
                  folding: true,
                  bracketPairColorization: { enabled: true },
                }}
                theme="vs-light"
              />
            </div>
          </Card>
        )}
      </Space>

      {/* YAML Diff 對比 Modal */}
      <Modal
        title={
          <Space>
            <DiffOutlined />
            <span>{t('config:edit.confirmDiffTitle')}</span>
          </Space>
        }
        open={diffModalVisible}
        onCancel={() => setDiffModalVisible(false)}
        width={1200}
        footer={[
          <Button key="cancel" onClick={() => setDiffModalVisible(false)}>
            {t('common:actions.cancel')}
          </Button>,
          <Button
            key="submit"
            type="primary"
            loading={submitting}
            onClick={handleConfirmDiff}
          >
            {t('config:edit.confirmUpdate')}
          </Button>,
        ]}
      >
        <Alert
          message={t('config:edit.reviewChanges')}
          description={t('config:edit.reviewChangesSecretDesc')}
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <div style={{ display: 'flex', gap: 16, marginBottom: 8 }}>
          <div style={{ flex: 1 }}>
            <Text strong style={{ color: '#cf1322' }}>{t('config:edit.originalConfig')}</Text>
          </div>
          <div style={{ flex: 1 }}>
            <Text strong style={{ color: '#389e0d' }}>{t('config:edit.modifiedConfig')}</Text>
          </div>
        </div>
        <div style={{ border: '1px solid #d9d9d9', borderRadius: '4px' }}>
          <DiffEditor
            height="500px"
            language="yaml"
            original={originalYaml}
            modified={pendingYaml}
            options={{
              readOnly: true,
              minimap: { enabled: false },
              fontSize: 13,
              lineNumbers: 'on',
              scrollBeyondLastLine: false,
              automaticLayout: true,
              renderSideBySide: true,
              enableSplitViewResizing: true,
            }}
            theme="vs-light"
          />
        </div>
      </Modal>
    </div>
  );
};

export default SecretEdit;

import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Button,
  Space,
  Segmented,
  Spin,
  App,
  Alert,
  Tooltip,
  Modal,
  Typography,
} from 'antd';
import {
  ArrowLeftOutlined,
  SaveOutlined,
  FormOutlined,
  CodeOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  DiffOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { WorkloadService } from '../../services/workloadService';
import { parseApiError } from '../../utils/api';
import { useTranslation } from 'react-i18next';
import { getNamespaces } from '../../services/namespaceService';
import { secretService } from '../../services/configService';
import WorkloadForm from '../../components/workload/WorkloadForm';
import { WorkloadYamlService } from '../../services/workloadYamlService';
import MonacoEditor, { DiffEditor } from '@monaco-editor/react';
import * as YAML from 'yaml';
import { Form } from 'antd';
import type { WorkloadFormData } from '../../types/workload';

const { Text } = Typography;

type WorkloadType = 'Deployment' | 'StatefulSet' | 'DaemonSet' | 'Rollout' | 'Job' | 'CronJob';

const DeploymentCreate: React.FC = () => {
  const navigate = useNavigate();
  const { clusterId } = useParams<{ clusterId: string }>();
  const [searchParams] = useSearchParams();
  const { message: messageApi, modal } = App.useApp();
const { t } = useTranslation(["workload", "common"]);
const workloadType = (searchParams.get('type') || 'Deployment') as WorkloadType;
  const editNamespace = searchParams.get('namespace');
  const editName = searchParams.get('name');
  const isEdit = !!(editNamespace && editName);
  
  // 編輯模式預設使用 YAML 編輯器（避免表單格式化導致欄位丟失）
  const [editMode, setEditMode] = useState<'form' | 'yaml'>(isEdit ? 'yaml' : 'form');
  const [submitting, setSubmitting] = useState(false);
  const [loading, setLoading] = useState(false);
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{ success: boolean; message: string } | null>(null);
  
  // 表單例項
  const [form] = Form.useForm();
  
  // 表單資料
  const [formData, setFormData] = useState<WorkloadFormData | null>(null);
  
  // YAML 資料
  const [yamlContent, setYamlContent] = useState(getDefaultYaml());
  
  // 原始 YAML（編輯模式用於 diff 對比）
  const [originalYaml, setOriginalYaml] = useState<string>('');
  
  // Diff 彈窗狀態
  const [diffModalVisible, setDiffModalVisible] = useState(false);
  const [pendingYaml, setPendingYaml] = useState<string>('');
  
  // 命名空間列表
  const [namespaces, setNamespaces] = useState<string[]>(['default']);
  
  // 映像拉取憑證列表
  const [imagePullSecretsList, setImagePullSecretsList] = useState<string[]>([]);

  // Rollout CRD 安裝狀態
  const [rolloutCRDEnabled, setRolloutCRDEnabled] = useState<boolean | null>(null);
  
  // 當前選擇的命名空間
  const currentNamespace = Form.useWatch('namespace', form) || 'default';
  
  // 獲取預設YAML
  function getDefaultYaml(): string {
    const defaultData: WorkloadFormData = {
      name: '',
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
    return WorkloadYamlService.formDataToYAML(workloadType, defaultData);
  }

  // 載入所有命名空間列表
  useEffect(() => {
    const loadAllNamespaces = async () => {
      if (!clusterId) return;
      try {
        const nsList = await getNamespaces(Number(clusterId));
        if (nsList && nsList.length > 0) {
          setNamespaces(nsList.map(ns => ns.name));
        } else {
          setNamespaces(['default']);
        }
      } catch (error) {
        console.error('獲取命名空間列表失敗:', error);
        setNamespaces(['default']);
      }
    };
    loadAllNamespaces();
  }, [clusterId]);
  
  // 載入映像拉取憑證列表 (當命名空間變化時)
  useEffect(() => {
    const loadImagePullSecrets = async () => {
      if (!clusterId || !currentNamespace) return;
      try {
        // 呼叫後端 API 獲取 dockerconfigjson 型別的 secrets
        const data = await secretService.getSecrets(Number(clusterId), {
          namespace: currentNamespace,
          type: 'kubernetes.io/dockerconfigjson',
        });
        if (data && data.items) {
          setImagePullSecretsList(data.items.map((s) => s.name));
        } else {
          setImagePullSecretsList([]);
        }
      } catch (error) {
        console.error('獲取映像拉取憑證失敗:', error);
        setImagePullSecretsList([]);
      }
    };
    loadImagePullSecrets();
  }, [clusterId, currentNamespace]);

  // Rollout 類型：檢查 CRD 是否已安裝
  useEffect(() => {
    if (workloadType !== 'Rollout' || !clusterId) return;
    WorkloadService.checkRolloutCRD(clusterId)
      .then((res: { enabled: boolean }) => setRolloutCRDEnabled(res.enabled))
      .catch(() => setRolloutCRDEnabled(false));
  }, [workloadType, clusterId]);

  // 如果是編輯模式，載入現有資料
  useEffect(() => {
    const loadWorkload = async () => {
      if (!isEdit || !clusterId || !editNamespace || !editName) return;
      
      setLoading(true);
      try {
        const response = await WorkloadService.getWorkloadDetail(
          clusterId,
          workloadType,
          editNamespace,
          editName
        );
        
        if (response) {
          let yaml: string;
          if (response.yaml && typeof response.yaml === 'string') {
            yaml = response.yaml;
          } else {
            const rawResource = response.raw || response.workload;
            yaml = YAML.stringify(rawResource);
          }
          
          // 儲存原始 YAML 用於 diff 對比
          setOriginalYaml(yaml);
          setYamlContent(yaml);
          
          // 解析為表單資料
          const parsedData = WorkloadYamlService.yamlToFormData(yaml);
          if (parsedData) {
            // 先設定 formData state，這會觸發 WorkloadForm 的 useEffect
            setFormData(parsedData);
            // 延遲設定表單值，確保元件已掛載
            setTimeout(() => {
              form.setFieldsValue(parsedData);
            }, 100);
          }
        }
      } catch (error) {
        console.error('載入工作負載失敗:', error);
        messageApi.error(t('messages.loadWorkloadFailed'));
      } finally {
        setLoading(false);
      }
    };
    
    loadWorkload();
  }, [isEdit, clusterId, editNamespace, editName, workloadType, messageApi, form]);

  // 表單轉YAML
  const formToYaml = useCallback((): string => {
    try {
      const values = form.getFieldsValue(true);
      
      // 構建完整的表單資料
      const formData: WorkloadFormData = {
        name: values.name || '',
        namespace: values.namespace || 'default',
        replicas: values.replicas,
        description: values.description,
        labels: values.labels,
        annotations: values.annotations,
        containers: values.containers || [],
        initContainers: values.initContainers,
        volumes: values.volumes,
        imagePullSecrets: values.imagePullSecrets,
        scheduling: values.scheduling,
        tolerations: values.tolerations,
        strategy: values.strategy,
        minReadySeconds: values.minReadySeconds,
        revisionHistoryLimit: values.revisionHistoryLimit,
        progressDeadlineSeconds: values.progressDeadlineSeconds,
        terminationGracePeriodSeconds: values.terminationGracePeriodSeconds,
        dnsPolicy: values.dnsPolicy,
        dnsConfig: values.dnsConfig,
        hostNetwork: values.hostNetwork,
        // 特定型別欄位
        serviceName: values.serviceName,
        schedule: values.schedule,
        suspend: values.suspend,
        concurrencyPolicy: values.concurrencyPolicy,
        completions: values.completions,
        parallelism: values.parallelism,
        backoffLimit: values.backoffLimit,
        activeDeadlineSeconds: values.activeDeadlineSeconds,
        // Argo Rollout 策略
        rolloutStrategy: values.rolloutStrategy,
      };
      
      return WorkloadYamlService.formDataToYAML(workloadType, formData);
    } catch (error) {
      console.error('表單轉YAML失敗:', error);
      return yamlContent;
    }
  }, [form, workloadType, yamlContent]);

  // YAML轉表單
  const yamlToForm = useCallback((): boolean => {
    try {
      const parsedData = WorkloadYamlService.yamlToFormData(yamlContent);
      if (parsedData) {
        form.setFieldsValue(parsedData);
        setFormData(parsedData);
        return true;
      }
      return false;
    } catch (err) {
      messageApi.error(t('messages.yamlFormatError') + ': ' + (err instanceof Error ? err.message : ''));
      return false;
    }
  }, [yamlContent, form, messageApi]);

  // 切換編輯模式
  const handleModeChange = (newMode: string) => {
    if (newMode === editMode) return;
    
    if (editMode === 'form' && newMode === 'yaml') {
      // 從表單切換到YAML
      const yaml = formToYaml();
      setYamlContent(yaml);
    } else if (editMode === 'yaml' && newMode === 'form') {
      // 從YAML切換到表單
      if (!yamlToForm()) {
        return; // 解析失敗，不切換
      }
    }
    
    setEditMode(newMode as 'form' | 'yaml');
    setDryRunResult(null);
  };

  // Dry-run 預檢
  const handleDryRun = async () => {
    let yaml: string;
    
    if (editMode === 'form') {
      yaml = formToYaml();
    } else {
      yaml = yamlContent;
    }
    
    // 驗證 YAML 格式
    try {
      YAML.parse(yaml);
    } catch (err) {
      setDryRunResult({
        success: false,
        message: t('workload:create.yamlFormatError') + ': ' + (err instanceof Error ? err.message : t('common:unknown')),
      });
      return;
    }
    
    setDryRunning(true);
    setDryRunResult(null);
    
    try {
      await WorkloadService.applyYAML(clusterId!, yaml, true);
      
      setDryRunResult({
        success: true,
        message: t('create.dryRunPassed'),
      });
    } catch (error: unknown) {
      setDryRunResult({
        success: false,
        message: parseApiError(error),
      });
    } finally {
      setDryRunning(false);
    }
  };

  // 提交建立/更新
  const submitYaml = async (yaml: string) => {
    if (!clusterId) {
      messageApi.error(t('messages.clusterNotFound'));
      return;
    }
    
    setSubmitting(true);
    try {
      await WorkloadService.applyYAML(clusterId, yaml, false);
      
      messageApi.success(isEdit ? t('messages.updateSuccess') : t('messages.createSuccess'));
      navigate(`/clusters/${clusterId}/workloads`);
    } catch (error: unknown) {
      console.error('提交失敗:', error);
      messageApi.error(error instanceof Error ? error.message : t('messages.operationFailed'));
    } finally {
      setSubmitting(false);
    }
  };

  // 處理提交
  const handleSubmit = async () => {
    // 先進行預檢
    let yaml: string;
    
    if (editMode === 'form') {
      try {
        await form.validateFields();
        yaml = formToYaml();
      } catch {
        messageApi.error(t('messages.checkForm'));
        return;
      }
    } else {
      yaml = yamlContent;
    }
    
    // 驗證 YAML 格式
    try {
      YAML.parse(yaml);
    } catch (err) {
      messageApi.error(t('messages.yamlFormatError') + ': ' + (err instanceof Error ? err.message : ''));
      return;
    }
    
    // 編輯模式下顯示 diff 對比彈窗
    if (isEdit && originalYaml) {
      setPendingYaml(yaml);
      setDiffModalVisible(true);
    } else {
      // 建立模式直接確認
      modal.confirm({
        title: t('create.confirmCreate'),
        content: (
          <div>
            <p>{t('create.confirmCreateDesc')}</p>
            <p style={{ color: '#666', fontSize: 12 }}>{t('create.confirmCreateHint')}</p>
          </div>
        ),
        okText: t('common:actions.confirm'),
        cancelText: t('common:actions.cancel'),
        onOk: () => submitYaml(yaml),
      });
    }
  };

  // 確認 diff 後提交
  const handleConfirmDiff = () => {
    setDiffModalVisible(false);
    submitYaml(pendingYaml);
  };

  // 表單值變化時更新
  const handleFormValuesChange = (changedValues: Partial<WorkloadFormData>, allValues: WorkloadFormData) => {
    setFormData(allValues);
    setDryRunResult(null);
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}>
        <Spin size="large" tip={t("common:messages.loading")} />
      </div>
    );
  }

  return (
    <div style={{ padding: 24 }}>
      {/* 頭部 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Space>
          <Button 
            icon={<ArrowLeftOutlined />} 
            onClick={() => navigate(-1)}
          >
            {t('create.back')}
          </Button>
          <h2 style={{ margin: 0 }}>
            {isEdit ? t('create.edit') : t('create.create')} {workloadType}
          </h2>
          {/* 編輯模式只支援 YAML 編輯，避免表單格式化導致複雜欄位丟失 */}
          {isEdit ? (
            <Tooltip title={t("create.yamlModeOnly")}>
              <Space>
                <CodeOutlined />
                <span>{t('create.yamlMode')}</span>
              </Space>
            </Tooltip>
          ) : (
            <Segmented
              value={editMode}
              onChange={handleModeChange}
              options={[
                { value: 'form', icon: <FormOutlined />, label: t('create.formMode') },
                { value: 'yaml', icon: <CodeOutlined />, label: t('create.yamlMode') },
              ]}
            />
          )}
        </Space>
        
        <Space>
          <Tooltip title={t("create.preCheckTooltip")}>
            <Button
              onClick={handleDryRun}
              loading={dryRunning}
              disabled={workloadType === 'Rollout' && rolloutCRDEnabled === false}
              icon={dryRunResult?.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
            >
              {t('create.preCheck')}
            </Button>
          </Tooltip>
          <Button onClick={() => navigate(-1)}>
            {t('create.cancel')}
          </Button>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={handleSubmit}
            loading={submitting}
            disabled={workloadType === 'Rollout' && rolloutCRDEnabled === false}
          >
            {isEdit ? t('create.update') : t('create.create')}
          </Button>
        </Space>
      </div>
      
      {/* Rollout CRD 未安裝警告 */}
      {workloadType === 'Rollout' && rolloutCRDEnabled === false && (
        <Alert
          message="Argo Rollouts 未安裝"
          description="此叢集尚未安裝 Argo Rollouts，無法建立 Rollout 資源。請先在叢集中安裝 Argo Rollouts。"
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {/* 預檢結果 */}
      {dryRunResult && (
        <Alert
          message={dryRunResult.success ? t('create.dryRunCheckPassed') : t('create.dryRunCheckFailed')}
          description={dryRunResult.message}
          type={dryRunResult.success ? 'success' : 'error'}
          showIcon
          closable
          onClose={() => setDryRunResult(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      
      {/* 內容區域 */}
      {editMode === 'form' ? (
        <WorkloadForm
          workloadType={workloadType}
          namespaces={namespaces}
          imagePullSecretsList={imagePullSecretsList}
          initialData={formData || undefined}
          onValuesChange={handleFormValuesChange}
          form={form}
          isEdit={isEdit}
        />
      ) : (
        <Card title={t('create.yamlEdit')}>
          <MonacoEditor
            height="600px"
            language="yaml"
            value={yamlContent}
            onChange={(value) => {
              setYamlContent(value || '');
              setDryRunResult(null);
            }}
            options={{
              minimap: { enabled: false },
              fontSize: 14,
              lineNumbers: 'on',
              wordWrap: 'on',
              automaticLayout: true,
              scrollBeyondLastLine: false,
            }}
          />
        </Card>
      )}

      {/* Diff 對比彈窗 */}
      <Modal
        title={
          <Space>
            <DiffOutlined />
            <span>{t('create.diffTitle')}</span>
          </Space>
        }
        open={diffModalVisible}
        onCancel={() => setDiffModalVisible(false)}
        onOk={handleConfirmDiff}
        width="90%"
        style={{ top: 20 }}
        okText={t("create.confirmUpdate")}
        cancelText={t("create.cancel")}
        destroyOnHidden
      >
        <div style={{ marginBottom: 16 }}>
          <Space>
            <Text type="secondary">
              {t('create.diffDesc')}
            </Text>
          </Space>
        </div>
        <div style={{ border: '1px solid #d9d9d9', borderRadius: 4 }}>
          <DiffEditor
            height="60vh"
            language="yaml"
            original={originalYaml}
            modified={pendingYaml}
            options={{
              readOnly: true,
              minimap: { enabled: false },
              fontSize: 13,
              lineNumbers: 'on',
              wordWrap: 'on',
              automaticLayout: true,
              scrollBeyondLastLine: false,
              renderSideBySide: true,
              diffWordWrap: 'on',
            }}
          />
        </div>
      </Modal>
    </div>
  );
};

export default DeploymentCreate;

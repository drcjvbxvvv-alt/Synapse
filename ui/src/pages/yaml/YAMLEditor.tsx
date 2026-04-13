import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import { message, App } from 'antd';
import { loader } from '@monaco-editor/react';
import * as monaco from 'monaco-editor';
import { WorkloadService } from '../../services/workloadService';
import { useTranslation } from 'react-i18next';
import * as YAML from 'yaml';

import YAMLSubmitBar from './YAMLSubmitBar';
import YAMLEditorPane from './YAMLEditorPane';
import YAMLDiffView from './YAMLDiffView';

// 配置Monaco Editor使用本地資源
loader.config({ monaco });

const YAMLEditor: React.FC = () => {
  const { modal } = App.useApp();
  const { t } = useTranslation(['yaml', 'common']);
  const { clusterId } = useParams<{ clusterId: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  // 從URL參數獲取工作負載資訊
  const workloadRef = searchParams.get('workload'); // namespace/name
  const workloadType = searchParams.get('type');

  const [yaml, setYaml] = useState('');
  const [originalYaml, setOriginalYaml] = useState('');
  const [loading, setLoading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [dryRun, setDryRun] = useState(true);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewResult, setPreviewResult] = useState<Record<string, unknown> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editorLoading, setEditorLoading] = useState(true);

  // Diff 對比相關狀態
  const [diffModalVisible, setDiffModalVisible] = useState(false);
  const [pendingYaml, setPendingYaml] = useState<string>('');
  const [dryRunResult, setDryRunResult] = useState<{
    success: boolean;
    message: string;
  } | null>(null);

  // 檢查是否有未儲存的更改
  const hasUnsavedChanges = yaml !== originalYaml;

  // Monaco Editor載入處理
  const handleEditorWillMount = () => {
    setEditorLoading(true);
  };

  const handleEditorDidMount = () => {
    setEditorLoading(false);
  };

  const handleEditorValidation = (markers: unknown[]) => {
    if (markers && markers.length > 0) {
      console.warn('Editor validation markers:', markers);
    }
  };

  // 載入現有工作負載的YAML
  const loadWorkloadYAML = useCallback(async () => {
    if (!clusterId || !workloadRef || !workloadType) return;

    const [namespace, name] = workloadRef.split('/');
    if (!namespace || !name) return;

    setLoading(true);
    setError(null);
    try {
      const response = await WorkloadService.getWorkloadDetail(
        clusterId,
        workloadType,
        namespace,
        name
      );

      const yamlContent = response.yaml || YAML.stringify(response.raw);
      setYaml(yamlContent);
      setOriginalYaml(yamlContent);
    } catch (error) {
      console.error('載入YAML失敗:', error);
      const errorMsg =
        t('messages.loadError') +
        ': ' +
        (error instanceof Error ? error.message : t('messages.unknownError'));
      setError(errorMsg);
      message.error(errorMsg);
    } finally {
      setLoading(false);
    }
  }, [clusterId, workloadRef, workloadType, t]);

  // 應用YAML
  const handleApply = async (isDryRun = false) => {
    if (!clusterId || !yaml.trim()) {
      message.error(t('messages.emptyContent'));
      return;
    }

    setApplying(true);
    setDryRunResult(null);
    try {
      const response = await WorkloadService.applyYAML(clusterId, yaml, isDryRun);

      if (isDryRun) {
        setPreviewResult(response as Record<string, unknown>);
        setDryRunResult({
          success: true,
          message: t('messages.dryRunPassed'),
        });
        message.success(t('messages.validateSuccess'));
      } else {
        message.success(t('messages.applySuccess'));
        setOriginalYaml(yaml);
        setDiffModalVisible(false);
      }
    } catch (error) {
      console.error(`YAML ${isDryRun ? 'validate' : 'apply'} failed:`, error);
      const errorMsg = t('messages.yamlFailed', {
        action: isDryRun ? t('messages.validateFailed') : t('messages.applyFailed'),
      });
      if (isDryRun) {
        setDryRunResult({
          success: false,
          message: errorMsg,
        });
      }
      message.error(errorMsg);
    } finally {
      setApplying(false);
    }
  };

  // 預覽YAML (Dry Run)
  const handlePreview = () => {
    handleApply(true);
  };

  // 儲存並應用YAML - 先預檢，再展示 diff 對比
  const handleSave = async () => {
    if (!clusterId || !yaml.trim()) {
      message.error(t('messages.emptyContent'));
      return;
    }

    if (workloadRef && originalYaml) {
      setApplying(true);
      try {
        await WorkloadService.applyYAML(clusterId, yaml, true);
        setPendingYaml(yaml);
        setDiffModalVisible(true);
      } catch (error) {
        console.error('預檢失敗:', error);
        message.error(t('messages.preCheckFailed'));
      } finally {
        setApplying(false);
      }
    } else {
      modal.confirm({
        title: t('confirm.applyYaml'),
        content: t('confirm.applyYamlDesc'),
        okText: t('common:actions.confirm'),
        cancelText: t('common:actions.cancel'),
        onOk: () => handleApply(false),
      });
    }
  };

  // 確認 Diff 後提交
  const handleConfirmDiff = () => {
    handleApply(false);
  };

  // 重置YAML
  const handleReset = () => {
    modal.confirm({
      title: t('confirm.resetTitle'),
      content: t('confirm.resetDesc'),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      centered: true,
      onOk: () => {
        setYaml(originalYaml);
        message.success(t('messages.resetSuccess'));
      },
    });
  };

  // 返回處理 — 有未儲存更改時確認
  const handleBack = () => {
    if (hasUnsavedChanges) {
      modal.confirm({
        title: t('confirm.leave'),
        content: t('confirm.leaveDesc'),
        okText: t('common:actions.confirm'),
        cancelText: t('common:actions.cancel'),
        onOk: () => navigate(-1),
      });
    } else {
      navigate(-1);
    }
  };

  // 生成預設YAML模板
  const generateDefaultYAML = useCallback((type: string) => {
    const templates: Record<string, string> = {
      Deployment: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: my-container
        image: nginx:latest
        ports:
        - containerPort: 80
`,
      Rollout: `apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: my-rollout
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: my-container
        image: nginx:latest
        ports:
        - containerPort: 80
  strategy:
    canary:
      steps:
      - setWeight: 20
      - pause: {}
      - setWeight: 50
      - pause: {duration: 10}
      - setWeight: 80
      - pause: {duration: 10}
`,
      StatefulSet: `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-statefulset
  namespace: default
spec:
  serviceName: my-service
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: my-container
        image: nginx:latest
        ports:
        - containerPort: 80
`,
      DaemonSet: `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: my-daemonset
  namespace: default
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: my-container
        image: nginx:latest
        ports:
        - containerPort: 80
`,
      Job: `apiVersion: batch/v1
kind: Job
metadata:
  name: my-job
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: my-container
        image: busybox
        command: ['sh', '-c', 'echo Hello Kubernetes! && sleep 30']
      restartPolicy: Never
  backoffLimit: 4
`,
      CronJob: `apiVersion: batch/v1
kind: CronJob
metadata:
  name: my-cronjob
  namespace: default
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: my-container
            image: busybox
            command: ['sh', '-c', 'echo Hello Kubernetes! && date']
          restartPolicy: Never
`,
    };

    return (
      templates[type] ||
      `apiVersion: v1
kind: ${type}
metadata:
  name: my-resource
  namespace: default
spec: {}
`
    );
  }, []);

  useEffect(() => {
    if (!clusterId || !workloadType) {
      setError(t('messages.missingParams'));
      return;
    }

    if (workloadRef) {
      loadWorkloadYAML();
    } else {
      const defaultYAML = generateDefaultYAML(workloadType);
      setYaml(defaultYAML);
      setOriginalYaml(defaultYAML);
      setError(null);
    }
  }, [clusterId, workloadRef, workloadType, loadWorkloadYAML, generateDefaultYAML, t]);

  // 頁面離開前提醒
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (hasUnsavedChanges) {
        e.preventDefault();
        e.returnValue = '';
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [hasUnsavedChanges]);

  return (
    <div style={{ padding: '24px', height: 'calc(100vh - 64px)' }}>
      <YAMLSubmitBar
        workloadRef={workloadRef}
        workloadType={workloadType}
        hasUnsavedChanges={hasUnsavedChanges}
        applying={applying}
        dryRun={dryRun}
        error={error}
        dryRunResult={dryRunResult}
        onBack={handleBack}
        onSave={handleSave}
        onPreview={handlePreview}
        onReset={handleReset}
        onDryRunChange={setDryRun}
        onRetry={loadWorkloadYAML}
        onCloseDryRunResult={() => setDryRunResult(null)}
      />

      <YAMLEditorPane
        yaml={yaml}
        loading={loading}
        editorLoading={editorLoading}
        onYamlChange={setYaml}
        onEditorWillMount={handleEditorWillMount}
        onEditorDidMount={handleEditorDidMount}
        onEditorValidation={handleEditorValidation}
      />

      <YAMLDiffView
        previewVisible={previewVisible}
        previewResult={previewResult}
        onPreviewClose={() => setPreviewVisible(false)}
        onPreviewApply={handleSave}
        diffModalVisible={diffModalVisible}
        originalYaml={originalYaml}
        pendingYaml={pendingYaml}
        applying={applying}
        onDiffClose={() => setDiffModalVisible(false)}
        onDiffConfirm={handleConfirmDiff}
      />
    </div>
  );
};

export default YAMLEditor;

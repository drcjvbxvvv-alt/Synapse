import React, { useEffect, useState, useCallback } from 'react';
import {
  Card,
  Button,
  Space,
  message,
  Alert,
  Modal,
  Typography,
  App,
  Segmented,
  Row,
  Col,
  Input,
  Select,
} from 'antd';
import {
  ArrowLeftOutlined,
  SaveOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  DiffOutlined,
  FormOutlined,
  CodeOutlined,
  PlusOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { IngressService } from '../../services/ingressService';
import { ResourceService } from '../../services/resourceService';
import MonacoEditor, { DiffEditor } from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { parseApiError, showApiError } from '../../utils/api';
import PageSkeleton from '../../components/PageSkeleton';

const { Text, Title } = Typography;

const IngressEdit: React.FC = () => {
  const navigate = useNavigate();
  const { modal } = App.useApp();
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();

const { t } = useTranslation(['network', 'common']);
const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [ingressName, setIngressName] = useState('');
  const [yamlContent, setYamlContent] = useState('');
  const [originalYaml, setOriginalYaml] = useState('');

  // 編輯模式
  const [editMode, setEditMode] = useState<'form' | 'yaml'>('form');

  // 表單狀態
  const [formIngressClass, setFormIngressClass] = useState('');
  const [formLabels, setFormLabels] = useState<{key:string;value:string}[]>([]);
  const [formAnnotations, setFormAnnotations] = useState<{key:string;value:string}[]>([]);
  const [formRules, setFormRules] = useState<{host:string; paths:{path:string;pathType:string;serviceName:string;servicePort:string}[]}[]>([]);

  // 預檢相關狀態
  const [dryRunning, setDryRunning] = useState(false);
  const [dryRunResult, setDryRunResult] = useState<{
    success: boolean;
    message: string;
  } | null>(null);

  // Diff 對比相關狀態
  const [diffModalVisible, setDiffModalVisible] = useState(false);
  const [pendingYaml, setPendingYaml] = useState<string>('');

  const parseYamlToForm = (yamlStr: string) => {
    try {
      const parsed = YAML.parse(yamlStr) as unknown;
      const doc = parsed as Record<string, Record<string, unknown>>;
      const spec = (doc?.spec ?? {}) as Record<string, unknown>;
      const metadata = (doc?.metadata ?? {}) as Record<string, unknown>;
      setFormIngressClass((spec?.ingressClassName as string) || '');
      setFormLabels(Object.entries((metadata?.labels ?? {}) as Record<string, string>).map(([k, v]) => ({key: k, value: String(v)})));
      setFormAnnotations(Object.entries((metadata?.annotations ?? {}) as Record<string, string>).map(([k, v]) => ({key: k, value: String(v)})));
      setFormRules(((spec?.rules ?? []) as Record<string, unknown>[]).map((r) => ({
        host: (r.host as string) || '',
        paths: ((r.http as Record<string, unknown>)?.paths ?? [] as unknown[]).map((p) => {
          const path = p as Record<string, unknown>;
          const backend = (path.backend as Record<string, unknown>)?.service as Record<string, unknown> | undefined;
          return {
            path: (path.path as string) || '/',
            pathType: (path.pathType as string) || 'Prefix',
            serviceName: (backend?.name as string) || '',
            servicePort: String((backend?.port as Record<string, unknown>)?.number || 80),
          };
        }),
      })));
    } catch {
      // intentional
    }
  };

  // 載入 Ingress 詳情
  const loadIngress = useCallback(async () => {
    if (!clusterId || !namespace || !name) return;
    setLoading(true);
    try {
      const response = await IngressService.getIngressYAML(clusterId, namespace, name);
      setIngressName(name);
      const yamlStr = response.yaml || '';
      setYamlContent(yamlStr);
      setOriginalYaml(yamlStr);
      parseYamlToForm(yamlStr);
    } catch (error: unknown) {
      showApiError(error, t('network:editPage.loadIngressError'));
      navigate(`/clusters/${clusterId}/network`);
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, name, navigate, t]);

  useEffect(() => {
    loadIngress();
  }, [loadIngress]);

  const buildYamlFromForm = () => {
    const labelsObj = Object.fromEntries(formLabels.filter(l => l.key).map(l => [l.key, l.value]));
    const annotationsObj = Object.fromEntries(formAnnotations.filter(l => l.key).map(l => [l.key, l.value]));
    const rulesArr = formRules.map(r => ({
      host: r.host,
      http: { paths: r.paths.map(p => ({
        path: p.path,
        pathType: p.pathType,
        backend: { service: { name: p.serviceName, port: { number: isNaN(Number(p.servicePort)) ? p.servicePort : Number(p.servicePort) } } },
      })) },
    }));
    let existingMeta = {name: ingressName, namespace: namespace || ''};
    try {
      const parsed = YAML.parse(originalYaml) as unknown;
      const p = parsed as Record<string, Record<string, string>>;
      existingMeta = {name: p?.metadata?.name || ingressName, namespace: p?.metadata?.namespace || namespace || ''};
    } catch {
      // intentional
    }
    const obj = {
      apiVersion: 'networking.k8s.io/v1', kind: 'Ingress',
      metadata: { ...existingMeta, labels: labelsObj, annotations: annotationsObj },
      spec: {
        ...(formIngressClass ? {ingressClassName: formIngressClass} : {}),
        rules: rulesArr,
      },
    };
    return YAML.stringify(obj);
  };

  const handleModeChange = (mode: string) => {
    if (mode === 'yaml') {
      const y = buildYamlFromForm();
      setYamlContent(y);
    } else {
      parseYamlToForm(yamlContent);
    }
    setEditMode(mode as 'form' | 'yaml');
  };

  // 預檢（Dry Run）
  const handleDryRun = async () => {
    if (!clusterId) return;

    let currentYaml = yamlContent;
    if (editMode === 'form') {
      currentYaml = buildYamlFromForm();
      setYamlContent(currentYaml);
    }

    // 驗證 YAML 格式
    try {
      YAML.parse(currentYaml);
    } catch (error) {
      message.error(t('network:editPage.yamlFormatError', { error: error instanceof Error ? error.message : t('network:editPage.unknownError') }));
      return;
    }

    setDryRunning(true);
    setDryRunResult(null);

    try {
      await ResourceService.applyYAML(clusterId, 'Ingress', currentYaml, true);
      setDryRunResult({
        success: true,
        message: t('network:editPage.dryRunSuccess'),
      });
    } catch (error: unknown) {
      setDryRunResult({
        success: false,
        message: parseApiError(error) || t('network:editPage.dryRunFailed'),
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
      await ResourceService.applyYAML(clusterId, 'Ingress', pendingYaml, false);
      message.success(t('network:editPage.ingressUpdateSuccess'));
      setDiffModalVisible(false);
      navigate(`/clusters/${clusterId}/network`);
    } catch (error: unknown) {
      showApiError(error, t('network:editPage.updateFailed'));
    } finally {
      setSubmitting(false);
    }
  };

  // 提交 - 先預檢，再展示 diff
  const handleSubmit = async () => {
    if (!clusterId || !namespace || !name) return;

    let currentYaml = yamlContent;
    if (editMode === 'form') {
      currentYaml = buildYamlFromForm();
      setYamlContent(currentYaml);
    }

    // 驗證 YAML 格式
    try {
      YAML.parse(currentYaml);
    } catch (error) {
      message.error(t('network:editPage.yamlFormatError', { error: error instanceof Error ? error.message : t('network:editPage.unknownError') }));
      return;
    }

    // 執行預檢
    setSubmitting(true);
    try {
      await ResourceService.applyYAML(clusterId, 'Ingress', currentYaml, true);
      // 預檢透過，展示 diff 對比
      setPendingYaml(currentYaml);
      setDiffModalVisible(true);
    } catch (error: unknown) {
      message.error(t('network:editPage.dryRunFailedPrefix', { error: parseApiError(error) || t('network:editPage.unknownError') }));
    } finally {
      setSubmitting(false);
    }
  };

  // 返回上一頁
  const handleBack = () => {
    if (yamlContent !== originalYaml || editMode === 'form') {
      modal.confirm({
        title: t('network:editPage.confirmLeave'),
        content: t('network:editPage.confirmLeaveDesc'),
        okText: t('common:actions.confirm'),
        cancelText: t('common:actions.cancel'),
        onOk: () => navigate(`/clusters/${clusterId}/network`),
      });
    } else {
      navigate(`/clusters/${clusterId}/network`);
    }
  };

  if (loading) return <PageSkeleton variant="detail" />;

  const hasChanges = editMode === 'form' ? true : yamlContent !== originalYaml;

  return (
    <div style={{ padding: '24px' }}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        {/* 頭部 */}
        <Card>
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <Button icon={<ArrowLeftOutlined />} onClick={handleBack}>
                {t('network:editPage.back')}
              </Button>
              <Title level={4} style={{ margin: 0 }}>
                {t('network:editPage.editIngress', { name: ingressName })}
              </Title>
              {hasChanges && editMode === 'yaml' && (
                <Text type="warning">{t('network:editPage.unsavedChanges')}</Text>
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
                {t('network:editPage.dryRun')}
              </Button>
              <Button onClick={handleBack}>
                {t('network:editPage.cancel')}
              </Button>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={submitting}
                onClick={handleSubmit}
                disabled={editMode === 'yaml' ? !hasChanges : false}
              >
                {t('network:editPage.save')}
              </Button>
            </Space>
          </Space>
        </Card>

        {/* 預檢結果提示 */}
        {dryRunResult && (
          <Alert
            message={dryRunResult.success ? t('network:editPage.dryRunSuccessTitle') : t('network:editPage.dryRunFailedTitle')}
            description={dryRunResult.message}
            type={dryRunResult.success ? 'success' : 'error'}
            showIcon
            icon={dryRunResult.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
            closable
            onClose={() => setDryRunResult(null)}
          />
        )}

        {/* 表單或 YAML 編輯器 */}
        {editMode === 'form' ? (
          <Space direction="vertical" style={{width: '100%'}} size="middle">
            {/* 基本資訊 */}
            <Card title="基本資訊">
              <Row gutter={16}>
                <Col span={12}><div><label>名稱</label><Input value={ingressName} disabled /></div></Col>
                <Col span={12}><div><label>命名空間</label><Input value={namespace} disabled /></div></Col>
                <Col span={12}><div style={{marginTop: 8}}><label>Ingress Class</label><Input value={formIngressClass} onChange={e => setFormIngressClass(e.target.value)} placeholder="nginx" /></div></Col>
              </Row>
            </Card>
            {/* 標籤 */}
            <Card title="標籤" extra={<Button size="small" icon={<PlusOutlined />} onClick={() => setFormLabels(p => [...p, {key: '', value: ''}])}>新增</Button>}>
              {formLabels.map((item, i) => (
                <Row key={i} gutter={8} style={{marginBottom: 8}}>
                  <Col span={10}><Input placeholder="key" value={item.key} onChange={e => setFormLabels(p => p.map((x, j) => j === i ? {...x, key: e.target.value} : x))} /></Col>
                  <Col span={10}><Input placeholder="value" value={item.value} onChange={e => setFormLabels(p => p.map((x, j) => j === i ? {...x, value: e.target.value} : x))} /></Col>
                  <Col span={4}><Button danger icon={<DeleteOutlined />} onClick={() => setFormLabels(p => p.filter((_, j) => j !== i))} /></Col>
                </Row>
              ))}
            </Card>
            {/* 注解 */}
            <Card title="注解 (Annotations)" extra={<Button size="small" icon={<PlusOutlined />} onClick={() => setFormAnnotations(p => [...p, {key: '', value: ''}])}>新增</Button>}>
              {formAnnotations.map((item, i) => (
                <Row key={i} gutter={8} style={{marginBottom: 8}}>
                  <Col span={10}><Input placeholder="key" value={item.key} onChange={e => setFormAnnotations(p => p.map((x, j) => j === i ? {...x, key: e.target.value} : x))} /></Col>
                  <Col span={10}><Input placeholder="value" value={item.value} onChange={e => setFormAnnotations(p => p.map((x, j) => j === i ? {...x, value: e.target.value} : x))} /></Col>
                  <Col span={4}><Button danger icon={<DeleteOutlined />} onClick={() => setFormAnnotations(p => p.filter((_, j) => j !== i))} /></Col>
                </Row>
              ))}
            </Card>
            {/* 規則 */}
            <Card title="路由規則" extra={<Button size="small" icon={<PlusOutlined />} onClick={() => setFormRules(p => [...p, {host: '', paths: [{path: '/', pathType: 'Prefix', serviceName: '', servicePort: '80'}]}])}>新增規則</Button>}>
              {formRules.map((rule, ri) => (
                <Card key={ri} size="small" style={{marginBottom: 12}}
                  title={<Input placeholder="Host (e.g. example.com)" value={rule.host} onChange={e => setFormRules(p => p.map((r, j) => j === ri ? {...r, host: e.target.value} : r))} style={{width: 300}} />}
                  extra={<Button danger size="small" icon={<DeleteOutlined />} onClick={() => setFormRules(p => p.filter((_, j) => j !== ri))}>刪除規則</Button>}>
                  {rule.paths.map((path, pi) => (
                    <Row key={pi} gutter={8} style={{marginBottom: 8}} align="middle">
                      <Col span={5}><Input placeholder="Path /" value={path.path} onChange={e => setFormRules(p => p.map((r, j) => j === ri ? {...r, paths: r.paths.map((pp, k) => k === pi ? {...pp, path: e.target.value} : pp)} : r))} /></Col>
                      <Col span={4}>
                        <Select value={path.pathType} style={{width: '100%'}} onChange={v => setFormRules(p => p.map((r, j) => j === ri ? {...r, paths: r.paths.map((pp, k) => k === pi ? {...pp, pathType: v} : pp)} : r))}
                          options={['Prefix', 'Exact', 'ImplementationSpecific'].map(v => ({value: v, label: v}))} />
                      </Col>
                      <Col span={8}><Input placeholder="Service 名稱" value={path.serviceName} onChange={e => setFormRules(p => p.map((r, j) => j === ri ? {...r, paths: r.paths.map((pp, k) => k === pi ? {...pp, serviceName: e.target.value} : pp)} : r))} /></Col>
                      <Col span={4}><Input placeholder="Port" value={path.servicePort} onChange={e => setFormRules(p => p.map((r, j) => j === ri ? {...r, paths: r.paths.map((pp, k) => k === pi ? {...pp, servicePort: e.target.value} : pp)} : r))} /></Col>
                      <Col span={3}><Button danger size="small" icon={<DeleteOutlined />} onClick={() => setFormRules(p => p.map((r, j) => j === ri ? {...r, paths: r.paths.filter((_, k) => k !== pi)} : r))} /></Col>
                    </Row>
                  ))}
                  <Button size="small" icon={<PlusOutlined />} onClick={() => setFormRules(p => p.map((r, j) => j === ri ? {...r, paths: [...r.paths, {path: '/', pathType: 'Prefix', serviceName: '', servicePort: '80'}]} : r))}>新增路徑</Button>
                </Card>
              ))}
            </Card>
          </Space>
        ) : (
          /* YAML 編輯器 */
          <Card title={t('network:editPage.yamlEditor')}>
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
            <span>{t('network:editPage.confirmChanges')}</span>
          </Space>
        }
        open={diffModalVisible}
        onCancel={() => setDiffModalVisible(false)}
        width={1200}
        footer={[
          <Button key="cancel" onClick={() => setDiffModalVisible(false)}>
            {t('network:editPage.cancel')}
          </Button>,
          <Button
            key="submit"
            type="primary"
            loading={submitting}
            onClick={handleConfirmDiff}
          >
            {t('network:editPage.confirmUpdate')}
          </Button>,
        ]}
      >
        <Alert
          message={t('network:editPage.reviewChanges')}
          description={t('network:editPage.reviewChangesDesc')}
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
        <div style={{ display: 'flex', gap: 16, marginBottom: 8 }}>
          <div style={{ flex: 1 }}>
            <Text strong style={{ color: '#cf1322' }}>{t('network:editPage.originalConfig')}</Text>
          </div>
          <div style={{ flex: 1 }}>
            <Text strong style={{ color: '#389e0d' }}>{t('network:editPage.modifiedConfig')}</Text>
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

export default IngressEdit;

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
  InputNumber,
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
import { ServiceService } from '../../services/serviceService';
import { ResourceService } from '../../services/resourceService';
import MonacoEditor, { DiffEditor } from '@monaco-editor/react';
import * as YAML from 'yaml';
import { useTranslation } from 'react-i18next';
import { parseApiError, showApiError } from '../../utils/api';
import PageSkeleton from '../../components/PageSkeleton';

const { Text, Title } = Typography;

const ServiceEdit: React.FC = () => {
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
  const [serviceName, setServiceName] = useState('');
  const [yamlContent, setYamlContent] = useState('');
  const [originalYaml, setOriginalYaml] = useState('');

  // 編輯模式
  const [editMode, setEditMode] = useState<'form' | 'yaml'>('form');

  // 表單狀態
  const [formServiceType, setFormServiceType] = useState('ClusterIP');
  const [formPorts, setFormPorts] = useState<{name:string;protocol:string;port:number;targetPort:string;nodePort?:number}[]>([]);
  const [formSelector, setFormSelector] = useState<{key:string;value:string}[]>([]);
  const [formLabels, setFormLabels] = useState<{key:string;value:string}[]>([]);

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
      const p = parsed as Record<string, Record<string, unknown>>;
      const spec = (p?.spec ?? {}) as Record<string, unknown>;
      const metadata = (p?.metadata ?? {}) as Record<string, unknown>;
      setFormServiceType((spec?.type as string) || 'ClusterIP');
      setFormPorts(((spec?.ports ?? []) as Record<string, unknown>[]).map((port) => ({
        name: (port.name as string) || '',
        protocol: (port.protocol as string) || 'TCP',
        port: (port.port as number) || 80,
        targetPort: String(port.targetPort || 80),
        nodePort: port.nodePort as number | undefined,
      })));
      setFormSelector(Object.entries((spec?.selector ?? {}) as Record<string, string>).map(([k, v]) => ({key: k, value: String(v)})));
      setFormLabels(Object.entries((metadata?.labels ?? {}) as Record<string, string>).map(([k, v]) => ({key: k, value: String(v)})));
    } catch {
      // intentional
    }
  };

  // 載入 Service 詳情
  const loadService = useCallback(async () => {
    if (!clusterId || !namespace || !name) return;
    setLoading(true);
    try {
      const response = await ServiceService.getServiceYAML(clusterId, namespace, name);
      setServiceName(name);
      const yamlStr = response.yaml || '';
      setYamlContent(yamlStr);
      setOriginalYaml(yamlStr);
      parseYamlToForm(yamlStr);
    } catch (error: unknown) {
      showApiError(error, t('network:editPage.loadServiceError'));
      navigate(`/clusters/${clusterId}/network`);
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, name, navigate, t]);

  useEffect(() => {
    loadService();
  }, [loadService]);

  const buildYamlFromForm = () => {
    const labelsObj = Object.fromEntries(formLabels.filter(l => l.key).map(l => [l.key, l.value]));
    const selectorObj = Object.fromEntries(formSelector.filter(l => l.key).map(l => [l.key, l.value]));
    const portsArr = formPorts.map(p => ({
      name: p.name || undefined,
      protocol: p.protocol,
      port: Number(p.port),
      targetPort: isNaN(Number(p.targetPort)) ? p.targetPort : Number(p.targetPort),
      ...(formServiceType === 'NodePort' && p.nodePort ? {nodePort: p.nodePort} : {}),
    }));
    let existingMeta = {name: name || '', namespace: namespace || ''};
    try {
      const parsed = YAML.parse(originalYaml) as unknown;
      const p = parsed as Record<string, Record<string, string>>;
      existingMeta = {name: p?.metadata?.name || name || '', namespace: p?.metadata?.namespace || namespace || ''};
    } catch {
      // intentional
    }
    const obj = {
      apiVersion: 'v1', kind: 'Service',
      metadata: { ...existingMeta, labels: labelsObj },
      spec: { type: formServiceType, selector: selectorObj, ports: portsArr },
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
      await ResourceService.applyYAML(clusterId, 'Service', currentYaml, true);
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
      await ResourceService.applyYAML(clusterId, 'Service', pendingYaml, false);
      message.success(t('network:editPage.serviceUpdateSuccess'));
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
      await ResourceService.applyYAML(clusterId, 'Service', currentYaml, true);
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
                {t('network:editPage.editService', { name: serviceName })}
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
                <Col span={12}><div style={{marginBottom: 8}}><label>名稱</label><Input value={name} disabled /></div></Col>
                <Col span={12}><div style={{marginBottom: 8}}><label>命名空間</label><Input value={namespace} disabled /></div></Col>
                <Col span={12}>
                  <div style={{marginBottom: 8}}><label>類型</label>
                    <Select value={formServiceType} onChange={setFormServiceType} style={{width: '100%'}}
                      options={['ClusterIP', 'NodePort', 'LoadBalancer', 'ExternalName'].map(t => ({value: t, label: t}))} />
                  </div>
                </Col>
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
            {/* 選擇器 */}
            <Card title="Pod 選擇器" extra={<Button size="small" icon={<PlusOutlined />} onClick={() => setFormSelector(p => [...p, {key: '', value: ''}])}>新增</Button>}>
              {formSelector.map((item, i) => (
                <Row key={i} gutter={8} style={{marginBottom: 8}}>
                  <Col span={10}><Input placeholder="key" value={item.key} onChange={e => setFormSelector(p => p.map((x, j) => j === i ? {...x, key: e.target.value} : x))} /></Col>
                  <Col span={10}><Input placeholder="value" value={item.value} onChange={e => setFormSelector(p => p.map((x, j) => j === i ? {...x, value: e.target.value} : x))} /></Col>
                  <Col span={4}><Button danger icon={<DeleteOutlined />} onClick={() => setFormSelector(p => p.filter((_, j) => j !== i))} /></Col>
                </Row>
              ))}
            </Card>
            {/* 連接埠 */}
            <Card title="連接埠" extra={<Button size="small" icon={<PlusOutlined />} onClick={() => setFormPorts(p => [...p, {name: '', protocol: 'TCP', port: 80, targetPort: '80'}])}>新增</Button>}>
              {formPorts.map((port, i) => (
                <Row key={i} gutter={8} style={{marginBottom: 8}} align="middle">
                  <Col span={4}><Input placeholder="名稱" value={port.name} onChange={e => setFormPorts(p => p.map((x, j) => j === i ? {...x, name: e.target.value} : x))} /></Col>
                  <Col span={4}>
                    <Select value={port.protocol} onChange={v => setFormPorts(p => p.map((x, j) => j === i ? {...x, protocol: v} : x))} style={{width: '100%'}}
                      options={['TCP', 'UDP', 'SCTP'].map(v => ({value: v, label: v}))} />
                  </Col>
                  <Col span={4}><InputNumber placeholder="Port" value={port.port} min={1} max={65535} style={{width: '100%'}} onChange={v => setFormPorts(p => p.map((x, j) => j === i ? {...x, port: v || 80} : x))} /></Col>
                  <Col span={4}><Input placeholder="TargetPort" value={port.targetPort} onChange={e => setFormPorts(p => p.map((x, j) => j === i ? {...x, targetPort: e.target.value} : x))} /></Col>
                  {formServiceType === 'NodePort' && <Col span={4}><InputNumber placeholder="NodePort" value={port.nodePort} min={30000} max={32767} style={{width: '100%'}} onChange={v => setFormPorts(p => p.map((x, j) => j === i ? {...x, nodePort: v || undefined} : x))} /></Col>}
                  <Col span={4}><Button danger icon={<DeleteOutlined />} onClick={() => setFormPorts(p => p.filter((_, j) => j !== i))} /></Col>
                </Row>
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

export default ServiceEdit;

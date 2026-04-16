import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Steps,
  Switch,
  Tag,
  Typography,
  App,
} from 'antd';
import { SwapOutlined, CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { clusterService } from '../../services/clusterService';
import { multiclusterService, type MigrateCheckResult, type MigrateResult } from '../../services/multiclusterService';
import { namespaceService } from '../../services/namespaceService';
import type { Cluster } from '../../types';

const { Text, Title } = Typography;

interface Props {
  open: boolean;
  onClose: () => void;
  onMigrated?: () => void;
}

interface WorkloadOption {
  name: string;
  kind: string;
  namespace: string;
  replicas?: number;
}

const MigrationWizard: React.FC<Props> = ({ open, onClose, onMigrated }) => {
  const { t } = useTranslation(['multicluster', 'common']);
  const { message } = App.useApp();
  const [step, setStep] = useState(0);
  const [loading, setLoading] = useState(false);

  // Data
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [srcNamespaces, setSrcNamespaces] = useState<string[]>([]);
  const [dstNamespaces, setDstNamespaces] = useState<string[]>([]);
  const [workloads, setWorkloads] = useState<WorkloadOption[]>([]);

  // Form state
  const [srcClusterId, setSrcClusterId] = useState<string | undefined>();
  const [srcNamespace, setSrcNamespace] = useState('');
  const [workloadKind, setWorkloadKind] = useState('Deployment');
  const [workloadName, setWorkloadName] = useState('');
  const [dstClusterId, setDstClusterId] = useState<string | undefined>();
  const [dstNamespace, setDstNamespace] = useState('');
  const [newNamespace, setNewNamespace] = useState('');
  const [syncConfigMaps, setSyncConfigMaps] = useState(false);
  const [syncSecrets, setSyncSecrets] = useState(false);

  // Check result
  const [checkResult, setCheckResult] = useState<MigrateCheckResult | null>(null);
  const [checking, setChecking] = useState(false);
  const [migrateResult, setMigrateResult] = useState<{ success: boolean; message: string } | null>(null);

  useEffect(() => {
    if (open) {
      clusterService.getClusters({ pageSize: 100 }).then((res) => {
        setClusters(res?.items ?? []);
      }).catch(() => {});
    }
  }, [open]);

  const loadSrcNamespaces = useCallback((cid: string) => {
    namespaceService.getNamespaces(cid)
      .then((res) => setSrcNamespaces(res.map((n) => n.name)))
      .catch(() => {});
  }, []);

  const loadDstNamespaces = useCallback((cid: string) => {
    namespaceService.getNamespaces(cid)
      .then((res) => setDstNamespaces(res.map((n) => n.name)))
      .catch(() => {});
  }, []);

  const loadWorkloads = useCallback(async (cid: string, ns: string, kind: string) => {
    if (!cid || !ns) return;
    try {
      const { WorkloadService } = await import('../../services/workloadService');
      const res = await WorkloadService.getWorkloads(cid, ns, kind, 1, 200);
      setWorkloads((res?.items ?? []).map((w) => ({ name: w.name, kind, namespace: w.namespace, replicas: w.replicas })));
    } catch {
      setWorkloads([]);
    }
  }, []);

  const handleCheckResources = async () => {
    if (!srcClusterId || !dstClusterId || !workloadName) return;
    setChecking(true);
    setCheckResult(null);
    try {
      const res = await multiclusterService.migrateCheck({
        sourceClusterId: srcClusterId,
        sourceNamespace: srcNamespace,
        workloadKind,
        workloadName,
        targetClusterId: dstClusterId,
        targetNamespace: dstNamespace || newNamespace,
        syncConfigMaps,
        syncSecrets,
      });
      setCheckResult((res as { data?: MigrateCheckResult })?.data ?? (res as MigrateCheckResult));
    } catch (e) {
      message.error(t('multicluster:migrationWizard.precheck', { error: String(e) }));
    } finally {
      setChecking(false);
    }
  };

  const handleMigrate = async () => {
    setLoading(true);
    setMigrateResult(null);
    try {
      const res = await multiclusterService.migrate({
        sourceClusterId: srcClusterId!,
        sourceNamespace: srcNamespace,
        workloadKind,
        workloadName,
        targetClusterId: dstClusterId!,
        targetNamespace: dstNamespace || newNamespace,
        syncConfigMaps,
        syncSecrets,
      });
      const data = (res as { data?: MigrateResult })?.data ?? (res as MigrateResult);
      setMigrateResult({ success: data.success, message: data.message });
      if (data.success) {
        onMigrated?.();
      }
      setStep(4);
    } catch (e) {
      setMigrateResult({ success: false, message: String(e) });
      setStep(4);
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setStep(0);
    setSrcClusterId(undefined); setSrcNamespace(''); setWorkloadName('');
    setDstClusterId(undefined); setDstNamespace(''); setNewNamespace('');
    setSyncConfigMaps(false); setSyncSecrets(false);
    setCheckResult(null); setMigrateResult(null);
    onClose();
  };

  const targetNS = dstNamespace || newNamespace;

  const stepContent = [
    // Step 0: Source
    <div key="s0" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Title level={5} style={{ margin: 0 }}>{t('multicluster:migrationWizard.step0.title')}</Title>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label={t('multicluster:migrationWizard.step0.sourceCluster')} required style={{ marginBottom: 8 }}>
            <Select
              placeholder={t('multicluster:migrationWizard.step0.selectCluster')}
              value={srcClusterId}
              onChange={v => { setSrcClusterId(v); setSrcNamespace(''); setWorkloadName(''); loadSrcNamespaces(v); }}
              options={clusters.map(c => ({ value: c.id, label: c.name }))}
              showSearch
              optionFilterProp="label"
            />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label={t('multicluster:migrationWizard.step0.namespace')} required style={{ marginBottom: 8 }}>
            <Select
              placeholder={t('multicluster:migrationWizard.step0.selectNamespace')}
              value={srcNamespace || undefined}
              onChange={v => { setSrcNamespace(v); setWorkloadName(''); loadWorkloads(srcClusterId!, v, workloadKind); }}
              options={srcNamespaces.map(n => ({ value: n, label: n }))}
              disabled={!srcClusterId}
              showSearch
            />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label={t('multicluster:migrationWizard.step0.workloadType')} style={{ marginBottom: 8 }}>
            <Select
              value={workloadKind}
              onChange={v => { setWorkloadKind(v); setWorkloadName(''); if (srcClusterId && srcNamespace) loadWorkloads(srcClusterId, srcNamespace, v); }}
              options={[
                { value: 'Deployment', label: 'Deployment' },
                { value: 'StatefulSet', label: 'StatefulSet' },
                { value: 'DaemonSet', label: 'DaemonSet' },
              ]}
            />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label={t('multicluster:migrationWizard.step0.workloadName')} required style={{ marginBottom: 8 }}>
            <Select
              placeholder={t('multicluster:migrationWizard.step0.selectWorkload')}
              value={workloadName || undefined}
              onChange={setWorkloadName}
              options={workloads.map(w => ({ value: w.name, label: `${w.name}${w.replicas != null ? ` (${w.replicas} ${t('multicluster:migrationWizard.step0.replicas')})` : ''}` }))}
              disabled={!srcNamespace}
              showSearch
              optionFilterProp="label"
            />
          </Form.Item>
        </Col>
      </Row>
    </div>,

    // Step 1: Target
    <div key="s1" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Title level={5} style={{ margin: 0 }}>{t('multicluster:migrationWizard.step1.title')}</Title>
      <Alert
        type="info"
        showIcon
        message={t('multicluster:migrationWizard.step1.migratingSummary', { kind: workloadKind, namespace: srcNamespace, name: workloadName })}
        style={{ marginBottom: 4 }}
      />
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label={t('multicluster:migrationWizard.step1.targetCluster')} required style={{ marginBottom: 8 }}>
            <Select
              placeholder={t('multicluster:migrationWizard.step1.selectTargetCluster')}
              value={dstClusterId}
              onChange={v => { setDstClusterId(v); setDstNamespace(''); loadDstNamespaces(v); }}
              options={clusters.filter(c => c.id !== srcClusterId).map(c => ({ value: c.id, label: c.name }))}
              showSearch
              optionFilterProp="label"
            />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label={t('multicluster:migrationWizard.step1.targetNamespace')} style={{ marginBottom: 8 }}>
            <Select
              placeholder={t('multicluster:migrationWizard.step1.selectExisting')}
              value={dstNamespace || undefined}
              onChange={v => { setDstNamespace(v); setNewNamespace(''); }}
              options={dstNamespaces.map(n => ({ value: n, label: n }))}
              disabled={!dstClusterId}
              allowClear
              showSearch
            />
          </Form.Item>
        </Col>
      </Row>
      {dstClusterId && !dstNamespace && (
        <Form.Item label={t('multicluster:migrationWizard.step1.createNewNamespace')} style={{ marginBottom: 0 }}>
          <Input
            placeholder={t('multicluster:migrationWizard.step1.enterNamespaceName')}
            value={newNamespace}
            onChange={e => setNewNamespace(e.target.value)}
          />
        </Form.Item>
      )}
    </div>,

    // Step 2: Options + Check
    <div key="s2" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Title level={5} style={{ margin: 0 }}>{t('multicluster:migrationWizard.step2.title')}</Title>
      <Row gutter={24}>
        <Col span={12}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Text>{t('multicluster:migrationWizard.step2.syncConfigMaps')}</Text>
              <Switch checked={syncConfigMaps} onChange={setSyncConfigMaps} />
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Text>{t('multicluster:migrationWizard.step2.syncSecrets')}</Text>
              <Switch checked={syncSecrets} onChange={setSyncSecrets} />
            </div>
          </Space>
        </Col>
      </Row>
      <Button onClick={handleCheckResources} loading={checking} type="dashed" block>
        {t('multicluster:migrationWizard.step2.runPrecheck')}
      </Button>
      {checkResult && (
        <Alert
          type={checkResult.feasible ? 'success' : 'error'}
          showIcon
          message={checkResult.message}
          description={
            <div style={{ fontSize: 12, marginTop: 4 }}>
              <div>{t('multicluster:migrationWizard.step2.workloadRequirements', { cpu: checkResult.workloadCpuReq.toFixed(0), mem: checkResult.workloadMemReq.toFixed(0) })}</div>
              <div>{t('multicluster:migrationWizard.step2.targetAvailable', { cpu: checkResult.targetFreeCpu.toFixed(0), mem: checkResult.targetFreeMem.toFixed(0) })}</div>
              {(checkResult.configMapCount > 0 || checkResult.secretCount > 0) && (
                <div>{t('multicluster:migrationWizard.step2.detectedDependencies', { configMaps: checkResult.configMapCount, secrets: checkResult.secretCount })}</div>
              )}
            </div>
          }
        />
      )}
    </div>,

    // Step 3: Confirm
    <div key="s3" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Title level={5} style={{ margin: 0 }}>{t('multicluster:migrationWizard.step3.title')}</Title>
      <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 8, padding: 16 }}>
        <Row gutter={[8, 8]}>
          <Col span={8}><Text type="secondary">{t('multicluster:migrationWizard.step3.sourceCluster')}</Text></Col>
          <Col span={16}><Text strong>{clusters.find(c => c.id === srcClusterId)?.name}</Text></Col>
          <Col span={8}><Text type="secondary">{t('multicluster:migrationWizard.step3.sourceNamespace')}</Text></Col>
          <Col span={16}><Tag color="blue">{srcNamespace}</Tag></Col>
          <Col span={8}><Text type="secondary">{t('multicluster:migrationWizard.step3.workload')}</Text></Col>
          <Col span={16}><Tag>{workloadKind}/{workloadName}</Tag></Col>
          <Col span={8}><Text type="secondary">{t('multicluster:migrationWizard.step3.targetCluster')}</Text></Col>
          <Col span={16}><Text strong>{clusters.find(c => c.id === dstClusterId)?.name}</Text></Col>
          <Col span={8}><Text type="secondary">{t('multicluster:migrationWizard.step3.targetNamespace')}</Text></Col>
          <Col span={16}><Tag color="green">{targetNS}</Tag></Col>
          <Col span={8}><Text type="secondary">{t('multicluster:migrationWizard.step3.syncConfigMaps')}</Text></Col>
          <Col span={16}><Tag color={syncConfigMaps ? 'green' : 'default'}>{syncConfigMaps ? t('multicluster:migrationWizard.step3.yes') : t('multicluster:migrationWizard.step3.no')}</Tag></Col>
          <Col span={8}><Text type="secondary">{t('multicluster:migrationWizard.step3.syncSecrets')}</Text></Col>
          <Col span={16}><Tag color={syncSecrets ? 'green' : 'default'}>{syncSecrets ? t('multicluster:migrationWizard.step3.yes') : t('multicluster:migrationWizard.step3.no')}</Tag></Col>
        </Row>
      </div>
      <Alert
        type="warning"
        showIcon
        message={t('multicluster:migrationWizard.step3.warning')}
      />
    </div>,

    // Step 4: Result
    <div key="s4" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, padding: '24px 0' }}>
      {migrateResult?.success ? (
        <>
          <CheckCircleOutlined style={{ fontSize: 48, color: '#52c41a' }} />
          <Title level={4} style={{ margin: 0, color: '#52c41a' }}>{t('multicluster:migrationWizard.step4.success')}</Title>
          <Text>{migrateResult.message}</Text>
        </>
      ) : (
        <>
          <CloseCircleOutlined style={{ fontSize: 48, color: '#ff4d4f' }} />
          <Title level={4} style={{ margin: 0, color: '#ff4d4f' }}>{t('multicluster:migrationWizard.step4.failed')}</Title>
          <Text type="danger">{migrateResult?.message}</Text>
        </>
      )}
    </div>,
  ];

  const canProceed = () => {
    if (step === 0) return !!(srcClusterId && srcNamespace && workloadName);
    if (step === 1) return !!(dstClusterId && (dstNamespace || newNamespace));
    if (step === 2) return true;
    if (step === 3) return true;
    return false;
  };

  return (
    <Modal
      open={open}
      title={<span><SwapOutlined style={{ marginRight: 8 }} />{t('multicluster:migrationWizard.title')}</span>}
      onCancel={handleClose}
      width={760}
      footer={null}
      destroyOnClose
    >
      <Steps
        current={step}
        style={{ marginBottom: 24 }}
        items={[
          { title: t('multicluster:migrationWizard.steps.source') },
          { title: t('multicluster:migrationWizard.steps.target') },
          { title: t('multicluster:migrationWizard.steps.options') },
          { title: t('multicluster:migrationWizard.steps.confirm') },
          { title: t('multicluster:migrationWizard.steps.complete') },
        ]}
        size="small"
      />

      <div style={{ minHeight: 280 }}>
        {stepContent[step]}
      </div>

      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 20 }}>
        <Button onClick={handleClose}>{step === 4 ? t('multicluster:migrationWizard.buttons.close') : t('multicluster:migrationWizard.buttons.cancel')}</Button>
        {step > 0 && step < 4 && (
          <Button onClick={() => setStep(s => s - 1)}>{t('multicluster:migrationWizard.buttons.previous')}</Button>
        )}
        {step < 3 && (
          <Button type="primary" disabled={!canProceed()} onClick={() => setStep(s => s + 1)}>
            {t('multicluster:migrationWizard.buttons.next')}
          </Button>
        )}
        {step === 3 && (
          <Button type="primary" danger loading={loading} onClick={handleMigrate}>
            {t('multicluster:migrationWizard.buttons.confirm')}
          </Button>
        )}
      </div>
    </Modal>
  );
};

export default MigrationWizard;

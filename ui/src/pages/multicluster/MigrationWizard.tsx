import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  Modal,
  Progress,
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
import { clusterService } from '../../services/clusterService';
import { multiclusterService, type MigrateCheckResult } from '../../services/multiclusterService';
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
  const { message } = App.useApp();
  const [step, setStep] = useState(0);
  const [loading, setLoading] = useState(false);

  // Data
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [srcNamespaces, setSrcNamespaces] = useState<string[]>([]);
  const [dstNamespaces, setDstNamespaces] = useState<string[]>([]);
  const [workloads, setWorkloads] = useState<WorkloadOption[]>([]);

  // Form state
  const [srcClusterId, setSrcClusterId] = useState<number | undefined>();
  const [srcNamespace, setSrcNamespace] = useState('');
  const [workloadKind, setWorkloadKind] = useState('Deployment');
  const [workloadName, setWorkloadName] = useState('');
  const [dstClusterId, setDstClusterId] = useState<number | undefined>();
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
      clusterService.getClusters({ pageSize: 100 }).then((res: any) => {
        setClusters(res?.items ?? res?.data?.items ?? []);
      }).catch(() => {});
    }
  }, [open]);

  const loadSrcNamespaces = useCallback((cid: number) => {
    namespaceService.getNamespaces(String(cid))
      .then((res: any) => setSrcNamespaces((res?.items ?? []).map((n: any) => n.name)))
      .catch(() => {});
  }, []);

  const loadDstNamespaces = useCallback((cid: number) => {
    namespaceService.getNamespaces(String(cid))
      .then((res: any) => setDstNamespaces((res?.items ?? []).map((n: any) => n.name)))
      .catch(() => {});
  }, []);

  const loadWorkloads = useCallback(async (cid: number, ns: string, kind: string) => {
    if (!cid || !ns) return;
    try {
      const { WorkloadService } = await import('../../services/workloadService');
      const res = await WorkloadService.getWorkloads(String(cid), ns, kind.toLowerCase() + 's', 1, 200);
      const items = (res as any)?.items ?? [];
      setWorkloads(items.map((w: any) => ({ name: w.name, kind, namespace: w.namespace, replicas: w.replicas })));
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
      setCheckResult((res as any)?.data ?? res as any);
    } catch (e) {
      message.error('預檢失敗：' + String(e));
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
      const data = (res as any)?.data ?? res as any;
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
      <Title level={5} style={{ margin: 0 }}>選擇來源工作負載</Title>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label="來源叢集" required style={{ marginBottom: 8 }}>
            <Select
              placeholder="選擇叢集"
              value={srcClusterId}
              onChange={v => { setSrcClusterId(v); setSrcNamespace(''); setWorkloadName(''); loadSrcNamespaces(v); }}
              options={clusters.map(c => ({ value: c.id, label: c.name }))}
              showSearch
              optionFilterProp="label"
            />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label="命名空間" required style={{ marginBottom: 8 }}>
            <Select
              placeholder="選擇命名空間"
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
          <Form.Item label="工作負載類型" style={{ marginBottom: 8 }}>
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
          <Form.Item label="工作負載名稱" required style={{ marginBottom: 8 }}>
            <Select
              placeholder="選擇工作負載"
              value={workloadName || undefined}
              onChange={setWorkloadName}
              options={workloads.map(w => ({ value: w.name, label: `${w.name}${w.replicas != null ? ` (${w.replicas} 副本)` : ''}` }))}
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
      <Title level={5} style={{ margin: 0 }}>選擇目標叢集</Title>
      <Alert
        type="info"
        showIcon
        message={`將遷移：${workloadKind} / ${srcNamespace} / ${workloadName}`}
        style={{ marginBottom: 4 }}
      />
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item label="目標叢集" required style={{ marginBottom: 8 }}>
            <Select
              placeholder="選擇目標叢集"
              value={dstClusterId}
              onChange={v => { setDstClusterId(v); setDstNamespace(''); loadDstNamespaces(v); }}
              options={clusters.filter(c => c.id !== srcClusterId).map(c => ({ value: c.id, label: c.name }))}
              showSearch
              optionFilterProp="label"
            />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item label="目標命名空間" style={{ marginBottom: 8 }}>
            <Select
              placeholder="選擇現有命名空間（或在下方建立新的）"
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
        <Form.Item label="或建立新命名空間" style={{ marginBottom: 0 }}>
          <Input
            placeholder="輸入新命名空間名稱"
            value={newNamespace}
            onChange={e => setNewNamespace(e.target.value)}
          />
        </Form.Item>
      )}
    </div>,

    // Step 2: Options + Check
    <div key="s2" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Title level={5} style={{ margin: 0 }}>同步選項與資源預檢</Title>
      <Row gutter={24}>
        <Col span={12}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Text>同步相依 ConfigMap</Text>
              <Switch checked={syncConfigMaps} onChange={setSyncConfigMaps} />
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Text>同步相依 Secret</Text>
              <Switch checked={syncSecrets} onChange={setSyncSecrets} />
            </div>
          </Space>
        </Col>
      </Row>
      <Button onClick={handleCheckResources} loading={checking} type="dashed" block>
        執行資源預檢
      </Button>
      {checkResult && (
        <Alert
          type={checkResult.feasible ? 'success' : 'error'}
          showIcon
          message={checkResult.message}
          description={
            <div style={{ fontSize: 12, marginTop: 4 }}>
              <div>工作負載需求：CPU {checkResult.workloadCpuReq.toFixed(0)}m / MEM {checkResult.workloadMemReq.toFixed(0)} MiB</div>
              <div>目標叢集可用：CPU {checkResult.targetFreeCpu.toFixed(0)}m / MEM {checkResult.targetFreeMem.toFixed(0)} MiB</div>
              {(checkResult.configMapCount > 0 || checkResult.secretCount > 0) && (
                <div>偵測到相依：{checkResult.configMapCount} 個 ConfigMap，{checkResult.secretCount} 個 Secret</div>
              )}
            </div>
          }
        />
      )}
    </div>,

    // Step 3: Confirm
    <div key="s3" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Title level={5} style={{ margin: 0 }}>確認遷移</Title>
      <div style={{ background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 8, padding: 16 }}>
        <Row gutter={[8, 8]}>
          <Col span={8}><Text type="secondary">來源叢集</Text></Col>
          <Col span={16}><Text strong>{clusters.find(c => c.id === srcClusterId)?.name}</Text></Col>
          <Col span={8}><Text type="secondary">來源命名空間</Text></Col>
          <Col span={16}><Tag color="blue">{srcNamespace}</Tag></Col>
          <Col span={8}><Text type="secondary">工作負載</Text></Col>
          <Col span={16}><Tag>{workloadKind}/{workloadName}</Tag></Col>
          <Col span={8}><Text type="secondary">目標叢集</Text></Col>
          <Col span={16}><Text strong>{clusters.find(c => c.id === dstClusterId)?.name}</Text></Col>
          <Col span={8}><Text type="secondary">目標命名空間</Text></Col>
          <Col span={16}><Tag color="green">{targetNS}</Tag></Col>
          <Col span={8}><Text type="secondary">同步 ConfigMap</Text></Col>
          <Col span={16}><Tag color={syncConfigMaps ? 'green' : 'default'}>{syncConfigMaps ? '是' : '否'}</Tag></Col>
          <Col span={8}><Text type="secondary">同步 Secret</Text></Col>
          <Col span={16}><Tag color={syncSecrets ? 'green' : 'default'}>{syncSecrets ? '是' : '否'}</Tag></Col>
        </Row>
      </div>
      <Alert
        type="warning"
        showIcon
        message="請確認以上資訊無誤，遷移操作將在目標叢集建立工作負載（若已存在則覆蓋）。"
      />
    </div>,

    // Step 4: Result
    <div key="s4" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, padding: '24px 0' }}>
      {migrateResult?.success ? (
        <>
          <CheckCircleOutlined style={{ fontSize: 48, color: '#52c41a' }} />
          <Title level={4} style={{ margin: 0, color: '#52c41a' }}>遷移成功</Title>
          <Text>{migrateResult.message}</Text>
        </>
      ) : (
        <>
          <CloseCircleOutlined style={{ fontSize: 48, color: '#ff4d4f' }} />
          <Title level={4} style={{ margin: 0, color: '#ff4d4f' }}>遷移失敗</Title>
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
      title={<span><SwapOutlined style={{ marginRight: 8 }} />跨叢集工作負載遷移精靈</span>}
      onCancel={handleClose}
      width={760}
      footer={null}
      destroyOnClose
    >
      <Steps
        current={step}
        style={{ marginBottom: 24 }}
        items={[
          { title: '來源' },
          { title: '目標' },
          { title: '選項' },
          { title: '確認' },
          { title: '完成' },
        ]}
        size="small"
      />

      <div style={{ minHeight: 280 }}>
        {stepContent[step]}
      </div>

      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 20 }}>
        <Button onClick={handleClose}>{step === 4 ? '關閉' : '取消'}</Button>
        {step > 0 && step < 4 && (
          <Button onClick={() => setStep(s => s - 1)}>上一步</Button>
        )}
        {step < 3 && (
          <Button type="primary" disabled={!canProceed()} onClick={() => setStep(s => s + 1)}>
            下一步
          </Button>
        )}
        {step === 3 && (
          <Button type="primary" danger loading={loading} onClick={handleMigrate}>
            確認執行遷移
          </Button>
        )}
      </div>
    </Modal>
  );
};

export default MigrationWizard;

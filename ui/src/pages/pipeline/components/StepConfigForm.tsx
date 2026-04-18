/**
 * StepConfigForm — 根據 Step 類型動態渲染設定欄位
 *
 * 每個類型對應後端的 Config struct，欄位名稱與 JSON key 一致。
 */
import React from 'react';
import { Input, InputNumber, Select, Switch, Flex, Typography, theme } from 'antd';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

interface StepConfigFormProps {
  stepType: string;
  config: Record<string, unknown>;
  command?: string;
  image?: string;
  onChange: (patch: { config?: Record<string, unknown>; command?: string; image?: string }) => void;
  registryOptions?: { label: string; value: string }[];
}

// ─── Field helper ──────────────────────────────────────────────────────────

interface FieldProps {
  label: string;
  tooltip?: string;
  required?: boolean;
  children: React.ReactNode;
  span?: number; // flex basis
}

const Field: React.FC<FieldProps> = ({ label, required, children, span }) => {
  const { token } = theme.useToken();
  return (
    <div style={{ flex: span ?? 1, minWidth: 0 }}>
      <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
        {label}{required && <span style={{ color: token.colorError }}> *</span>}
      </Text>
      {children}
    </div>
  );
};

// ─── Config updater ────────────────────────────────────────────────────────

const useConfigUpdater = (
  config: Record<string, unknown>,
  onChange: StepConfigFormProps['onChange'],
) => {
  return (key: string, value: unknown) => {
    const newConfig = { ...config, [key]: value };
    // Remove empty strings and undefined
    if (value === '' || value === undefined) {
      delete newConfig[key];
    }
    onChange({ config: newConfig });
  };
};

// ─── Type-specific forms ───────────────────────────────────────────────────

const StepConfigForm: React.FC<StepConfigFormProps> = ({
  stepType,
  config,
  command,
  image,
  onChange,
  registryOptions,
}) => {
  const { token } = theme.useToken();
  const { t } = useTranslation(['pipeline']);
  const set = useConfigUpdater(config, onChange);
  const gap = token.marginMD;

  switch (stepType) {
    // ── build-image ────────────────────────────────────────────────────
    case 'build-image':
      return (
        <Flex vertical gap={gap}>
          <Flex gap={gap}>
            <Field label="映像推送位址" required>
              <Input
                value={config.destination as string}
                onChange={(e) => set('destination', e.target.value)}
                placeholder="harbor.example.com/project/app:latest"
              />
            </Field>
            <Field label="Registry" span={0.6}>
              <Select
                value={config.registry as string}
                onChange={(v) => set('registry', v)}
                options={registryOptions}
                placeholder="選擇 Registry"
                allowClear
                style={{ width: '100%' }}
              />
            </Field>
          </Flex>
          <Flex gap={gap}>
            <Field label="Dockerfile">
              <Input
                value={config.dockerfile as string}
                onChange={(e) => set('dockerfile', e.target.value)}
                placeholder="Dockerfile"
              />
            </Field>
            <Field label="Build Context">
              <Input
                value={config.context as string}
                onChange={(e) => set('context', e.target.value)}
                placeholder="."
              />
            </Field>
          </Flex>
          <Flex gap={gap} align="center">
            <Field label="啟用快取" span={0.3}>
              <Switch
                checked={config.cache as boolean}
                onChange={(v) => set('cache', v)}
              />
            </Field>
            {config.cache && (
              <Field label="快取 Repo">
                <Input
                  value={config.cache_repo as string}
                  onChange={(e) => set('cache_repo', e.target.value)}
                  placeholder="harbor.example.com/cache"
                />
              </Field>
            )}
          </Flex>
        </Flex>
      );

    // ── deploy ──────────────────────────────────────────────────────────
    case 'deploy':
      return (
        <Flex vertical gap={gap}>
          <Flex gap={gap}>
            <Field label="Manifest 檔案" required>
              <Select
                mode="tags"
                value={config.manifests as string[] ?? (config.manifest ? [config.manifest as string] : [])}
                onChange={(v) => set('manifests', v)}
                style={{ width: '100%' }}
                placeholder="k8s/deployment.yaml"
                tokenSeparators={[',']}
              />
            </Field>
            <Field label="Namespace" span={0.5}>
              <Input
                value={config.namespace as string}
                onChange={(e) => set('namespace', e.target.value)}
                placeholder="default"
              />
            </Field>
          </Flex>
          <Field label="Dry Run" span={0.3}>
            <Switch
              checked={config.dry_run as boolean}
              onChange={(v) => set('dry_run', v)}
            />
          </Field>
        </Flex>
      );

    // ── trivy-scan ──────────────────────────────────────────────────────
    case 'trivy-scan':
      return (
        <Flex vertical gap={gap}>
          <Field label="掃描映像" required>
            <Input
              value={config.image as string}
              onChange={(e) => set('image', e.target.value)}
              placeholder="harbor.example.com/project/app:latest"
            />
          </Field>
          <Flex gap={gap}>
            <Field label="嚴重程度篩選">
              <Select
                mode="tags"
                value={config.severity ? (config.severity as string).split(',') : []}
                onChange={(v) => set('severity', v.join(','))}
                options={[
                  { label: 'CRITICAL', value: 'CRITICAL' },
                  { label: 'HIGH', value: 'HIGH' },
                  { label: 'MEDIUM', value: 'MEDIUM' },
                  { label: 'LOW', value: 'LOW' },
                ]}
                placeholder="全部"
                style={{ width: '100%' }}
                allowClear
              />
            </Field>
            <Field label="輸出格式" span={0.5}>
              <Select
                value={config.format as string}
                onChange={(v) => set('format', v)}
                options={[
                  { label: 'Table', value: 'table' },
                  { label: 'JSON', value: 'json' },
                  { label: 'SARIF', value: 'sarif' },
                ]}
                placeholder="table"
                style={{ width: '100%' }}
                allowClear
              />
            </Field>
          </Flex>
        </Flex>
      );

    // ── push-image ──────────────────────────────────────────────────────
    case 'push-image':
      return (
        <Flex vertical gap={gap}>
          <Flex gap={gap}>
            <Field label="來源映像位址" required>
              <Input
                value={config.source as string}
                onChange={(e) => set('source', e.target.value)}
                placeholder="harbor.example.com/staging/app:v1"
              />
            </Field>
            <Field label="目標推送位址" required>
              <Input
                value={config.destination as string}
                onChange={(e) => set('destination', e.target.value)}
                placeholder="harbor.example.com/prod/app:v1"
              />
            </Field>
          </Flex>
          <Field label="Registry" span={0.5}>
            <Select
              value={config.registry as string}
              onChange={(v) => set('registry', v)}
              options={registryOptions}
              placeholder="選擇 Registry"
              allowClear
              style={{ width: '100%' }}
            />
          </Field>
        </Flex>
      );

    // ── deploy-helm ─────────────────────────────────────────────────────
    case 'deploy-helm':
      return (
        <Flex vertical gap={gap}>
          <Flex gap={gap}>
            <Field label="Release 名稱" required>
              <Input
                value={config.release as string}
                onChange={(e) => set('release', e.target.value)}
                placeholder="my-app"
              />
            </Field>
            <Field label="Chart" required>
              <Input
                value={config.chart as string}
                onChange={(e) => set('chart', e.target.value)}
                placeholder="./charts/my-app 或 repo/chart"
              />
            </Field>
            <Field label="Namespace" span={0.5}>
              <Input
                value={config.namespace as string}
                onChange={(e) => set('namespace', e.target.value)}
                placeholder="default"
              />
            </Field>
          </Flex>
          <Flex gap={gap}>
            <Field label="Values 檔案">
              <Input
                value={config.values as string}
                onChange={(e) => set('values', e.target.value)}
                placeholder="values-prod.yaml"
              />
            </Field>
            <Field label="Chart 版本" span={0.5}>
              <Input
                value={config.version as string}
                onChange={(e) => set('version', e.target.value)}
                placeholder="latest"
              />
            </Field>
          </Flex>
          <Flex gap={gap} align="center">
            <Field label="Wait" span={0.3}>
              <Switch checked={config.wait as boolean} onChange={(v) => set('wait', v)} />
            </Field>
            <Field label="Dry Run" span={0.3}>
              <Switch checked={config.dry_run as boolean} onChange={(v) => set('dry_run', v)} />
            </Field>
            {config.wait && (
              <Field label="Timeout">
                <Input
                  value={config.timeout as string}
                  onChange={(e) => set('timeout', e.target.value)}
                  placeholder="5m"
                />
              </Field>
            )}
          </Flex>
        </Flex>
      );

    // ── build-jar ───────────────────────────────────────────────────────
    case 'build-jar':
      return (
        <Flex vertical gap={gap}>
          <Flex gap={gap}>
            <Field label="構建工具">
              <Select
                value={(config.build_tool as string) ?? 'maven'}
                onChange={(v) => set('build_tool', v)}
                options={[
                  { label: 'Maven', value: 'maven' },
                  { label: 'Gradle', value: 'gradle' },
                ]}
                style={{ width: '100%' }}
              />
            </Field>
            <Field label="Java 版本">
              <Select
                value={config.java_version as string}
                onChange={(v) => set('java_version', v)}
                options={[
                  { label: '17', value: '17' },
                  { label: '21', value: '21' },
                  { label: '11', value: '11' },
                ]}
                placeholder="17 (預設)"
                allowClear
                style={{ width: '100%' }}
              />
            </Field>
          </Flex>
          <Field label={(config.build_tool === 'gradle') ? 'Gradle Tasks' : 'Maven Goals'}>
            <Input
              value={((config.build_tool === 'gradle') ? config.tasks : config.goals) as string}
              onChange={(e) => set(
                (config.build_tool === 'gradle') ? 'tasks' : 'goals',
                e.target.value,
              )}
              placeholder={(config.build_tool === 'gradle') ? 'clean build -x test' : 'clean package -DskipTests'}
            />
          </Field>
        </Flex>
      );

    // ── deploy-argocd-sync ──────────────────────────────────────────────
    case 'deploy-argocd-sync':
      return (
        <Flex vertical gap={gap}>
          <Flex gap={gap}>
            <Field label="ArgoCD App 名稱" required>
              <Input
                value={config.app_name as string}
                onChange={(e) => set('app_name', e.target.value)}
                placeholder="my-app"
              />
            </Field>
            <Field label="Server URL">
              <Input
                value={config.server as string}
                onChange={(e) => set('server', e.target.value)}
                placeholder="argocd-server.argocd.svc"
              />
            </Field>
          </Flex>
          <Flex gap={gap} align="center">
            <Field label="Prune" span={0.3}>
              <Switch checked={config.prune as boolean} onChange={(v) => set('prune', v)} />
            </Field>
            <Field label="Wait" span={0.3}>
              <Switch checked={config.wait as boolean} onChange={(v) => set('wait', v)} />
            </Field>
            <Field label="Dry Run" span={0.3}>
              <Switch checked={config.dry_run as boolean} onChange={(v) => set('dry_run', v)} />
            </Field>
          </Flex>
        </Flex>
      );

    // ── smoke-test ──────────────────────────────────────────────────────
    case 'smoke-test':
      return (
        <Flex vertical gap={gap}>
          <Flex gap={gap}>
            <Field label="URL" required>
              <Input
                value={config.url as string}
                onChange={(e) => set('url', e.target.value)}
                placeholder="http://my-service:8080/health"
              />
            </Field>
            <Field label="HTTP Method" span={0.4}>
              <Select
                value={(config.method as string) ?? 'GET'}
                onChange={(v) => set('method', v)}
                options={['GET', 'POST', 'PUT', 'HEAD'].map((m) => ({ label: m, value: m }))}
                style={{ width: '100%' }}
              />
            </Field>
            <Field label="預期狀態碼" span={0.4}>
              <InputNumber
                value={(config.expected_status as number) ?? 200}
                onChange={(v) => set('expected_status', v)}
                style={{ width: '100%' }}
              />
            </Field>
          </Flex>
          <Flex gap={gap}>
            <Field label="重試次數" span={0.5}>
              <InputNumber
                value={(config.retries as number) ?? 3}
                onChange={(v) => set('retries', v)}
                min={0}
                max={20}
                style={{ width: '100%' }}
              />
            </Field>
            <Field label="重試間隔 (秒)" span={0.5}>
              <InputNumber
                value={(config.retry_interval as number) ?? 5}
                onChange={(v) => set('retry_interval', v)}
                min={1}
                style={{ width: '100%' }}
              />
            </Field>
          </Flex>
        </Flex>
      );

    // ── notify ──────────────────────────────────────────────────────────
    case 'notify':
      return (
        <Flex vertical gap={gap}>
          <Field label="Webhook URL" required>
            <Input
              value={config.url as string}
              onChange={(e) => set('url', e.target.value)}
              placeholder="https://hooks.slack.com/services/..."
            />
          </Field>
          <Field label="Body（JSON 模板）">
            <Input.TextArea
              rows={3}
              value={config.body as string}
              onChange={(e) => set('body', e.target.value)}
              placeholder='{"text": "Pipeline {{.RunID}} completed"}'
            />
          </Field>
        </Flex>
      );

    // ── approval ────────────────────────────────────────────────────────
    case 'approval':
      return (
        <Text type="secondary" style={{ display: 'block', padding: `${token.paddingSM}px 0` }}>
          此步驟會暫停 Pipeline 執行，等待人工點擊「核准」或「拒絕」後才繼續。
        </Text>
      );

    // ── shell / run-script / custom ─────────────────────────────────────
    case 'shell':
    case 'run-script':
    case 'custom':
      return (
        <Flex vertical gap={gap}>
          {stepType === 'custom' && (
            <Field label="容器映像" required>
              <Input
                value={image}
                onChange={(e) => onChange({ image: e.target.value })}
                placeholder="alpine:3.20"
              />
            </Field>
          )}
          <Field label="命令" required>
            <Input.TextArea
              rows={3}
              value={command}
              onChange={(e) => onChange({ command: e.target.value })}
              placeholder="echo 'Hello World'"
              style={{ fontFamily: 'monospace' }}
            />
          </Field>
        </Flex>
      );

    // ── deploy-rollout ──────────────────────────────────────────────────
    case 'deploy-rollout':
      return (
        <Flex vertical gap={gap}>
          <Flex gap={gap}>
            <Field label="Rollout 名稱" required>
              <Input
                value={config.rollout_name as string}
                onChange={(e) => set('rollout_name', e.target.value)}
                placeholder="my-rollout"
              />
            </Field>
            <Field label="Namespace" required>
              <Input
                value={config.namespace as string}
                onChange={(e) => set('namespace', e.target.value)}
                placeholder="default"
              />
            </Field>
            <Field label="映像" required>
              <Input
                value={config.image as string}
                onChange={(e) => set('image', e.target.value)}
                placeholder="harbor.example.com/app:v2"
              />
            </Field>
          </Flex>
        </Flex>
      );

    // ── gitops-sync ─────────────────────────────────────────────────────
    case 'gitops-sync':
      return (
        <Text type="secondary" style={{ display: 'block', padding: `${token.paddingSM}px 0` }}>
          Git commit + push 更新 GitOps 倉庫。設定在 Pipeline 的 Project 關聯中。
        </Text>
      );

    // ── fallback ────────────────────────────────────────────────────────
    default:
      return (
        <Field label="Config（JSON）">
          <Input.TextArea
            rows={4}
            value={JSON.stringify(config, null, 2)}
            onChange={(e) => {
              try {
                onChange({ config: JSON.parse(e.target.value) });
              } catch {
                // ignore parse errors while typing
              }
            }}
            style={{ fontFamily: 'monospace' }}
          />
        </Field>
      );
  }
};

export default StepConfigForm;

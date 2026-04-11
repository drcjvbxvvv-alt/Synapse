/**
 * Pipeline 共用型別與常數
 */
import React from 'react';
import {
  SecurityScanOutlined,
  CloudUploadOutlined,
  RocketOutlined,
  CodeOutlined,
  ContainerOutlined,
} from '@ant-design/icons';

// ─── 型別 ─────────────────────────────────────────────────────────────────────

export type StepStatus = 'idle' | 'running' | 'success' | 'failed' | 'skipped';
export type Scenario   = 'success' | 'trivy-fail';

export interface StepDef {
  id: string;
  label: string;
  subLabel: string;
  icon: React.ReactNode;
  duration: number;   // ms，模擬執行時間
  mockLogs: string[];
}

// ─── 步驟定義 ──────────────────────────────────────────────────────────────────

export const STEPS: StepDef[] = [
  {
    id: 'build-jar',
    label: 'Build JAR',
    subLabel: 'Maven 3.9',
    icon: React.createElement(CodeOutlined),
    duration: 3200,
    mockLogs: [
      '[INFO] Scanning for projects...',
      '[INFO] Building backend-service 2.1.4-SNAPSHOT',
      '[INFO] --- maven-compiler-plugin:3.11:compile ---',
      '[INFO] Compiling 247 source files to /workspace/target/classes',
      '[INFO] --- maven-surefire-plugin:3.0:test ---',
      '[INFO] Tests run: 142, Failures: 0, Errors: 0, Skipped: 3',
      '[INFO] --- maven-jar-plugin:3.3:jar ---',
      '[INFO] Building jar: /workspace/target/backend-service.jar',
      '[INFO] BUILD SUCCESS',
      '[INFO] Total time: 52.3 s',
    ],
  },
  {
    id: 'build-image',
    label: 'Build Image',
    subLabel: 'Kaniko',
    icon: React.createElement(ContainerOutlined),
    duration: 4100,
    mockLogs: [
      'INFO[0000] Retrieving image manifest openjdk:17-slim',
      'INFO[0002] Unpacking rootfs as cmd COPY requires it.',
      'INFO[0004] COPY target/backend-service.jar /app/app.jar',
      'INFO[0004] RUN addgroup -S app && adduser -S app -G app',
      'INFO[0006] EXPOSE 8080',
      'INFO[0006] ENTRYPOINT ["java", "-jar", "/app/app.jar"]',
      'INFO[0007] Pushing image to harbor.company.com/prod/backend-service:abc1234',
      'INFO[0009] Pushed image digest: sha256:4f2a9c...',
    ],
  },
  {
    id: 'trivy-scan',
    label: 'Trivy Scan',
    subLabel: 'Security',
    icon: React.createElement(SecurityScanOutlined),
    duration: 2800,
    mockLogs: [
      '2024-04-06T10:23:01Z INFO  Vulnerability scanning is enabled',
      '2024-04-06T10:23:01Z INFO  Detected OS: debian 11.8',
      '2024-04-06T10:23:03Z INFO  Scanning packages...',
      '2024-04-06T10:23:05Z INFO  Scanning library vulnerabilities...',
      '',
      'harbor.company.com/prod/backend-service:abc1234',
      '═══════════════════════════════════',
      'Total: 3 (CRITICAL: 0, HIGH: 1, MEDIUM: 2, LOW: 0)',
      '',
      '┌─────────────────┬──────────────────┬──────────┬──────────────────┐',
      '│ Library         │ Vulnerability    │ Severity │ Fixed Version    │',
      '├─────────────────┼──────────────────┼──────────┼──────────────────┤',
      '│ log4j-core      │ CVE-2024-23672   │ HIGH     │ 2.23.1           │',
      '│ commons-text    │ CVE-2024-11024   │ MEDIUM   │ 1.11.0           │',
      '│ jackson-databind│ CVE-2024-28849   │ MEDIUM   │ 2.16.2           │',
      '└─────────────────┴──────────────────┴──────────┴──────────────────┘',
      '',
      '✅ No CRITICAL vulnerabilities. Threshold not exceeded.',
    ],
  },
  {
    id: 'push-harbor',
    label: 'Push Harbor',
    subLabel: 'Registry',
    icon: React.createElement(CloudUploadOutlined),
    duration: 1800,
    mockLogs: [
      'Logging into harbor.company.com...',
      'Login Succeeded',
      'Pushing harbor.company.com/prod/backend-service:abc1234',
      'The push refers to repository [harbor.company.com/prod/backend-service]',
      '4f2a9c...: Pushed',
      'abc1234: digest: sha256:9d8e2f... size: 1847',
      '',
      '✅ Image successfully pushed to Harbor',
      '   Repository: harbor.company.com/prod/backend-service',
      '   Tag:        abc1234',
      '   Digest:     sha256:9d8e2f...',
    ],
  },
  {
    id: 'deploy',
    label: 'Deploy',
    subLabel: 'K8s',
    icon: React.createElement(RocketOutlined),
    duration: 2400,
    mockLogs: [
      'Applying manifests to cluster: production-k8s',
      'Namespace: app-production',
      '',
      'deployment.apps/backend-service configured',
      'service/backend-service unchanged',
      '',
      'Waiting for rollout to finish...',
      'Waiting for deployment "backend-service" rollout to finish: 1 out of 3 new replicas have been updated...',
      'Waiting for deployment "backend-service" rollout to finish: 2 out of 3 new replicas have been updated...',
      'Waiting for deployment "backend-service" rollout to finish: 1 old replicas are pending termination...',
      'Waiting for deployment "backend-service" rollout to finish: 0 old replicas are pending termination...',
      '',
      '✅ deployment "backend-service" successfully rolled out',
      '   Ready: 3/3  |  Up-to-date: 3  |  Available: 3',
    ],
  },
];

// ─── Trivy 失敗場景 log ────────────────────────────────────────────────────────

export const TRIVY_FAIL_LOGS = [
  '2024-04-06T10:23:01Z INFO  Vulnerability scanning is enabled',
  '2024-04-06T10:23:01Z INFO  Detected OS: debian 11.8',
  '2024-04-06T10:23:03Z INFO  Scanning packages...',
  '',
  'harbor.company.com/prod/backend-service:abc1234',
  '═══════════════════════════════════',
  'Total: 8 (CRITICAL: 3, HIGH: 4, MEDIUM: 1)',
  '',
  '┌──────────────────┬──────────────────┬──────────┬──────────────────┐',
  '│ Library          │ Vulnerability    │ Severity │ Fixed Version    │',
  '├──────────────────┼──────────────────┼──────────┼──────────────────┤',
  '│ log4j-core       │ CVE-2021-44228   │ CRITICAL │ 2.17.1           │',
  '│ log4j-core       │ CVE-2021-45105   │ CRITICAL │ 2.17.1           │',
  '│ openssl          │ CVE-2023-0286    │ CRITICAL │ 3.0.9            │',
  '│ spring-webmvc    │ CVE-2024-22233   │ HIGH     │ 6.1.4            │',
  '└──────────────────┴──────────────────┴──────────┴──────────────────┘',
  '',
  '❌ CRITICAL vulnerabilities found: 3',
  '   Threshold: CRITICAL=0',
  '   Pipeline FAILED — image will NOT be pushed to Harbor',
];

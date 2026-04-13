import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Table,
  Button,
  Space,
  Spin,
  Tag,
  Typography,
  theme,
} from 'antd';
import EmptyState from '@/components/EmptyState';
import NotInstalledCard from '@/components/NotInstalledCard';
import { ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import securityService from '@/services/securityService';
import type { GatekeeperSummary, ConstraintSummary } from '@/services/securityService';

const { Text } = Typography;

interface GatekeeperTabProps {
  clusterId: number;
}

export function GatekeeperTab({ clusterId }: GatekeeperTabProps) {
  const { t } = useTranslation('security');
  const { token } = theme.useToken();
  const [data, setData] = useState<GatekeeperSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const summary = await securityService.getGatekeeperViolations(clusterId);
      setData(summary);
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } }; message?: string };
      setError(err?.response?.data?.message ?? err?.message ?? t('gatekeeper.notInstalled'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const columns: ColumnsType<ConstraintSummary> = [
    {
      title: t('gatekeeper.constraintKind'),
      dataIndex: 'kind',
      key: 'kind',
      width: 200,
    },
    {
      title: t('gatekeeper.constraintName'),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: t('gatekeeper.violationCount'),
      dataIndex: 'violation_count',
      key: 'count',
      width: 120,
      render: (v: number) => (
        v > 0
          ? <Tag color="red">{v}</Tag>
          : <Tag color="green">0</Tag>
      ),
    },
  ];

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: token.paddingXL }}><Spin /></div>
    );
  }

  if (error || data?.installed === false) {
    return (
      <NotInstalledCard
        title={t('gatekeeper.notDetectedMsg')}
        description={t('gatekeeper.notDetectedDesc')}
        command="kubectl apply -f https://raw.githubusercontent.com/open-policy-agent/gatekeeper/v3.22.0/deploy/gatekeeper.yaml"
        docsUrl="https://open-policy-agent.github.io/gatekeeper/website/docs/install/"
        onRecheck={fetchData}
        recheckLoading={loading}
      />
    );
  }

  return (
    <>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        {data && (
          <Space>
            <Text>
              {t('gatekeeper.totalViolations')}:{' '}
              <Text strong style={{ color: data.total_violations > 0 ? token.colorError : token.colorSuccess }}>
                {data.total_violations}
              </Text>
            </Text>
          </Space>
        )}
        <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>
          {t('gatekeeper.refresh')}
        </Button>
      </div>

      <Table
        scroll={{ x: 'max-content' }}
        dataSource={data?.constraints ?? []}
        columns={columns}
        rowKey={(r) => r.kind + r.name}
        size="small"
        locale={{ emptyText: <EmptyState description={t('gatekeeper.noData')} /> }}
        pagination={false}
        expandable={{
          expandedRowRender: (record) => (
            <Table
              scroll={{ x: 'max-content' }}
              dataSource={record.violations}
              columns={[
                { title: t('gatekeeper.namespace'), dataIndex: 'namespace', key: 'ns', width: 140, render: (v) => v || '-' },
                { title: t('gatekeeper.resource'), dataIndex: 'resource', key: 'res', width: 200 },
                { title: t('gatekeeper.message'), dataIndex: 'message', key: 'msg' },
              ]}
              rowKey={(v) => v.resource + v.namespace + v.message}
              size="small"
              pagination={false}
            />
          ),
          rowExpandable: (record) => record.violation_count > 0,
        }}
      />
    </>
  );
}

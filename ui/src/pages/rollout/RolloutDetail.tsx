/**
 * RolloutDetail — Argo Rollout 詳情頁（P3-3）
 *
 * 功能：
 *  - 顯示 Rollout 基本資訊、副本狀態
 *  - Canary 步驟進度視覺化（Steps Timeline）
 *  - BlueGreen active/preview selector 顯示
 *  - Analysis Runs 列表
 *  - Promote / PromoteFull / Abort / Delete 操作
 *  - 3s 輪詢（Stopped/Degraded 狀態停止輪詢）
 */
import React, { useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Button,
  Space,
  Tag,
  Breadcrumb,
  Descriptions,
  Table,
  App,
  theme,
  Spin,
  Typography,
  Flex,
  Tooltip,
  Popconfirm,
  Progress,
  Card,
  Steps,
  Row,
  Col,
  Statistic,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  ArrowLeftOutlined,
  ReloadOutlined,
  FastForwardOutlined,
  StopOutlined,
  RocketOutlined,
  DeleteOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import rolloutService, { type RolloutInfo, type AnalysisRun } from '../../services/rolloutService';
import EmptyState from '../../components/EmptyState';

const { Text, Title } = Typography;

// ─── Status tag ───────────────────────────────────────────────────────────────

function RolloutStatusTag({ status }: { status: string }) {
  const { t } = useTranslation('rollout');
  const colorMap: Record<string, string> = {
    Healthy: 'success',
    Stopped: 'default',
    Degraded: 'error',
  };
  return (
    <Tag color={colorMap[status] ?? 'processing'}>
      {t(`rollout:status.${status}`, { defaultValue: status })}
    </Tag>
  );
}

// ─── Analysis status icon ─────────────────────────────────────────────────────

function analysisStatusIcon(status: string) {
  switch (status) {
    case 'Successful': return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
    case 'Running':    return <LoadingOutlined spin style={{ color: '#1677ff' }} />;
    case 'Failed':     return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
    default:           return <ClockCircleOutlined style={{ color: '#999' }} />;
  }
}

// ─── Main component ───────────────────────────────────────────────────────────

const RolloutDetail: React.FC = () => {
  const { clusterId, namespace, name } = useParams<{
    clusterId: string;
    namespace: string;
    name: string;
  }>();
  const { token } = theme.useToken();
  const { message, modal } = App.useApp();
  const { t } = useTranslation(['rollout', 'common']);
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const cid = Number(clusterId ?? 0);
  const ns = namespace ?? '';
  const rname = name ?? '';

  // ─── Query ──────────────────────────────────────────────────────────────────

  const isActive = useCallback(
    (rollout?: RolloutInfo) =>
      rollout?.status !== 'Stopped' && rollout?.status !== 'Healthy' && rollout?.status !== 'Degraded',
    [],
  );

  const { data: rollout, isLoading, refetch } = useQuery({
    queryKey: ['rollout', cid, ns, rname],
    queryFn: () => rolloutService.get(cid, ns, rname),
    enabled: cid > 0 && !!ns && !!rname,
    refetchInterval: (query) => (isActive(query.state.data) ? 3000 : false),
    staleTime: 2000,
  });

  const { data: analysisData, isLoading: analysisLoading } = useQuery({
    queryKey: ['rollout-analysis', cid, ns, rname],
    queryFn: () => rolloutService.getAnalysisRuns(cid, ns, rname),
    enabled: cid > 0 && !!ns && !!rname,
    staleTime: 10_000,
  });

  const analysisRuns = analysisData?.items ?? [];

  // ─── Mutations ───────────────────────────────────────────────────────────────

  const promoteMutation = useMutation({
    mutationFn: () => rolloutService.promote(cid, ns, rname),
    onSuccess: () => {
      message.success(t('rollout:messages.promoteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['rollout', cid, ns, rname] });
    },
    onError: () => message.error(t('rollout:messages.promoteFailed')),
  });

  const promoteFullMutation = useMutation({
    mutationFn: () => rolloutService.promoteFull(cid, ns, rname),
    onSuccess: () => {
      message.success(t('rollout:messages.promoteFullSuccess'));
      queryClient.invalidateQueries({ queryKey: ['rollout', cid, ns, rname] });
    },
    onError: () => message.error(t('rollout:messages.promoteFullFailed')),
  });

  const abortMutation = useMutation({
    mutationFn: () => rolloutService.abort(cid, ns, rname),
    onSuccess: () => {
      message.success(t('rollout:messages.abortSuccess'));
      queryClient.invalidateQueries({ queryKey: ['rollout', cid, ns, rname] });
    },
    onError: () => message.error(t('rollout:messages.abortFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: () => rolloutService.delete(cid, ns, rname),
    onSuccess: () => {
      message.success(t('rollout:messages.deleteSuccess'));
      navigate(`/clusters/${cid}/rollouts`);
    },
    onError: () => message.error(t('rollout:messages.deleteFailed')),
  });

  const handleDelete = useCallback(() => {
    modal.confirm({
      title: t('common:confirm.deleteTitle'),
      content: t('common:confirm.deleteDesc', { name: rname }),
      okText: t('common:actions.delete'),
      okButtonProps: { danger: true },
      onOk: () => deleteMutation.mutate(),
    });
  }, [modal, t, rname, deleteMutation]);

  // ─── Render guards ───────────────────────────────────────────────────────────

  if (isLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!rollout) {
    return <EmptyState description={t('common:messages.noData')} />;
  }

  const isMutating =
    promoteMutation.isPending ||
    promoteFullMutation.isPending ||
    abortMutation.isPending ||
    deleteMutation.isPending;

  // ─── Canary step items ────────────────────────────────────────────────────────

  const stepCount = rollout.current_step_count ?? 0;
  const stepIndex = rollout.current_step_index ?? 0;
  const canarySteps =
    stepCount > 0
      ? Array.from({ length: stepCount }, (_, i) => ({
          title: `${t('rollout:detail.stepIndex')} ${i + 1}`,
          status:
            i < stepIndex
              ? ('finish' as const)
              : i === stepIndex
              ? ('process' as const)
              : ('wait' as const),
        }))
      : [];

  // ─── Analysis runs columns ────────────────────────────────────────────────────

  const analysisColumns: TableColumnsType<AnalysisRun> = [
    {
      title: t('rollout:analysis.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (n: string) => <Text style={{ fontSize: token.fontSizeSM }}>{n}</Text>,
    },
    {
      title: t('rollout:analysis.status'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (s: string) => (
        <Space size={4}>
          {analysisStatusIcon(s)}
          <Text style={{ fontSize: token.fontSizeSM }}>{s}</Text>
        </Space>
      ),
    },
    {
      title: t('rollout:analysis.successful'),
      key: 'successful',
      width: 80,
      render: (_, r) => (
        <Text style={{ fontSize: token.fontSizeSM, color: token.colorSuccess }}>
          {r.metrics.reduce((sum, m) => sum + m.successful, 0)}
        </Text>
      ),
    },
    {
      title: t('rollout:analysis.failed'),
      key: 'failed',
      width: 80,
      render: (_, r) => (
        <Text style={{ fontSize: token.fontSizeSM, color: token.colorError }}>
          {r.metrics.reduce((sum, m) => sum + m.failed, 0)}
        </Text>
      ),
    },
    {
      title: t('rollout:analysis.startedAt'),
      dataIndex: 'started_at',
      key: 'started_at',
      width: 150,
      render: (time: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(time).format('YYYY-MM-DD HH:mm')}
        </Text>
      ),
    },
  ];

  // ─── Render ──────────────────────────────────────────────────────────────────

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
      {/* Header */}
      <div style={{
        background: token.colorBgContainer,
        borderBottom: `1px solid ${token.colorBorder}`,
        padding: `${token.paddingSM}px ${token.paddingLG}px`,
        flexShrink: 0,
      }}>
        <Breadcrumb
          style={{ marginBottom: token.marginXS }}
          items={[
            {
              title: (
                <Button
                  type="link"
                  size="small"
                  icon={<ArrowLeftOutlined />}
                  style={{ padding: 0 }}
                  onClick={() => navigate(`/clusters/${cid}/rollouts`)}
                >
                  {t('rollout:page.title')}
                </Button>
              ),
            },
            { title: rname },
          ]}
        />

        <Flex justify="space-between" align="center">
          <Flex align="center" gap={token.marginSM}>
            <Title level={5} style={{ margin: 0 }}>{rname}</Title>
            <RolloutStatusTag status={rollout.status} />
            <Tag color={rollout.strategy === 'Canary' ? 'blue' : 'purple'}>
              {t(`rollout:strategy.${rollout.strategy}`, { defaultValue: rollout.strategy })}
            </Tag>
            <Tag>{ns}</Tag>
          </Flex>

          <Space>
            <Tooltip title={t('common:actions.refresh')}>
              <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
            </Tooltip>

            {/* Promote */}
            <Popconfirm
              title={t('rollout:confirm.promoteTitle')}
              description={t('rollout:confirm.promoteDesc', { name: rname })}
              onConfirm={() => promoteMutation.mutate()}
              okText={t('common:actions.confirm')}
              cancelText={t('common:actions.cancel')}
            >
              <Button
                icon={<FastForwardOutlined />}
                loading={promoteMutation.isPending}
                disabled={isMutating}
              >
                {t('rollout:actions.promote')}
              </Button>
            </Popconfirm>

            {/* Promote Full (Canary only) */}
            {rollout.strategy === 'Canary' && (
              <Popconfirm
                title={t('rollout:confirm.promoteFullTitle')}
                description={t('rollout:confirm.promoteFullDesc', { name: rname })}
                onConfirm={() => promoteFullMutation.mutate()}
                okText={t('common:actions.confirm')}
                cancelText={t('common:actions.cancel')}
              >
                <Button
                  icon={<RocketOutlined />}
                  loading={promoteFullMutation.isPending}
                  disabled={isMutating}
                >
                  {t('rollout:actions.promoteFull')}
                </Button>
              </Popconfirm>
            )}

            {/* Abort */}
            <Popconfirm
              title={t('rollout:confirm.abortTitle')}
              description={t('rollout:confirm.abortDesc', { name: rname })}
              onConfirm={() => abortMutation.mutate()}
              okText={t('common:actions.confirm')}
              cancelText={t('common:actions.cancel')}
              okButtonProps={{ danger: true }}
            >
              <Button
                danger
                icon={<StopOutlined />}
                loading={abortMutation.isPending}
                disabled={isMutating}
              >
                {t('rollout:actions.abort')}
              </Button>
            </Popconfirm>

            {/* Delete */}
            <Tooltip title={t('rollout:actions.delete')}>
              <Button
                danger
                icon={<DeleteOutlined />}
                loading={deleteMutation.isPending}
                disabled={isMutating}
                onClick={handleDelete}
              />
            </Tooltip>
          </Space>
        </Flex>
      </div>

      {/* Body */}
      <div style={{ flex: 1, overflow: 'auto', padding: token.paddingLG }}>
        <Row gutter={[token.marginMD, token.marginMD]}>

          {/* ── Overview ──────────────────────────────────────────────────── */}
          <Col span={24}>
            <Card
              title={t('rollout:detail.overview')}
              size="small"
              variant="borderless"
              style={{ border: `1px solid ${token.colorBorder}` }}
            >
              <Row gutter={token.marginLG}>
                <Col xs={24} md={12}>
                  <Descriptions size="small" column={2}>
                    <Descriptions.Item label={t('rollout:table.namespace')}>
                      <Tag>{rollout.namespace}</Tag>
                    </Descriptions.Item>
                    <Descriptions.Item label={t('rollout:table.strategy')}>
                      <Tag color={rollout.strategy === 'Canary' ? 'blue' : 'purple'}>
                        {t(`rollout:strategy.${rollout.strategy}`, { defaultValue: rollout.strategy })}
                      </Tag>
                    </Descriptions.Item>
                    <Descriptions.Item label={t('rollout:table.createdAt')}>
                      <Text style={{ fontSize: token.fontSizeSM }}>
                        {dayjs(rollout.created_at).format('YYYY-MM-DD HH:mm:ss')}
                      </Text>
                    </Descriptions.Item>
                    {rollout.images?.length > 0 && (
                      <Descriptions.Item label={t('rollout:table.images')} span={2}>
                        <Space direction="vertical" size={2}>
                          {rollout.images.map((img) => (
                            <Text key={img} code style={{ fontSize: token.fontSizeSM }}>{img}</Text>
                          ))}
                        </Space>
                      </Descriptions.Item>
                    )}
                  </Descriptions>
                </Col>
                <Col xs={24} md={12}>
                  <Row gutter={token.marginMD}>
                    {[
                      { label: t('rollout:table.replicas'), value: rollout.replicas },
                      { label: 'Ready', value: rollout.ready_replicas },
                      { label: 'Available', value: rollout.available_replicas },
                      { label: 'Updated', value: rollout.updated_replicas },
                    ].map(({ label, value }) => (
                      <Col key={label} span={6}>
                        <Statistic title={label} value={value} valueStyle={{ fontSize: token.fontSizeLG }} />
                      </Col>
                    ))}
                  </Row>
                </Col>
              </Row>
            </Card>
          </Col>

          {/* ── Canary / BlueGreen specific ───────────────────────────────── */}
          {rollout.strategy === 'Canary' && (
            <Col span={24}>
              <Card
                title={t('rollout:detail.canaryProgress')}
                size="small"
                variant="borderless"
                style={{ border: `1px solid ${token.colorBorder}` }}
              >
                <Row gutter={token.marginLG} align="middle">
                  {/* Weight bar */}
                  <Col xs={24} md={10}>
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      {t('rollout:detail.currentWeight')}
                    </Text>
                    <Flex align="center" gap={token.marginSM} style={{ marginTop: token.marginXS }}>
                      <Progress
                        percent={rollout.current_weight ?? 0}
                        strokeColor={{ '0%': '#1677ff', '100%': '#52c41a' }}
                        style={{ flex: 1 }}
                      />
                      <Text strong>{rollout.current_weight ?? 0}%</Text>
                    </Flex>
                    {rollout.desired_weight != null && (
                      <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                        {t('rollout:detail.desiredWeight')}: {rollout.desired_weight}%
                      </Text>
                    )}
                  </Col>

                  {/* Steps timeline */}
                  {canarySteps.length > 0 && (
                    <Col xs={24} md={14}>
                      <Steps
                        size="small"
                        current={stepIndex}
                        items={canarySteps}
                        style={{ overflowX: 'auto' }}
                      />
                    </Col>
                  )}
                </Row>

                <Descriptions size="small" column={2} style={{ marginTop: token.marginMD }}>
                  {rollout.stable_rs && (
                    <Descriptions.Item label={t('rollout:detail.stableRS')}>
                      <Text code style={{ fontSize: token.fontSizeSM }}>{rollout.stable_rs}</Text>
                    </Descriptions.Item>
                  )}
                  {rollout.canary_rs && (
                    <Descriptions.Item label={t('rollout:detail.canaryRS')}>
                      <Text code style={{ fontSize: token.fontSizeSM }}>{rollout.canary_rs}</Text>
                    </Descriptions.Item>
                  )}
                </Descriptions>
              </Card>
            </Col>
          )}

          {rollout.strategy === 'BlueGreen' && (
            <Col span={24}>
              <Card
                title="BlueGreen"
                size="small"
                variant="borderless"
                style={{ border: `1px solid ${token.colorBorder}` }}
              >
                <Descriptions size="small" column={2}>
                  {rollout.active_selector && (
                    <Descriptions.Item label={t('rollout:detail.activeSelector')}>
                      <Tag color="blue">{rollout.active_selector}</Tag>
                    </Descriptions.Item>
                  )}
                  {rollout.preview_selector && (
                    <Descriptions.Item label={t('rollout:detail.previewSelector')}>
                      <Tag color="purple">{rollout.preview_selector}</Tag>
                    </Descriptions.Item>
                  )}
                </Descriptions>
              </Card>
            </Col>
          )}

          {/* ── Analysis Runs ─────────────────────────────────────────────── */}
          <Col span={24}>
            <Card
              title={t('rollout:detail.analysisRuns')}
              size="small"
              variant="borderless"
              style={{ border: `1px solid ${token.colorBorder}` }}
            >
              <Table<AnalysisRun>
                columns={analysisColumns}
                dataSource={analysisRuns}
                rowKey="name"
                loading={analysisLoading}
                size="small"
                pagination={false}
                locale={{
                  emptyText: <EmptyState description={t('rollout:detail.noAnalysis')} />,
                }}
              />
            </Card>
          </Col>
        </Row>
      </div>
    </div>
  );
};

export default RolloutDetail;

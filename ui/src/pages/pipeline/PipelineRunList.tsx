/**
 * PipelineRunList — Pipeline Run 歷史列表頁
 *
 * 功能：
 *  - 顯示某 Pipeline 的所有 Run 紀錄
 *  - 狀態篩選 + 手動取消
 *  - 點擊 Run 進入 DAG 詳情頁
 */
import React, { useCallback, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Table,
  Button,
  Tag,
  Space,
  Tooltip,
  App,
  theme,
  Popconfirm,
  Flex,
  Typography,
  Breadcrumb,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  ArrowLeftOutlined,
  EyeOutlined,
  StopOutlined,
  PlayCircleOutlined,
  LoadingOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  SecurityScanOutlined,
  MinusCircleOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import pipelineService, { type PipelineRun } from '../../services/pipelineService';
import TriggerRunModal from './components/TriggerRunModal';
import ScanResultDrawer from './components/ScanResultDrawer';

const { Text } = Typography;

function RunStatusTag({ status }: { status: string }) {
  switch (status) {
    case 'queued':           return <Tag color="default" icon={<ClockCircleOutlined />}>排隊中</Tag>;
    case 'running':          return <Tag color="processing" icon={<LoadingOutlined spin />}>執行中</Tag>;
    case 'success':          return <Tag color="success" icon={<CheckCircleOutlined />}>成功</Tag>;
    case 'failed':           return <Tag color="error" icon={<CloseCircleOutlined />}>失敗</Tag>;
    case 'cancelled':        return <Tag color="warning">已取消</Tag>;
    case 'waiting_approval': return <Tag color="gold">待審核</Tag>;
    default:                 return <Tag>{status}</Tag>;
  }
}

const PipelineRunList: React.FC = () => {
  const { pipelineId } = useParams<{ pipelineId: string }>();
  const pid = Number(pipelineId ?? 0);
  const navigate = useNavigate();
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['pipeline', 'common']);
  const queryClient = useQueryClient();
  const [scanRunId, setScanRunId] = useState<number | null>(null);

  const [triggerOpen, setTriggerOpen] = useState(false);

  const { data: pipeline } = useQuery({
    queryKey: ['pipeline', pid],
    queryFn: () => pipelineService.get(pid),
    enabled: pid > 0,
  });

  const { data, isLoading } = useQuery({
    queryKey: ['pipeline-runs', pid],
    queryFn: () => pipelineService.listRuns(pid),
    enabled: pid > 0,
    refetchInterval: 5000,
  });

  const runs: PipelineRun[] = data?.items ?? (Array.isArray(data) ? data : []);

  const cancelMutation = useMutation({
    mutationFn: (runId: number) => pipelineService.cancelRun(pid, runId),
    onSuccess: () => {
      message.success(t('pipeline:runDetail.cancelSuccess'));
      queryClient.invalidateQueries({ queryKey: ['pipeline-runs', pid] });
    },
    onError: () => message.error(t('pipeline:runDetail.cancelFailed')),
  });

  const handleView = useCallback((run: PipelineRun) => {
    navigate(`/pipelines/${pid}/runs/${run.id}`);
  }, [navigate, pid]);

  const columns: TableColumnsType<PipelineRun> = [
    {
      title: 'Run ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
      render: (id: number, record) => (
        <Button type="link" style={{ padding: 0 }} onClick={() => handleView(record)}>
          #{id}
        </Button>
      ),
    },
    {
      title: t('pipeline:run.status.label', { defaultValue: '狀態' }),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string) => <RunStatusTag status={status} />,
    },
    {
      title: t('pipeline:run.triggerTypeLabel'),
      dataIndex: 'trigger_type',
      key: 'trigger_type',
      width: 100,
      render: (type: string) => (
        <Text type="secondary">{t(`pipeline:run.triggerType.${type}`, { defaultValue: type })}</Text>
      ),
    },
    {
      title: t('pipeline:run.scanResult', { defaultValue: '安全掃描' }),
      key: 'scan_result',
      width: 140,
      render: (_: unknown, record: Record<string, unknown>) => {
        const scan = record.scan_result as { status: string; critical: number; high: number; medium: number; low: number } | undefined;
        if (!scan) return <Text type="secondary"><MinusCircleOutlined /> {t('pipeline:run.scanNone', { defaultValue: '未掃描' })}</Text>;
        const total = scan.critical + scan.high + scan.medium + scan.low;
        const runId = (record as Record<string, unknown>).id as number;
        if (total === 0) return (
          <Tag color="success" icon={<SecurityScanOutlined />} style={{ cursor: 'pointer' }} onClick={() => setScanRunId(runId)}>
            {t('pipeline:run.scanPass', { defaultValue: '通過' })}
          </Tag>
        );
        const parts: string[] = [];
        if (scan.critical > 0) parts.push(`${scan.critical}C`);
        if (scan.high > 0) parts.push(`${scan.high}H`);
        if (scan.medium > 0) parts.push(`${scan.medium}M`);
        if (scan.low > 0) parts.push(`${scan.low}L`);
        const color = scan.critical > 0 ? 'error' : scan.high > 0 ? 'warning' : 'default';
        return (
          <Tag color={color} icon={<SecurityScanOutlined />} style={{ cursor: 'pointer' }} onClick={() => setScanRunId(runId)}>
            {parts.join(' ')}
          </Tag>
        );
      },
    },
    {
      title: 'Namespace',
      dataIndex: 'namespace',
      key: 'namespace',
      width: 120,
    },
    {
      title: t('pipeline:run.error', { defaultValue: '錯誤' }),
      dataIndex: 'error',
      key: 'error',
      ellipsis: true,
      render: (error: string) => error ? <Text type="danger">{error}</Text> : '—',
    },
    {
      title: t('pipeline:run.createdAt', { defaultValue: '建立時間' }),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (time: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(time).format('YYYY-MM-DD HH:mm:ss')}
        </Text>
      ),
    },
    {
      title: t('common:table.actions', { defaultValue: '操作' }),
      key: 'actions',
      width: 100,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Tooltip title={t('pipeline:runDetail.viewDetail', { defaultValue: '查看詳情' })}>
            <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handleView(record)} />
          </Tooltip>
          {(record.status === 'queued' || record.status === 'running') && (
            <Popconfirm
              title={t('pipeline:runDetail.cancelConfirm', { defaultValue: '確認取消此 Run？' })}
              onConfirm={() => cancelMutation.mutate(record.id)}
              okText={t('common:actions.confirm')}
              cancelText={t('common:actions.cancel')}
            >
              <Tooltip title={t('pipeline:runDetail.cancel', { defaultValue: '取消' })}>
                <Button type="link" size="small" danger icon={<StopOutlined />} />
              </Tooltip>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: token.paddingLG }}>
      <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
        <Breadcrumb
          style={{ display: 'flex', alignItems: 'center' }}
          items={[
            { title: <Button type="link" icon={<ArrowLeftOutlined />} style={{ padding: 0, height: 'auto' }} onClick={() => navigate('/pipelines')}>Pipelines</Button> },
            { title: `Pipeline #${pid}` },
            { title: t('pipeline:run.history', { defaultValue: '執行紀錄' }) },
          ]}
        />
        <Button
          type="primary"
          icon={<PlayCircleOutlined />}
          onClick={() => setTriggerOpen(true)}
        >
          {t('pipeline:run.trigger', { defaultValue: '手動觸發' })}
        </Button>
      </Flex>

      <Table<PipelineRun>
        columns={columns}
        dataSource={runs}
        rowKey="id"
        loading={isLoading}
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
      />

      {pipeline && (
        <TriggerRunModal
          open={triggerOpen}
          onClose={() => setTriggerOpen(false)}
          pipeline={pipeline}
        />
      )}

      <ScanResultDrawer
        open={scanRunId !== null}
        onClose={() => setScanRunId(null)}
        pipelineId={pid}
        runId={scanRunId ?? 0}
      />
    </div>
  );
};

export default PipelineRunList;

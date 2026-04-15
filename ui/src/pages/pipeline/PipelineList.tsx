/**
 * PipelineList — Pipeline 管理列表頁（P3-1）
 *
 * 功能：
 *  - 卡片 / 列表雙模式切換
 *  - 搜尋過濾
 *  - 建立 / 編輯 / 刪除 Pipeline
 *  - 手動觸發 Run
 *  - 開啟 YAML 編輯器（PipelineEditor）
 */
import React, { useState, useCallback } from 'react';
import {
  Card,
  Table,
  Button,
  Input,
  Space,
  Flex,
  Tooltip,
  Popconfirm,
  Tag,
  Row,
  Col,
  Typography,
  App,
  theme,
  Segmented,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  SearchOutlined,
  PlayCircleOutlined,
  AppstoreOutlined,
  UnorderedListOutlined,
  CodeOutlined,
  LockOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import pipelineService, { type Pipeline } from '../../services/pipelineService';
import EmptyState from '../../components/EmptyState';
import PipelineEditor from './PipelineEditor';
import TriggerRunModal from './components/TriggerRunModal';
import PipelineSecretManager from './components/PipelineSecretManager';
import PipelineAllowedImages from './components/PipelineAllowedImages';

const { Text } = Typography;

type ViewMode = 'card' | 'table';

// ─── Main Component ─────────────────────────────────────────────────────────

const PipelineList: React.FC = () => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['pipeline', 'common']);
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [viewMode, setViewMode] = useState<ViewMode>('card');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Pipeline | null>(null);
  const [triggerOpen, setTriggerOpen] = useState(false);
  const [triggerPipeline, setTriggerPipeline] = useState<Pipeline | null>(null);
  const [secretOpen, setSecretOpen] = useState(false);
  const [secretPipeline, setSecretPipeline] = useState<Pipeline | null>(null);
  const [allowedImagesOpen, setAllowedImagesOpen] = useState(false);

  // ─── Query ────────────────────────────────────────────────────────────────

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['pipelines'],
    queryFn: () => pipelineService.list(),
    staleTime: 15_000,
  });

  const items: Pipeline[] = (data?.items ?? []).filter((p) =>
    p.name.toLowerCase().includes(search.toLowerCase()) ||
    p.description?.toLowerCase().includes(search.toLowerCase())
  );

  // ─── Mutations ────────────────────────────────────────────────────────────

  const deleteMutation = useMutation({
    mutationFn: (id: number) => pipelineService.delete(id),
    onSuccess: () => {
      message.success(t('pipeline:messages.deleteSuccess'));
      queryClient.invalidateQueries({ queryKey: ['pipelines'] });
    },
    onError: () => message.error(t('pipeline:messages.deleteFailed')),
  });


  // ─── Handlers ─────────────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    setEditorOpen(true);
  }, []);

  const handleEdit = useCallback((pipeline: Pipeline) => {
    setEditing(pipeline);
    setEditorOpen(true);
  }, []);

  const handleDelete = useCallback(
    (pipeline: Pipeline) => { deleteMutation.mutate(pipeline.id); },
    [deleteMutation]
  );

  const handleTrigger = useCallback((pipeline: Pipeline) => {
    setTriggerPipeline(pipeline);
    setTriggerOpen(true);
  }, []);

  const handleSecrets = useCallback((pipeline: Pipeline) => {
    setSecretPipeline(pipeline);
    setSecretOpen(true);
  }, []);

  const handleEditorClose = useCallback(() => {
    setEditorOpen(false);
    setEditing(null);
  }, []);

  const handleEditorSuccess = useCallback(() => {
    setEditorOpen(false);
    setEditing(null);
    queryClient.invalidateQueries({ queryKey: ['pipelines'] });
  }, [queryClient]);

  // ─── Table columns ────────────────────────────────────────────────────────

  const columns: TableColumnsType<Pipeline> = [
    {
      title: t('pipeline:table.name'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string, record) => (
        <Button
          type="link"
          style={{ padding: 0, fontWeight: 600 }}
          onClick={() => handleEdit(record)}
        >
          {name}
        </Button>
      ),
    },
    {
      title: t('pipeline:table.version'),
      dataIndex: 'current_version_id',
      key: 'version',
      width: 100,
      render: (vid: number | null) =>
        vid ? (
          <Tag color="blue">v{vid}</Tag>
        ) : (
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
            {t('pipeline:card.noVersion')}
          </Text>
        ),
    },
    {
      title: t('pipeline:table.concurrencyPolicy'),
      dataIndex: 'concurrency_policy',
      key: 'concurrency_policy',
      width: 140,
      render: (policy: string) => (
        <Text style={{ fontSize: token.fontSizeSM }}>
          {t(`pipeline:policy.${policy}`)}
        </Text>
      ),
    },
    {
      title: t('pipeline:table.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (time: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(time).format('YYYY-MM-DD HH:mm')}
        </Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 140,
      fixed: 'right',
      render: (_, record) => <PipelineActions record={record} onEdit={handleEdit} onDelete={handleDelete} onTrigger={handleTrigger} onSecrets={handleSecrets} />,
    },
  ];

  // ─── Toolbar ──────────────────────────────────────────────────────────────

  const toolbar = (
    <Flex justify="space-between" align="center" style={{ marginBottom: token.marginMD }}>
      <Space>
        <Input
          prefix={<SearchOutlined />}
          placeholder={t('common:search.placeholder')}
          allowClear
          style={{ width: 240 }}
          onChange={(e) => setSearch(e.target.value)}
        />
      </Space>
      <Space>
        <Segmented
          value={viewMode}
          onChange={(v) => setViewMode(v as ViewMode)}
          options={[
            { value: 'card', icon: <AppstoreOutlined />, label: t('pipeline:view.card') },
            { value: 'table', icon: <UnorderedListOutlined />, label: t('pipeline:view.table') },
          ]}
        />
        <Tooltip title={t('pipeline:allowedImages.manageButton')}>
          <Button icon={<SafetyOutlined />} onClick={() => setAllowedImagesOpen(true)} />
        </Tooltip>
        <Tooltip title={t('common:actions.refresh')}>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
        </Tooltip>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          {t('common:actions.create')}
        </Button>
      </Space>
    </Flex>
  );

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <>
      {/* Page header */}
      <div style={{ marginBottom: token.marginLG }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {t('pipeline:page.title')}
        </Typography.Title>
        <Text type="secondary">{t('pipeline:page.subtitle')}</Text>
      </div>

      <Card variant="borderless">
        {toolbar}

        {viewMode === 'card' ? (
          items.length === 0 && !isLoading ? (
            <EmptyState description={t('common:messages.noData')} />
          ) : (
            <Row gutter={[token.marginMD, token.marginMD]}>
              {items.map((pipeline) => (
                <Col key={pipeline.id} xs={24} sm={12} lg={8} xl={6}>
                  <PipelineCard
                    pipeline={pipeline}
                    onEdit={handleEdit}
                    onDelete={handleDelete}
                    onTrigger={handleTrigger}
                    onSecrets={handleSecrets}
                  />
                </Col>
              ))}
            </Row>
          )
        ) : (
          <Table<Pipeline>
            columns={columns}
            dataSource={items}
            rowKey="id"
            loading={isLoading}
            size="small"
            scroll={{ x: 'max-content' }}
            pagination={{
              pageSize: 20,
              showSizeChanger: true,
              showTotal: (total) => t('common:pagination.total', { total }),
            }}
            locale={{
              emptyText: <EmptyState description={t('common:messages.noData')} />,
            }}
          />
        )}
      </Card>

      {/* Editor Drawer */}
      <PipelineEditor
        open={editorOpen}
        pipeline={editing}
        onClose={handleEditorClose}
        onSuccess={handleEditorSuccess}
      />

      {/* Trigger Run Modal */}
      {triggerPipeline && (
        <TriggerRunModal
          open={triggerOpen}
          onClose={() => { setTriggerOpen(false); setTriggerPipeline(null); }}
          pipeline={triggerPipeline}
        />
      )}

      {/* Secret Manager Drawer */}
      {secretPipeline && (
        <PipelineSecretManager
          open={secretOpen}
          onClose={() => { setSecretOpen(false); setSecretPipeline(null); }}
          pipeline={secretPipeline}
        />
      )}

      {/* Allowed Images Drawer */}
      <PipelineAllowedImages
        open={allowedImagesOpen}
        onClose={() => setAllowedImagesOpen(false)}
      />
    </>
  );
};

// ─── Pipeline Card ───────────────────────────────────────────────────────────

interface PipelineCardProps {
  pipeline: Pipeline;
  onEdit: (p: Pipeline) => void;
  onDelete: (p: Pipeline) => void;
  onTrigger: (p: Pipeline) => void;
  onSecrets: (p: Pipeline) => void;
}

const PipelineCard: React.FC<PipelineCardProps> = ({ pipeline, onEdit, onDelete, onTrigger, onSecrets }) => {
  const { token } = theme.useToken();
  const { t } = useTranslation(['pipeline', 'common', 'cicd']);

  return (
    <Card
      variant="borderless"
      style={{
        borderRadius: token.borderRadiusLG,
        border: `1px solid ${token.colorBorder}`,
        cursor: 'pointer',
        transition: 'box-shadow 0.2s',
      }}
      styles={{ body: { padding: token.paddingMD } }}
      hoverable
      onClick={() => onEdit(pipeline)}
    >
      {/* Header: name + version tag */}
      <Flex justify="space-between" align="flex-start" style={{ marginBottom: token.marginSM }}>
        <Text strong style={{ fontSize: token.fontSizeLG, flex: 1, marginRight: token.marginSM }}>
          {pipeline.name}
        </Text>
        {pipeline.current_version_id ? (
          <Tag color="blue" style={{ flexShrink: 0 }}>
            {t('pipeline:card.version', { version: pipeline.current_version_id })}
          </Tag>
        ) : (
          <Tag style={{ flexShrink: 0 }}>{t('pipeline:card.noVersion')}</Tag>
        )}
      </Flex>

      {/* Description */}
      {pipeline.description && (
        <Text
          type="secondary"
          style={{ fontSize: token.fontSizeSM, display: 'block', marginBottom: token.marginSM }}
          ellipsis={{ tooltip: pipeline.description }}
        >
          {pipeline.description}
        </Text>
      )}

      {/* Footer: created time + actions */}
      <Flex justify="space-between" align="center" style={{ marginTop: token.marginSM }}>
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(pipeline.created_at).format('YYYY-MM-DD')}
        </Text>

        {/* Actions — stop propagation to prevent card click */}
        <Space size={0} onClick={(e) => e.stopPropagation()}>
          <Tooltip title={t('pipeline:run.trigger')}>
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => onTrigger(pipeline)}
            />
          </Tooltip>
          <Tooltip title={t('cicd:secret.manageButton')}>
            <Button
              type="link"
              size="small"
              icon={<LockOutlined />}
              onClick={() => onSecrets(pipeline)}
            />
          </Tooltip>
          <Tooltip title={t('pipeline:form.editTitle')}>
            <Button
              type="link"
              size="small"
              icon={<CodeOutlined />}
              onClick={() => onEdit(pipeline)}
            />
          </Tooltip>
          <Popconfirm
            title={t('common:confirm.deleteTitle')}
            description={t('common:confirm.deleteDesc', { name: pipeline.name })}
            onConfirm={() => onDelete(pipeline)}
            okText={t('common:actions.delete')}
            okButtonProps={{ danger: true }}
            cancelText={t('common:actions.cancel')}
          >
            <Tooltip title={t('common:actions.delete')}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      </Flex>
    </Card>
  );
};

// ─── Pipeline Actions (table row) ────────────────────────────────────────────

interface PipelineActionsProps {
  record: Pipeline;
  onEdit: (p: Pipeline) => void;
  onDelete: (p: Pipeline) => void;
  onTrigger: (p: Pipeline) => void;
  onSecrets: (p: Pipeline) => void;
}

const PipelineActions: React.FC<PipelineActionsProps> = ({ record, onEdit, onDelete, onTrigger, onSecrets }) => {
  const { t } = useTranslation(['pipeline', 'common', 'cicd']);

  return (
    <Space size={0}>
      <Tooltip title={t('pipeline:run.trigger')}>
        <Button
          type="link"
          size="small"
          icon={<PlayCircleOutlined />}
          onClick={() => onTrigger(record)}
        />
      </Tooltip>
      <Tooltip title={t('cicd:secret.manageButton')}>
        <Button
          type="link"
          size="small"
          icon={<LockOutlined />}
          onClick={() => onSecrets(record)}
        />
      </Tooltip>
      <Tooltip title={t('pipeline:form.editTitle')}>
        <Button
          type="link"
          size="small"
          icon={<EditOutlined />}
          onClick={() => onEdit(record)}
        />
      </Tooltip>
      <Popconfirm
        title={t('common:confirm.deleteTitle')}
        description={t('common:confirm.deleteDesc', { name: record.name })}
        onConfirm={() => onDelete(record)}
        okText={t('common:actions.delete')}
        okButtonProps={{ danger: true }}
        cancelText={t('common:actions.cancel')}
      >
        <Tooltip title={t('common:actions.delete')}>
          <Button type="link" size="small" danger icon={<DeleteOutlined />} />
        </Tooltip>
      </Popconfirm>
    </Space>
  );
};

export default PipelineList;

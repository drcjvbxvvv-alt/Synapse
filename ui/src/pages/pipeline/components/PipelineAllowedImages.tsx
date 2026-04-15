/**
 * PipelineAllowedImages — Step 映像白名單管理（PlatformAdmin only）
 *
 * 功能：
 *  - 顯示/編輯全域 Step 映像白名單（glob pattern 列表）
 *  - 新增 / 刪除 pattern
 *  - 說明：白名單留空時表示不限制映像
 */
import React, { useState, useCallback, useEffect } from 'react';
import {
  Drawer,
  Button,
  Space,
  Table,
  Input,
  Popconfirm,
  Tooltip,
  Typography,
  Alert,
  App,
  theme,
  Flex,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';

import { pipelineAllowedImagesService } from '../../../services/pipelineService';

const { Text } = Typography;

interface Props {
  open: boolean;
  onClose: () => void;
}

const PipelineAllowedImages: React.FC<Props> = ({ open, onClose }) => {
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const { t } = useTranslation(['pipeline', 'common']);
  const queryClient = useQueryClient();

  const [newPattern, setNewPattern] = useState('');
  const [patterns, setPatterns] = useState<string[]>([]);
  const [dirty, setDirty] = useState(false);

  // ─── Query ───────────────────────────────────────────────────────────────

  const { data: queryData, isLoading } = useQuery({
    queryKey: ['pipeline-allowed-images'],
    queryFn: () => pipelineAllowedImagesService.get(),
    enabled: open,
    staleTime: 30_000,
  });

  useEffect(() => {
    if (queryData) {
      setPatterns(queryData.patterns ?? []);
      setDirty(false);
    }
  }, [queryData]);

  // ─── Mutation ────────────────────────────────────────────────────────────

  const saveMutation = useMutation({
    mutationFn: (p: string[]) => pipelineAllowedImagesService.update(p),
    onSuccess: () => {
      message.success(t('pipeline:allowedImages.saveSuccess'));
      setDirty(false);
      queryClient.invalidateQueries({ queryKey: ['pipeline-allowed-images'] });
    },
    onError: () => message.error(t('pipeline:allowedImages.saveFailed')),
  });

  // ─── Handlers ────────────────────────────────────────────────────────────

  const handleAdd = useCallback(() => {
    const trimmed = newPattern.trim();
    if (!trimmed) return;
    if (patterns.includes(trimmed)) {
      message.warning(t('pipeline:allowedImages.duplicate'));
      return;
    }
    setPatterns((prev) => [...prev, trimmed]);
    setNewPattern('');
    setDirty(true);
  }, [newPattern, patterns, message, t]);

  const handleDelete = useCallback((pattern: string) => {
    setPatterns((prev) => prev.filter((p) => p !== pattern));
    setDirty(true);
  }, []);

  const handleSave = useCallback(() => {
    saveMutation.mutate(patterns);
  }, [saveMutation, patterns]);

  const handleClose = useCallback(() => {
    if (dirty) {
      setDirty(false);
    }
    onClose();
  }, [dirty, onClose]);

  // ─── Table columns ───────────────────────────────────────────────────────

  const columns: TableColumnsType<string> = [
    {
      title: t('pipeline:allowedImages.pattern'),
      dataIndex: undefined,
      key: 'pattern',
      render: (_, pattern: string) => (
        <Text code style={{ fontSize: token.fontSizeSM }}>{pattern}</Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 80,
      fixed: 'right',
      render: (_, pattern: string) => (
        <Popconfirm
          title={t('common:confirm.deleteTitle')}
          description={t('common:confirm.deleteDesc', { name: pattern })}
          onConfirm={() => handleDelete(pattern)}
          okText={t('common:actions.delete')}
          okButtonProps={{ danger: true }}
          cancelText={t('common:actions.cancel')}
        >
          <Tooltip title={t('common:actions.delete')}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />} />
          </Tooltip>
        </Popconfirm>
      ),
    },
  ];

  // ─── Render ──────────────────────────────────────────────────────────────

  return (
    <Drawer
      title={t('pipeline:allowedImages.title')}
      open={open}
      onClose={handleClose}
      width={640}
      extra={
        <Button
          type="primary"
          icon={<SaveOutlined />}
          loading={saveMutation.isPending}
          disabled={!dirty}
          onClick={handleSave}
        >
          {t('common:actions.save')}
        </Button>
      }
    >
      <Alert
        type="info"
        showIcon
        message={t('pipeline:allowedImages.hint')}
        style={{ marginBottom: token.marginMD }}
      />

      {/* Add pattern row */}
      <Flex gap={token.marginSM} style={{ marginBottom: token.marginMD }}>
        <Input
          value={newPattern}
          onChange={(e) => setNewPattern(e.target.value)}
          placeholder={t('pipeline:allowedImages.patternPlaceholder')}
          onPressEnter={handleAdd}
          style={{ flex: 1 }}
        />
        <Button icon={<PlusOutlined />} onClick={handleAdd}>
          {t('common:actions.create')}
        </Button>
      </Flex>

      <Table<string>
        columns={columns}
        dataSource={patterns}
        rowKey={(p) => p}
        loading={isLoading}
        size="small"
        pagination={false}
        scroll={{ y: 400 }}
        locale={{
          emptyText: (
            <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
              {t('pipeline:allowedImages.empty')}
            </Text>
          ),
        }}
      />
    </Drawer>
  );
};

export default PipelineAllowedImages;

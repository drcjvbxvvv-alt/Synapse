/**
 * RegistryBrowser — Registry Repository / Tag 瀏覽（M15 UI）
 *
 * 功能：
 *  - 左欄：Repository 列表（名稱 + Tag 數 + Pull 數）
 *  - 右欄：選中 Repository 的 Tag 列表（名稱 + Digest + 大小 + 建立時間）
 */
import React, { useState } from 'react';
import {
  Drawer,
  Table,
  Typography,
  Flex,
  theme,
  Tooltip,
  Tag,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import dayjs from 'dayjs';

import registryService, {
  type Registry,
  type RegistryRepository,
  type RegistryTag,
} from '../../../services/registryService';
import EmptyState from '../../../components/EmptyState';

const { Text } = Typography;

interface Props {
  open: boolean;
  registry: Registry | null;
  onClose: () => void;
}

function formatBytes(bytes?: number): string {
  if (!bytes) return '—';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

const RegistryBrowser: React.FC<Props> = ({ open, registry, onClose }) => {
  const { token } = theme.useToken();
  const { t } = useTranslation(['cicd', 'common']);
  const [selectedRepo, setSelectedRepo] = useState<string | null>(null);

  // ─── Repositories ────────────────────────────────────────────────────────

  const { data: repoData, isLoading: repoLoading } = useQuery({
    queryKey: ['registry-repos', registry?.id],
    queryFn: () => registryService.listRepositories(registry!.id, registry?.default_project),
    enabled: open && !!registry,
    staleTime: 30_000,
  });

  const repos = repoData?.items ?? [];

  // ─── Tags ────────────────────────────────────────────────────────────────

  const { data: tagData, isLoading: tagLoading } = useQuery({
    queryKey: ['registry-tags', registry?.id, selectedRepo],
    queryFn: () => registryService.listTags(registry!.id, selectedRepo!),
    enabled: open && !!registry && !!selectedRepo,
    staleTime: 30_000,
  });

  const tags = tagData?.items ?? [];

  // ─── Repo columns ─────────────────────────────────────────────────────────

  const repoColumns: TableColumnsType<RegistryRepository> = [
    {
      title: t('cicd:registryBrowser.repoName'),
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
      render: (name: string) => (
        <Text
          style={{ cursor: 'pointer', color: token.colorPrimary, fontSize: token.fontSizeSM }}
          onClick={() => setSelectedRepo(name)}
        >
          {name}
        </Text>
      ),
    },
    {
      title: t('cicd:registryBrowser.tagCount'),
      dataIndex: 'tag_count',
      key: 'tag_count',
      width: 70,
      render: (n?: number) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{n ?? '—'}</Text>
      ),
    },
    {
      title: t('cicd:registryBrowser.pullCount'),
      dataIndex: 'pull_count',
      key: 'pull_count',
      width: 80,
      render: (n?: number) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{n ?? '—'}</Text>
      ),
    },
  ];

  // ─── Tag columns ─────────────────────────────────────────────────────────

  const tagColumns: TableColumnsType<RegistryTag> = [
    {
      title: t('cicd:registryBrowser.tagName'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <Tag color="blue">{name}</Tag>,
    },
    {
      title: t('cicd:registryBrowser.size'),
      dataIndex: 'size',
      key: 'size',
      width: 90,
      render: (size?: number) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{formatBytes(size)}</Text>
      ),
    },
    {
      title: t('cicd:registryBrowser.digest'),
      dataIndex: 'digest',
      key: 'digest',
      ellipsis: true,
      render: (d?: string) =>
        d ? (
          <Tooltip title={d}>
            <Text
              type="secondary"
              style={{ fontSize: token.fontSizeSM, fontFamily: 'monospace' }}
            >
              {d.slice(0, 19)}…
            </Text>
          </Tooltip>
        ) : (
          <Text type="secondary">—</Text>
        ),
    },
    {
      title: t('cicd:registryBrowser.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 140,
      render: (time?: string) =>
        time ? (
          <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
            {dayjs(time).format('YYYY-MM-DD HH:mm')}
          </Text>
        ) : (
          <Text type="secondary">—</Text>
        ),
    },
  ];

  // ─── Render ──────────────────────────────────────────────────────────────

  return (
    <Drawer
      title={t('cicd:registryBrowser.title', { name: registry?.name ?? '' })}
      open={open}
      onClose={() => { setSelectedRepo(null); onClose(); }}
      width={900}
    >
      <Flex gap={token.marginMD} style={{ height: '100%' }} align="flex-start">
        {/* Left: Repositories */}
        <div style={{ width: 300, flexShrink: 0 }}>
          <Text strong style={{ display: 'block', marginBottom: token.marginSM }}>
            {t('cicd:registryBrowser.repositories')}
          </Text>
          <Table<RegistryRepository>
            columns={repoColumns}
            dataSource={repos}
            rowKey="name"
            loading={repoLoading}
            size="small"
            pagination={false}
            scroll={{ y: 500 }}
            rowClassName={(r) =>
              r.name === selectedRepo ? 'ant-table-row-selected' : ''
            }
            onRow={(r) => ({ onClick: () => setSelectedRepo(r.name) })}
            locale={{ emptyText: <EmptyState description={t('cicd:registryBrowser.noRepos')} /> }}
          />
        </div>

        {/* Right: Tags */}
        <div style={{ flex: 1, minWidth: 0 }}>
          <Text strong style={{ display: 'block', marginBottom: token.marginSM }}>
            {selectedRepo
              ? `${t('cicd:registryBrowser.tags')}: ${selectedRepo}`
              : t('cicd:registryBrowser.selectRepo')}
          </Text>
          {selectedRepo ? (
            <Table<RegistryTag>
              columns={tagColumns}
              dataSource={tags}
              rowKey="name"
              loading={tagLoading}
              size="small"
              pagination={{ pageSize: 20, showSizeChanger: false }}
              scroll={{ x: 'max-content' }}
              locale={{ emptyText: <EmptyState description={t('cicd:registryBrowser.noTags')} /> }}
            />
          ) : (
            <EmptyState description={t('cicd:registryBrowser.selectRepo')} />
          )}
        </div>
      </Flex>
    </Drawer>
  );
};

export default RegistryBrowser;

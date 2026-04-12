import React from 'react';
import {
  Card,
  Table,
  Input,
  Button,
  Flex,
  Space,
  Tooltip,
  theme,
} from 'antd';
import type { TableProps } from 'antd';
import { SearchOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import EmptyState from './EmptyState';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface TableListLayoutProps<T> {
  // ── Toolbar ───────────────────────────────────────────────────────────────
  /** Controlled search text value */
  searchText?: string;
  /** Called when the search input changes */
  onSearch?: (text: string) => void;
  /** Search input placeholder (defaults to common:search.placeholder) */
  searchPlaceholder?: string;
  /** Additional filter selects / inputs rendered to the right of the search box */
  filters?: React.ReactNode;
  /** Action buttons rendered on the right side of the toolbar (e.g. Create button) */
  extra?: React.ReactNode;
  /** Refresh callback — renders a reload button when provided */
  onRefresh?: () => void;
  /** Shows spinner on the reload button */
  refreshing?: boolean;

  // ── Table ─────────────────────────────────────────────────────────────────
  /** All standard Ant Design Table props */
  tableProps: TableProps<T>;
}

// ─── Component ─────────────────────────────────────────────────────────────

/**
 * TableListLayout — Card + toolbar (search/filters/actions) + Table.
 *
 * Enforces:
 *   - size="small" on Table
 *   - scroll={{ x: 'max-content' }}
 *   - EmptyState as locale.emptyText
 *   - token-based spacing
 *
 * Usage:
 *   <TableListLayout
 *     searchText={searchText}
 *     onSearch={setSearchText}
 *     extra={<Button type="primary" ...>Create</Button>}
 *     onRefresh={refetch}
 *     refreshing={isLoading}
 *     tableProps={{ columns, dataSource: filtered, rowKey: 'id', loading: isLoading }}
 *   />
 */
export function TableListLayout<T extends object>({
  searchText,
  onSearch,
  searchPlaceholder,
  filters,
  extra,
  onRefresh,
  refreshing,
  tableProps,
}: TableListLayoutProps<T>) {
  const { t } = useTranslation('common');
  const { token } = theme.useToken();

  const showToolbar =
    onSearch !== undefined ||
    filters !== undefined ||
    extra !== undefined ||
    onRefresh !== undefined;

  return (
    <Card variant="borderless">
      {showToolbar && (
        <Flex
          justify="space-between"
          align="center"
          style={{ marginBottom: token.marginMD }}
        >
          {/* Left: search + filters */}
          <Space wrap>
            {onSearch !== undefined && (
              <Input
                prefix={<SearchOutlined />}
                placeholder={searchPlaceholder ?? t('search.placeholder', '搜尋...')}
                allowClear
                value={searchText}
                style={{ width: 240 }}
                onChange={(e) => onSearch(e.target.value)}
              />
            )}
            {filters}
          </Space>

          {/* Right: refresh + custom actions */}
          <Space>
            {onRefresh && (
              <Tooltip title={t('actions.refresh', '重新整理')}>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={onRefresh}
                  loading={refreshing}
                />
              </Tooltip>
            )}
            {extra}
          </Space>
        </Flex>
      )}

      <Table<T>
        size="small"
        scroll={{ x: 'max-content' }}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (total) => t('pagination.total', { total, defaultValue: `共 ${total} 筆` }),
        }}
        locale={{
          emptyText: <EmptyState description={t('messages.noData', '暫無資料')} />,
        }}
        {...tableProps}
      />
    </Card>
  );
}

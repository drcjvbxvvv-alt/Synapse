import React from 'react';
import { Button, Space, Select, Card, Table, Input, DatePicker } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import EmptyState from '../../../components/EmptyState';
import type { Dayjs } from 'dayjs';
import type { LogEntry } from '../../../services/logService';
import { getSearchColumns } from '../columns';
import { levelOptions } from '../constants';

const { RangePicker } = DatePicker;

interface SearchTabProps {
  searchResults: LogEntry[];
  searchLoading: boolean;
  searchKeyword: string;
  setSearchKeyword: (v: string) => void;
  searchNamespaces: string[];
  setSearchNamespaces: (v: string[]) => void;
  searchLevels: string[];
  setSearchLevels: (v: string[]) => void;
  searchDateRange: [Dayjs, Dayjs] | null;
  setSearchDateRange: (v: [Dayjs, Dayjs] | null) => void;
  namespaces: string[];
  handleSearch: () => void;
}

export const SearchTab: React.FC<SearchTabProps> = ({
  searchResults,
  searchLoading,
  searchKeyword,
  setSearchKeyword,
  searchNamespaces,
  setSearchNamespaces,
  searchLevels,
  setSearchLevels,
  searchDateRange,
  setSearchDateRange,
  namespaces,
  handleSearch,
}) => {
  const { t } = useTranslation(['logs', 'common']);

  return (
    <div>
      <Card size="small" style={{ marginBottom: 16 }}>
        <Space wrap style={{ width: '100%' }}>
          <Input.Search
            placeholder={t('logs:center.searchKeywordPlaceholder')}
            style={{ width: 300 }}
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            onSearch={handleSearch}
            enterButton={<SearchOutlined />}
          />

          <Select
            mode="multiple"
            placeholder={t('common:table.namespace')}
            style={{ width: 200 }}
            value={searchNamespaces}
            onChange={setSearchNamespaces}
            options={namespaces.map((ns) => ({ label: ns, value: ns }))}
          />

          <Select
            mode="multiple"
            placeholder={t('logs:center.logLevel')}
            style={{ width: 150 }}
            value={searchLevels}
            onChange={setSearchLevels}
            options={levelOptions}
          />

          <RangePicker
            showTime
            value={searchDateRange}
            onChange={(dates) => setSearchDateRange(dates as [Dayjs, Dayjs] | null)}
            placeholder={[t('logs:center.startTime'), t('logs:center.endTime')]}
          />

          <Button
            type="primary"
            icon={<SearchOutlined />}
            onClick={handleSearch}
            loading={searchLoading}
          >
            {t('logs:center.searchBtn')}
          </Button>
        </Space>
      </Card>

      <Card size="small" title={t('logs:center.searchResults', { count: searchResults.length })}>
        <Table
          columns={getSearchColumns(t)}
          dataSource={searchResults}
          rowKey="id"
          loading={searchLoading}
          pagination={{
            pageSize: 50,
            showSizeChanger: true,
            showTotal: (total) => t('common:table.totalCount', { count: total }),
          }}
          size="small"
          scroll={{ y: 'calc(100vh - 550px)' }}
          locale={{ emptyText: <EmptyState description={t('logs:center.noSearchResults')} /> }}
        />
      </Card>
    </div>
  );
};

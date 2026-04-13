import React from 'react';
import { Button, Space, Table, DatePicker, Tooltip } from 'antd';
import { ReloadOutlined, DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import EmptyState from '../../../components/EmptyState';
import type { CostItem } from '../../../services/costService';
import { costService } from '../../../services/costService';
import { getWorkloadCostColumns } from '../columns';

interface WorkloadCostTabProps {
  wlCosts: CostItem[];
  wlTotal: number;
  wlPage: number;
  wlLoading: boolean;
  month: string;
  clusterId: string;
  onMonthChange: (month: string) => void;
  onPageChange: (page: number) => void;
  onRefresh: () => void;
}

export const WorkloadCostTab: React.FC<WorkloadCostTabProps> = ({
  wlCosts,
  wlTotal,
  wlPage,
  wlLoading,
  month,
  clusterId,
  onMonthChange,
  onPageChange,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <DatePicker.MonthPicker
          value={dayjs(month, 'YYYY-MM')}
          onChange={v => v && onMonthChange(v.format('YYYY-MM'))}
          allowClear={false}
          style={{ width: 130 }}
        />
        <Button icon={<ReloadOutlined />} onClick={onRefresh}>
          {t('common:actions.refresh')}
        </Button>
        <Tooltip title={t('cost:export.button')}>
          <Button
            icon={<DownloadOutlined />}
            href={costService.getExportURL(clusterId, month)}
            target="_blank"
          >
            {t('cost:export.button')}
          </Button>
        </Tooltip>
      </Space>

      <Table
        rowKey="name"
        columns={getWorkloadCostColumns(t)}
        dataSource={wlCosts}
        loading={wlLoading}
        size="small"
        scroll={{ x: 800 }}
        pagination={{
          current: wlPage,
          total: wlTotal,
          pageSize: 20,
          onChange: onPageChange,
        }}
        locale={{ emptyText: <EmptyState description={t('cost:noWorkloads')} /> }}
      />
    </div>
  );
};

import React from 'react';
import { Row, Col, Card, Select } from 'antd';
import { Line } from '@ant-design/charts';
import { useTranslation } from 'react-i18next';
import type { TrendData } from '../hooks/useOverview';

interface TrendChartsProps {
  podTrendData: TrendData[];
  nodeTrendData: TrendData[];
  podTimeRange: '7d' | '30d';
  nodeTimeRange: '7d' | '30d';
  onPodTimeRangeChange: (val: '7d' | '30d') => void;
  onNodeTimeRangeChange: (val: '7d' | '30d') => void;
  getTrendConfig: (data: TrendData[], yAxisTitle: string) => object;
}

const cardStyle = { boxShadow: '0 1px 4px rgba(0,0,0,0.08)', borderRadius: 8 };
const cardHeadStyle = { borderBottom: '1px solid #f0f0f0', padding: '12px 16px', minHeight: 48 };

const EmptyChart: React.FC = () => {
  const { t } = useTranslation('common');
  return (
    <div style={{ textAlign: 'center', padding: 60, color: '#9ca3af' }}>
      {t('messages.noData')}
    </div>
  );
};

export const TrendCharts: React.FC<TrendChartsProps> = ({
  podTrendData,
  nodeTrendData,
  podTimeRange,
  nodeTimeRange,
  onPodTimeRangeChange,
  onNodeTimeRangeChange,
  getTrendConfig,
}) => {
  const { t } = useTranslation(['overview', 'common']);

  const chartBodyStyle = { padding: '8px 16px', height: 'calc(100% - 57px)' };

  const timeRangeSelect = (value: '7d' | '30d', onChange: (val: '7d' | '30d') => void) => (
    <Select value={value} onChange={onChange} size="small" style={{ width: 100 }}>
      <Select.Option value="7d">{t('common:units.last7Days')}</Select.Option>
      <Select.Option value="30d">{t('common:units.last30Days')}</Select.Option>
    </Select>
  );

  return (
    <Row gutter={16}>
      <Col span={12}>
        <Card
          title={t('trend.podTrend')}
          variant="borderless"
          style={{ ...cardStyle, height: 400 }}
          headStyle={cardHeadStyle}
          styles={{ body: chartBodyStyle }}
          extra={timeRangeSelect(podTimeRange, onPodTimeRangeChange)}
        >
          {podTrendData.length > 0 ? (
            <Line {...getTrendConfig(podTrendData, 'Pod')} height={300} />
          ) : (
            <EmptyChart />
          )}
        </Card>
      </Col>
      <Col span={12}>
        <Card
          title={t('trend.nodeTrend')}
          variant="borderless"
          style={{ ...cardStyle, height: 400 }}
          headStyle={cardHeadStyle}
          styles={{ body: chartBodyStyle }}
          extra={timeRangeSelect(nodeTimeRange, onNodeTimeRangeChange)}
        >
          {nodeTrendData.length > 0 ? (
            <Line {...getTrendConfig(nodeTrendData, 'Node')} height={300} />
          ) : (
            <EmptyChart />
          )}
        </Card>
      </Col>
    </Row>
  );
};

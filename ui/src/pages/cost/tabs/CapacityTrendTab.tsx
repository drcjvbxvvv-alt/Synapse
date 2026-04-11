import React from 'react';
import EmptyState from '@/components/EmptyState';
import { Button, Space, Row, Col, Card, Statistic, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid,
  Tooltip as RechartTooltip, Legend, ResponsiveContainer,
} from 'recharts';
import type { CapacityTrendPoint, ForecastResult } from '../../../services/costService';

interface CapacityTrendTabProps {
  capacityTrend: CapacityTrendPoint[];
  capacityTrendLoading: boolean;
  forecast: ForecastResult | null;
  forecastLoading: boolean;
  onRefresh: () => void;
}

export const CapacityTrendTab: React.FC<CapacityTrendTabProps> = ({
  capacityTrend,
  capacityTrendLoading,
  forecast,
  forecastLoading,
  onRefresh,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={onRefresh} loading={capacityTrendLoading || forecastLoading}>
          {t('common:actions.refresh')}
        </Button>
      </Space>

      {capacityTrend.length === 0 && !capacityTrendLoading ? (
        <EmptyState description={t('cost:capacityTrend.noData')} />
      ) : (
        <>
          <Card title="CPU & 記憶體佔用率月度趨勢" size="small" style={{ marginBottom: 16 }} loading={capacityTrendLoading}>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={capacityTrend} margin={{ top: 10, right: 30, left: 10, bottom: 5 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="month" />
                <YAxis unit="%" domain={[0, 100]} />
                <RechartTooltip formatter={(v) => [`${Number(v ?? 0).toFixed(1)}%`]} />
                <Legend />
                <Line type="monotone" dataKey="cpu_occupancy_percent" name="CPU 佔用率" stroke="#7eb8d4" strokeWidth={2} dot={{ r: 4 }} />
                <Line type="monotone" dataKey="memory_occupancy_percent" name="記憶體佔用率" stroke="#a8c9a5" strokeWidth={2} dot={{ r: 4 }} />
              </LineChart>
            </ResponsiveContainer>
          </Card>

          <Card title="容量耗盡預測（線性外推）" size="small" loading={forecastLoading}>
            {forecast && (
              <Row gutter={16}>
                <Col xs={24} sm={12}>
                  <Card size="small" title="CPU 佔用率預測" style={{ background: '#fafafa' }}>
                    <Statistic title="當前佔用率" value={forecast.current_cpu_percent} precision={1} suffix="%" />
                    <div style={{ marginTop: 12 }}>
                      <Space direction="vertical" size={4}>
                        <span>到達 80%：
                          {forecast.cpu_80_percent_date
                            ? <Tag color="orange">{forecast.cpu_80_percent_date}</Tag>
                            : <Tag color="green">預測期內不到達</Tag>}
                        </span>
                        <span>到達 100%：
                          {forecast.cpu_100_percent_date
                            ? <Tag color="red">{forecast.cpu_100_percent_date}</Tag>
                            : <Tag color="green">預測期內不到達</Tag>}
                        </span>
                      </Space>
                    </div>
                  </Card>
                </Col>
                <Col xs={24} sm={12}>
                  <Card size="small" title="記憶體佔用率預測" style={{ background: '#fafafa' }}>
                    <Statistic title="當前佔用率" value={forecast.current_memory_percent} precision={1} suffix="%" />
                    <div style={{ marginTop: 12 }}>
                      <Space direction="vertical" size={4}>
                        <span>到達 80%：
                          {forecast.memory_80_percent_date
                            ? <Tag color="orange">{forecast.memory_80_percent_date}</Tag>
                            : <Tag color="green">預測期內不到達</Tag>}
                        </span>
                        <span>到達 100%：
                          {forecast.memory_100_percent_date
                            ? <Tag color="red">{forecast.memory_100_percent_date}</Tag>
                            : <Tag color="green">預測期內不到達</Tag>}
                        </span>
                      </Space>
                    </div>
                  </Card>
                </Col>
                <Col xs={24} style={{ marginTop: 8 }}>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    基於近 {forecast.based_on_months} 個月歷史資料進行線性外推，預測期 180 天。數據點不足或趨勢下降時顯示「預測期內不到達」。
                  </Typography.Text>
                </Col>
              </Row>
            )}
          </Card>
        </>
      )}
    </div>
  );
};

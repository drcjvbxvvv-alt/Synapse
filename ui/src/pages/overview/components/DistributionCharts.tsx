import React from 'react';
import { Row, Col, Card } from 'antd';
import { Pie } from '@ant-design/charts';
import { useTranslation } from 'react-i18next';
import type { ChartDistribution } from '../hooks/useOverview';

interface DistributionChartsProps {
  podDistribution: ChartDistribution[];
  nodeDistribution: ChartDistribution[];
  cpuDistribution: ChartDistribution[];
  memoryDistribution: ChartDistribution[];
  totalPods: number;
  totalNodes: number;
  totalCPU: number;
  totalMemory: number;
  formatNumber: (num: number, unit?: string) => string;
  getPieConfig: (data: ChartDistribution[], labelSuffix?: string, title?: string) => object;
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

export const DistributionCharts: React.FC<DistributionChartsProps> = ({
  podDistribution,
  nodeDistribution,
  cpuDistribution,
  memoryDistribution,
  totalPods,
  totalNodes,
  totalCPU,
  totalMemory,
  formatNumber,
  getPieConfig,
}) => {
  const { t } = useTranslation(['overview', 'common']);

  const chartBodyStyle = { padding: '8px 16px', height: 'calc(100% - 57px)' };

  return (
    <>
      {/* Row 4: Pod + Node distribution */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={12}>
          <Card
            title={
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span>{t('distribution.podDistribution')}</span>
                <span style={{ fontSize: 16, color: '#3b82f6', fontWeight: 'bold' }}>
                  {formatNumber(totalPods, t('common:units.count'))}
                </span>
              </div>
            }
            bordered={false}
            style={{ ...cardStyle, height: 400 }}
            headStyle={cardHeadStyle}
            styles={{ body: chartBodyStyle }}
          >
            {podDistribution.length > 0 ? (
              <Pie {...getPieConfig(podDistribution, t('common:units.count'), t('distribution.podDistribution'))} height={300} />
            ) : (
              <EmptyChart />
            )}
          </Card>
        </Col>
        <Col span={12}>
          <Card
            title={
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span>{t('distribution.nodeDistribution')}</span>
                <span style={{ fontSize: 16, color: '#3b82f6', fontWeight: 'bold' }}>
                  {totalNodes}{t('common:units.count')}
                </span>
              </div>
            }
            bordered={false}
            style={{ ...cardStyle, height: 400 }}
            headStyle={cardHeadStyle}
            styles={{ body: chartBodyStyle }}
          >
            {nodeDistribution.length > 0 ? (
              <Pie {...getPieConfig(nodeDistribution, t('common:units.count'), t('distribution.nodeDistribution'))} height={300} />
            ) : (
              <EmptyChart />
            )}
          </Card>
        </Col>
      </Row>

      {/* Row 5: CPU + Memory distribution */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={12}>
          <Card
            title={
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span>{t('distribution.cpuDistribution')}</span>
                <span style={{ fontSize: 16, color: '#3b82f6', fontWeight: 'bold' }}>
                  {formatNumber(totalCPU, t('common:units.cores'))}
                </span>
              </div>
            }
            bordered={false}
            style={{ ...cardStyle, height: 400 }}
            headStyle={cardHeadStyle}
            styles={{ body: chartBodyStyle }}
          >
            {cpuDistribution.length > 0 ? (
              <Pie {...getPieConfig(cpuDistribution, t('common:units.cores'), t('distribution.cpuDistribution'))} height={300} />
            ) : (
              <EmptyChart />
            )}
          </Card>
        </Col>
        <Col span={12}>
          <Card
            title={
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span>{t('distribution.memoryDistribution')}</span>
                <span style={{ fontSize: 16, color: '#3b82f6', fontWeight: 'bold' }}>
                  {(totalMemory / 1024).toFixed(2)}TB
                </span>
              </div>
            }
            bordered={false}
            style={{ ...cardStyle, height: 400 }}
            headStyle={cardHeadStyle}
            styles={{ body: chartBodyStyle }}
          >
            {memoryDistribution.length > 0 ? (
              <Pie {...getPieConfig(memoryDistribution, 'GB', t('distribution.memoryDistribution'))} height={300} />
            ) : (
              <EmptyChart />
            )}
          </Card>
        </Col>
      </Row>
    </>
  );
};

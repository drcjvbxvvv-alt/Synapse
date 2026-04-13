import React from 'react';
import {
  Drawer,
  Descriptions,
  Tag,
  Progress,
  Spin,
  Alert,
  Space,
  Typography,
  theme,
  Statistic,
  Row,
  Col,
  Card,
  Badge,
} from 'antd';
import { ThunderboltOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { sloService, type SLO, type SLOStatus, type SLOStatusValue } from '../../services/sloService';

const { Text } = Typography;

const STATUS_COLOR: Record<SLOStatusValue, string> = {
  ok:       'success',
  warning:  'warning',
  critical: 'error',
  unknown:  'default',
};

interface Props {
  open: boolean;
  clusterId: number;
  slo: SLO;
  onClose: () => void;
}


const BurnRateCell: React.FC<{ label: string; value: number | null; warning: number; critical: number }> = ({
  label, value, warning, critical,
}) => {
  const { token } = theme.useToken();
  if (value == null || isNaN(value)) {
    return (
      <div>
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{label}</Text>
        <div style={{ fontSize: token.fontSizeLG, fontWeight: 600, color: token.colorTextTertiary }}>—</div>
      </div>
    );
  }
  const color = value >= critical
    ? token.colorError
    : value >= warning
    ? token.colorWarning
    : token.colorSuccess;

  return (
    <div>
      <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>{label}</Text>
      <div style={{ fontSize: token.fontSizeLG, fontWeight: 600, color }}>
        {value.toFixed(2)}x
      </div>
    </div>
  );
};

const SLOStatusDrawer: React.FC<Props> = ({ open, clusterId, slo, onClose }) => {
  const { token } = theme.useToken();
  const { t } = useTranslation(['slo', 'common']);

  const getStatusLabel = (status: SLOStatusValue) => {
    const statusMap: Record<SLOStatusValue, string> = {
      ok: t('slo:status.ok'),
      warning: t('slo:status.warning'),
      critical: t('slo:status.critical'),
      unknown: t('slo:status.unknown'),
    };
    return statusMap[status] || status;
  };

  const { data: status, isLoading, isError } = useQuery<SLOStatus>({
    queryKey: ['slo-status', clusterId, slo.id],
    queryFn: () => sloService.getStatus(clusterId, slo.id),
    enabled: open,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  });

  const ebPct = status?.has_data
    ? Math.round((status.error_budget_remaining ?? 0) * 100)
    : null;

  return (
    <Drawer
      title={t('slo:drawer.title', { name: slo.name })}
      open={open}
      onClose={onClose}
      width={560}
    >
      {isLoading && (
        <div style={{ textAlign: 'center', padding: token.paddingLG * 2 }}>
          <Spin tip={t('slo:drawer.loading')} size="large" />
        </div>
      )}

      {isError && (
        <Alert type="error" showIcon message={t('slo:drawer.errorLoadingStatus')} />
      )}

      {status && !isLoading && (
        <Space direction="vertical" style={{ width: '100%' }} size={token.marginMD}>

          {/* Chaos active banner */}
          {status.chaos_active && (
            <Alert
              type="warning"
              showIcon
              icon={<ThunderboltOutlined />}
              message={t('slo:drawer.chaosActive')}
              description={t('slo:drawer.chaosActiveDesc')}
            />
          )}

          {/* Overall status */}
          <div style={{ textAlign: 'center', padding: `${token.paddingMD}px 0` }}>
            <Tag color={STATUS_COLOR[status.status]} style={{ fontSize: 14, padding: '4px 16px' }}>
              {getStatusLabel(status.status)}
            </Tag>
            {status.chaos_active && (
              <Badge
                count={t('slo:drawer.chaosPaused')}
                style={{ backgroundColor: '#faad14', marginLeft: 8 }}
              />
            )}
          </div>

          {!status.has_data && (
            <Alert
              type="info"
              showIcon
              message={t('slo:drawer.noData')}
              description={t('slo:drawer.noDataDesc')}
            />
          )}

          {status.has_data && (
            <>
              {/* SLI + Error Budget */}
              <Card size="small" title={t('slo:drawer.sliAndErrorBudget')} variant="borderless"
                style={{ background: token.colorBgLayout }}>
                <Row gutter={token.marginMD}>
                  <Col span={12}>
                    <Statistic
                      title={t('slo:drawer.sliTitle', { window: slo.window })}
                      value={status.sli_percent ?? 0}
                      precision={4}
                      suffix="%"
                      valueStyle={{
                        color: (status.sli_percent ?? 0) >= slo.target * 100
                          ? token.colorSuccess
                          : token.colorError,
                      }}
                    />
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      {t('slo:drawer.target')}：{(slo.target * 100).toFixed(3)}%
                    </Text>
                  </Col>
                  <Col span={12}>
                    <div style={{ marginBottom: token.marginXS }}>
                      <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                        {t('slo:drawer.remainingErrorBudget')}
                      </Text>
                    </div>
                    <Progress
                      percent={ebPct ?? 0}
                      size="small"
                      status={
                        (ebPct ?? 0) < 10 ? 'exception'
                          : (ebPct ?? 0) < 30 ? 'normal'
                          : 'success'
                      }
                    />
                    <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                      {t('slo:drawer.remaining', { percent: ebPct ?? 0 })}
                    </Text>
                  </Col>
                </Row>
              </Card>

              {/* Burn rates */}
              <Card size="small" title={t('slo:drawer.burnRates')} variant="borderless"
                style={{ background: token.colorBgLayout }}>
                <Row gutter={token.marginMD}>
                  <Col span={6}>
                    <BurnRateCell
                      label="1h"
                      value={status.burn_rate_1h}
                      warning={slo.burn_rate_warning}
                      critical={slo.burn_rate_critical}
                    />
                  </Col>
                  <Col span={6}>
                    <BurnRateCell
                      label="6h"
                      value={status.burn_rate_6h}
                      warning={slo.burn_rate_warning}
                      critical={slo.burn_rate_critical}
                    />
                  </Col>
                  <Col span={6}>
                    <BurnRateCell
                      label="24h"
                      value={status.burn_rate_24h}
                      warning={slo.burn_rate_warning}
                      critical={slo.burn_rate_critical}
                    />
                  </Col>
                  <Col span={6}>
                    <BurnRateCell
                      label={slo.window}
                      value={status.burn_rate_window}
                      warning={slo.burn_rate_warning}
                      critical={slo.burn_rate_critical}
                    />
                  </Col>
                </Row>
                <div style={{ marginTop: token.marginSM, fontSize: token.fontSizeSM, color: token.colorTextSecondary }}>
                  {t('slo:drawer.threshold', { warning: slo.burn_rate_warning, critical: slo.burn_rate_critical })}
                </div>
              </Card>
            </>
          )}

          {/* SLO config summary */}
          <Descriptions
            size="small"
            column={1}
            bordered
            title={t('slo:drawer.configTitle')}
            labelStyle={{ width: 120 }}
          >
            <Descriptions.Item label={t('slo:drawer.sliType')}>{slo.sli_type}</Descriptions.Item>
            <Descriptions.Item label={t('slo:drawer.promql')}>
              <Text code style={{ fontSize: token.fontSizeSM, wordBreak: 'break-all' }}>
                {slo.prom_query}
              </Text>
            </Descriptions.Item>
            {slo.total_query && (
              <Descriptions.Item label={t('slo:drawer.totalQuery')}>
                <Text code style={{ fontSize: token.fontSizeSM, wordBreak: 'break-all' }}>
                  {slo.total_query}
                </Text>
              </Descriptions.Item>
            )}
            <Descriptions.Item label={t('slo:drawer.description')}>
              {slo.description || <Text type="secondary">—</Text>}
            </Descriptions.Item>
          </Descriptions>
        </Space>
      )}
    </Drawer>
  );
};

export default SLOStatusDrawer;

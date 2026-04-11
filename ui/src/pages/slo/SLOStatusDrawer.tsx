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
import { useQuery } from '@tanstack/react-query';
import { sloService, type SLO, type SLOStatus, type SLOStatusValue } from '../../services/sloService';

const { Text } = Typography;

const STATUS_COLOR: Record<SLOStatusValue, string> = {
  ok:       'success',
  warning:  'warning',
  critical: 'error',
  unknown:  'default',
};

const STATUS_LABEL: Record<SLOStatusValue, string> = {
  ok:       '正常',
  warning:  '警告',
  critical: '嚴重',
  unknown:  '無資料',
};

interface Props {
  open: boolean;
  clusterId: number;
  slo: SLO;
  onClose: () => void;
}

function fmt(v: number | null | undefined, digits = 4): string {
  if (v == null || isNaN(v)) return '—';
  return v.toFixed(digits);
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
      title={`SLO 狀態：${slo.name}`}
      open={open}
      onClose={onClose}
      width={560}
    >
      {isLoading && (
        <div style={{ textAlign: 'center', padding: token.paddingLG * 2 }}>
          <Spin tip="計算中..." size="large" />
        </div>
      )}

      {isError && (
        <Alert type="error" showIcon message="無法載入 SLO 狀態，請確認 Prometheus 連線正常。" />
      )}

      {status && !isLoading && (
        <Space direction="vertical" style={{ width: '100%' }} size={token.marginMD}>

          {/* Chaos active banner */}
          {status.chaos_active && (
            <Alert
              type="warning"
              showIcon
              icon={<ThunderboltOutlined />}
              message="混沌實驗進行中"
              description="此 Namespace 目前有 Chaos Mesh 實驗正在注入故障，SLO 告警已自動暫停，評估數據可能受影響。"
            />
          )}

          {/* Overall status */}
          <div style={{ textAlign: 'center', padding: `${token.paddingMD}px 0` }}>
            <Tag color={STATUS_COLOR[status.status]} style={{ fontSize: 14, padding: '4px 16px' }}>
              {STATUS_LABEL[status.status]}
            </Tag>
            {status.chaos_active && (
              <Badge
                count="混沌暫停"
                style={{ backgroundColor: '#faad14', marginLeft: 8 }}
              />
            )}
          </div>

          {!status.has_data && (
            <Alert
              type="info"
              showIcon
              message="Prometheus 尚無資料"
              description="SLO 已設定但 Prometheus 查詢未回傳數值，請確認 PromQL 正確且指標已存在。"
            />
          )}

          {status.has_data && (
            <>
              {/* SLI + Error Budget */}
              <Card size="small" title="SLI 與錯誤預算" variant="borderless"
                style={{ background: token.colorBgLayout }}>
                <Row gutter={token.marginMD}>
                  <Col span={12}>
                    <Statistic
                      title={`SLI（${slo.window}）`}
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
                      目標：{(slo.target * 100).toFixed(3)}%
                    </Text>
                  </Col>
                  <Col span={12}>
                    <div style={{ marginBottom: token.marginXS }}>
                      <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                        剩餘誤差預算
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
                      {ebPct}% 剩餘
                    </Text>
                  </Col>
                </Row>
              </Card>

              {/* Burn rates */}
              <Card size="small" title="燃燒率" variant="borderless"
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
                  閾值：≥{slo.burn_rate_warning}x 警告 ／ ≥{slo.burn_rate_critical}x 嚴重
                </div>
              </Card>
            </>
          )}

          {/* SLO config summary */}
          <Descriptions
            size="small"
            column={1}
            bordered
            title="SLO 設定"
            labelStyle={{ width: 120 }}
          >
            <Descriptions.Item label="SLI 類型">{slo.sli_type}</Descriptions.Item>
            <Descriptions.Item label="PromQL">
              <Text code style={{ fontSize: token.fontSizeSM, wordBreak: 'break-all' }}>
                {slo.prom_query}
              </Text>
            </Descriptions.Item>
            {slo.total_query && (
              <Descriptions.Item label="TotalQuery">
                <Text code style={{ fontSize: token.fontSizeSM, wordBreak: 'break-all' }}>
                  {slo.total_query}
                </Text>
              </Descriptions.Item>
            )}
            <Descriptions.Item label="描述">
              {slo.description || <Text type="secondary">—</Text>}
            </Descriptions.Item>
          </Descriptions>
        </Space>
      )}
    </Drawer>
  );
};

export default SLOStatusDrawer;

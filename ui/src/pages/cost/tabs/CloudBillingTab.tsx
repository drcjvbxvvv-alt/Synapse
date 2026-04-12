import EmptyState from '@/components/EmptyState';
import React from 'react';
import {
  Button, Space, Row, Col, Card, Statistic, Form, Input, Radio,
  Divider, Table, Spin, Alert, DatePicker, Typography,
} from 'antd';
import { SaveOutlined, SyncOutlined, ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip as RechartTooltip, ResponsiveContainer,
} from 'recharts';
import type { FormInstance } from 'antd';
import type { CloudBillingConfig, CloudBillingOverview, UpdateBillingConfigReq } from '../../../services/cloudBillingService';
import { GRID_STYLE, TOOLTIP_STYLE } from '../constants';

const { Text } = Typography;

interface CloudBillingTabProps {
  billingConfig: CloudBillingConfig | null;
  billingConfigLoading: boolean;
  billingOverview: CloudBillingOverview | null;
  billingOverviewLoading: boolean;
  billingProvider: 'disabled' | 'aws' | 'gcp';
  setBillingProvider: (provider: 'disabled' | 'aws' | 'gcp') => void;
  billingSaving: boolean;
  billingSyncing: boolean;
  billingMonth: string;
  setBillingMonth: (month: string) => void;
  billingForm: FormInstance<UpdateBillingConfigReq>;
  saveBillingConfig: () => void;
  syncBilling: () => void;
  loadBillingOverview: (month?: string) => void;
}

export const CloudBillingTab: React.FC<CloudBillingTabProps> = ({
  billingConfig,
  billingConfigLoading,
  billingOverview,
  billingOverviewLoading,
  billingProvider,
  setBillingProvider,
  billingSaving,
  billingSyncing,
  billingMonth,
  setBillingMonth,
  billingForm,
  saveBillingConfig,
  syncBilling,
  loadBillingOverview,
}) => {
  const { t } = useTranslation(['cost', 'common']);

  return (
    <div>
      {/* Config card */}
      <Card
        title={t('cost:billing.configTitle')}
        size="small"
        style={{ marginBottom: 16 }}
        loading={billingConfigLoading}
        extra={
          <Space>
            <Button
              icon={<SaveOutlined />}
              type="primary"
              loading={billingSaving}
              onClick={saveBillingConfig}
            >
              {t('cost:billing.saveConfig')}
            </Button>
          </Space>
        }
      >
        <Form form={billingForm} layout="vertical" style={{ maxWidth: 600 }}>
          <Form.Item name="provider" label={t('cost:billing.provider')} initialValue="disabled">
            <Radio.Group onChange={e => setBillingProvider(e.target.value)}>
              <Radio.Button value="disabled">{t('cost:billing.providerDisabled')}</Radio.Button>
              <Radio.Button value="aws">AWS</Radio.Button>
              <Radio.Button value="gcp">GCP</Radio.Button>
            </Radio.Group>
          </Form.Item>

          {billingProvider === 'aws' && (
            <>
              <Divider orientation="left" plain style={{ fontSize: 13 }}>AWS Cost Explorer</Divider>
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item name="aws_access_key_id" label={t('cost:billing.accessKeyId')} rules={[{ required: true, message: t('common:validation.required') }]}>
                    <Input placeholder="AKIA..." />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item name="aws_secret_access_key" label={`${t('cost:billing.secretAccessKey')}${billingConfig?.aws_secret_set ? ` (${t('cost:billing.secretSet')})` : ''}`} rules={[{ required: true, message: t('common:validation.required') }]}>
                    <Input.Password placeholder={billingConfig?.aws_secret_set ? `******** (${t('cost:billing.keepOriginal')})` : t('cost:billing.inputSecret')} />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item name="aws_region" label={t('cost:billing.region')} initialValue="us-east-1" rules={[{ required: true, message: t('common:validation.required') }]}>
                    <Input placeholder="us-east-1" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item name="aws_linked_account_id" label={t('cost:billing.linkedAccountId')} rules={[{ required: true, message: t('common:validation.required') }]}>
                    <Input placeholder="123456789012" />
                  </Form.Item>
                </Col>
              </Row>
            </>
          )}

          {billingProvider === 'gcp' && (
            <>
              <Divider orientation="left" plain style={{ fontSize: 13 }}>GCP Cloud Billing</Divider>
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item name="gcp_project_id" label="Project ID" rules={[{ required: true, message: t('common:validation.required') }]}>
                    <Input placeholder="my-project-id" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item name="gcp_billing_account_id" label="Billing Account ID" rules={[{ required: true, message: t('common:validation.required') }]}>
                    <Input placeholder="XXXXXX-XXXXXX-XXXXXX" />
                  </Form.Item>
                </Col>
                <Col span={24}>
                  <Form.Item
                    name="gcp_service_account_json"
                    label={`${t('cost:billing.serviceAccountJson')}${billingConfig?.gcp_service_account_set ? ` (${t('cost:billing.secretSet')})` : ''}`}
                    rules={[{ required: true, message: t('common:validation.required') }]}
                  >
                    <Input.TextArea
                      rows={5}
                      placeholder={billingConfig?.gcp_service_account_set ? t('cost:billing.savedKeepOriginal') : t('cost:billing.pasteServiceAccount')}
                    />
                  </Form.Item>
                </Col>
              </Row>
            </>
          )}
        </Form>

        {billingConfig && billingConfig.provider !== 'disabled' && (
          <div style={{ marginTop: 8 }}>
            {billingConfig.last_synced_at
              ? <Typography.Text type="secondary">{t('cost:billing.lastSync')}{billingConfig.last_synced_at}</Typography.Text>
              : <Typography.Text type="secondary">{t('cost:billing.neverSynced')}</Typography.Text>}
            {billingConfig.last_error && (
              <Alert type="error" message={billingConfig.last_error} showIcon style={{ marginTop: 8 }} />
            )}
          </div>
        )}
      </Card>

      {/* Sync controls + overview */}
      {billingConfig?.provider !== 'disabled' && (
        <Card
          title={t('cost:billing.overview')}
          size="small"
          extra={
            <Space>
              <DatePicker.MonthPicker
                value={dayjs(billingMonth, 'YYYY-MM')}
                onChange={v => v && setBillingMonth(v.format('YYYY-MM'))}
                allowClear={false}
                style={{ width: 120 }}
              />
              <Button
                icon={<SyncOutlined />}
                loading={billingSyncing}
                onClick={syncBilling}
              >
                {t('cost:billing.syncBtn')}
              </Button>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => loadBillingOverview(billingMonth)}
                loading={billingOverviewLoading}
              >
                {t('common:actions.refresh')}
              </Button>
            </Space>
          }
        >
          <Spin spinning={billingOverviewLoading}>
            {!billingOverview ? (
              <EmptyState description={t('cost:billing.emptyData')} />
            ) : (
              <>
                <Row gutter={16} style={{ marginBottom: 20 }}>
                  <Col xs={24} sm={8}>
                    <Statistic
                      title={`${billingOverview.month} ${t('cost:billing.totalCost')}`}
                      value={billingOverview.total_amount}
                      precision={2}
                      suffix={billingOverview.currency}
                      valueStyle={{ color: '#1677ff' }}
                    />
                  </Col>
                  <Col xs={24} sm={8}>
                    <Statistic
                      title="CPU 單位成本"
                      value={billingOverview.cpu_unit_cost}
                      precision={4}
                      suffix="USD/core-hr"
                    />
                  </Col>
                  <Col xs={24} sm={8}>
                    <Statistic
                      title="記憶體單位成本"
                      value={billingOverview.memory_unit_cost}
                      precision={4}
                      suffix="USD/GiB-hr"
                    />
                  </Col>
                </Row>
                {billingOverview.services?.length > 0 && (
                  <Row gutter={16}>
                    <Col xs={24} lg={14}>
                      <ResponsiveContainer width="100%" height={260}>
                        <BarChart
                          data={billingOverview.services.slice(0, 12).map(s => ({
                            name: s.service.replace('Amazon ', '').replace('Google ', ''),
                            amount: +s.amount.toFixed(2),
                          }))}
                          layout="vertical"
                          margin={{ top: 5, right: 30, left: 10, bottom: 5 }}
                        >
                          <CartesianGrid {...GRID_STYLE} />
                          <XAxis type="number" unit={` ${billingOverview.currency}`} />
                          <YAxis type="category" dataKey="name" width={130} tick={{ fontSize: 11 }} />
                          <RechartTooltip {...TOOLTIP_STYLE} formatter={(v) => [`${billingOverview.currency} ${v}`, '費用']} />
                          <Bar dataKey="amount" fill="#5B8FF9"
                            radius={[0, 5, 5, 0]}
                            maxBarSize={44}
                            isAnimationActive={true}
                            animationBegin={0}
                            animationDuration={800}
                            animationEasing="ease-out"
                          />
                        </BarChart>
                      </ResponsiveContainer>
                    </Col>
                    <Col xs={24} lg={10}>
                      <Table
                        rowKey="id"
                        size="small"
                        dataSource={billingOverview.services}
                        pagination={{ pageSize: 10, size: 'small' }}
                        columns={[
                          { title: '服務', dataIndex: 'service', key: 'service', ellipsis: true },
                          {
                            title: '費用',
                            dataIndex: 'amount',
                            key: 'amount',
                            render: (v: number) => <Text strong>{`${billingOverview.currency} ${v.toFixed(2)}`}</Text>,
                            sorter: (a, b) => b.amount - a.amount,
                            defaultSortOrder: 'ascend' as const,
                          },
                        ]}
                      />
                    </Col>
                  </Row>
                )}
                {billingOverview.sync_error && (
                  <Alert type="warning" message={billingOverview.sync_error} showIcon style={{ marginTop: 12 }} />
                )}
              </>
            )}
          </Spin>
        </Card>
      )}
    </div>
  );
};

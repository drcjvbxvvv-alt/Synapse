import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Descriptions,
  Tag,
  Button,
  Space,
  Spin,
  message,
  Row,
  Col,
  Statistic,
  Typography,
  Divider,
  Empty,
} from 'antd';
import {
  ArrowLeftOutlined,
  ReloadOutlined,
  DatabaseOutlined,
  ContainerOutlined,
  CloudServerOutlined,
  KeyOutlined,
  TagsOutlined,
} from '@ant-design/icons';
import { getNamespaceDetail, type NamespaceDetailData } from '../../services/namespaceService';
import { useTranslation } from 'react-i18next';
const { Title, Text } = Typography;

const NamespaceDetail: React.FC = () => {
  const { clusterId, namespace } = useParams<{ clusterId: string; namespace: string }>();
  const navigate = useNavigate();
  const [namespaceDetail, setNamespaceDetail] = useState<NamespaceDetailData | null>(null);
  const [loading, setLoading] = useState(false);
  const { t } = useTranslation(["namespace", "common"]);

  const fetchNamespaceDetail = useCallback(async () => {
    if (!clusterId || !namespace) return;
    setLoading(true);
    try {
      const data = await getNamespaceDetail(Number(clusterId), namespace);
      setNamespaceDetail(data);
    } catch (error) {
      message.error(t('messages.fetchDetailError'));
      console.error('Error fetching namespace detail:', error);
    } finally {
      setLoading(false);
    }
  }, [clusterId, namespace, t]);

  useEffect(() => {
    fetchNamespaceDetail();
  }, [fetchNamespaceDetail]);

  const handleBack = () => {
    navigate(`/clusters/${clusterId}/namespaces`);
  };

  if (loading) {
    return (
      <div style={{ padding: 24, textAlign: 'center' }}>
        <Spin size="large" tip={t("common:messages.loading")} />
      </div>
    );
  }

  if (!namespaceDetail) {
    return (
      <div style={{ padding: 24 }}>
        <Empty description={t("detail.notFound")} />
      </div>
    );
  }

  return (
    <div style={{ padding: 24 }}>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {/* 头部操作栏 */}
        <Card>
          <Space>
            <Button icon={<ArrowLeftOutlined />} onClick={handleBack}>{t('common:actions.back')}</Button>
            <Divider type="vertical" />
            <Title level={4} style={{ margin: 0 }}>
              {t('detail.subtitle', { name: namespace })}
            </Title>
            <Tag color={namespaceDetail.status === 'Active' ? 'green' : 'orange'}>
              {namespaceDetail.status === 'Active' ? t('common:status.active') : namespaceDetail.status}
            </Tag>
            <Button icon={<ReloadOutlined />} onClick={fetchNamespaceDetail}>{t('common:actions.refresh')}</Button>
          </Space>
        </Card>

        {/* 资源统计卡片 */}
        <Card title={t("detail.resourceStats")} bordered={false}>
          <Row gutter={[16, 16]}>
            <Col xs={24} sm={12} md={6}>
              <Card>
                <Statistic
                  title="Pod"
                  value={namespaceDetail.resourceCount.pods}
                  prefix={<ContainerOutlined />}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Card>
                <Statistic
                  title={t("detail.services")}
                  value={namespaceDetail.resourceCount.services}
                  prefix={<CloudServerOutlined />}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Card>
                <Statistic
                  title="ConfigMap"
                  value={namespaceDetail.resourceCount.configMaps}
                  prefix={<DatabaseOutlined />}
                  valueStyle={{ color: '#faad14' }}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Card>
                <Statistic
                  title="Secret"
                  value={namespaceDetail.resourceCount.secrets}
                  prefix={<KeyOutlined />}
                  valueStyle={{ color: '#f5222d' }}
                />
              </Card>
            </Col>
          </Row>
        </Card>

        {/* 基本信息 */}
        <Card title={t("detail.basicInfo")}>
          <Descriptions bordered column={2}>
            <Descriptions.Item label={t("detail.name")} span={2}>
              {namespaceDetail.name}
            </Descriptions.Item>
            <Descriptions.Item label={t("detail.status")}>
              <Tag color={namespaceDetail.status === 'Active' ? 'green' : 'orange'}>
                {namespaceDetail.status === 'Active' ? t('common:status.active') : namespaceDetail.status}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t("detail.createdAt")}>
              {namespaceDetail.creationTimestamp}
            </Descriptions.Item>
          </Descriptions>
        </Card>

        {/* 资源配额 */}
        {namespaceDetail.resourceQuota && (
          <Card title={t("detail.resourceQuota")}>
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Card type="inner" title="CPU">
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <div>
                      <Text type="secondary">{t("detail.usedQuota")}: </Text>
                      <Text strong>{namespaceDetail.resourceQuota.used.cpu || '0'}</Text>
                    </div>
                    <div>
                      <Text type="secondary">{t("detail.totalQuota")}: </Text>
                      <Text strong>{namespaceDetail.resourceQuota.hard.cpu || '0'}</Text>
                    </div>
                  </Space>
                </Card>
              </Col>
              <Col span={12}>
                <Card type="inner" title={t("common:resources.memory")}>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <div>
                      <Text type="secondary">{t("detail.usedQuota")}: </Text>
                      <Text strong>{namespaceDetail.resourceQuota.used.memory || '0'}</Text>
                    </div>
                    <div>
                      <Text type="secondary">{t("detail.totalQuota")}: </Text>
                      <Text strong>{namespaceDetail.resourceQuota.hard.memory || '0'}</Text>
                    </div>
                  </Space>
                </Card>
              </Col>
            </Row>
          </Card>
        )}

        {/* 标签 */}
        <Card title={t("detail.labels")}>
          {namespaceDetail.labels && Object.keys(namespaceDetail.labels).length > 0 ? (
            <Space size={[8, 8]} wrap>
              {Object.entries(namespaceDetail.labels).map(([key, value]) => (
                <Tag key={key} icon={<TagsOutlined />} color="blue">
                  {key}: {value}
                </Tag>
              ))}
            </Space>
          ) : (
            <Text type="secondary">{t("detail.noLabels")}</Text>
          )}
        </Card>

        {/* 注解 */}
        <Card title={t("detail.annotations")}>
          {namespaceDetail.annotations && Object.keys(namespaceDetail.annotations).length > 0 ? (
            <Descriptions bordered column={1}>
              {Object.entries(namespaceDetail.annotations).map(([key, value]) => (
                <Descriptions.Item key={key} label={key}>
                  <Text code>{value}</Text>
                </Descriptions.Item>
              ))}
            </Descriptions>
          ) : (
            <Text type="secondary">{t("detail.noAnnotations")}</Text>
          )}
        </Card>
      </Space>
    </div>
  );
};

export default NamespaceDetail;


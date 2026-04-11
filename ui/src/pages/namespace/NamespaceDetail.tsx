import React, { useState, useEffect, useCallback } from 'react';
import EmptyState from '@/components/EmptyState';
import { useParams, useNavigate } from 'react-router-dom';
import {
  App,
  Card,
  Descriptions,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Table,
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
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import {
  ArrowLeftOutlined,
  ReloadOutlined,
  DatabaseOutlined,
  ContainerOutlined,
  CloudServerOutlined,
  KeyOutlined,
  TagsOutlined,
} from '@ant-design/icons';
import {
  getNamespaceDetail,
  listResourceQuotas, createResourceQuota, updateResourceQuota, deleteResourceQuota,
  listLimitRanges, createLimitRange, deleteLimitRange,
  type NamespaceDetailData,
} from '../../services/namespaceService';
import { useTranslation } from 'react-i18next';
const { Title, Text } = Typography;

const NamespaceDetail: React.FC = () => {
  const { clusterId, namespace } = useParams<{ clusterId: string; namespace: string }>();
  const navigate = useNavigate();
  const { message: messageApi } = App.useApp();
  const [namespaceDetail, setNamespaceDetail] = useState<NamespaceDetailData | null>(null);
  const [loading, setLoading] = useState(false);
  const { t } = useTranslation(["namespace", "common"]);

  // Quota state
  const [quotas, setQuotas] = useState<any[]>([]);
  const [quotaModal, setQuotaModal] = useState(false);
  const [editingQuota, setEditingQuota] = useState<any | null>(null);
  const [quotaForm] = Form.useForm();

  // LimitRange state
  const [limitRanges, setLimitRanges] = useState<any[]>([]);
  const [lrModal, setLrModal] = useState(false);
  const [lrForm] = Form.useForm();

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

  const loadQuotas = useCallback(async () => {
    if (!clusterId || !namespace) return;
    try {
      const res = await listResourceQuotas(clusterId, namespace);
      setQuotas(res?.items ?? []);
    } catch { setQuotas([]); }
  }, [clusterId, namespace]);

  const loadLimitRanges = useCallback(async () => {
    if (!clusterId || !namespace) return;
    try {
      const res = await listLimitRanges(clusterId, namespace);
      setLimitRanges(res?.items ?? []);
    } catch { setLimitRanges([]); }
  }, [clusterId, namespace]);

  useEffect(() => { loadQuotas(); loadLimitRanges(); }, [loadQuotas, loadLimitRanges]);

  // ResourceQuota handlers
  const openCreateQuota = () => { setEditingQuota(null); quotaForm.resetFields(); setQuotaModal(true); };
  const openEditQuota = (q: any) => {
    setEditingQuota(q);
    quotaForm.setFieldsValue({
      name: q.name,
      cpu: q.hard?.['limits.cpu'] || q.hard?.cpu || '',
      memory: q.hard?.['limits.memory'] || q.hard?.memory || '',
      pods: q.hard?.pods || '',
    });
    setQuotaModal(true);
  };
  const handleSaveQuota = async () => {
    const vals = await quotaForm.validateFields();
    const hard: Record<string, string> = {};
    if (vals.cpu) hard['limits.cpu'] = vals.cpu;
    if (vals.memory) hard['limits.memory'] = vals.memory;
    if (vals.pods) hard.pods = vals.pods;
    try {
      if (editingQuota) {
        await updateResourceQuota(clusterId!, namespace!, editingQuota.name, { hard });
        messageApi.success('ResourceQuota 更新成功');
      } else {
        await createResourceQuota(clusterId!, namespace!, { name: vals.name, hard });
        messageApi.success('ResourceQuota 建立成功');
      }
      setQuotaModal(false);
      loadQuotas();
    } catch (e) { messageApi.error('操作失敗: ' + String(e)); }
  };
  const handleDeleteQuota = async (name: string) => {
    try {
      await deleteResourceQuota(clusterId!, namespace!, name);
      messageApi.success('已刪除');
      loadQuotas();
    } catch (e) { messageApi.error('刪除失敗: ' + String(e)); }
  };

  // LimitRange handlers
  const openCreateLR = () => { lrForm.resetFields(); setLrModal(true); };
  const handleSaveLR = async () => {
    const vals = await lrForm.validateFields();
    const limits = [{
      type: vals.type,
      max: vals.maxCpu || vals.maxMemory ? { ...(vals.maxCpu ? { cpu: vals.maxCpu } : {}), ...(vals.maxMemory ? { memory: vals.maxMemory } : {}) } : {},
      min: vals.minCpu || vals.minMemory ? { ...(vals.minCpu ? { cpu: vals.minCpu } : {}), ...(vals.minMemory ? { memory: vals.minMemory } : {}) } : {},
      default: vals.defCpu || vals.defMemory ? { ...(vals.defCpu ? { cpu: vals.defCpu } : {}), ...(vals.defMemory ? { memory: vals.defMemory } : {}) } : {},
      defaultRequest: vals.reqCpu || vals.reqMemory ? { ...(vals.reqCpu ? { cpu: vals.reqCpu } : {}), ...(vals.reqMemory ? { memory: vals.reqMemory } : {}) } : {},
    }];
    try {
      await createLimitRange(clusterId!, namespace!, { name: vals.name, limits });
      messageApi.success('LimitRange 建立成功');
      setLrModal(false);
      loadLimitRanges();
    } catch (e) { messageApi.error('建立失敗: ' + String(e)); }
  };
  const handleDeleteLR = async (name: string) => {
    try {
      await deleteLimitRange(clusterId!, namespace!, name);
      messageApi.success('已刪除');
      loadLimitRanges();
    } catch (e) { messageApi.error('刪除失敗: ' + String(e)); }
  };

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
        <EmptyState description={t("detail.notFound")} />
      </div>
    );
  }

  return (
    <div style={{ padding: 24 }}>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {/* 頭部操作欄 */}
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

        {/* 資源統計卡片 */}
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

        {/* 基本資訊 */}
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

        {/* 資源配額 */}
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

        {/* ResourceQuota 管理 */}
        <Card
          title="ResourceQuota 配額管理"
          extra={<Button size="small" type="primary" icon={<PlusOutlined />} onClick={openCreateQuota}>建立</Button>}
        >
          <Table
            scroll={{ x: 'max-content' }}
            size="small"
            dataSource={quotas}
            rowKey="name"
            pagination={false}
            columns={[
              { title: '名稱', dataIndex: 'name', key: 'name' },
              { title: 'CPU (Hard)', dataIndex: 'hard', key: 'cpu', render: (h: any) => h?.['limits.cpu'] || h?.cpu || '-' },
              { title: 'Memory (Hard)', dataIndex: 'hard', key: 'mem', render: (h: any) => h?.['limits.memory'] || h?.memory || '-' },
              { title: 'Pods', dataIndex: 'hard', key: 'pods', render: (h: any) => h?.pods || '-' },
              {
                title: '操作', key: 'action', width: 120,
                render: (_: any, record: any) => (
                  <Space>
                    <Button size="small" icon={<EditOutlined />} onClick={() => openEditQuota(record)} />
                    <Popconfirm title="確定刪除？" onConfirm={() => handleDeleteQuota(record.name)} okText="刪除" okButtonProps={{ danger: true }} cancelText="取消">
                      <Button size="small" danger icon={<DeleteOutlined />} />
                    </Popconfirm>
                  </Space>
                ),
              },
            ]}
          />
        </Card>

        {/* LimitRange 管理 */}
        <Card
          title="LimitRange 資源限制"
          extra={<Button size="small" type="primary" icon={<PlusOutlined />} onClick={openCreateLR}>建立</Button>}
        >
          <Table
            scroll={{ x: 'max-content' }}
            size="small"
            dataSource={limitRanges}
            rowKey="name"
            pagination={false}
            columns={[
              { title: '名稱', dataIndex: 'name', key: 'name' },
              { title: '型別', dataIndex: 'limits', key: 'type', render: (l: any[]) => l?.map(i => i.type).join(', ') || '-' },
              { title: 'CPU Max', dataIndex: 'limits', key: 'cpumax', render: (l: any[]) => l?.[0]?.max?.cpu || '-' },
              { title: 'Memory Max', dataIndex: 'limits', key: 'memmax', render: (l: any[]) => l?.[0]?.max?.memory || '-' },
              {
                title: '操作', key: 'action', width: 80,
                render: (_: any, record: any) => (
                  <Popconfirm title="確定刪除？" onConfirm={() => handleDeleteLR(record.name)} okText="刪除" okButtonProps={{ danger: true }} cancelText="取消">
                    <Button size="small" danger icon={<DeleteOutlined />} />
                  </Popconfirm>
                ),
              },
            ]}
          />
        </Card>

        {/* ResourceQuota Modal */}
        <Modal open={quotaModal} title={editingQuota ? '編輯 ResourceQuota' : '建立 ResourceQuota'} onCancel={() => setQuotaModal(false)} onOk={handleSaveQuota} okText="儲存" cancelText="取消" destroyOnClose>
          <Form form={quotaForm} layout="vertical" style={{ marginTop: 12 }}>
            {!editingQuota && <Form.Item name="name" label="名稱" rules={[{ required: true }]}><Input /></Form.Item>}
            <Form.Item name="cpu" label="CPU Limit（例如：4）"><Input placeholder="4 / 4000m" /></Form.Item>
            <Form.Item name="memory" label="Memory Limit（例如：8Gi）"><Input placeholder="8Gi / 8192Mi" /></Form.Item>
            <Form.Item name="pods" label="最大 Pod 數"><Input placeholder="100" /></Form.Item>
          </Form>
        </Modal>

        {/* LimitRange Modal */}
        <Modal open={lrModal} title="建立 LimitRange" onCancel={() => setLrModal(false)} onOk={handleSaveLR} okText="建立" cancelText="取消" destroyOnClose>
          <Form form={lrForm} layout="vertical" style={{ marginTop: 12 }}>
            <Form.Item name="name" label="名稱" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="type" label="型別" initialValue="Container" rules={[{ required: true }]}>
              <Select options={[{ value: 'Container' }, { value: 'Pod' }, { value: 'PersistentVolumeClaim' }]} />
            </Form.Item>
            <Row gutter={12}>
              <Col span={12}><Form.Item name="maxCpu" label="CPU Max"><Input placeholder="2" /></Form.Item></Col>
              <Col span={12}><Form.Item name="maxMemory" label="Memory Max"><Input placeholder="2Gi" /></Form.Item></Col>
              <Col span={12}><Form.Item name="minCpu" label="CPU Min"><Input placeholder="100m" /></Form.Item></Col>
              <Col span={12}><Form.Item name="minMemory" label="Memory Min"><Input placeholder="128Mi" /></Form.Item></Col>
              <Col span={12}><Form.Item name="defCpu" label="CPU Default Limit"><Input placeholder="500m" /></Form.Item></Col>
              <Col span={12}><Form.Item name="defMemory" label="Memory Default Limit"><Input placeholder="512Mi" /></Form.Item></Col>
              <Col span={12}><Form.Item name="reqCpu" label="CPU Default Request"><Input placeholder="250m" /></Form.Item></Col>
              <Col span={12}><Form.Item name="reqMemory" label="Memory Default Request"><Input placeholder="256Mi" /></Form.Item></Col>
            </Row>
          </Form>
        </Modal>

        {/* 標籤 */}
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

        {/* 註解 */}
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


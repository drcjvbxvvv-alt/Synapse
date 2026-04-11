import React from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Input,
  Select,
  Tooltip,
  Popconfirm,
  Typography,
  Row,
  Col,
  Divider,
} from 'antd';
import {
  PlusOutlined,
  SearchOutlined,
  QuestionCircleOutlined,
  ReloadOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import {
  getPermissionTypeName,
  getPermissionTypeColor,
  formatNamespaces,
} from '../../services/permissionService';
import CustomRoleEditor from '../../components/CustomRoleEditor';
import { useTranslation } from 'react-i18next';
import { usePermissionManagement } from './hooks/usePermissionManagement';
import { createPermissionColumns } from './columns';
import { PermissionModal } from './components/PermissionModal';
import { SyncModal } from './components/SyncModal';

const { Title, Paragraph } = Typography;
const { Option } = Select;

const PermissionManagement: React.FC = () => {
  const { t } = useTranslation(['permission', 'common']);

  const {
    loading,
    permissionTypes,
    clusters,
    users,
    userGroups,
    selectedRowKeys,
    filterCluster,
    filterNamespace,
    searchKeyword,
    modalVisible,
    editingPermission,
    form,
    selectedPermissionType,
    assignType,
    allNamespaces,
    namespaceOptions,
    namespacesLoading,
    syncModalVisible,
    syncLoading,
    syncStatus,
    selectedClusterForSync,
    customRoleEditorVisible,
    customRoleClusterId,
    customRoleClusterName,
    filteredPermissions,
    defaultPermissionTypes,
    setSelectedRowKeys,
    setFilterCluster,
    setFilterNamespace,
    setSearchKeyword,
    setModalVisible,
    setAllNamespaces,
    setAssignType,
    setSyncModalVisible,
    loadData,
    handleAdd,
    handleEdit,
    handleDelete,
    handleBatchDelete,
    handleSubmit,
    handleOpenSyncModal,
    handleSyncPermissions,
    handleClusterChangeInForm,
    handlePermissionTypeSelect,
    handleOpenCustomRoleEditor,
    handleCustomRoleSuccess,
    setCustomRoleEditorVisible,
  } = usePermissionManagement();

  const columns = createPermissionColumns({
    t,
    clusters,
    formatNamespaces,
    getPermissionTypeName,
    getPermissionTypeColor,
    onEdit: handleEdit,
    onDelete: (record) => handleDelete(record.id),
  });

  const displayTypes = permissionTypes.length > 0 ? permissionTypes : defaultPermissionTypes;

  return (
    <div style={{ padding: '0' }}>
      {/* Page header */}
      <div style={{ marginBottom: 24, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Space>
          <Title level={4} style={{ margin: 0 }}>{t('permission:title')}</Title>
          <Tooltip title={t('permission:tooltip')}>
            <QuestionCircleOutlined style={{ color: '#999' }} />
          </Tooltip>
        </Space>
        <Space>
          <Button icon={<SyncOutlined />} onClick={handleOpenSyncModal}>
            {t('permission:syncPermissions')}
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            {t('permission:addPermission')}
          </Button>
        </Space>
      </div>

      {/* Permission type description cards */}
      <Card style={{ marginBottom: 24 }} styles={{ body: { padding: '20px 24px' } }}>
        <Row gutter={24}>
          {displayTypes.map((type, index, arr) => (
            <Col
              flex="1"
              key={type.type}
              style={{
                borderRight: index < arr.length - 1 ? '1px solid #f0f0f0' : 'none',
                paddingRight: index < arr.length - 1 ? 16 : 0,
                paddingLeft: index > 0 ? 16 : 0,
              }}
            >
              <Title level={5} style={{ marginBottom: 8, fontSize: 14, fontWeight: 600, color: '#1f2937' }}>
                {type.name}
              </Title>
              <Paragraph
                type="secondary"
                style={{ marginBottom: 0, fontSize: 12, lineHeight: 1.8, color: '#6b7280' }}
              >
                {type.description}
              </Paragraph>
            </Col>
          ))}
        </Row>
      </Card>

      {/* Filter toolbar */}
      <Card style={{ marginBottom: 16 }} styles={{ body: { padding: '12px 16px' } }}>
        <Space size="large" wrap>
          <Space>
            <Popconfirm
              title={t('permission:actions.confirmBatchDelete')}
              onConfirm={handleBatchDelete}
              disabled={selectedRowKeys.length === 0}
            >
              <Button disabled={selectedRowKeys.length === 0}>
                {t('common:actions.batchDelete')}
              </Button>
            </Popconfirm>
          </Space>
          <Divider type="vertical" style={{ height: 24 }} />
          <Space>
            <Select
              placeholder={t('permission:filter.selectCluster')}
              allowClear
              style={{ width: 200 }}
              value={filterCluster || undefined}
              onChange={(v) => setFilterCluster(v || '')}
            >
              {clusters.map((c) => (
                <Option key={c.id} value={c.id.toString()}>
                  {c.name}
                </Option>
              ))}
            </Select>
            <Select
              placeholder={t('permission:filter.namespace')}
              allowClear
              style={{ width: 120 }}
              value={filterNamespace || undefined}
              onChange={(v) => setFilterNamespace(v || '')}
            >
              <Option value="*">{t('permission:filter.all')}</Option>
            </Select>
            <Input.Search
              placeholder={t('permission:filter.keyword')}
              allowClear
              style={{ width: 200 }}
              value={searchKeyword}
              onChange={(e) => setSearchKeyword(e.target.value)}
              enterButton={<SearchOutlined />}
            />
            <Button icon={<ReloadOutlined />} onClick={loadData}>
              {t('common:actions.refresh')}
            </Button>
          </Space>
        </Space>
      </Card>

      {/* Permissions table */}
      <Card styles={{ body: { padding: 0 } }}>
        <Table
          scroll={{ x: 'max-content' }}
          rowKey="id"
          columns={columns}
          dataSource={filteredPermissions}
          loading={loading}
          rowSelection={{
            selectedRowKeys,
            onChange: setSelectedRowKeys,
          }}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => t('permission:pagination.total', { total }),
          }}
        />
      </Card>

      {/* Add/Edit permission modal */}
      <PermissionModal
        visible={modalVisible}
        loading={loading}
        editingPermission={editingPermission}
        form={form}
        permissionTypes={permissionTypes}
        clusters={clusters}
        users={users}
        userGroups={userGroups}
        allNamespaces={allNamespaces}
        namespaceOptions={namespaceOptions}
        namespacesLoading={namespacesLoading}
        selectedPermissionType={selectedPermissionType}
        assignType={assignType}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        onAllNamespacesChange={setAllNamespaces}
        onAssignTypeChange={(type) => {
          setAssignType(type);
          form.setFieldsValue({ user_ids: undefined, user_group_ids: undefined });
        }}
        onPermissionTypeSelect={handlePermissionTypeSelect}
        onClusterChange={handleClusterChangeInForm}
        onOpenCustomRoleEditor={handleOpenCustomRoleEditor}
      />

      {/* Sync permissions modal */}
      <SyncModal
        visible={syncModalVisible}
        clusters={clusters}
        syncStatus={syncStatus}
        syncLoading={syncLoading}
        selectedClusterForSync={selectedClusterForSync}
        onCancel={() => setSyncModalVisible(false)}
        onSync={handleSyncPermissions}
      />

      {/* Custom role editor */}
      <CustomRoleEditor
        visible={customRoleEditorVisible}
        clusterId={customRoleClusterId}
        clusterName={customRoleClusterName}
        onCancel={() => setCustomRoleEditorVisible(false)}
        onSuccess={handleCustomRoleSuccess}
      />
    </div>
  );
};

export default PermissionManagement;

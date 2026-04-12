import React from 'react';
import {
  Modal,
  Form,
  Select,
  Input,
  Space,
  Button,
  Row,
  Col,
  Checkbox,
  Spin,
  Badge,
  Typography,
} from 'antd';
import { UserOutlined, TeamOutlined } from '@ant-design/icons';
import type { FormInstance } from 'antd';
import { useTranslation } from 'react-i18next';
import type {
  ClusterPermission,
  PermissionTypeInfo,
  User,
  UserGroup,
  Cluster,
  PermissionType,
} from '../../../types';
import {
  requiresAllNamespaces,
  allowsPartialNamespaces,
} from '../../../services/permissionService';
import { PermissionTypeCard } from './PermissionTypeCard';

const { Option } = Select;
const { Text } = Typography;

interface PermissionModalProps {
  visible: boolean;
  loading: boolean;
  editingPermission: ClusterPermission | null;
  form: FormInstance;
  permissionTypes: PermissionTypeInfo[];
  clusters: Cluster[];
  users: User[];
  userGroups: UserGroup[];
  allNamespaces: boolean;
  namespaceOptions: string[];
  namespacesLoading: boolean;
  selectedPermissionType: PermissionType;
  assignType: 'user' | 'group';
  onOk: () => void;
  onCancel: () => void;
  onAllNamespacesChange: (checked: boolean) => void;
  onAssignTypeChange: (type: 'user' | 'group') => void;
  onPermissionTypeSelect: (type: PermissionType) => void;
  onClusterChange: (value: number) => void;
  onOpenCustomRoleEditor: () => void;
}

export const PermissionModal: React.FC<PermissionModalProps> = ({
  visible,
  loading,
  editingPermission,
  form,
  permissionTypes,
  clusters,
  users,
  userGroups,
  allNamespaces,
  namespaceOptions,
  namespacesLoading,
  selectedPermissionType,
  assignType,
  onOk,
  onCancel,
  onAllNamespacesChange,
  onAssignTypeChange,
  onPermissionTypeSelect,
  onClusterChange,
  onOpenCustomRoleEditor,
}) => {
  const { t } = useTranslation(['permission', 'common']);

  return (
    <Modal
      title={editingPermission ? t('permission:editPermission') : t('permission:addPermission')}
      open={visible}
      onOk={onOk}
      onCancel={onCancel}
      width={720}
      destroyOnHidden
    >
      <Spin spinning={loading}>
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            permission_type: 'admin',
          }}
        >
          {/* Cluster selection */}
          <Form.Item
            name="cluster_id"
            label={t('permission:form.selectCluster')}
            rules={[{ required: true, message: t('permission:form.selectClusterRequired') }]}
          >
            <Select
              placeholder={t('permission:form.selectClusterPlaceholder')}
              disabled={!!editingPermission}
              showSearch
              optionFilterProp="children"
              onChange={onClusterChange}
            >
              {clusters.map((c) => (
                <Option key={c.id} value={parseInt(c.id)}>
                  <Space>
                    {c.name}
                    {c.status === 'healthy' ? (
                      <Badge status="success" />
                    ) : (
                      <Badge status="error" />
                    )}
                  </Space>
                </Option>
              ))}
            </Select>
          </Form.Item>

          {/* Permission type selection */}
          <Form.Item label={t('permission:form.permissionType')} required>
            <Row gutter={[12, 12]}>
              {permissionTypes.map((type) => (
                <Col xs={12} sm={8} md={Math.ceil(24 / Math.min(permissionTypes.length, 4))} key={type.type}>
                  <PermissionTypeCard
                    type={type}
                    selected={selectedPermissionType === type.type}
                    onClick={() => onPermissionTypeSelect(type.type as PermissionType)}
                  />
                </Col>
              ))}
            </Row>
          </Form.Item>

          {/* Custom role ref when permission type is custom */}
          {selectedPermissionType === 'custom' && (
            <Form.Item
              name="custom_role_ref"
              label={t('permission:form.customRoleName')}
              rules={[{ required: true, message: t('permission:form.customRoleRequired') }]}
              extra={
                <Space style={{ marginTop: 8 }}>
                  <Text type="secondary">{t('permission:form.noSuitableRole')}</Text>
                  <Button
                    type="link"
                    size="small"
                    style={{ padding: 0 }}
                    onClick={onOpenCustomRoleEditor}
                  >
                    {t('permission:form.createClusterRole')}
                  </Button>
                </Space>
              }
            >
              <Input placeholder={t('permission:form.customRolePlaceholder')} />
            </Form.Item>
          )}

          {/* Assign target (only when creating) */}
          {!editingPermission && (
            <>
              <Form.Item label={t('permission:form.assignTo')}>
                <Space>
                  <Button
                    type={assignType === 'user' ? 'primary' : 'default'}
                    icon={<UserOutlined />}
                    onClick={() => onAssignTypeChange('user')}
                  >
                    {t('permission:columns.user')}
                  </Button>
                  <Button
                    type={assignType === 'group' ? 'primary' : 'default'}
                    icon={<TeamOutlined />}
                    onClick={() => onAssignTypeChange('group')}
                  >
                    {t('permission:columns.userGroup')}
                  </Button>
                </Space>
              </Form.Item>

              {assignType === 'user' ? (
                <Form.Item
                  name="user_ids"
                  label={t('permission:form.selectUser')}
                  rules={[{ required: true, message: t('permission:form.selectUserRequired') }]}
                >
                  <Select
                    mode="multiple"
                    placeholder={t('permission:form.selectUserPlaceholder')}
                    showSearch
                    optionFilterProp="label"
                    options={users.map((u) => ({
                      label: `${u.display_name || u.username} (${u.username})`,
                      value: u.id,
                    }))}
                  />
                </Form.Item>
              ) : (
                <Form.Item
                  name="user_group_ids"
                  label={t('permission:form.selectUserGroup')}
                  rules={[{ required: true, message: t('permission:form.selectUserGroupRequired') }]}
                >
                  <Select
                    mode="multiple"
                    placeholder={t('permission:form.selectUserGroupPlaceholder')}
                    showSearch
                    optionFilterProp="label"
                    options={userGroups.map((g) => ({
                      label: `${g.name} (${t('permission:form.memberCount', { count: g.users?.length || 0 })})`,
                      value: g.id,
                    }))}
                  />
                </Form.Item>
              )}
            </>
          )}

          {/* Namespace scope */}
          <Form.Item
            label={t('permission:form.namespaceScope')}
            extra={requiresAllNamespaces(selectedPermissionType)
              ? <Text type="warning">{t('permission:form.allNamespacesRequired')}</Text>
              : null}
          >
            <Checkbox
              checked={allNamespaces}
              onChange={(e) => onAllNamespacesChange(e.target.checked)}
              disabled={requiresAllNamespaces(selectedPermissionType)}
            >
              {t('permission:form.allNamespaces')}
            </Checkbox>
          </Form.Item>

          {!allNamespaces && allowsPartialNamespaces(selectedPermissionType) && (
            <Form.Item
              name="namespaces"
              label={t('permission:form.selectNamespace')}
              rules={[{ required: true, message: t('permission:form.selectNamespaceRequired') }]}
            >
              <Select
                mode="tags"
                placeholder={t('permission:form.namespacePlaceholder')}
                tokenSeparators={[',']}
                loading={namespacesLoading}
                showSearch
                optionFilterProp="children"
              >
                {namespaceOptions.map((ns) => (
                  <Option key={ns} value={ns}>{ns}</Option>
                ))}
              </Select>
            </Form.Item>
          )}
        </Form>
      </Spin>
    </Modal>
  );
};

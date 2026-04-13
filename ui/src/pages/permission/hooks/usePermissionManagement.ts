import { useState, useEffect, useCallback } from 'react';
import { App, Form } from 'antd';
import type {
  ClusterPermission,
  PermissionTypeInfo,
  User,
  UserGroup,
  Cluster,
  PermissionType,
} from '../../../types';
import permissionService, {
  requiresAllNamespaces,
} from '../../../services/permissionService';
import { clusterService } from '../../../services/clusterService';
import { getNamespaces as fetchClusterNamespaces } from '../../../services/namespaceService';
import rbacService from '../../../services/rbacService';
import type { SyncStatusResult } from '../../../services/rbacService';
import { useTranslation } from 'react-i18next';
import { parseApiError } from '../../../utils/api';

export const defaultPermissionTypeKeys = ['admin', 'ops', 'dev', 'readonly', 'custom'] as const;

export function usePermissionManagement() {
  const { t } = useTranslation(['permission', 'common']);
  const { message } = App.useApp();
  const [form] = Form.useForm();

  const defaultPermissionTypes: PermissionTypeInfo[] = defaultPermissionTypeKeys.map(type => ({
    type,
    name: t(`permission:types.${type}.name`),
    description: t(`permission:types.${type}.description`),
    resources: type === 'admin' ? ['*'] : type === 'readonly' ? ['*'] : type === 'custom' ? [] : ['pods', 'deployments', 'services'],
    actions: type === 'admin' ? ['*'] : type === 'readonly' ? ['get', 'list', 'watch'] : type === 'custom' ? [] : ['get', 'list', 'watch', 'create', 'update', 'delete'],
    allowPartialNamespaces: type !== 'admin' && type !== 'ops',
    requireAllNamespaces: type === 'admin' || type === 'ops',
  }));

  // Core data state
  const [loading, setLoading] = useState(false);
  const [permissions, setPermissions] = useState<ClusterPermission[]>([]);
  const [permissionTypes, setPermissionTypes] = useState<PermissionTypeInfo[]>([]);
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [userGroups, setUserGroups] = useState<UserGroup[]>([]);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);

  // Filter state
  const [filterCluster, setFilterCluster] = useState<string>('');
  const [filterNamespace, setFilterNamespace] = useState<string>('');
  const [searchKeyword, setSearchKeyword] = useState('');

  // Modal state
  const [modalVisible, setModalVisible] = useState(false);
  const [editingPermission, setEditingPermission] = useState<ClusterPermission | null>(null);

  // Form state
  const [selectedPermissionType, setSelectedPermissionType] = useState<PermissionType>('admin');
  const [assignType, setAssignType] = useState<'user' | 'group'>('user');
  const [allNamespaces, setAllNamespaces] = useState(true);
  const [namespaceOptions, setNamespaceOptions] = useState<string[]>([]);
  const [namespacesLoading, setNamespacesLoading] = useState(false);

  // Sync modal state
  const [syncModalVisible, setSyncModalVisible] = useState(false);
  const [syncLoading, setSyncLoading] = useState(false);
  const [syncStatus, setSyncStatus] = useState<Record<string, SyncStatusResult>>({});
  const [selectedClusterForSync, setSelectedClusterForSync] = useState<string | null>(null);

  // Custom role editor state
  const [customRoleEditorVisible, setCustomRoleEditorVisible] = useState(false);
  const [customRoleClusterId, setCustomRoleClusterId] = useState<string>('0');
  const [customRoleClusterName, setCustomRoleClusterName] = useState<string>('');

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [permissionsRes, typesRes, clustersRes, usersRes, groupsRes] = await Promise.all([
        permissionService.getAllClusterPermissions(),
        permissionService.getPermissionTypes(),
        clusterService.getClusters(),
        permissionService.getUsers(),
        permissionService.getUserGroups(),
      ]);

      setPermissions(permissionsRes || []);
      setPermissionTypes((typesRes || []).map(pt => ({
        ...pt,
        name: t(`permission:types.${pt.type}.name`),
        description: t(`permission:types.${pt.type}.description`),
      })));
      setClusters(clustersRes?.items || []);
      setUsers(usersRes || []);
      setUserGroups(groupsRes || []);
    } catch (error) {
      console.error('Failed to load data:', error);
      message.error(t('permission:loadError'));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Computed filtered permissions
  const filteredPermissions = permissions.filter((p) => {
    if (filterCluster && p.cluster_id.toString() !== filterCluster) return false;
    if (searchKeyword) {
      const keyword = searchKeyword.toLowerCase();
      const username = p.username?.toLowerCase() || '';
      const groupName = p.user_group_name?.toLowerCase() || '';
      if (!username.includes(keyword) && !groupName.includes(keyword)) return false;
    }
    return true;
  });

  const handleAdd = () => {
    setEditingPermission(null);
    setSelectedPermissionType('admin');
    setAssignType('user');
    setAllNamespaces(true);
    form.resetFields();
    setModalVisible(true);
  };

  const handleOpenSyncModal = () => {
    setSyncModalVisible(true);
    loadAllSyncStatus();
  };

  const loadAllSyncStatus = async () => {
    const statusMap: Record<string, SyncStatusResult> = {};
    for (const cluster of clusters) {
      try {
        const res = await rbacService.getSyncStatus(Number(cluster.id));
        if (res) {
          statusMap[cluster.id] = res;
        }
      } catch (err) {
        console.error(`獲取叢集 ${cluster.name} 同步狀態失敗:`, err);
      }
    }
    setSyncStatus(statusMap);
  };

  const handleSyncPermissions = async (clusterId: string) => {
    setSelectedClusterForSync(clusterId);
    setSyncLoading(true);
    try {
      const res = await rbacService.syncPermissions(Number(clusterId));
      message.success(res?.message || t('permission:sync.syncSuccess'));
      const statusRes = await rbacService.getSyncStatus(Number(clusterId));
      if (statusRes) {
        setSyncStatus(prev => ({ ...prev, [clusterId]: statusRes }));
      }
    } catch {
      message.error(t('permission:sync.syncFailed'));
    } finally {
      setSyncLoading(false);
      setSelectedClusterForSync(null);
    }
  };

  const handleEdit = (record: ClusterPermission) => {
    setEditingPermission(record);
    setSelectedPermissionType(record.permission_type);
    setAllNamespaces(record.namespaces.includes('*'));
    form.setFieldsValue({
      cluster_id: record.cluster_id,
      permission_type: record.permission_type,
      namespaces: record.namespaces.filter((n) => n !== '*'),
      custom_role_ref: record.custom_role_ref,
    });
    setModalVisible(true);
    if (record.cluster_id) {
      setNamespacesLoading(true);
      fetchClusterNamespaces(record.cluster_id)
        .then((nsList) => setNamespaceOptions(nsList.map((ns) => ns.name)))
        .catch(() => setNamespaceOptions([]))
        .finally(() => setNamespacesLoading(false));
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await permissionService.deleteClusterPermission(id);
      message.success(t('common:messages.deleteSuccess'));
      loadData();
    } catch {
      message.error(t('common:messages.deleteError'));
    }
  };

  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('permission:actions.selectDeleteFirst'));
      return;
    }
    try {
      await permissionService.batchDeleteClusterPermissions(selectedRowKeys as number[]);
      message.success(t('permission:actions.batchDeleteSuccess'));
      setSelectedRowKeys([]);
      loadData();
    } catch {
      message.error(t('permission:actions.batchDeleteError'));
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();

      if (editingPermission) {
        await permissionService.updateClusterPermission(editingPermission.id, {
          permission_type: selectedPermissionType,
          namespaces: allNamespaces ? ['*'] : (values.namespaces || []),
          custom_role_ref: selectedPermissionType === 'custom' ? values.custom_role_ref : undefined,
        });
        message.success(t('common:messages.saveSuccess'));
      } else {
        await permissionService.createClusterPermission({
          cluster_id: values.cluster_id,
          permission_type: selectedPermissionType,
          namespaces: allNamespaces ? ['*'] : (values.namespaces || []),
          custom_role_ref: selectedPermissionType === 'custom' ? values.custom_role_ref : undefined,
          user_ids: assignType === 'user' ? values.user_ids : undefined,
          user_group_ids: assignType === 'group' ? values.user_group_ids : undefined,
        });
        message.success(t('permission:actions.addSuccess'));
      }

      setModalVisible(false);
      loadData();
    } catch (error: unknown) {
      const msg = parseApiError(error);
      if (msg) message.error(msg);
    }
  };

  const handleClusterChangeInForm = (value: number) => {
    setNamespaceOptions([]);
    form.setFieldsValue({ namespaces: [] });
    if (value) {
      setNamespacesLoading(true);
      fetchClusterNamespaces(value)
        .then((nsList) => setNamespaceOptions(nsList.map((ns) => ns.name)))
        .catch(() => setNamespaceOptions([]))
        .finally(() => setNamespacesLoading(false));
    }
  };

  const handlePermissionTypeSelect = (type: PermissionType) => {
    setSelectedPermissionType(type);
    if (requiresAllNamespaces(type)) {
      setAllNamespaces(true);
    }
  };

  const handleOpenCustomRoleEditor = () => {
    const clusterId = form.getFieldValue('cluster_id');
    if (!clusterId) {
      message.warning(t('permission:form.selectClusterFirst'));
      return;
    }
    const cluster = clusters.find(c => c.id === clusterId);
    setCustomRoleClusterId(clusterId);
    setCustomRoleClusterName(cluster?.name || '');
    setCustomRoleEditorVisible(true);
  };

  const handleCustomRoleSuccess = (roleName: string) => {
    form.setFieldValue('custom_role_ref', roleName);
    setCustomRoleEditorVisible(false);
  };

  return {
    // State
    loading,
    permissions,
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
    // Setters
    setSelectedRowKeys,
    setFilterCluster,
    setFilterNamespace,
    setSearchKeyword,
    setModalVisible,
    setAllNamespaces,
    setAssignType,
    setSyncModalVisible,
    setCustomRoleEditorVisible,
    // Handlers
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
  };
}

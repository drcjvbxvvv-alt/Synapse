import React, { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Modal,
  Form,
  Input,
  Drawer,
  Select,
  App,
} from 'antd';
import { PlusOutlined, UserAddOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { usePermission } from '../../hooks/usePermission';
import type { UserGroup, User, CreateUserGroupRequest, UpdateUserGroupRequest } from '../../types';
import permissionService from '../../services/permissionService';
import dayjs from 'dayjs';

const UserGroupManagement: React.FC = () => {
  const { t } = useTranslation('permission');
  const { message, modal } = App.useApp();
  const { canDelete } = usePermission();
  const [form] = Form.useForm();

  const [loading, setLoading] = useState(false);
  const [groups, setGroups] = useState<UserGroup[]>([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingGroup, setEditingGroup] = useState<UserGroup | null>(null);

  const [drawerVisible, setDrawerVisible] = useState(false);
  const [drawerGroup, setDrawerGroup] = useState<UserGroup | null>(null);
  const [drawerLoading, setDrawerLoading] = useState(false);
  const [users, setUsers] = useState<User[]>([]);
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [addMemberLoading, setAddMemberLoading] = useState(false);

  const loadGroups = useCallback(async () => {
    setLoading(true);
    try {
      const res = await permissionService.getUserGroups();
      setGroups(res || []);
    } catch {
      message.error(t('group.messages.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    loadGroups();
  }, [loadGroups]);

  const handleCreate = () => {
    setEditingGroup(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: UserGroup) => {
    setEditingGroup(record);
    form.setFieldsValue({
      name: record.name,
      description: record.description || '',
    });
    setModalVisible(true);
  };

  const handleDelete = (record: UserGroup) => {
    modal.confirm({
      title: t('group.confirm.delete'),
      content: t('group.confirm.deleteContent', { name: record.name }),
      okText: t('group.confirm.ok'),
      cancelText: t('group.confirm.cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await permissionService.deleteUserGroup(record.id);
          message.success(t('group.messages.deleteSuccess'));
          loadGroups();
        } catch {
          message.error(t('group.messages.deleteFailed'));
        }
      },
    });
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      if (editingGroup) {
        const data: UpdateUserGroupRequest = {
          name: values.name,
          description: values.description || undefined,
        };
        await permissionService.updateUserGroup(editingGroup.id, data);
        message.success(t('group.messages.updateSuccess'));
      } else {
        const data: CreateUserGroupRequest = {
          name: values.name,
          description: values.description || undefined,
        };
        await permissionService.createUserGroup(data);
        message.success(t('group.messages.createSuccess'));
      }
      setModalVisible(false);
      loadGroups();
    } catch (err) {
      if ((err as { errorFields?: unknown[] }).errorFields) {
        return;
      }
      message.error(editingGroup ? t('group.messages.updateFailed') : t('group.messages.createFailed'));
    }
  };

  const openMemberDrawer = useCallback(
    async (group: UserGroup) => {
      setDrawerGroup(group);
      setDrawerVisible(true);
      setDrawerLoading(true);
      try {
        const [groupRes, usersRes] = await Promise.all([
          permissionService.getUserGroup(group.id),
          permissionService.getUsers(),
        ]);
        setDrawerGroup(groupRes || group);
        setUsers(usersRes || []);
      } catch {
        message.error(t('group.messages.loadDataFailed'));
      } finally {
        setDrawerLoading(false);
      }
    },
    [message, t]
  );

  const closeMemberDrawer = () => {
    setDrawerVisible(false);
    setDrawerGroup(null);
    setSelectedUserId(null);
    loadGroups();
  };

  const handleAddMember = async () => {
    if (!drawerGroup || !selectedUserId) {
      message.warning(t('group.messages.selectUser'));
      return;
    }
    const memberIds = (drawerGroup.users || []).map((u) => u.id);
    if (memberIds.includes(selectedUserId)) {
      message.warning(t('group.messages.userAlreadyInGroup'));
      return;
    }
    setAddMemberLoading(true);
    try {
      await permissionService.addUserToGroup(drawerGroup.id, selectedUserId);
      message.success(t('group.messages.addSuccess'));
      setSelectedUserId(null);
      const groupRes = await permissionService.getUserGroup(drawerGroup.id);
      setDrawerGroup(groupRes || drawerGroup);
    } catch {
      message.error(t('group.messages.addFailed'));
    } finally {
      setAddMemberLoading(false);
    }
  };

  const handleRemoveMember = (user: User) => {
    if (!drawerGroup) return;
    modal.confirm({
      title: t('group.confirm.remove'),
      content: t('group.confirm.removeContent', { name: user.display_name || user.username }),
      okText: t('group.confirm.ok'),
      cancelText: t('group.confirm.cancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await permissionService.removeUserFromGroup(drawerGroup.id, user.id);
          message.success(t('group.messages.removeSuccess'));
          const groupRes = await permissionService.getUserGroup(drawerGroup.id);
          setDrawerGroup(groupRes || drawerGroup);
        } catch {
          message.error(t('group.messages.removeFailed'));
        }
      },
    });
  };

  const memberUserIds = (drawerGroup?.users || []).map((u) => u.id);
  const availableUsers = users.filter((u) => !memberUserIds.includes(u.id));

  const columns: ColumnsType<UserGroup> = [
    {
      title: t('group.columns.name'),
      dataIndex: 'name',
      key: 'name',
      width: 180,
    },
    {
      title: t('group.columns.description'),
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: t('group.columns.memberCount'),
      key: 'memberCount',
      width: 100,
      render: (_, record) => record.users?.length ?? 0,
    },
    {
      title: t('group.columns.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (val: string) => (val ? dayjs(val).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: t('group.columns.actions'),
      key: 'action',
      width: 220,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<UserAddOutlined />}
            onClick={() => openMemberDrawer(record)}
          >
            {t('group.actions.manage')}
          </Button>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            {t('group.actions.edit')}
          </Button>
          {canDelete() && (
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              onClick={() => handleDelete(record)}
            >
              {t('group.actions.delete')}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 0 }}>
      <div style={{ marginBottom: 24, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ margin: 0, fontSize: 20 }}>{t('group.title')}</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          {t('group.create')}
        </Button>
      </div>

      <Card styles={{ body: { padding: 0 } }}>
        <Table
          rowKey="id"
          columns={columns}
          dataSource={groups}
          loading={loading}
          scroll={{ x: 'max-content' }}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (tot) => t('group.pagination.total', { total: tot }),
          }}
        />
      </Card>

      <Modal
        title={editingGroup ? t('group.editTitle') : t('group.createTitle')}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label={t('group.form.name')}
            rules={[{ required: true, message: t('group.form.nameRequired') }]}
          >
            <Input placeholder={t('group.form.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('group.form.description')}>
            <Input.TextArea rows={3} placeholder={t('group.form.descriptionPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={t('group.drawer.title', { name: drawerGroup?.name ?? '' })}
        placement="right"
        width={520}
        open={drawerVisible}
        onClose={closeMemberDrawer}
        destroyOnHidden
      >
        <div style={{ marginBottom: 16 }}>
          <Space.Compact style={{ width: '100%' }}>
            <Select
              placeholder={t('group.drawer.selectUser')}
              allowClear
              showSearch
              optionFilterProp="label"
              style={{ flex: 1 }}
              value={selectedUserId}
              onChange={setSelectedUserId}
              options={availableUsers.map((u) => ({
                value: u.id,
                label: `${u.display_name || u.username} (${u.username})`,
              }))}
            />
            <Button
              type="primary"
              loading={addMemberLoading}
              onClick={handleAddMember}
              icon={<UserAddOutlined />}
            >
              {t('group.drawer.add')}
            </Button>
          </Space.Compact>
        </div>

        <Table
          scroll={{ x: 'max-content' }}
          rowKey="id"
          loading={drawerLoading}
          dataSource={drawerGroup?.users || []}
          columns={[
            {
              title: t('group.drawer.columns.username'),
              dataIndex: 'username',
              key: 'username',
            },
            {
              title: t('group.drawer.columns.displayName'),
              dataIndex: 'display_name',
              key: 'display_name',
              render: (val: string) => val || '-',
            },
            {
              title: t('group.drawer.columns.actions'),
              key: 'action',
              width: 80,
              render: (_, record) => (
                canDelete() ? (
                  <Button
                    type="link"
                    size="small"
                    danger
                    onClick={() => handleRemoveMember(record)}
                  >
                    {t('group.drawer.remove')}
                  </Button>
                ) : null
              ),
            },
          ]}
        />
      </Drawer>
    </div>
  );
};

export default UserGroupManagement;

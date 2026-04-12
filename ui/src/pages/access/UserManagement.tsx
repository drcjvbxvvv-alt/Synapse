import React, { useState, useEffect, useCallback } from 'react';
import {
  Button,
  Space,
  Form,
  Input,
  Select,
  Tag,
  App,
  Tooltip,
} from 'antd';
import {
  PlusOutlined,
  SearchOutlined,
  EditOutlined,
  DeleteOutlined,
  LockOutlined,
  StopOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import userService from '../../services/userService';
import type { User, CreateUserRequest, UpdateUserRequest } from '../../types';
import { TableListLayout } from '../../components/TableListLayout';
import { FormModal } from '../../components/FormModal';

const UserManagement: React.FC = () => {
  const { t } = useTranslation('permission');
  const { message, modal } = App.useApp();
  const [form] = Form.useForm();
  const [resetForm] = Form.useForm();

  const [loading, setLoading] = useState(false);
  const [users, setUsers] = useState<User[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [filterStatus, setFilterStatus] = useState<string>('');
  const [filterAuthType, setFilterAuthType] = useState<string>('');

  const [modalVisible, setModalVisible] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [resetModalVisible, setResetModalVisible] = useState(false);
  const [resetUserId, setResetUserId] = useState<number | null>(null);

  const loadUsers = useCallback(async () => {
    setLoading(true);
    try {
      const res = await userService.getUsers({
        page,
        pageSize,
        search: search || undefined,
        status: filterStatus || undefined,
        auth_type: filterAuthType || undefined,
      });
      setUsers(res.items || []);
      setTotal(res.total ?? 0);
    } catch (err) {
      message.error(t('user.messages.loadFailed'));
      console.error('Failed to load users:', err);
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, search, filterStatus, filterAuthType, message, t]);

  useEffect(() => {
    loadUsers();
  }, [loadUsers]);

  const handleCreate = () => {
    setEditingUser(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: User) => {
    setEditingUser(record);
    form.setFieldsValue({
      username: record.username,
      display_name: record.display_name,
      email: record.email,
    });
    setModalVisible(true);
  };

  const handleToggleStatus = async (record: User) => {
    if (record.username === 'admin') {
      message.warning(t('user.messages.adminCannotDisable'));
      return;
    }
    const newStatus = record.status === 'active' ? 'inactive' : 'active';
    const action = newStatus === 'active' ? t('user.actions.enable') : t('user.actions.disable');
    modal.confirm({
      title: t('user.confirm.toggleStatus', { action }),
      content: t('user.confirm.toggleStatusContent', { action, name: record.display_name || record.username }),
      okText: t('user.confirm.ok'),
      cancelText: t('user.confirm.cancel'),
      onOk: async () => {
        try {
          await userService.updateUserStatus(record.id, newStatus);
          message.success(newStatus === 'active' ? t('user.messages.enableSuccess') : t('user.messages.disableSuccess'));
          loadUsers();
        } catch (err) {
          message.error(t('user.messages.toggleFailed'));
          console.error(err);
        }
      },
    });
  };

  const handleResetPassword = (record: User) => {
    setResetUserId(record.id);
    resetForm.resetFields();
    setResetModalVisible(true);
  };

  const handleDelete = (record: User) => {
    if (record.username === 'admin') {
      message.warning(t('user.messages.adminCannotDelete'));
      return;
    }
    modal.confirm({
      title: t('user.confirm.delete'),
      content: t('user.confirm.deleteContent', { name: record.display_name || record.username }),
      okText: t('user.confirm.ok'),
      okType: 'danger',
      cancelText: t('user.confirm.cancel'),
      onOk: async () => {
        try {
          await userService.deleteUser(record.id);
          message.success(t('user.messages.deleteSuccess'));
          loadUsers();
        } catch (err) {
          message.error(t('user.messages.deleteFailed'));
          console.error(err);
        }
      },
    });
  };

  const isAdmin = (record: User) => record.username === 'admin';

  const columns: ColumnsType<User> = [
    {
      title: t('user.columns.username'),
      dataIndex: 'username',
      key: 'username',
      width: 120,
    },
    {
      title: t('user.columns.displayName'),
      dataIndex: 'display_name',
      key: 'display_name',
      width: 120,
      render: (text) => text || '-',
    },
    {
      title: t('user.columns.email'),
      dataIndex: 'email',
      key: 'email',
      width: 180,
      render: (text) => text || '-',
    },
    {
      title: t('user.columns.status'),
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (status: string) =>
        status === 'active' ? (
          <Tag color="success">{t('user.status.active')}</Tag>
        ) : (
          <Tag color="error">{t('user.status.inactive')}</Tag>
        ),
    },
    {
      title: t('user.columns.authType'),
      dataIndex: 'auth_type',
      key: 'auth_type',
      width: 100,
      render: (authType: string) => (authType === 'ldap' ? t('user.authType.ldap') : t('user.authType.local')),
    },
    {
      title: t('user.columns.lastLogin'),
      dataIndex: 'last_login_at',
      key: 'last_login_at',
      width: 170,
      render: (val: string) => (val ? dayjs(val).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: t('user.columns.actions'),
      key: 'action',
      width: 240,
      fixed: 'right',
      render: (_, record) => (
        <Space wrap>
          <Tooltip title={t('user.actions.edit')}>
            <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
              {t('user.actions.edit')}
            </Button>
          </Tooltip>
          {!isAdmin(record) && (
            <Button
              type="link"
              size="small"
              icon={record.status === 'active' ? <StopOutlined /> : <CheckCircleOutlined />}
              onClick={() => handleToggleStatus(record)}
            >
              {record.status === 'active' ? t('user.actions.disable') : t('user.actions.enable')}
            </Button>
          )}
          {record.auth_type === 'local' && (
            <Button
              type="link"
              size="small"
              icon={<LockOutlined />}
              onClick={() => handleResetPassword(record)}
            >
              {t('user.actions.resetPassword')}
            </Button>
          )}
          {!isAdmin(record) && (
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              onClick={() => handleDelete(record)}
            >
              {t('user.actions.delete')}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 0 }}>
      <TableListLayout<User>
        filters={
          <>
            <Input.Search
              placeholder={t('user.searchPlaceholder')}
              allowClear
              style={{ width: 260 }}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              onSearch={(v) => {
                setSearch(v);
                setPage(1);
              }}
              enterButton={<SearchOutlined />}
            />
            <Select
              placeholder={t('user.statusFilter')}
              allowClear
              style={{ width: 120 }}
              value={filterStatus || undefined}
              onChange={(v) => { setFilterStatus(v ?? ''); setPage(1); }}
            >
              <Select.Option value="">{t('user.status.all')}</Select.Option>
              <Select.Option value="active">{t('user.status.active')}</Select.Option>
              <Select.Option value="inactive">{t('user.status.inactive')}</Select.Option>
            </Select>
            <Select
              placeholder={t('user.authTypeFilter')}
              allowClear
              style={{ width: 120 }}
              value={filterAuthType || undefined}
              onChange={(v) => { setFilterAuthType(v ?? ''); setPage(1); }}
            >
              <Select.Option value="">{t('user.authType.all')}</Select.Option>
              <Select.Option value="local">{t('user.authType.local')}</Select.Option>
              <Select.Option value="ldap">{t('user.authType.ldap')}</Select.Option>
            </Select>
          </>
        }
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            {t('user.createUser')}
          </Button>
        }
        onRefresh={loadUsers}
        refreshing={loading}
        tableProps={{
          columns,
          dataSource: users,
          rowKey: 'id',
          loading,
          pagination: {
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (tot) => t('user.pagination.total', { total: tot }),
            onChange: (p, ps) => {
              setPage(p);
              setPageSize(ps || 20);
            },
          },
        }}
      />

      {/* 新增 / 編輯使用者 */}
      <FormModal
        open={modalVisible}
        onClose={() => setModalVisible(false)}
        onSubmit={async () => {
          const values = await form.validateFields();
          if (editingUser) {
            const data: UpdateUserRequest = {
              display_name: values.display_name,
              email: values.email,
            };
            await userService.updateUser(editingUser.id, data);
            message.success(t('user.messages.updateSuccess'));
          } else {
            const data: CreateUserRequest = {
              username: values.username,
              password: values.password,
              display_name: values.display_name,
              email: values.email,
            };
            await userService.createUser(data);
            message.success(t('user.messages.createSuccess'));
          }
          setModalVisible(false);
          loadUsers();
        }}
        form={form}
        isEdit={!!editingUser}
        createTitle={t('user.createUser')}
        editTitle={t('user.editUser')}
        width={520}
      >
        {!editingUser && (
          <>
            <Form.Item
              name="username"
              label={t('user.form.username')}
              rules={[{ required: true, message: t('user.form.usernameRequired') }]}
            >
              <Input placeholder={t('user.form.usernamePlaceholder')} />
            </Form.Item>
            <Form.Item
              name="password"
              label={t('user.form.password')}
              rules={[
                { required: true, message: t('user.form.passwordRequired') },
                { min: 6, message: t('user.form.passwordMinLength') },
              ]}
            >
              <Input.Password placeholder={t('user.form.passwordPlaceholder')} />
            </Form.Item>
          </>
        )}
        <Form.Item name="display_name" label={t('user.form.displayName')}>
          <Input placeholder={t('user.form.displayNamePlaceholder')} />
        </Form.Item>
        <Form.Item name="email" label={t('user.form.email')}>
          <Input placeholder={t('user.form.emailPlaceholder')} />
        </Form.Item>
      </FormModal>

      {/* 重設密碼 */}
      <FormModal
        open={resetModalVisible}
        onClose={() => { setResetModalVisible(false); setResetUserId(null); }}
        onSubmit={async () => {
          if (resetUserId === null) return;
          const values = await resetForm.validateFields();
          await userService.resetPassword(resetUserId, values.new_password);
          message.success(t('user.messages.passwordResetSuccess'));
          setResetModalVisible(false);
          setResetUserId(null);
        }}
        form={resetForm}
        title={t('user.resetPassword.title')}
        width={400}
      >
        <Form.Item
          name="new_password"
          label={t('user.resetPassword.newPassword')}
          rules={[
            { required: true, message: t('user.resetPassword.newPasswordRequired') },
            { min: 6, message: t('user.form.passwordMinLength') },
          ]}
        >
          <Input.Password placeholder={t('user.resetPassword.newPasswordPlaceholder')} />
        </Form.Item>
      </FormModal>
    </div>
  );
};

export default UserManagement;

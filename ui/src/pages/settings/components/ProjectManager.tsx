/**
 * ProjectManager — Drawer that lists and manages Projects for a Git Provider (M14.1)
 *
 * Features:
 *  - List all Projects under a Git Provider
 *  - Create / Edit / Delete Project
 *  - Each Project maps a repo URL to this provider for precise webhook matching
 */
import React, { useState, useCallback, useEffect } from 'react';
import {
  Drawer,
  Table,
  Button,
  Space,
  Tooltip,
  Popconfirm,
  Modal,
  Form,
  Input,
  Typography,
  App,
  theme,
  Flex,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';

import projectService, { type Project } from '../../../services/projectService';
import { translateGitError } from '../../../utils/gitErrors';
import EmptyState from '../../../components/EmptyState';

const { Text } = Typography;

// ─── Props ────────────────────────────────────────────────────────────────────

interface ProjectManagerProps {
  open: boolean;
  onClose: () => void;
  providerId: number;
  providerName: string;
}

// ─── Main component ───────────────────────────────────────────────────────────

const ProjectManager: React.FC<ProjectManagerProps> = ({
  open,
  onClose,
  providerId,
  providerName,
}) => {
  const { t } = useTranslation(['cicd', 'common']);
  const { token } = theme.useToken();
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const [form] = Form.useForm();

  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Project | null>(null);

  const queryKey = ['projects', providerId];

  // ─── Data ────────────────────────────────────────────────────────────────────

  const { data, isLoading, refetch } = useQuery({
    queryKey,
    queryFn: () => projectService.list(providerId),
    enabled: open && providerId > 0,
    staleTime: 15_000,
  });

  const projects = data?.items ?? [];

  // ─── Mutations ───────────────────────────────────────────────────────────────

  const createMutation = useMutation({
    mutationFn: (values: Parameters<typeof projectService.create>[1]) =>
      projectService.create(providerId, values),
    onSuccess: () => {
      message.success(t('cicd:project.messages.createSuccess'));
      queryClient.invalidateQueries({ queryKey });
      setFormOpen(false);
    },
    onError: (err) => message.error(translateGitError(err, t) || t('cicd:project.messages.createFailed')),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, values }: { id: number; values: Parameters<typeof projectService.update>[2] }) =>
      projectService.update(providerId, id, values),
    onSuccess: () => {
      message.success(t('cicd:project.messages.updateSuccess'));
      queryClient.invalidateQueries({ queryKey });
      setFormOpen(false);
    },
    onError: (err) => message.error(translateGitError(err, t) || t('cicd:project.messages.updateFailed')),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => projectService.delete(providerId, id),
    onSuccess: () => {
      message.success(t('cicd:project.messages.deleteSuccess'));
      queryClient.invalidateQueries({ queryKey });
    },
    onError: () => message.error(t('cicd:project.messages.deleteFailed')),
  });

  // ─── Handlers ────────────────────────────────────────────────────────────────

  const handleCreate = useCallback(() => {
    setEditing(null);
    form.resetFields();
    setFormOpen(true);
  }, [form]);

  const handleEdit = useCallback(
    (record: Project) => {
      setEditing(record);
      form.setFieldsValue({
        name: record.name,
        repo_url: record.repo_url,
        default_branch: record.default_branch,
        description: record.description,
      });
      setFormOpen(true);
    },
    [form],
  );

  const handleSubmit = useCallback(async () => {
    const values = await form.validateFields();
    if (editing) {
      updateMutation.mutate({ id: editing.id, values });
    } else {
      createMutation.mutate(values);
    }
  }, [form, editing, createMutation, updateMutation]);

  // Reset form when drawer closes
  useEffect(() => {
    if (!open) {
      setFormOpen(false);
      setEditing(null);
      form.resetFields();
    }
  }, [open, form]);

  // ─── Columns ─────────────────────────────────────────────────────────────────

  const columns: TableColumnsType<Project> = [
    {
      title: t('cicd:project.table.name'),
      dataIndex: 'name',
      key: 'name',
      width: 180,
      ellipsis: true,
    },
    {
      title: t('cicd:project.table.repoUrl'),
      dataIndex: 'repo_url',
      key: 'repo_url',
      ellipsis: true,
      render: (url: string) => (
        <Text
          style={{ color: token.colorPrimary, fontSize: token.fontSizeSM }}
          ellipsis
        >
          {url}
        </Text>
      ),
    },
    {
      title: t('cicd:project.table.defaultBranch'),
      dataIndex: 'default_branch',
      key: 'default_branch',
      width: 130,
    },
    {
      title: t('cicd:project.table.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (time: string) => (
        <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
          {dayjs(time).format('YYYY-MM-DD HH:mm')}
        </Text>
      ),
    },
    {
      title: t('common:table.actions'),
      key: 'actions',
      width: 90,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Tooltip title={t('common:actions.edit')}>
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record)}
            />
          </Tooltip>
          <Popconfirm
            title={t('common:confirm.deleteTitle')}
            description={t('common:confirm.deleteDesc', { name: record.name })}
            onConfirm={() => deleteMutation.mutate(record.id)}
            okText={t('common:actions.delete')}
            okButtonProps={{ danger: true }}
            cancelText={t('common:actions.cancel')}
          >
            <Tooltip title={t('common:actions.delete')}>
              <Button type="link" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // ─── Render ──────────────────────────────────────────────────────────────────

  return (
    <>
      <Drawer
        title={t('cicd:project.drawerTitle', { name: providerName })}
        open={open}
        onClose={onClose}
        width={800}
        extra={
          <Space>
            <Tooltip title={t('common:actions.refresh')}>
              <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading} />
            </Tooltip>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              {t('common:actions.create')}
            </Button>
          </Space>
        }
      >
        <Table<Project>
          columns={columns}
          dataSource={projects}
          rowKey="id"
          loading={isLoading}
          size="small"
          scroll={{ x: 'max-content' }}
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (n) => t('common:pagination.total', { total: n }),
          }}
          locale={{
            emptyText: (
              <EmptyState description={t('cicd:project.noProjects')} />
            ),
          }}
        />
      </Drawer>

      {/* Create / Edit modal */}
      <Modal
        title={editing ? t('cicd:project.form.editTitle') : t('cicd:project.form.createTitle')}
        open={formOpen}
        onCancel={() => setFormOpen(false)}
        onOk={handleSubmit}
        okText={editing ? t('common:actions.save') : t('common:actions.create')}
        cancelText={t('common:actions.cancel')}
        confirmLoading={createMutation.isPending || updateMutation.isPending}
        destroyOnHidden
        width={520}
      >
        <Form form={form} layout="vertical" style={{ marginTop: token.marginMD }}>
          <Form.Item
            name="name"
            label={t('cicd:project.form.name')}
            rules={[{ required: true, message: t('common:validation.required') }]}
          >
            <Input placeholder={t('cicd:project.form.namePlaceholder')} />
          </Form.Item>

          <Form.Item
            name="repo_url"
            label={t('cicd:project.form.repoUrl')}
            rules={[
              { required: true, message: t('common:validation.required') },
              { type: 'url', message: t('common:validation.invalid') },
            ]}
          >
            <Input placeholder={t('cicd:project.form.repoUrlPlaceholder')} />
          </Form.Item>

          <Form.Item
            name="default_branch"
            label={t('cicd:project.form.defaultBranch')}
          >
            <Input placeholder={t('cicd:project.form.defaultBranchPlaceholder')} />
          </Form.Item>

          <Form.Item
            name="description"
            label={t('cicd:project.form.description')}
          >
            <Input.TextArea
              rows={3}
              placeholder={t('cicd:project.form.descriptionPlaceholder')}
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

export default ProjectManager;

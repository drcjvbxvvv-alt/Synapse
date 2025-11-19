/** genAI_main_start */
import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Table,
  Button,
  Space,
  Tag,
  Select,
  Input,
  Modal,
  Tooltip,
  Badge,
  Dropdown,
  App,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  EyeOutlined,
  MoreOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { WorkloadService } from '../../services/workloadService';
import type { WorkloadInfo } from '../../services/workloadService';
import type { ColumnsType } from 'antd/es/table';

const { Option } = Select;
const { Search } = Input;

interface DaemonSetTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

const DaemonSetTab: React.FC<DaemonSetTabProps> = ({ clusterId, onCountChange }) => {
  const navigate = useNavigate();
  const { message } = App.useApp();
  
  // 数据状态
  const [workloads, setWorkloads] = useState<WorkloadInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  
  // 分页状态
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  
  // 筛选状态
  const [selectedNamespace, setSelectedNamespace] = useState<string>('');
  const [searchText, setSearchText] = useState('');
  
  // 命名空间列表状态
  const [namespaces, setNamespaces] = useState<Array<{ name: string; count: number }>>([]);
  const [namespacesLoading, setNamespacesLoading] = useState(false);
  
  // 操作状态
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  
  // 防抖引用
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // 获取命名空间列表
  const loadNamespaces = useCallback(async () => {
    if (!clusterId) return;
    
    setNamespacesLoading(true);
    try {
      const response = await WorkloadService.getWorkloadNamespaces(clusterId, 'DaemonSet');
      
      if (response.code === 200 && response.data) {
        setNamespaces(response.data);
      } else {
        setNamespaces([]);
      }
    } catch (error) {
      console.error('获取命名空间列表失败:', error);
      setNamespaces([]);
    } finally {
      setNamespacesLoading(false);
    }
  }, [clusterId]);

  // 加载DaemonSet列表
  const loadWorkloads = useCallback(async (page: number = currentPage, search: string = '') => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloads(
        clusterId,
        selectedNamespace || undefined,
        'DaemonSet',
        page,
        pageSize,
        search || undefined
      );
      
      if (response.code === 200) {
        setWorkloads(response.data.items || []);
        setTotal(response.data.total || response.data.items?.length || 0);
        onCountChange?.(response.data.total || response.data.items?.length || 0);
      } else {
        message.error(response.message || '获取DaemonSet列表失败');
      }
    } catch (error) {
      console.error('获取DaemonSet列表失败:', error);
      message.error('获取DaemonSet列表失败');
    } finally {
      setLoading(false);
    }
  }, [clusterId, selectedNamespace, currentPage, pageSize, onCountChange, message]);

  // 删除
  const handleDelete = async (workload: WorkloadInfo) => {
    if (!clusterId) return;
    
    try {
      const response = await WorkloadService.deleteWorkload(
        clusterId,
        workload.namespace,
        workload.name,
        workload.type
      );
      
      if (response.code === 200) {
        message.success('删除成功');
        loadWorkloads();
      } else {
        message.error(response.message || '删除失败');
      }
    } catch (error) {
      console.error('删除失败:', error);
      message.error('删除失败');
    }
  };

  // 批量删除
  const handleBatchDelete = async () => {
    if (!clusterId || selectedRowKeys.length === 0) return;
    
    const selectedWorkloads = workloads.filter(w => 
      selectedRowKeys.includes(`${w.namespace}-${w.name}-${w.type}`)
    );
    
    try {
      const deletePromises = selectedWorkloads.map(workload =>
        WorkloadService.deleteWorkload(
          clusterId,
          workload.namespace,
          workload.name,
          workload.type
        )
      );
      
      const results = await Promise.allSettled(deletePromises);
      const successCount = results.filter(r => r.status === 'fulfilled').length;
      const failCount = results.length - successCount;
      
      if (failCount === 0) {
        message.success(`成功删除 ${successCount} 个DaemonSet`);
      } else {
        message.warning(`删除完成：成功 ${successCount} 个，失败 ${failCount} 个`);
      }
      
      setSelectedRowKeys([]);
      loadWorkloads();
    } catch (error) {
      console.error('批量删除失败:', error);
      message.error('批量删除失败');
    }
  };

  // 搜索防抖处理
  useEffect(() => {
    if (!searchText || searchText.trim().length === 0) {
      setCurrentPage(1);
      loadWorkloads(1, '');
      return;
    }
    
    if (searchText.trim().length <= 2) {
      return;
    }
    
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }
    
    searchTimeoutRef.current = setTimeout(() => {
      setCurrentPage(1);
      loadWorkloads(1, searchText);
    }, 500);
    
    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [searchText, loadWorkloads]);

  // 加载数据
  useEffect(() => {
    loadWorkloads();
  }, [loadWorkloads]);

  // 加载命名空间
  useEffect(() => {
    loadNamespaces();
  }, [loadNamespaces]);

  // 行选择配置
  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

  const columns: ColumnsType<WorkloadInfo> = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      fixed: 'left' as const,
      render: (text: string, record: WorkloadInfo) => (
        <Button
          type="link"
          onClick={() => navigate(`/clusters/${clusterId}/workloads/${record.namespace}/${record.name}?type=${record.type}`)}
          style={{ 
            padding: 0, 
            height: 'auto',
            whiteSpace: 'normal',
            wordBreak: 'break-all',
            textAlign: 'left'
          }}
        >
          <div style={{
            whiteSpace: 'normal',
            wordBreak: 'break-all',
            lineHeight: '1.4'
          }}>
            {text}
          </div>
        </Button>
      ),
    },
    {
      title: '命名空间',
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'ready',
      key: 'ready',
      width: 120,
      render: (_text: string, record: WorkloadInfo) => {
        const { status, color } = WorkloadService.formatStatus(record);
        return <Badge status={color as 'success' | 'error' | 'default' | 'processing' | 'warning'} text={status} />;
      },
    },
    {
      title: '期望/当前/就绪/可用',
      key: 'replicas',
      width: 160,
      render: (record: WorkloadInfo) => (
        <span>
          {record.replicas || 0} / {record.currentReplicas || 0} / {record.readyReplicas || 0} / {record.availableReplicas || 0}
        </span>
      ),
    },
    {
      title: '镜像',
      dataIndex: 'images',
      key: 'images',
      width: 200,
      render: (images: string[]) => {
        if (!images || images.length === 0) return '-';
        
        const firstImage = images[0];
        const imageName = firstImage.split('/').pop()?.split(':')[0] || firstImage;
        
        return (
          <div>
            <Tooltip title={firstImage}>
              <Tag style={{ marginBottom: 2, maxWidth: 150, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {imageName}
              </Tag>
            </Tooltip>
            {images.length > 1 && (
              <Tooltip title={images.slice(1).join('\n')}>
                <Tag style={{ marginBottom: 2 }}>
                  +{images.length - 1}
                </Tag>
              </Tooltip>
            )}
          </div>
        );
      },
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 150,
      render: (text: string) => {
        if (!text) return '-';
        const date = new Date(text);
        return (
          <Tooltip title={date.toLocaleString('zh-CN')}>
            <span>{date.toLocaleDateString('zh-CN')}</span>
          </Tooltip>
        );
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      fixed: 'right' as const,
      render: (record: WorkloadInfo) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => navigate(`/clusters/${clusterId}/workloads/${record.namespace}/${record.name}?type=${record.type}`)}
            style={{ padding: '0 4px' }}
          >
            详情
          </Button>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'delete',
                  icon: <DeleteOutlined />,
                  label: '删除',
                  danger: true,
                  onClick: () => {
                    Modal.confirm({
                      title: '确认删除',
                      content: `确定要删除DaemonSet ${record.name} 吗？`,
                      okText: '确定',
                      cancelText: '取消',
                      okType: 'danger',
                      onOk: () => handleDelete(record),
                    });
                  },
                },
              ],
            }}
            trigger={['click']}
          >
            <Button
              type="link"
              size="small"
              icon={<MoreOutlined />}
              style={{ padding: '0 4px' }}
            />
          </Dropdown>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <style>{`
        .namespace-select-dropdown .ant-select-item-option-content {
          white-space: normal;
          word-break: break-word;
        }
      `}</style>
      
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Space>
          <Select
            placeholder="选择命名空间"
            style={{ width: 200 }}
            value={selectedNamespace || undefined}
            onChange={(value) => {
              setSelectedNamespace(value || '');
              setCurrentPage(1);
            }}
            allowClear
            loading={namespacesLoading}
            showSearch
            popupClassName="namespace-select-dropdown"
            filterOption={(input, option) => {
              if (!option?.children) return false;
              const text = String(option.children);
              return text.toLowerCase().includes(input.toLowerCase());
            }}
          >
            {namespaces.map(ns => (
              <Option key={ns.name} value={ns.name}>
                {ns.name} ({ns.count})
              </Option>
            ))}
          </Select>

          <Search
            placeholder="搜索DaemonSet名称"
            style={{ width: 250 }}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            allowClear
          />

          {selectedRowKeys.length > 0 && (
            <Button
              danger
              icon={<DeleteOutlined />}
              onClick={() => {
                Modal.confirm({
                  title: '批量删除确认',
                  content: `确定要删除选中的 ${selectedRowKeys.length} 个DaemonSet吗？`,
                  okText: '确定',
                  cancelText: '取消',
                  okType: 'danger',
                  onOk: handleBatchDelete,
                });
              }}
            >
              批量删除 ({selectedRowKeys.length})
            </Button>
          )}
        </Space>

        <Space>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => navigate(`/clusters/${clusterId}/yaml/apply?type=DaemonSet`)}
          >
            创建DaemonSet
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => {
              loadWorkloads();
              loadNamespaces();
            }}
          >
            刷新
          </Button>
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={workloads}
        rowKey={(record) => `${record.namespace}-${record.name}-${record.type}`}
        rowSelection={rowSelection}
        loading={loading}
        scroll={{ x: 1400 }}
        size="middle"
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 个DaemonSet`,
          onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />
    </div>
  );
};

export default DaemonSetTab;
/** genAI_main_end */


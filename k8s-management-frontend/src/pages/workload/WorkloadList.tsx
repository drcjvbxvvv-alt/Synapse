import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Select,
  Input,
  Modal,
  message,
  Tooltip,
  Badge,
  Dropdown,
  InputNumber,
  Popconfirm,
  Checkbox,
  Alert,
} from 'antd';
import {
  ReloadOutlined,
  SearchOutlined,
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ExpandAltOutlined,
  EyeOutlined,
  MoreOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import { WorkloadService } from '../../services/workloadService';
import type { WorkloadInfo } from '../../services/workloadService';
import { clusterService } from '../../services/clusterService';
import type { Cluster } from '../../types';

const { Option } = Select;
const { Search } = Input;

interface WorkloadListProps {}

const WorkloadList: React.FC<WorkloadListProps> = () => {
  const { clusterId: routeClusterId } = useParams<{ clusterId: string }>();
  const navigate = useNavigate();
  
  const [workloads, setWorkloads] = useState<WorkloadInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [selectedNamespace, setSelectedNamespace] = useState<string>('');
  const [selectedType, setSelectedType] = useState<string>('');
  const [searchText, setSearchText] = useState('');
  const [scaleModalVisible, setScaleModalVisible] = useState(false);
  const [scaleWorkload, setScaleWorkload] = useState<WorkloadInfo | null>(null);
  const [scaleReplicas, setScaleReplicas] = useState(1);
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [selectedClusterId, setSelectedClusterId] = useState<string>(routeClusterId || '1');

  // 获取集群列表
  const fetchClusters = useCallback(async () => {
    try {
      const response = await clusterService.getClusters();
      setClusters(response.data.items || []);
    } catch (error) {
      message.error('获取集群列表失败');
      console.error('获取集群列表失败:', error);
    }
  }, []);

  // 获取工作负载列表
  const fetchWorkloads = useCallback(async () => {
    if (!selectedClusterId) return;
    
    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloads(
        selectedClusterId,
        selectedNamespace || undefined,
        selectedType || undefined,
        currentPage,
        pageSize
      );
      
      if (response.code === 200) {
        // 后端返回的数据结构是 { items: [], total: number }
        setWorkloads(response.data.items || []);
        setTotal(response.data.total || response.data.items?.length || 0);
      } else {
        message.error(response.message || '获取工作负载列表失败');
      }
    } catch (error) {
      console.error('获取工作负载列表失败:', error);
      message.error('获取工作负载列表失败');
    } finally {
      setLoading(false);
    }
  }, [selectedClusterId, selectedNamespace, selectedType, currentPage, pageSize]);

  // 集群切换
  const handleClusterChange = (clusterId: string) => {
    setSelectedClusterId(clusterId);
    setCurrentPage(1);
    // 重置搜索和筛选条件
    setSearchText('');
    setSelectedNamespace('');
    setSelectedType('');
  };

  // 扩缩容工作负载
  const handleScale = async () => {
    if (!scaleWorkload || !selectedClusterId) return;
    
    try {
      const response = await WorkloadService.scaleWorkload(
        selectedClusterId,
        scaleWorkload.namespace,
        scaleWorkload.name,
        scaleWorkload.type,
        scaleReplicas
      );
      
      if (response.code === 200) {
        message.success('扩缩容成功');
        setScaleModalVisible(false);
        fetchWorkloads();
      } else {
        message.error(response.message || '扩缩容失败');
      }
    } catch (error) {
      console.error('扩缩容失败:', error);
      message.error('扩缩容失败');
    }
  };

  // 删除工作负载
  // 删除工作负载
  const handleDelete = async (workload: WorkloadInfo) => {
    if (!selectedClusterId) return;
    
    try {
      const response = await WorkloadService.deleteWorkload(
        selectedClusterId,
        workload.namespace,
        workload.name,
        workload.type
      );
      
      if (response.code === 200) {
        message.success('删除成功');
        fetchWorkloads();
      } else {
        message.error(response.message || '删除失败');
      }
    } catch (error) {
      console.error('删除失败:', error);
      message.error('删除失败');
    }
  };

  // 批量删除工作负载
  const handleBatchDelete = async () => {
    if (!selectedClusterId || selectedRowKeys.length === 0) return;
    
    const selectedWorkloads = workloads.filter(w => 
      selectedRowKeys.includes(`${w.namespace}-${w.name}-${w.type}`)
    );
    
    try {
      const deletePromises = selectedWorkloads.map(workload =>
        WorkloadService.deleteWorkload(
          selectedClusterId,
          workload.namespace,
          workload.name,
          workload.type
        )
      );
      
      const results = await Promise.allSettled(deletePromises);
      const successCount = results.filter(r => r.status === 'fulfilled').length;
      const failCount = results.length - successCount;
      
      if (failCount === 0) {
        message.success(`成功删除 ${successCount} 个工作负载`);
      } else {
        message.warning(`删除完成：成功 ${successCount} 个，失败 ${failCount} 个`);
      }
      
      setBatchDeleteModalVisible(false);
      setSelectedRowKeys([]);
      fetchWorkloads();
    } catch (error) {
      console.error('批量删除失败:', error);
      message.error('批量删除失败');
    }
  };

  // 行选择配置
  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
    onSelectAll: (selected: boolean, selectedRows: WorkloadInfo[], changeRows: WorkloadInfo[]) => {
      if (selected) {
        const allKeys = filteredWorkloads.map(w => `${w.namespace}-${w.name}-${w.type}`);
        setSelectedRowKeys(allKeys);
      } else {
        setSelectedRowKeys([]);
      }
    },
  };

  // 获取唯一的命名空间列表
  const getNamespaces = () => {
    const namespaces = Array.from(new Set(workloads.map(w => w.namespace)));
    return namespaces.sort();
  };

  // 过滤工作负载
  const filteredWorkloads = workloads.filter(workload => {
    if (searchText && !workload.name.toLowerCase().includes(searchText.toLowerCase())) {
      return false;
    }
    return true;
  });

  // 初始化加载
  useEffect(() => {
    fetchClusters();
  }, [fetchClusters]);

  // 当选中的集群ID变化时，重新获取数据
  useEffect(() => {
    if (selectedClusterId) {
      fetchWorkloads();
    }
  }, [selectedClusterId, fetchWorkloads]);

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: WorkloadInfo) => (
        <Button
          type="link"
          onClick={() => navigate(`/clusters/${selectedClusterId}/workloads/${record.namespace}/${record.name}?type=${record.type}`)}
        >
          {text}
        </Button>
      ),
    },
    {
      title: '命名空间',
      dataIndex: 'namespace',
      key: 'namespace',
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (text: string) => {
        const typeConfig = WorkloadService.getWorkloadTypes().find(t => t.value === text);
        return (
          <Tag color="green">
            {typeConfig?.icon} {typeConfig?.label || text}
          </Tag>
        );
      },
    },
    {
      title: '状态',
      dataIndex: 'ready',
      key: 'ready',
      render: (text: string, record: WorkloadInfo) => {
        const { status, color } = WorkloadService.formatStatus(record);
        return <Badge status={color as any} text={status} />;
      },
    },
    {
      title: '副本数',
      key: 'replicas',
      render: (record: WorkloadInfo) => {
        if (record.type.toLowerCase() === 'daemonset') {
          return <span>-</span>;
        }
        return (
          <span>
            {record.readyReplicas || 0} / {record.replicas || 0}
          </span>
        );
      },
    },
    {
      title: '镜像',
      dataIndex: 'images',
      key: 'images',
      render: (images: string[]) => (
        <div>
          {images.slice(0, 2).map((image, index) => (
            <Tooltip key={index} title={image}>
              <Tag style={{ marginBottom: 4 }}>
                {image.split('/').pop()?.split(':')[0] || image}
              </Tag>
            </Tooltip>
          ))}
          {images.length > 2 && (
            <Tag>+{images.length - 2} more</Tag>
          )}
        </div>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (text: string) => {
        if (!text) return '-';
        return new Date(text).toLocaleString('zh-CN');
      },
    },
    {
      title: '操作',
      key: 'actions',
      render: (record: WorkloadInfo) => {
        const canScale = ['deployment', 'statefulset', 'Deployment', 'StatefulSet'].includes(record.type);
        
        const menuItems = [
          {
            key: 'view',
            icon: <EyeOutlined />,
            label: '查看详情',
            onClick: () => navigate(`/clusters/${selectedClusterId}/workloads/${record.namespace}/${record.name}?type=${record.type}`),
          },
          ...(canScale ? [{
            key: 'scale',
            icon: <ExpandAltOutlined />,
            label: '扩缩容',
            onClick: () => {
              setScaleWorkload(record);
              setScaleReplicas(record.replicas || 1);
              setScaleModalVisible(true);
            },
          }] : []),
          {
            key: 'delete',
            icon: <DeleteOutlined />,
            label: '删除',
            danger: true,
          },
        ];

        return (
          <Space>
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => navigate(`/clusters/${selectedClusterId}/workloads/${record.namespace}/${record.name}?type=${record.type}`)}
            >
              详情
            </Button>
            {canScale && (
              <Button
                type="link"
                size="small"
                icon={<ExpandAltOutlined />}
                onClick={() => {
                  setScaleWorkload(record);
                  setScaleReplicas(record.replicas || 1);
                  setScaleModalVisible(true);
                }}
              >
                扩缩容
              </Button>
            )}
            <Popconfirm
              title="确认删除"
              description={`确定要删除工作负载 ${record.name} 吗？`}
              onConfirm={() => handleDelete(record)}
              okText="确定"
              cancelText="取消"
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
              >
                删除
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      {/* 页面头部 */}
      <div style={{ marginBottom: 24 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h1>工作负载管理</h1>
            <p>管理集群中的工作负载，包括 Deployment、StatefulSet、DaemonSet 等</p>
          </div>
          <Space>
            <Select
              value={selectedClusterId}
              style={{ width: 200 }}
              onChange={handleClusterChange}
              placeholder="选择集群"
              loading={clusters.length === 0}
            >
              {clusters.map(cluster => (
                <Option key={cluster.id} value={cluster.id.toString()}>
                  {cluster.name}
                </Option>
              ))}
            </Select>
            <Button icon={<ReloadOutlined />} onClick={fetchWorkloads} loading={loading}>
              刷新
            </Button>
          </Space>
        </div>
      </div>

      <Card>
        <div style={{ marginBottom: 16 }}>
          <Space wrap>
            <Select
              placeholder="选择命名空间"
              style={{ width: 200 }}
              value={selectedNamespace}
              onChange={setSelectedNamespace}
              allowClear
            >
              {getNamespaces().map(ns => (
                <Option key={ns} value={ns}>{ns}</Option>
              ))}
            </Select>
            
            <Select
              placeholder="选择工作负载类型"
              style={{ width: 200 }}
              value={selectedType}
              onChange={setSelectedType}
              allowClear
            >
              {WorkloadService.getWorkloadTypes().map(type => (
                <Option key={type.value} value={type.value}>
                  {type.icon} {type.label}
                </Option>
              ))}
            </Select>
            
            <Search
              placeholder="搜索工作负载名称"
              style={{ width: 300 }}
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              allowClear
            />
            
            <Button
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => navigate(`/clusters/${selectedClusterId}/yaml/apply`)}
            >
              创建工作负载
            </Button>
            
            {selectedRowKeys.length > 0 && (
              <Button
                danger
                icon={<DeleteOutlined />}
                onClick={() => setBatchDeleteModalVisible(true)}
              >
                批量删除 ({selectedRowKeys.length})
              </Button>
            )}
          </Space>
        </div>

        <Table
          columns={columns}
          dataSource={filteredWorkloads}
          rowKey={(record) => `${record.namespace}-${record.name}-${record.type}`}
          loading={loading}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total, range) => `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
            onChange: (page, size) => {
              setCurrentPage(page);
              setPageSize(size || 20);
            },
          }}
        />
      </Card>

      {/* 扩缩容模态框 */}
      {/* 扩缩容模态框 */}
      <Modal
        title="扩缩容工作负载"
        open={scaleModalVisible}
        onOk={handleScale}
        onCancel={() => setScaleModalVisible(false)}
        okText="确定"
        cancelText="取消"
      >
        {scaleWorkload && (
          <div>
            <p>工作负载: <strong>{scaleWorkload.name}</strong></p>
            <p>命名空间: <strong>{scaleWorkload.namespace}</strong></p>
            <p>当前副本数: <strong>{scaleWorkload.replicas || 0}</strong></p>
            <div style={{ marginTop: 16 }}>
              <label>目标副本数: </label>
              <InputNumber
                min={0}
                max={100}
                value={scaleReplicas}
                onChange={(value) => setScaleReplicas(value || 1)}
                style={{ marginLeft: 8 }}
              />
            </div>
          </div>
        )}
      </Modal>

      {/* 批量删除模态框 */}
      <Modal
        title="批量删除工作负载"
        open={batchDeleteModalVisible}
        onOk={handleBatchDelete}
        onCancel={() => setBatchDeleteModalVisible(false)}
        okText="确定删除"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        width={600}
      >
        <div>
          <Alert
            message="警告"
            description={`您即将删除 ${selectedRowKeys.length} 个工作负载，此操作不可撤销！`}
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
          />
          
          <div style={{ maxHeight: 300, overflowY: 'auto' }}>
            <h4>将要删除的工作负载：</h4>
            {workloads
              .filter(w => selectedRowKeys.includes(`${w.namespace}-${w.name}-${w.type}`))
              .map(workload => (
                <div key={`${workload.namespace}-${workload.name}-${workload.type}`} 
                     style={{ padding: '8px', border: '1px solid #f0f0f0', marginBottom: '4px', borderRadius: '4px' }}>
                  <Space>
                    <Tag color="blue">{workload.namespace}</Tag>
                    <span><strong>{workload.name}</strong></span>
                    <Tag color="green">{workload.type}</Tag>
                  </Space>
                </div>
              ))}
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default WorkloadList;
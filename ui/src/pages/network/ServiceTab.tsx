import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Table,
  Button,
  Space,
  Tag,
  Input,
  Select,
  Popconfirm,
  Typography,
  Tooltip,
  Modal,
  Descriptions,
  App,
  Tabs,
  Form,
  InputNumber,
  Checkbox,
  Drawer,
} from 'antd';
import {
  ReloadOutlined,
  SearchOutlined,
  PlusOutlined,
  MinusCircleOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import MonacoEditor from '@monaco-editor/react';
import * as YAML from 'yaml';
import { ServiceService } from '../../services/serviceService';
import type { Service } from '../../types';
import type { ColumnsType, TablePaginationConfig } from 'antd/es/table';
import type { FilterValue, SorterResult } from 'antd/es/table/interface';
import ServiceCreateModal from './ServiceCreateModal';
import { useTranslation } from 'react-i18next';

const { Text, Link } = Typography;

interface ServiceTabProps {
  clusterId: string;
  onCountChange?: (count: number) => void;
}

interface KubernetesServiceYAML {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace: string;
    labels: Record<string, string>;
    annotations: Record<string, string>;
  };
  spec: {
    type?: string;
    selector?: Record<string, string>;
    ports: Array<{
      name?: string;
      protocol: string;
      port: number;
      targetPort: number | string;
      nodePort?: number;
    }>;
    sessionAffinity?: string;
  };
}

interface LabelItem {
  key: string;
  value: string;
}

const ServiceTab: React.FC<ServiceTabProps> = ({ clusterId, onCountChange }) => {
  const navigate = useNavigate();
  const { message } = App.useApp();
  
  // 数据状态
const { t } = useTranslation(['network', 'common']);
const [allServices, setAllServices] = useState<Service[]>([]); // 所有原始数据
  const [services, setServices] = useState<Service[]>([]); // 当前页显示的数据
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  
  // 分页状态
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  
  // 选择行状态
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  
  // 多条件搜索状态
  interface SearchCondition {
    field: 'name' | 'namespace' | 'type' | 'clusterIP' | 'selector';
    value: string;
  }
  const [searchConditions, setSearchConditions] = useState<SearchCondition[]>([]);
  const [currentSearchField, setCurrentSearchField] = useState<'name' | 'namespace' | 'type' | 'clusterIP' | 'selector'>('name');
  const [currentSearchValue, setCurrentSearchValue] = useState('');

  // 列设置状态
  const [columnSettingsVisible, setColumnSettingsVisible] = useState(false);
  const [visibleColumns, setVisibleColumns] = useState<string[]>([
    'name', 'type', 'access', 'ports', 'selector', 'createdAt'
  ]);
  
  // 排序状态
  const [sortField, setSortField] = useState<string>('');
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);
  
  // YAML查看Modal
  const [yamlModalVisible, setYamlModalVisible] = useState(false);
  const [currentYaml, setCurrentYaml] = useState('');
  const [yamlLoading, setYamlLoading] = useState(false);

  // Endpoints查看Modal
  interface EndpointsData {
    name: string;
    namespace: string;
    subsets?: Array<{
      addresses?: Array<{ ip: string; nodeName?: string }>;
      ports?: Array<{ name?: string; port: number; protocol: string }>;
    }>;
  }
  const [endpointsModalVisible, setEndpointsModalVisible] = useState(false);
  const [currentEndpoints, setCurrentEndpoints] = useState<EndpointsData | null>(null);
  const [endpointsLoading, setEndpointsLoading] = useState(false);

  // YAML编辑Modal
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editYaml, setEditYaml] = useState('');
  const [editingService, setEditingService] = useState<Service | null>(null);
  const [saveLoading, setSaveLoading] = useState(false);
  const [editMode, setEditMode] = useState<'form' | 'yaml'>('yaml');
  const [editForm] = Form.useForm();

  // 创建Modal
  const [createModalVisible, setCreateModalVisible] = useState(false);
  
  // 命名空间列表
  const [namespaces, setNamespaces] = useState<{ name: string; count: number }[]>([]);

  // 添加搜索条件
  const addSearchCondition = () => {
    if (!currentSearchValue.trim()) return;
    
    const newCondition: SearchCondition = {
      field: currentSearchField,
      value: currentSearchValue.trim(),
    };
    
    setSearchConditions([...searchConditions, newCondition]);
    setCurrentSearchValue('');
  };

  // 删除搜索条件
  const removeSearchCondition = (index: number) => {
    setSearchConditions(searchConditions.filter((_, i) => i !== index));
  };

  // 清空所有搜索条件
  const clearAllConditions = () => {
    setSearchConditions([]);
    setCurrentSearchValue('');
  };

  // 获取搜索字段的显示名称
  const getFieldLabel = (field: string): string => {
    const labels: Record<string, string> = {
      name: t('network:service.search.name'),
      namespace: t('network:service.search.namespace'),
      type: t('network:service.search.type'),
      clusterIP: t('network:service.search.clusterIP'),
      selector: t('network:service.search.selector'),
    };
    return labels[field] || field;
  };

  // 客户端过滤服务列表
  const filterServices = useCallback((items: Service[]): Service[] => {
    if (searchConditions.length === 0) return items;

    return items.filter(service => {
      // 按字段分组条件
      const conditionsByField = searchConditions.reduce((acc, condition) => {
        if (!acc[condition.field]) {
          acc[condition.field] = [];
        }
        acc[condition.field].push(condition.value.toLowerCase());
        return acc;
      }, {} as Record<string, string[]>);

      // 不同字段之间是 AND 关系
      // 相同字段之间是 OR 关系
      return Object.entries(conditionsByField).every(([field, values]) => {
        let serviceValue: string | number | boolean | undefined;
        
        if (field === 'selector') {
          serviceValue = ServiceService.formatSelector(service.selector);
        } else {
          const value = service[field as keyof Service];
          // 处理复杂类型（对象、数组等）
          if (typeof value === 'object' && value !== null) {
            serviceValue = JSON.stringify(value);
          } else {
            serviceValue = value as string | number | boolean | undefined;
          }
        }
        
        const itemStr = String(serviceValue || '').toLowerCase();
        return values.some(searchValue => itemStr.includes(searchValue));
      });
    });
  }, [searchConditions]);

  // 加载命名空间列表
  useEffect(() => {
    const loadNamespaces = async () => {
      if (!clusterId) return;
      try {
        const nsList = await ServiceService.getServiceNamespaces(clusterId);
        setNamespaces(nsList);
      } catch (error) {
        console.error('加载命名空间失败:', error);
      }
    };

    loadNamespaces();
  }, [clusterId]);

  // 获取Service列表（获取所有数据，不分页）
  const loadServices = useCallback(async () => {
    if (!clusterId) return;
    
    setLoading(true);
    try {
      const response = await ServiceService.getServices(
        clusterId,
        '_all_',
        '',
        undefined,
        1,
        10000 // 获取所有数据
      );
      
      if (response.code === 200) {
        const items = response.data.items || [];
        // 保存原始数据，筛选和分页会在useEffect中自动处理
        setAllServices(items);
      } else {
        message.error(response.message || t('network:service.messages.fetchError'));
      }
    } catch (error) {
      console.error('Failed to fetch Service list:', error);
      message.error(t('network:service.messages.fetchError'));
    } finally {
      setLoading(false);
    }
  }, [clusterId, message]);

  // 当搜索条件改变时重置到第一页
  useEffect(() => {
    setCurrentPage(1);
  }, [searchConditions]);

  // 当allServices、搜索条件、分页参数、排序参数改变时，重新计算显示数据
  useEffect(() => {
    if (allServices.length === 0) {
      setServices([]);
      setTotal(0);
      onCountChange?.(0);
      return;
    }
    
    // 1. 应用客户端过滤
    let filteredItems = filterServices(allServices);
    
    // 2. 应用排序
    if (sortField && sortOrder) {
      filteredItems = [...filteredItems].sort((a, b) => {
        const aValue = a[sortField as keyof Service];
        const bValue = b[sortField as keyof Service];
        
        // 处理 undefined 值
        if (aValue === undefined && bValue === undefined) return 0;
        if (aValue === undefined) return sortOrder === 'ascend' ? 1 : -1;
        if (bValue === undefined) return sortOrder === 'ascend' ? -1 : 1;
        
        // 字符串类型比较
        const aStr = String(aValue);
        const bStr = String(bValue);
        
        if (sortOrder === 'ascend') {
          return aStr > bStr ? 1 : aStr < bStr ? -1 : 0;
        } else {
          return bStr > aStr ? 1 : bStr < aStr ? -1 : 0;
        }
      });
    }
    
    // 3. 计算分页
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    const paginatedItems = filteredItems.slice(startIndex, endIndex);
    
    setServices(paginatedItems);
    setTotal(filteredItems.length);
    onCountChange?.(filteredItems.length);
  }, [allServices, filterServices, currentPage, pageSize, sortField, sortOrder, onCountChange]);

  // 初始加载数据
  useEffect(() => {
    loadServices();
  }, [loadServices]);

  // 查看YAML
  const handleViewYAML = async (service: Service) => {
    setYamlModalVisible(true);
    setYamlLoading(true);
    try {
      const response = await ServiceService.getServiceYAML(
        clusterId,
        service.namespace,
        service.name
      );
      
      if (response.code === 200) {
        setCurrentYaml(response.data.yaml);
      } else {
        message.error(response.message || t('network:service.messages.fetchYAMLError'));
      }
    } catch (error) {
      console.error('Failed to fetch YAML:', error);
      message.error(t('network:service.messages.fetchYAMLError'));
    } finally {
      setYamlLoading(false);
    }
  };

  // 查看Endpoints
  const handleViewEndpoints = async (service: Service) => {
    setEndpointsModalVisible(true);
    setEndpointsLoading(true);
    try {
      const response = await ServiceService.getServiceEndpoints(
        clusterId,
        service.namespace,
        service.name
      );
      
      if (response.code === 200) {
        setCurrentEndpoints(response.data);
      } else {
        message.error(response.message || t('network:service.messages.fetchEndpointsError'));
      }
    } catch (error) {
      console.error('Failed to fetch Endpoints:', error);
      message.error(t('network:service.messages.fetchEndpointsError'));
    } finally {
      setEndpointsLoading(false);
    }
  };

  // 删除Service
  const handleDelete = async (service: Service) => {
    try {
      const response = await ServiceService.deleteService(
        clusterId,
        service.namespace,
        service.name
      );
      
      if (response.code === 200) {
        message.success(t('common:messages.deleteSuccess'));
        loadServices();
      } else {
        message.error(response.message || t('network:service.messages.deleteError'));
      }
    } catch (error) {
      console.error('Failed to delete:', error);
      message.error(t('common:messages.deleteError'));
    }
  };

  // 批量删除
  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      message.warning(t('network:service.messages.selectDelete'));
      return;
    }

    Modal.confirm({
      title: t('common:messages.confirmDelete'),
      content: t('network:service.messages.confirmDeleteBatch', { count: selectedRowKeys.length }),
      okText: t('common:actions.confirm'),
      cancelText: t('common:actions.cancel'),
      onOk: async () => {
        try {
          const selectedServices = services.filter(s => 
            selectedRowKeys.includes(`${s.namespace}/${s.name}`)
          );
          
          const deletePromises = selectedServices.map(service =>
            ServiceService.deleteService(clusterId, service.namespace, service.name)
          );
          
          const results = await Promise.allSettled(deletePromises);
          const successCount = results.filter(r => r.status === 'fulfilled').length;
          const failCount = results.length - successCount;
          
          if (failCount === 0) {
            message.success(t('network:service.messages.batchDeleteSuccess', { count: successCount }));
          } else {
            message.warning(t('network:service.messages.batchDeletePartial', { success: successCount, fail: failCount }));
          }
          
          setSelectedRowKeys([]);
          loadServices();
        } catch (error) {
          console.error('Batch delete failed:', error);
          message.error(t('network:service.messages.batchDeleteError'));
        }
      }
    });
  };

  // 导出功能（导出所有筛选后的数据，包含所有列）
  const handleExport = () => {
    try {
      // 获取所有筛选后的数据（不限于当前页）
      const filteredData = filterServices(allServices);
      
      if (filteredData.length === 0) {
        message.warning(t('common:messages.noExportData'));
        return;
      }

      // 导出筛选后的所有数据（包含所有列）
      const dataToExport = filteredData.map(s => ({
        [t('network:service.export.name')]: s.name,
        [t('network:service.export.namespace')]: s.namespace,
        [t('network:service.export.type')]: s.type,
        'ClusterIP': s.clusterIP || '-',
        [t('network:service.export.ports')]: ServiceService.formatPorts(s),
        [t('network:service.export.selector')]: ServiceService.formatSelector(s.selector),
        [t('network:service.export.createdAt')]: s.createdAt ? new Date(s.createdAt).toLocaleString(undefined, {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        }).replace(/\//g, '-') : '-',
      }));

      // 导出为CSV
      const headers = Object.keys(dataToExport[0]);
      const csvContent = [
        headers.join(','),
        ...dataToExport.map(row => 
          headers.map(header => {
            const value = row[header as keyof typeof row];
            return `"${value}"`;
          }).join(',')
        )
      ].join('\n');
      
      const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `service-list-${Date.now()}.csv`;
      link.click();
      message.success(t('common:messages.exportCount', { count: filteredData.length }));
    } catch (error) {
      console.error('Export failed:', error);
      message.error(t('common:messages.exportError'));
    }
  };

  // 列设置保存
  const handleColumnSettingsSave = () => {
    setColumnSettingsVisible(false);
    message.success(t('common:messages.columnSettingsSaved'));
  };

  // 编辑Service - 跳转到独立的编辑页面
  const handleEdit = (service: Service) => {
    navigate(`/clusters/${clusterId}/network/service/${service.namespace}/${service.name}/edit`);
  };

  // 保存编辑
  const handleSaveEdit = async () => {
    if (!editingService) return;

    setSaveLoading(true);
    try {
      if (editMode === 'yaml') {
        // YAML方式更新
        const response = await ServiceService.updateService(
          clusterId,
          editingService.namespace,
          editingService.name,
          {
            namespace: editingService.namespace,
            yaml: editYaml,
          }
        );
        
        if (response.code === 200) {
          message.success(t('common:messages.saveSuccess'));
          setEditModalVisible(false);
          setEditYaml('');
          setEditingService(null);
          setEditMode('yaml');
          loadServices();
        } else {
          message.error(response.message || t('network:service.messages.updateError'));
        }
      } else {
        // 表单方式更新
        const values = await editForm.validateFields();
        
        // 构建Service YAML
        const serviceYaml: KubernetesServiceYAML = {
          apiVersion: 'v1',
          kind: 'Service',
          metadata: {
            name: values.name,
            namespace: values.namespace,
            labels: {},
            annotations: {},
          },
          spec: {
            type: values.type,
            selector: {},
            ports: values.ports || [],
            sessionAffinity: values.sessionAffinity || 'None',
          },
        };

        // 添加labels
        if (values.labels && Array.isArray(values.labels) && values.labels.length > 0) {
          (values.labels as LabelItem[]).forEach((label) => {
            if (label && label.key) {
              serviceYaml.metadata.labels[label.key] = label.value || '';
            }
          });
        }

        // 添加annotations
        if (values.annotations && Array.isArray(values.annotations) && values.annotations.length > 0) {
          (values.annotations as LabelItem[]).forEach((annotation) => {
            if (annotation && annotation.key) {
              serviceYaml.metadata.annotations[annotation.key] = annotation.value || '';
            }
          });
        }

        // 添加selectors
        if (values.selectors && Array.isArray(values.selectors) && values.selectors.length > 0) {
          (values.selectors as LabelItem[]).forEach((selector) => {
            if (selector && selector.key) {
              serviceYaml.spec.selector![selector.key] = selector.value || '';
            }
          });
        }

        const yamlString = YAML.stringify(serviceYaml);
        
        const response = await ServiceService.updateService(
          clusterId,
          values.namespace,
          values.name,
          {
            namespace: values.namespace,
            yaml: yamlString,
          }
        );
        
        if (response.code === 200) {
          message.success(t('common:messages.saveSuccess'));
          setEditModalVisible(false);
          setEditYaml('');
          setEditingService(null);
          setEditMode('yaml');
          editForm.resetFields();
          loadServices();
        } else {
          message.error(response.message || t('network:service.messages.updateError'));
        }
      }
    } catch (error) {
      console.error('Failed to update:', error);
      message.error(t('common:messages.saveError'));
    } finally {
      setSaveLoading(false);
    }
  };

  // 行选择配置
  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => {
      setSelectedRowKeys(keys as string[]);
    },
  };

  // 定义所有可用列
  const allColumns: ColumnsType<Service> = [
    {
      title: t('network:service.columns.name'),
      dataIndex: 'name',
      key: 'name',
      fixed: 'left' as const,
      width: 200,
      sorter: true,
      sortOrder: sortField === 'name' ? sortOrder : null,
      render: (name: string, record: Service) => (
        <div>
          <Link strong onClick={() => handleViewYAML(record)}>
            {name}
          </Link>
          <div style={{ fontSize: 12, color: '#999' }}>
            {record.namespace}
          </div>
        </div>
      ),
    },
    {
      title: t('common:table.namespace'),
      dataIndex: 'namespace',
      key: 'namespace',
      width: 130,
      sorter: true,
      sortOrder: sortField === 'namespace' ? sortOrder : null,
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('common:table.type'),
      dataIndex: 'type',
      key: 'type',
      width: 150,
      render: (type: string) => (
        <Tag color={ServiceService.getTypeColor(type)}>
          {ServiceService.getTypeTag(type)}
        </Tag>
      ),
    },
    {
      title: t('network:service.columns.access'),
      key: 'access',
      width: 200,
      render: (_: unknown, record: Service) => {
        const addresses = ServiceService.formatAccessAddress(record);
        return (
          <div>
            {addresses.slice(0, 2).map((addr, idx) => (
              <div key={idx} style={{ fontSize: 12 }}>
                {addr}
              </div>
            ))}
            {addresses.length > 2 && (
              <Tooltip title={addresses.slice(2).join(', ')}>
                <Text type="secondary" style={{ fontSize: 12, cursor: 'pointer' }}>
                  +{addresses.length - 2} {t('network:service.columns.more')}
                </Text>
              </Tooltip>
            )}
          </div>
        );
      },
    },
    {
      title: t('network:service.columns.ports'),
      key: 'ports',
      width: 180,
      render: (_: unknown, record: Service) => (
        <Tooltip title={ServiceService.formatPorts(record)}>
          <Text ellipsis style={{ width: 160, display: 'block' }}>
            {ServiceService.formatPorts(record)}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('network:service.columns.selector'),
      key: 'selector',
      width: 200,
      render: (_: unknown, record: Service) => (
        <Tooltip title={ServiceService.formatSelector(record.selector)}>
          <Text ellipsis style={{ width: 180, display: 'block' }}>
            {ServiceService.formatSelector(record.selector)}
          </Text>
        </Tooltip>
      ),
    },
    {
      title: t('common:table.createdAt'),
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      sorter: true,
      sortOrder: sortField === 'createdAt' ? sortOrder : null,
      render: (createdAt: string) => {
        if (!createdAt) return '-';
        const date = new Date(createdAt);
        const formatted = date.toLocaleString('zh-CN', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        }).replace(/\//g, '-');
        return <span>{formatted}</span>;
      },
    },
    {
      title: t('common:table.actions'),
      key: 'action',
      fixed: 'right' as const,
      width: 180,
      render: (_: unknown, record: Service) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            onClick={() => handleViewYAML(record)}
          >
            YAML
          </Button>
          <Button
            type="link"
            size="small"
            onClick={() => handleEdit(record)}
          >
            {t('common:actions.edit')}
          </Button>
          <Button
            type="link"
            size="small"
            onClick={() => handleViewEndpoints(record)}
          >
            Endpoints
          </Button>
          <Popconfirm
            title={t('network:service.messages.confirmDeleteItem')}
            description={t('network:service.messages.confirmDeleteDesc', { name: record.name })}
            onConfirm={() => handleDelete(record)}
            okText={t('common:actions.confirm')}
            cancelText={t('common:actions.cancel')}
          >
            <Button
              type="link"
              size="small"
              danger
            >
              {t('common:actions.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // 根据可见性过滤列
  const columns = allColumns.filter(col => {
    if (col.key === 'action') return true; // 操作列始终显示
    if (col.key === 'name') return true; // 名称列始终显示
    return visibleColumns.includes(col.key as string);
  });

  // 表格排序处理
  const handleTableChange = (
    _pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter: SorterResult<Service> | SorterResult<Service>[]
  ) => {
    const singleSorter = Array.isArray(sorter) ? sorter[0] : sorter;
    
    if (singleSorter && singleSorter.field) {
      const fieldName = String(singleSorter.field);
      setSortField(fieldName);
      setSortOrder(singleSorter.order || null);
    } else {
      setSortField('');
      setSortOrder(null);
    }
  };

  return (
    <div>
      {/* 操作按钮栏 */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Space>
          <Button
            disabled={selectedRowKeys.length === 0}
            onClick={handleBatchDelete}
            danger
          >
            {t('common:actions.batchDelete')}
          </Button>
          <Button onClick={handleExport}>
            {t('common:actions.export')}
          </Button>
        </Space>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setCreateModalVisible(true)}
        >
          {t('network:service.createService')}
        </Button>
      </div>

      {/* 多条件搜索栏 */}
      <div style={{ marginBottom: 16 }}>
        {/* 搜索输入框 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: 8 }}>
          <Input
            prefix={<SearchOutlined />}
            placeholder={t('common:search.placeholder')}
            style={{ flex: 1 }}
            value={currentSearchValue}
            onChange={(e) => setCurrentSearchValue(e.target.value)}
            onPressEnter={addSearchCondition}
            allowClear
            addonBefore={
              <Select 
                value={currentSearchField} 
                onChange={setCurrentSearchField} 
                style={{ width: 120 }}
              >
                <Select.Option value="name">{t('network:service.search.name')}</Select.Option>
                <Select.Option value="namespace">{t('network:service.search.namespace')}</Select.Option>
                <Select.Option value="type">{t('network:service.search.type')}</Select.Option>
                <Select.Option value="clusterIP">{t('network:service.search.clusterIP')}</Select.Option>
                <Select.Option value="selector">{t('network:service.search.selector')}</Select.Option>
              </Select>
            }
          />
          <Button
            icon={<ReloadOutlined />}
            onClick={() => {
              loadServices();
            }}
          >
          </Button>
          <Button icon={<SettingOutlined />} onClick={() => setColumnSettingsVisible(true)} />
        </div>

        {/* 搜索条件标签 */}
        {searchConditions.length > 0 && (
          <div>
            <Space size="small" wrap>
              {searchConditions.map((condition, index) => (
                <Tag
                  key={index}
                  closable
                  onClose={() => removeSearchCondition(index)}
                  color="blue"
                >
                  {getFieldLabel(condition.field)}: {condition.value}
                </Tag>
              ))}
              <Button
                size="small"
                type="link"
                onClick={clearAllConditions}
                style={{ padding: 0 }}
              >
                {t('common:actions.clearAll')}
              </Button>
            </Space>
          </div>
        )}
      </div>

      <Table
        columns={columns}
        dataSource={services}
        rowKey={(record) => `${record.namespace}/${record.name}`}
        rowSelection={rowSelection}
        loading={loading}
        scroll={{ x: 1200 }}
        size="middle"
        onChange={handleTableChange}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => t('network:service.pagination.total', { total }),
          onChange: (page, size) => {
            setCurrentPage(page);
            setPageSize(size || 20);
          },
          pageSizeOptions: ['10', '20', '50', '100'],
        }}
      />

      {/* YAML查看Modal */}
      <Modal
        title="Service YAML"
        open={yamlModalVisible}
        onCancel={() => setYamlModalVisible(false)}
        footer={null}
        width={800}
      >
        {yamlLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <span>{t('common:messages.loading')}</span>
          </div>
        ) : (
          <pre style={{ maxHeight: 600, overflow: 'auto', background: '#f5f5f5', padding: 16 }}>
            {currentYaml}
          </pre>
        )}
      </Modal>

      {/* Endpoints查看Modal */}
      <Modal
        title="Service Endpoints"
        open={endpointsModalVisible}
        onCancel={() => setEndpointsModalVisible(false)}
        footer={null}
        width={800}
      >
        {endpointsLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <span>{t('common:messages.loading')}</span>
          </div>
        ) : currentEndpoints ? (
          <Descriptions column={1} bordered>
            <Descriptions.Item label={t('network:service.endpoints.name')}>{currentEndpoints.name}</Descriptions.Item>
            <Descriptions.Item label={t('network:service.endpoints.namespace')}>{currentEndpoints.namespace}</Descriptions.Item>
            <Descriptions.Item label={t('network:service.endpoints.subsets')}>
              {currentEndpoints.subsets && currentEndpoints.subsets.length > 0 ? (
                currentEndpoints.subsets.map((subset: { addresses?: Array<{ ip: string; nodeName?: string }>; ports?: Array<{ name?: string; port: number; protocol: string }> }, idx: number) => (
                  <div key={idx} style={{ marginBottom: 16 }}>
                    <Text strong>{t('network:service.endpoints.addresses')}:</Text>
                    {subset.addresses?.map((addr, addrIdx: number) => (
                      <div key={addrIdx} style={{ marginLeft: 16 }}>
                        {addr.ip} {addr.nodeName && `(${t('network:service.endpoints.node')}: ${addr.nodeName})`}
                      </div>
                    ))}
                    <Text strong style={{ marginTop: 8, display: 'block' }}>{t('network:service.endpoints.ports')}:</Text>
                    {subset.ports?.map((port, portIdx: number) => (
                      <div key={portIdx} style={{ marginLeft: 16 }}>
                        {port.name && `${port.name}: `}{port.port}/{port.protocol}
                      </div>
                    ))}
                  </div>
                ))
              ) : (
                <Text type="secondary">{t('network:service.endpoints.none')}</Text>
              )}
            </Descriptions.Item>
          </Descriptions>
        ) : (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Text type="secondary">{t('network:service.messages.noEndpoints')}</Text>
          </div>
        )}
      </Modal>

      {/* 创建Service Modal */}
      <ServiceCreateModal
        visible={createModalVisible}
        clusterId={clusterId}
        onClose={() => setCreateModalVisible(false)}
        onSuccess={() => loadServices()}
      />

      {/* 编辑Modal */}
      <Modal
        title={t('network:service.edit.title', { name: editingService?.name })}
        open={editModalVisible}
        onCancel={() => {
          setEditModalVisible(false);
          setEditYaml('');
          setEditingService(null);
          setEditMode('yaml');
          editForm.resetFields();
        }}
        onOk={handleSaveEdit}
        confirmLoading={saveLoading}
        width={1000}
        okText={t('common:actions.save')}
        cancelText={t('common:actions.cancel')}
      >
        <Tabs activeKey={editMode} onChange={(key) => setEditMode(key as 'form' | 'yaml')}>
          <Tabs.TabPane tab={t('network:service.edit.formTab')} key="form">
            <Form form={editForm} layout="vertical">
              <Form.Item label={t('network:service.edit.name')} name="name" rules={[{ required: true, message: t('network:service.edit.nameRequired') }]}>
                <Input disabled placeholder={t('network:service.edit.namePlaceholder')} />
              </Form.Item>
              
              <Form.Item label={t('network:service.edit.namespace')} name="namespace" rules={[{ required: true, message: t('network:service.edit.namespaceRequired') }]}>
                <Select disabled placeholder={t('network:service.edit.namespacePlaceholder')}>
                  {namespaces.map((ns) => (
                    <Select.Option key={ns.name} value={ns.name}>
                      {ns.name}
                    </Select.Option>
                  ))}
                </Select>
              </Form.Item>
              
              <Form.Item label={t('network:service.edit.type')} name="type" rules={[{ required: true }]}>
                <Select>
                  <Select.Option value="ClusterIP">ClusterIP</Select.Option>
                  <Select.Option value="NodePort">NodePort</Select.Option>
                  <Select.Option value="LoadBalancer">LoadBalancer</Select.Option>
                  <Select.Option value="ExternalName">ExternalName</Select.Option>
                </Select>
              </Form.Item>
              
              <Form.Item label={t('network:service.edit.selector')}>
                <Form.List name="selectors">
                  {(fields, { add, remove }) => (
                    <>
                      {fields.map((field) => (
                        <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }}>
                          <Form.Item {...field} name={[field.name, 'key']} noStyle>
                            <Input placeholder={t('network:service.edit.key')} style={{ width: 150 }} />
                          </Form.Item>
                          <Form.Item {...field} name={[field.name, 'value']} noStyle>
                            <Input placeholder={t('network:service.edit.value')} style={{ width: 150 }} />
                          </Form.Item>
                          <MinusCircleOutlined onClick={() => remove(field.name)} />
                        </Space>
                      ))}
                      <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                        {t('network:service.edit.addSelector')}
                      </Button>
                    </>
                  )}
                </Form.List>
              </Form.Item>
              
              <Form.Item label={t('network:service.edit.port')}>
                <Form.List name="ports">
                  {(fields, { add, remove }) => (
                    <>
                      {fields.map((field) => (
                        <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                          <Form.Item {...field} name={[field.name, 'name']} noStyle>
                            <Input placeholder={t('network:service.edit.portName')} style={{ width: 100 }} />
                          </Form.Item>
                          <Form.Item {...field} name={[field.name, 'protocol']} noStyle initialValue="TCP">
                            <Select style={{ width: 80 }}>
                              <Select.Option value="TCP">TCP</Select.Option>
                              <Select.Option value="UDP">UDP</Select.Option>
                            </Select>
                          </Form.Item>
                          <Form.Item {...field} name={[field.name, 'port']} noStyle>
                            <InputNumber placeholder={t('network:service.edit.portNumber')} min={1} max={65535} style={{ width: 100 }} />
                          </Form.Item>
                          <Form.Item {...field} name={[field.name, 'targetPort']} noStyle>
                            <InputNumber placeholder={t('network:service.edit.targetPort')} min={1} max={65535} style={{ width: 100 }} />
                          </Form.Item>
                          <MinusCircleOutlined onClick={() => remove(field.name)} />
                        </Space>
                      ))}
                      <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                        {t('network:service.edit.addPort')}
                      </Button>
                    </>
                  )}
                </Form.List>
              </Form.Item>
              
              <Form.Item label={t('network:service.edit.sessionAffinity')} name="sessionAffinity" initialValue="None">
                <Select>
                  <Select.Option value="None">None</Select.Option>
                  <Select.Option value="ClientIP">ClientIP</Select.Option>
                </Select>
              </Form.Item>
              
              <Form.Item label={t('network:service.edit.labels')}>
                <Form.List name="labels">
                  {(fields, { add, remove }) => (
                    <>
                      {fields.map((field) => (
                        <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }}>
                          <Form.Item {...field} name={[field.name, 'key']} noStyle>
                            <Input placeholder={t('network:service.edit.key')} style={{ width: 150 }} />
                          </Form.Item>
                          <Form.Item {...field} name={[field.name, 'value']} noStyle>
                            <Input placeholder={t('network:service.edit.value')} style={{ width: 150 }} />
                          </Form.Item>
                          <MinusCircleOutlined onClick={() => remove(field.name)} />
                        </Space>
                      ))}
                      <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                        {t('network:service.edit.addLabel')}
                      </Button>
                    </>
                  )}
                </Form.List>
              </Form.Item>
              
              <Form.Item label={t('network:service.edit.annotations')}>
                <Form.List name="annotations">
                  {(fields, { add, remove }) => (
                    <>
                      {fields.map((field) => (
                        <Space key={field.key} style={{ display: 'flex', marginBottom: 8 }}>
                          <Form.Item {...field} name={[field.name, 'key']} noStyle>
                            <Input placeholder={t('network:service.edit.key')} style={{ width: 150 }} />
                          </Form.Item>
                          <Form.Item {...field} name={[field.name, 'value']} noStyle>
                            <Input placeholder={t('network:service.edit.value')} style={{ width: 150 }} />
                          </Form.Item>
                          <MinusCircleOutlined onClick={() => remove(field.name)} />
                        </Space>
                      ))}
                      <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                        {t('network:service.edit.addAnnotation')}
                      </Button>
                    </>
                  )}
                </Form.List>
              </Form.Item>
            </Form>
          </Tabs.TabPane>
          
          <Tabs.TabPane tab={t('network:service.edit.yamlTab')} key="yaml">
            <MonacoEditor
              height="600px"
              language="yaml"
              value={editYaml}
              onChange={(value) => setEditYaml(value || '')}
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                wordWrap: 'on',
                scrollBeyondLastLine: false,
              }}
            />
          </Tabs.TabPane>
        </Tabs>
      </Modal>

      {/* 列设置抽屉 */}
      <Drawer
        title={t('common:search.columnSettings')}
        placement="right"
        width={400}
        open={columnSettingsVisible}
        onClose={() => setColumnSettingsVisible(false)}
        footer={
          <div style={{ textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setColumnSettingsVisible(false)}>{t('common:actions.cancel')}</Button>
              <Button type="primary" onClick={handleColumnSettingsSave}>{t('common:actions.confirm')}</Button>
            </Space>
          </div>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8, color: '#666' }}>{t('common:search.selectColumns')}</p>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Checkbox
              checked={visibleColumns.includes('namespace')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'namespace']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'namespace'));
                }
              }}
            >
              {t('network:service.columnSettings.namespace')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('type')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'type']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'type'));
                }
              }}
            >
              {t('network:service.columnSettings.type')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('access')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'access']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'access'));
                }
              }}
            >
              {t('network:service.columnSettings.access')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('ports')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'ports']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'ports'));
                }
              }}
            >
              {t('network:service.columnSettings.ports')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('selector')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'selector']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'selector'));
                }
              }}
            >
              {t('network:service.columnSettings.selector')}
            </Checkbox>
            <Checkbox
              checked={visibleColumns.includes('createdAt')}
              onChange={(e) => {
                if (e.target.checked) {
                  setVisibleColumns([...visibleColumns, 'createdAt']);
                } else {
                  setVisibleColumns(visibleColumns.filter(c => c !== 'createdAt'));
                }
              }}
            >
              {t('network:service.columnSettings.createdAt')}
            </Checkbox>
          </Space>
        </div>
      </Drawer>
    </div>
  );
};

export default ServiceTab;

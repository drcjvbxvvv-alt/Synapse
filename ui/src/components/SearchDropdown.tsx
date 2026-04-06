import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Input,
  Dropdown,
  List,
  Tag,
  Space,
  Typography,
  Empty,
  Spin,
  Tabs,
} from 'antd';
import type { InputRef } from 'antd';
import {
  SearchOutlined,
  ClusterOutlined,
  DesktopOutlined,
  ContainerOutlined,
  RocketOutlined,
} from '@ant-design/icons';
import type { SearchResult } from '../types';
import { searchService } from '../services/searchService';

const { Text } = Typography;

interface SearchDropdownProps {
  onSearch?: (query: string) => void;
}

const SearchDropdown: React.FC<SearchDropdownProps> = ({ onSearch }) => {
  const navigate = useNavigate();
  const { t } = useTranslation('components');
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [visible, setVisible] = useState(false);
  const [activeTab, setActiveTab] = useState<string>('all');
  const searchRef = useRef<InputRef>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const performSearch = useCallback(async (searchQuery: string) => {
    setLoading(true);
    try {
      const searchResults = await searchService.quickSearch(searchQuery);
      setResults(searchResults);
      setVisible(searchResults.length > 0);
    } catch (error) {
      console.error('Search failed:', error);
      setResults([]);
      setVisible(false);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (query.trim()) {
        performSearch(query);
      } else {
        setResults([]);
        setVisible(false);
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [query, performSearch]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        if (searchRef.current && !searchRef.current.input?.contains(event.target as Node)) {
          setVisible(false);
        }
      }
    };

    if (visible) {
      document.addEventListener('mousedown', handleClickOutside);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [visible]);

  // 處理搜尋
  const handleSearch = (value: string) => {
    if (value.trim()) {
      navigate(`/search?q=${encodeURIComponent(value)}`);
      setVisible(false);
      if (onSearch) {
        onSearch(value);
      }
    }
  };

  // 獲取資源型別圖示
  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'cluster':
        return <ClusterOutlined style={{ color: '#1890ff' }} />;
      case 'node':
        return <DesktopOutlined style={{ color: '#52c41a' }} />;
      case 'pod':
        return <ContainerOutlined style={{ color: '#faad14' }} />;
      case 'workload':
        return <RocketOutlined style={{ color: '#722ed1' }} />;
      default:
        return <SearchOutlined />;
    }
  };

  // 獲取狀態標籤顏色
  const getStatusColor = (status: string, type: string) => {
    switch (type) {
      case 'cluster':
        return status === 'healthy' ? 'green' : 'red';
      case 'node':
        return status === 'Ready' ? 'green' : 'red';
      case 'pod':
        return status === 'Running' ? 'green' : 'orange';
      case 'workload':
        return 'blue';
      default:
        return 'default';
    }
  };

  // 處理資源點選
  const handleResourceClick = (result: SearchResult) => {
    setVisible(false);
    switch (result.type) {
      case 'cluster':
        navigate(`/clusters/${result.clusterId}/overview`);
        break;
      case 'node':
        navigate(`/clusters/${result.clusterId}/nodes/${result.name}`);
        break;
      case 'pod':
        navigate(`/clusters/${result.clusterId}/pods/${result.namespace}/${result.name}`);
        break;
      case 'workload':
        navigate(`/clusters/${result.clusterId}/workloads/${result.namespace}/${result.name}?type=${result.kind}`);
        break;
    }
  };

  // 獲取資源型別統計
  const getTypeStats = () => {
    const stats = {
      cluster: 0,
      node: 0,
      pod: 0,
      workload: 0,
    };
    
    results.forEach(result => {
      stats[result.type]++;
    });
    
    return stats;
  };

  // 按型別過濾結果
  const getFilteredResults = (type: string) => {
    if (type === 'all') {
      return results;
    }
    return results.filter(result => result.type === type);
  };

  const stats = getTypeStats();

  // 渲染搜尋結果列表
  const renderResultsList = (filteredResults: SearchResult[]) => (
    <List
      size="small"
      dataSource={filteredResults}
      style={{ backgroundColor: '#ffffff' }}
      renderItem={(item: SearchResult) => (
        <List.Item
          style={{ 
            padding: '12px 20px', 
            cursor: 'pointer',
            backgroundColor: '#ffffff',
            borderBottom: '1px solid #f0f0f0'
          }}
          onClick={() => handleResourceClick(item)}
          onMouseEnter={(e) => {
            e.currentTarget.style.backgroundColor = '#f5f5f5';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = '#ffffff';
          }}
        >
          <List.Item.Meta
            avatar={
              <div style={{ 
                width: '32px', 
                height: '32px', 
                display: 'flex', 
                alignItems: 'center', 
                justifyContent: 'center',
                backgroundColor: '#f0f0f0',
                borderRadius: '6px'
              }}>
                {getTypeIcon(item.type)}
              </div>
            }
            title={
              <Space>
                <Text strong style={{ fontSize: '15px', color: '#262626' }}>{item.name}</Text>
                <Tag color={getStatusColor(item.status, item.type)}>
                  {item.status}
                </Tag>
                {item.kind && <Tag color="blue">{item.kind}</Tag>}
              </Space>
            }
            description={
              <Space direction="vertical" size="small" style={{ marginTop: '4px' }}>
                <Text style={{ fontSize: '13px', color: '#595959' }}>
                  {item.type === 'cluster' && `API Server: ${item.description}`}
                  {item.type === 'node' && `Pod CIDR: ${item.description}`}
                  {item.type === 'pod' && `${t('searchDropdown.nodeLabel')}: ${item.description}`}
                  {item.type === 'workload' && `${t('searchDropdown.replicas')}: ${item.description}`}
                </Text>
                <Space>
                  <Text style={{ fontSize: '12px', color: '#8c8c8c' }}>
                    {t('searchDropdown.clusterLabel')}: {item.clusterName}
                  </Text>
                  {item.namespace && (
                    <Text style={{ fontSize: '12px', color: '#8c8c8c' }}>
                      {t('searchDropdown.namespaceLabel')}: {item.namespace}
                    </Text>
                  )}
                  {item.ip && (
                    <Text style={{ fontSize: '12px', color: '#8c8c8c' }}>
                      IP: {item.ip}
                    </Text>
                  )}
                </Space>
              </Space>
            }
          />
        </List.Item>
      )}
    />
  );

  // 標籤項配置
  const tabItems = [
    {
      key: 'all',
      label: `${t('searchDropdown.all')} (${results.length})`,
      children: renderResultsList(getFilteredResults('all')),
    },
    {
      key: 'cluster',
      label: `${t('searchDropdown.cluster')} (${stats.cluster})`,
      children: renderResultsList(getFilteredResults('cluster')),
    },
    {
      key: 'node',
      label: `${t('searchDropdown.node')} (${stats.node})`,
      children: renderResultsList(getFilteredResults('node')),
    },
    {
      key: 'pod',
      label: `${t('searchDropdown.pod')} (${stats.pod})`,
      children: renderResultsList(getFilteredResults('pod')),
    },
    {
      key: 'workload',
      label: `${t('searchDropdown.workload')} (${stats.workload})`,
      children: renderResultsList(getFilteredResults('workload')),
    },
  ];

  // 下拉選單內容
  const dropdownContent = (
    <div 
      ref={dropdownRef}
      style={{ 
        width: 500, 
        maxHeight: 500, 
        overflow: 'auto',
        backgroundColor: '#ffffff',
        borderRadius: '8px',
        boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
        border: '1px solid #d9d9d9'
      }}
      onClick={(e) => e.stopPropagation()}
    >
      {loading ? (
        <div style={{ 
          padding: '20px', 
          textAlign: 'center',
          backgroundColor: '#ffffff'
        }}>
          <Spin size="small" />
          <div style={{ marginTop: 8, color: '#666' }}>{t('searchDropdown.searching')}</div>
        </div>
      ) : results.length > 0 ? (
        <>
          {/* 使用Tabs元件 */}
          <div onClick={(e) => e.stopPropagation()}>
            <Tabs
              activeKey={activeTab}
              onChange={setActiveTab}
              items={tabItems}
              size="small"
              style={{ margin: 0 }}
              tabBarStyle={{ 
                margin: 0, 
                padding: '8px 16px 0 16px',
                backgroundColor: '#fafafa',
                borderRadius: '8px 8px 0 0'
              }}
            />
          </div>

          {/* 檢視更多 */}
          <div 
            style={{ 
              padding: '12px 20px', 
              borderTop: '1px solid #f0f0f0', 
              textAlign: 'center',
              backgroundColor: '#fafafa',
              borderRadius: '0 0 8px 8px'
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <Text 
              style={{ 
                cursor: 'pointer', 
                fontSize: '13px',
                fontWeight: 'bold',
                color: '#1890ff'
              }}
              onClick={() => handleSearch(query)}
            >
              {t('searchDropdown.viewAllResults')}
            </Text>
          </div>
        </>
      ) : query ? (
        <div style={{ 
          padding: '40px 20px', 
          textAlign: 'center',
          backgroundColor: '#ffffff'
        }}>
          <Empty 
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={t('searchDropdown.noResults')}
            style={{ margin: 0 }}
          />
        </div>
      ) : null}
    </div>
  );

  return (
    <Dropdown
      open={visible}
      onOpenChange={setVisible}
      popupRender={() => dropdownContent}
      placement="bottomLeft"
      trigger={['click']}
    >
      <Input.Search
        ref={searchRef}
        placeholder={t('searchDropdown.placeholder')}
        allowClear
        enterButton={<SearchOutlined />}
        size="middle"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onSearch={handleSearch}
        style={{ 
          width: '100%',
          backgroundColor: '#ffffff',
          border: '1px solid #d9d9d9',
          borderRadius: '6px'
        }}
        onFocus={() => {
          if (results.length > 0) {
            setVisible(true);
          }
        }}
      />
    </Dropdown>
  );
};

export default React.memo(SearchDropdown);

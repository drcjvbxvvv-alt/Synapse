import React, { useState, useEffect } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import {
  Card,
  Button,
  Space,
  message,
  Modal,
  Typography,
  Alert,
  Spin,
  Row,
  Col,
  Switch,
} from 'antd';
import {
  ArrowLeftOutlined,
  SaveOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { Editor } from '@monaco-editor/react';
import { WorkloadService } from '../../services/workloadService';

const { Title, Text } = Typography;

interface YAMLEditorProps {}

const YAMLEditor: React.FC<YAMLEditorProps> = () => {
  const { clusterId } = useParams<{ clusterId: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  
  // 从URL参数获取工作负载信息
  const workloadRef = searchParams.get('workload'); // namespace/name
  const workloadType = searchParams.get('type');
  
  const [yaml, setYaml] = useState('');
  const [originalYaml, setOriginalYaml] = useState('');
  const [loading, setLoading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [dryRun, setDryRun] = useState(true);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewResult, setPreviewResult] = useState<any>(null);

  // 加载现有工作负载的YAML
  const loadWorkloadYAML = async () => {
    if (!clusterId || !workloadRef || !workloadType) return;
    
    const [namespace, name] = workloadRef.split('/');
    if (!namespace || !name) return;
    
    setLoading(true);
    try {
      const response = await WorkloadService.getWorkloadDetail(
        clusterId,
        namespace,
        name,
        workloadType
      );
      
      if (response.code === 200) {
        const yamlContent = JSON.stringify(response.data.raw, null, 2);
        setYaml(yamlContent);
        setOriginalYaml(yamlContent);
      } else {
        message.error(response.message || '加载YAML失败');
      }
    } catch (error) {
      console.error('加载YAML失败:', error);
      message.error('加载YAML失败');
    } finally {
      setLoading(false);
    }
  };

  // 应用YAML
  const handleApply = async (isDryRun = false) => {
    if (!clusterId || !yaml.trim()) {
      message.error('YAML内容不能为空');
      return;
    }
    
    setApplying(true);
    try {
      const response = await WorkloadService.applyYAML(clusterId, yaml, isDryRun);
      
      if (response.code === 200) {
        if (isDryRun) {
          setPreviewResult(response.data);
          setPreviewVisible(true);
          message.success('YAML验证成功');
        } else {
          message.success('YAML应用成功');
          // 更新原始YAML
          setOriginalYaml(yaml);
        }
      } else {
        message.error(response.message || `YAML${isDryRun ? '验证' : '应用'}失败`);
      }
    } catch (error) {
      console.error(`YAML${isDryRun ? '验证' : '应用'}失败:`, error);
      message.error(`YAML${isDryRun ? '验证' : '应用'}失败`);
    } finally {
      setApplying(false);
    }
  };

  // 预览YAML
  const handlePreview = () => {
    handleApply(true);
  };

  // 保存并应用YAML
  const handleSave = () => {
    Modal.confirm({
      title: '确认应用YAML',
      content: '确定要应用这些YAML配置吗？这将更新集群中的资源。',
      okText: '确定',
      cancelText: '取消',
      onOk: () => handleApply(false),
    });
  };

  // 重置YAML
  const handleReset = () => {
    Modal.confirm({
      title: '确认重置',
      content: '确定要重置YAML内容吗？未保存的更改将丢失。',
      okText: '确定',
      cancelText: '取消',
      onOk: () => {
        setYaml(originalYaml);
        message.success('已重置YAML内容');
      },
    });
  };

  // 检查是否有未保存的更改
  const hasUnsavedChanges = yaml !== originalYaml;

  useEffect(() => {
    loadWorkloadYAML();
  }, [clusterId, workloadRef, workloadType]);

  // 页面离开前提醒
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (hasUnsavedChanges) {
        e.preventDefault();
        e.returnValue = '';
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [hasUnsavedChanges]);

  return (
    <div style={{ padding: '24px', height: 'calc(100vh - 64px)' }}>
      {/* 页面头部 */}
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => {
              if (hasUnsavedChanges) {
                Modal.confirm({
                  title: '确认离开',
                  content: '您有未保存的更改，确定要离开吗？',
                  okText: '确定',
                  cancelText: '取消',
                  onOk: () => navigate(-1),
                });
              } else {
                navigate(-1);
              }
            }}
          >
            返回
          </Button>
          <Title level={3} style={{ margin: 0 }}>
            YAML 编辑器
          </Title>
          {workloadRef && (
            <Text type="secondary">
              {workloadType}: {workloadRef}
            </Text>
          )}
          {hasUnsavedChanges && (
            <Text type="warning">• 有未保存的更改</Text>
          )}
        </Space>
        
        <div style={{ marginTop: 16 }}>
          <Space>
            <Button
              type="primary"
              icon={<SaveOutlined />}
              onClick={handleSave}
              loading={applying}
              disabled={!hasUnsavedChanges}
            >
              应用
            </Button>
            
            <Button
              icon={<EyeOutlined />}
              onClick={handlePreview}
              loading={applying}
            >
              预览
            </Button>
            
            <Button
              icon={<ReloadOutlined />}
              onClick={handleReset}
              disabled={!hasUnsavedChanges}
            >
              重置
            </Button>
            
            <div style={{ marginLeft: 16 }}>
              <Space>
                <Text>DryRun模式:</Text>
                <Switch
                  checked={dryRun}
                  onChange={setDryRun}
                  checkedChildren="开"
                  unCheckedChildren="关"
                />
              </Space>
            </div>
          </Space>
        </div>
      </div>

      {/* 提示信息 */}
      {hasUnsavedChanges && (
        <Alert
          message="您有未保存的更改"
          description="请记得保存您的更改，或点击重置按钮恢复原始内容。"
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {/* YAML编辑器 */}
      <Card style={{ height: 'calc(100% - 120px)' }}>
        <Spin spinning={loading} tip="加载YAML中...">
          <div style={{ height: '100%' }}>
            <Editor
              height="100%"
              defaultLanguage="yaml"
              value={yaml}
              onChange={(value) => setYaml(value || '')}
              theme="vs-dark"
              options={{
                minimap: { enabled: true },
                fontSize: 14,
                lineNumbers: 'on',
                roundedSelection: false,
                scrollBeyondLastLine: false,
                automaticLayout: true,
                tabSize: 2,
                insertSpaces: true,
                wordWrap: 'on',
                folding: true,
                foldingStrategy: 'indentation',
                showFoldingControls: 'always',
                bracketPairColorization: { enabled: true },
              }}
            />
          </div>
        </Spin>
      </Card>

      {/* 预览模态框 */}
      <Modal
        title="YAML 预览结果"
        open={previewVisible}
        onCancel={() => setPreviewVisible(false)}
        footer={[
          <Button key="cancel" onClick={() => setPreviewVisible(false)}>
            关闭
          </Button>,
          <Button
            key="apply"
            type="primary"
            onClick={() => {
              setPreviewVisible(false);
              handleSave();
            }}
          >
            确认应用
          </Button>,
        ]}
        width={800}
      >
        {previewResult && (
          <div>
            <Alert
              message="验证成功"
              description="YAML格式正确，可以安全应用到集群。"
              type="success"
              showIcon
              style={{ marginBottom: 16 }}
            />
            <pre style={{
              background: '#f5f5f5',
              padding: '16px',
              borderRadius: '4px',
              overflow: 'auto',
              maxHeight: '400px'
            }}>
              {JSON.stringify(previewResult, null, 2)}
            </pre>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default YAMLEditor;
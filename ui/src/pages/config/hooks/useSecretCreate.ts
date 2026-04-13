/**
 * useSecretCreate — Secret 建立頁面的所有狀態與業務邏輯
 */
import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import * as YAML from 'yaml';
import { secretService, getNamespaces } from '../../../services/configService';
import { showApiError } from '../../../utils/api';

export interface LabelItem       { key: string; value: string; }
export interface AnnotationItem  { key: string; value: string; }
export interface DataItem        { key: string; value: string; visible: boolean; }

const DEFAULT_YAML = `apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: example-secret
  namespace: default
  labels: {}
  annotations: {}
data: {}`;

export function useSecretCreate() {
  const navigate = useNavigate();
  const { clusterId } = useParams<{ clusterId: string }>();
  const { t } = useTranslation(['config', 'common']);
  const { message } = App.useApp();

  // ── Edit mode ─────────────────────────────────────────────────────────────
  const [editMode, setEditMode] = useState<'form' | 'yaml'>('form');

  // ── Form state ────────────────────────────────────────────────────────────
  const [name, setName]                             = useState('');
  const [namespace, setNamespace]                   = useState('default');
  const [secretType, setSecretType]                 = useState('Opaque');
  const [serviceAccountName, setServiceAccountName] = useState('');
  const [labels, setLabels]                         = useState<LabelItem[]>([]);
  const [annotations, setAnnotations]               = useState<AnnotationItem[]>([]);
  const [dataItems, setDataItems]                   = useState<DataItem[]>([]);

  // ── Namespace list ────────────────────────────────────────────────────────
  const [namespaces, setNamespaces]             = useState<string[]>(['default']);
  const [loadingNamespaces, setLoadingNamespaces] = useState(false);

  // ── YAML state ────────────────────────────────────────────────────────────
  const [yamlContent, setYamlContent] = useState(DEFAULT_YAML);

  // ── Submitting ────────────────────────────────────────────────────────────
  const [submitting, setSubmitting] = useState(false);

  // ── Load namespaces on mount ───────────────────────────────────────────────
  const loadNamespaces = async () => {
    if (!clusterId) return;
    setLoadingNamespaces(true);
    try {
      const nsList = await getNamespaces(Number(clusterId));
      setNamespaces(nsList);
      if (nsList.length > 0 && !nsList.includes(namespace)) {
        setNamespace(nsList[0]);
      }
    } catch (error) {
      console.error('載入命名空間失敗:', error);
    } finally {
      setLoadingNamespaces(false);
    }
  };

  // ── Label handlers ────────────────────────────────────────────────────────
  const handleAddLabel    = () => setLabels(prev => [...prev, { key: '', value: '' }]);
  const handleRemoveLabel = (index: number) => setLabels(prev => prev.filter((_, i) => i !== index));
  const handleLabelChange = (index: number, field: 'key' | 'value', value: string) => {
    setLabels(prev => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  };

  // ── Annotation handlers ───────────────────────────────────────────────────
  const handleAddAnnotation    = () => setAnnotations(prev => [...prev, { key: '', value: '' }]);
  const handleRemoveAnnotation = (index: number) => setAnnotations(prev => prev.filter((_, i) => i !== index));
  const handleAnnotationChange = (index: number, field: 'key' | 'value', value: string) => {
    setAnnotations(prev => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  };

  // ── Data item handlers ────────────────────────────────────────────────────
  const handleAddDataItem    = () => setDataItems(prev => [...prev, { key: '', value: '', visible: false }]);
  const handleRemoveDataItem = (index: number) => setDataItems(prev => prev.filter((_, i) => i !== index));
  const handleDataKeyChange  = (index: number, value: string) => {
    setDataItems(prev => {
      const next = [...prev];
      next[index] = { ...next[index], key: value };
      return next;
    });
  };
  const handleDataValueChange = (index: number, value: string | undefined) => {
    setDataItems(prev => {
      const next = [...prev];
      next[index] = { ...next[index], value: value || '' };
      return next;
    });
  };
  const toggleDataVisibility = (index: number) => {
    setDataItems(prev => {
      const next = [...prev];
      next[index] = { ...next[index], visible: !next[index].visible };
      return next;
    });
  };

  // ── Mode conversion ───────────────────────────────────────────────────────
  const formToYaml = (): string => {
    const labelsObj: Record<string, string> = {};
    labels.forEach(l => { if (l.key) labelsObj[l.key] = l.value; });

    const annotationsObj: Record<string, string> = {};
    annotations.forEach(a => { if (a.key) annotationsObj[a.key] = a.value; });

    const dataObj: Record<string, string> = {};
    dataItems.forEach(d => { if (d.key) dataObj[d.key] = d.value; });

    return YAML.stringify({
      apiVersion: 'v1',
      kind: 'Secret',
      type: secretType || 'Opaque',
      metadata: {
        name: name || 'example-secret',
        namespace: namespace || 'default',
        labels: labelsObj,
        annotations: annotationsObj,
      },
      data: dataObj,
    });
  };

  const yamlToForm = (yamlStr: string): boolean => {
    try {
      const obj = YAML.parse(yamlStr);
      setName(obj.metadata?.name || '');
      setNamespace(obj.metadata?.namespace || 'default');
      setSecretType(obj.type || 'Opaque');
      setLabels(Object.entries(obj.metadata?.labels || {}).map(([key, value]) => ({ key, value: String(value) })));
      setAnnotations(Object.entries(obj.metadata?.annotations || {}).map(([key, value]) => ({ key, value: String(value) })));
      setDataItems(Object.entries(obj.data || {}).map(([key, value]) => ({ key, value: String(value), visible: false })));
      return true;
    } catch (error) {
      message.error(t('config:create.messages.yamlFormatError', { error: error instanceof Error ? error.message : t('config:create.messages.unknownError') }));
      return false;
    }
  };

  const handleModeChange = (mode: 'form' | 'yaml') => {
    if (mode === editMode) return;
    if (mode === 'yaml') {
      setYamlContent(formToYaml());
      setEditMode('yaml');
    } else {
      if (yamlToForm(yamlContent)) setEditMode('form');
    }
  };

  // ── Submit ────────────────────────────────────────────────────────────────
  const handleSubmit = async () => {
    if (!clusterId) return;

    let secretName = '';
    let secretNamespace = '';
    let secretTypeValue = '';
    let labelsObj: Record<string, string> = {};
    let annotationsObj: Record<string, string> = {};
    let dataObj: Record<string, string> = {};

    if (editMode === 'yaml') {
      try {
        const obj = YAML.parse(yamlContent);
        secretName      = obj.metadata?.name;
        secretNamespace = obj.metadata?.namespace || 'default';
        secretTypeValue = obj.type || 'Opaque';
        labelsObj       = obj.metadata?.labels || {};
        annotationsObj  = obj.metadata?.annotations || {};
        dataObj         = obj.data || {};
        if (!secretName) {
          message.error(t('config:create.messages.secretNameRequired'));
          return;
        }
      } catch (error) {
        message.error(t('config:create.messages.yamlFormatError', { error: error instanceof Error ? error.message : t('config:create.messages.unknownError') }));
        return;
      }
    } else {
      if (!name) {
        message.error(t('config:create.messages.secretNameRequired'));
        return;
      }

      secretName      = name;
      secretNamespace = namespace;
      secretTypeValue = secretType;

      for (const label of labels) {
        if (label.key) {
          if (labelsObj[label.key]) {
            message.error(t('config:create.messages.labelKeyDuplicate', { key: label.key }));
            return;
          }
          labelsObj[label.key] = label.value;
        }
      }

      for (const annotation of annotations) {
        if (annotation.key) {
          if (annotationsObj[annotation.key]) {
            message.error(t('config:create.messages.annotationKeyDuplicate', { key: annotation.key }));
            return;
          }
          annotationsObj[annotation.key] = annotation.value;
        }
      }

      if (secretTypeValue === 'kubernetes.io/service-account-token') {
        if (!serviceAccountName) {
          message.error('請輸入 ServiceAccount 名稱');
          return;
        }
        annotationsObj['kubernetes.io/service-account.name'] = serviceAccountName;
      }

      for (const item of dataItems) {
        if (!item.key) {
          message.error(t('config:create.messages.dataKeyRequired'));
          return;
        }
        if (dataObj[item.key]) {
          message.error(t('config:create.messages.dataKeyDuplicate', { key: item.key }));
          return;
        }
        dataObj[item.key] = item.value;
      }
    }

    setSubmitting(true);
    try {
      await secretService.createSecret(Number(clusterId), {
        name: secretName,
        namespace: secretNamespace,
        type: secretTypeValue,
        labels: labelsObj,
        annotations: annotationsObj,
        data: dataObj,
      });
      message.success(t('config:create.messages.secretCreateSuccess'));
      navigate(`/clusters/${clusterId}/configs`);
    } catch (error: unknown) {
      showApiError(error, t('config:create.messages.secretCreateError'));
    } finally {
      setSubmitting(false);
    }
  };

  return {
    clusterId,
    t,
    editMode,
    submitting,
    // Form fields
    name, setName,
    namespace, setNamespace,
    secretType, setSecretType,
    serviceAccountName, setServiceAccountName,
    // Lists
    labels,
    annotations,
    dataItems,
    // Namespaces
    namespaces,
    loadingNamespaces,
    // YAML
    yamlContent, setYamlContent,
    // Handlers
    loadNamespaces,
    handleAddLabel, handleRemoveLabel, handleLabelChange,
    handleAddAnnotation, handleRemoveAnnotation, handleAnnotationChange,
    handleAddDataItem, handleRemoveDataItem, handleDataKeyChange, handleDataValueChange, toggleDataVisibility,
    handleModeChange,
    handleSubmit,
  };
}

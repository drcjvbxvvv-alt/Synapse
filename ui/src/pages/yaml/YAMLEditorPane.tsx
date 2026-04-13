import React from 'react';
import { Card, Spin } from 'antd';
import { Editor } from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';

export interface YAMLEditorPaneProps {
  yaml: string;
  loading: boolean;
  editorLoading: boolean;
  onYamlChange: (value: string) => void;
  onEditorWillMount: () => void;
  onEditorDidMount: () => void;
  onEditorValidation: (markers: unknown[]) => void;
}

const YAMLEditorPane: React.FC<YAMLEditorPaneProps> = ({
  yaml,
  loading,
  editorLoading,
  onYamlChange,
  onEditorWillMount,
  onEditorDidMount,
  onEditorValidation,
}) => {
  const { t } = useTranslation(['yaml']);

  return (
    <Card style={{ height: 'calc(100vh - 200px)', minHeight: '500px' }}>
      <Spin
        spinning={loading || editorLoading}
        tip={loading ? t('messages.loadingYaml') : t('messages.initEditor')}
      >
        <div style={{ height: '500px', width: '100%' }}>
          {yaml ? (
            <Editor
              height="500px"
              width="100%"
              defaultLanguage="yaml"
              value={yaml}
              onChange={(value) => onYamlChange(value || '')}
              loading={
                <div style={{ padding: '20px', textAlign: 'center' }}>
                  {t('messages.editorLoading')}
                </div>
              }
              beforeMount={onEditorWillMount}
              onMount={onEditorDidMount}
              onValidate={onEditorValidation}
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
          ) : (
            <div
              style={{
                height: '500px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: '#666',
                fontSize: '16px',
              }}
            >
              {loading ? t('messages.loading') : t('messages.noContent')}
            </div>
          )}
        </div>
      </Spin>
    </Card>
  );
};

export default YAMLEditorPane;

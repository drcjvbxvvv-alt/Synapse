import React from 'react';
import { Button, Space, Select, Switch, Tooltip, Tag, Badge, Card, Input, Empty } from 'antd';
import {
  PlayCircleOutlined,
  PauseCircleOutlined,
  ClearOutlined,
  DownloadOutlined,
  PlusOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import type { LogEntry, LogStreamTarget } from '../../../services/logService';
import { levelColors, levelOptions } from '../constants';

interface StreamTabProps {
  streaming: boolean;
  logs: LogEntry[];
  targets: LogStreamTarget[];
  showTimestamp: boolean;
  setShowTimestamp: (v: boolean) => void;
  showSource: boolean;
  setShowSource: (v: boolean) => void;
  autoScroll: boolean;
  setAutoScroll: (v: boolean) => void;
  levelFilter: string[];
  setLevelFilter: (v: string[]) => void;
  logSearchKeyword: string;
  setLogSearchKeyword: (v: string) => void;
  filteredLogs: LogEntry[];
  logsEndRef: React.RefObject<HTMLDivElement | null>;
  toggleStream: () => void;
  clearLogs: () => void;
  downloadLogs: () => void;
  removeTarget: (index: number) => void;
  openPodSelector: () => void;
  maxLines?: number;
}

// Highlight keyword in text
const highlightKeyword = (text: string, keyword: string) => {
  if (!keyword.trim() || !text) return text;
  const regex = new RegExp(`(${keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
  const parts = text.split(regex);
  return parts.map((part, i) =>
    regex.test(part) ? (
      <span key={i} style={{ backgroundColor: '#ffe58f', color: '#000', padding: '0 2px', borderRadius: 2 }}>
        {part}
      </span>
    ) : (
      part
    )
  );
};

export const StreamTab: React.FC<StreamTabProps> = ({
  streaming,
  targets,
  showTimestamp,
  setShowTimestamp,
  showSource,
  setShowSource,
  autoScroll,
  setAutoScroll,
  levelFilter,
  setLevelFilter,
  logSearchKeyword,
  setLogSearchKeyword,
  filteredLogs,
  logsEndRef,
  toggleStream,
  clearLogs,
  downloadLogs,
  removeTarget,
  openPodSelector,
  maxLines = 1000,
}) => {
  const { t } = useTranslation(['logs', 'common']);

  return (
    <div>
      {/* Toolbar */}
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
        <Space>
          <Button
            type={streaming ? 'default' : 'primary'}
            icon={streaming ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
            onClick={toggleStream}
            danger={streaming}
          >
            {streaming ? t('logs:center.stop') : t('logs:center.startMonitor')}
          </Button>
          <Button icon={<ClearOutlined />} onClick={clearLogs}>
            {t('logs:center.clear')}
          </Button>
          <Button
            icon={<DownloadOutlined />}
            onClick={downloadLogs}
            disabled={filteredLogs.length === 0}
          >
            {t('logs:center.download')}
          </Button>
        </Space>

        <Space>
          <Select
            mode="multiple"
            placeholder={t('logs:center.logLevelFilter')}
            style={{ width: 200 }}
            value={levelFilter}
            onChange={setLevelFilter}
            options={levelOptions.map(o => ({ ...o, label: t(`logs:center.${o.value}`) }))}
          />
          <Tooltip title={t('logs:center.showTimestamp')}>
            <Switch
              checked={showTimestamp}
              onChange={setShowTimestamp}
              checkedChildren={t('logs:center.timestamp')}
              unCheckedChildren={t('logs:center.timestamp')}
            />
          </Tooltip>
          <Tooltip title={t('logs:center.showSource')}>
            <Switch
              checked={showSource}
              onChange={setShowSource}
              checkedChildren={t('logs:center.source')}
              unCheckedChildren={t('logs:center.source')}
            />
          </Tooltip>
          <Tooltip title={t('logs:center.autoScroll')}>
            <Switch
              checked={autoScroll}
              onChange={setAutoScroll}
              checkedChildren={t('logs:center.scroll')}
              unCheckedChildren={t('logs:center.scroll')}
            />
          </Tooltip>
        </Space>
      </div>

      {/* Pod selector */}
      <Card size="small" style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontWeight: 500 }}>{t('logs:center.monitorTarget')}</span>
          {targets.map((target, i) => (
            <Tag key={i} closable onClose={() => removeTarget(i)} color="blue">
              {target.namespace}/{target.pod}
              {target.container && `:${target.container}`}
            </Tag>
          ))}
          <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={openPodSelector}>
            {t('logs:center.addPod')}
          </Button>
          {streaming && (
            <Badge status="processing" text={t('logs:center.monitoring')} style={{ marginLeft: 'auto' }} />
          )}
        </div>
      </Card>

      {/* Search box */}
      <div style={{ marginBottom: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
        <Input
          placeholder={t('logs:center.searchLogPlaceholder')}
          prefix={<SearchOutlined />}
          allowClear
          value={logSearchKeyword}
          onChange={(e) => setLogSearchKeyword(e.target.value)}
          style={{ width: 350 }}
        />
        {logSearchKeyword && (
          <span style={{ color: '#888' }}>
            {t('logs:center.matchCount', { filtered: filteredLogs.length, total: filteredLogs.length })}
          </span>
        )}
      </div>

      {/* Log display area */}
      <div
        style={{
          height: 'calc(100vh - 540px)',
          minHeight: 400,
          backgroundColor: '#1e1e1e',
          borderRadius: 8,
          overflow: 'auto',
          fontFamily: "'Fira Code', 'Monaco', 'Menlo', monospace",
          fontSize: 13,
          lineHeight: 1.6,
        }}
      >
        {filteredLogs.length === 0 ? (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
              color: '#666',
            }}
          >
            <Empty
              description={streaming ? t('logs:center.waitingLogs') : t('logs:center.selectPodFirst')}
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          </div>
        ) : (
          <div style={{ padding: 16 }}>
            {filteredLogs.map((log, index) => (
              <div
                key={log.id || index}
                style={{ display: 'flex', gap: 8, marginBottom: 2, color: '#d4d4d4' }}
              >
                {showTimestamp && (
                  <span style={{ color: '#6a9955', whiteSpace: 'nowrap' }}>
                    {dayjs(log.timestamp).format('HH:mm:ss.SSS')}
                  </span>
                )}
                {showSource && (
                  <span style={{ color: '#569cd6', whiteSpace: 'nowrap' }}>
                    [{logSearchKeyword ? highlightKeyword(`${log.namespace}/${log.pod_name}`, logSearchKeyword) : `${log.namespace}/${log.pod_name}`}]
                  </span>
                )}
                <span
                  style={{
                    color: levelColors[log.level] || '#d4d4d4',
                    fontWeight: log.level === 'error' ? 600 : 400,
                    wordBreak: 'break-all',
                  }}
                >
                  {logSearchKeyword ? highlightKeyword(log.message, logSearchKeyword) : log.message}
                </span>
              </div>
            ))}
            <div ref={logsEndRef} />
          </div>
        )}
      </div>

      {/* Status bar */}
      <div
        style={{
          marginTop: 8,
          display: 'flex',
          justifyContent: 'space-between',
          color: '#8c8c8c',
          fontSize: 12,
        }}
      >
        <span>{t('logs:center.totalLogs', { count: filteredLogs.length })}</span>
        <span>{t('logs:center.maxRetain', { max: maxLines })}</span>
      </div>
    </div>
  );
};

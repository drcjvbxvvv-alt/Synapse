import EmptyState from '@/components/EmptyState';
import React, { useState, useEffect } from 'react';
import {
  Badge, Button, Card, Descriptions, Space, Steps, Tag, Timeline, Typography,
} from 'antd';
import { CodeOutlined, PlayCircleOutlined } from '@ant-design/icons';
import axios from 'axios';

const { Text } = Typography;

interface TerminalCommand {
  id: number;
  sessionId: number;
  timestamp: string;
  rawInput: string;
  parsedCmd: string;
  exitCode?: number;
}

interface TerminalSession {
  id: number;
  userID: number;
  clusterID: number;
  targetType: string;
  namespace?: string;
  pod?: string;
  container?: string;
  startAt?: string;
  endAt?: string;
  status: string;
  commands: TerminalCommand[];
}

interface SessionReplayProps {
  sessionId: number;
}

const SessionReplay: React.FC<SessionReplayProps> = ({ sessionId }) => {
  const [session, setSession] = useState<TerminalSession | null>(null);
  const [commands, setCommands] = useState<TerminalCommand[]>([]);
  const [loading, setLoading] = useState(false);
  const [playIndex, setPlayIndex] = useState<number>(-1);
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    if (!sessionId) return;
    setLoading(true);
    Promise.all([
      axios.get(`/api/v1/audit/terminal/sessions/${sessionId}`),
      axios.get(`/api/v1/audit/terminal/sessions/${sessionId}/commands`),
    ]).then(([sRes, cRes]) => {
      setSession(sRes.data.session || sRes.data);
      setCommands(cRes.data.commands || cRes.data || []);
    }).catch(() => {}).finally(() => setLoading(false));
  }, [sessionId]);

  const handlePlay = () => {
    if (commands.length === 0) return;
    setPlaying(true);
    setPlayIndex(0);
    let idx = 0;
    const interval = setInterval(() => {
      idx++;
      if (idx >= commands.length) {
        clearInterval(interval);
        setPlaying(false);
        setPlayIndex(-1);
      } else {
        setPlayIndex(idx);
      }
    }, 800);
  };

  if (loading) return <Card loading />;
  if (!session) return <EmptyState description="找不到此 Terminal 會話" />;

  const displayCommands = playIndex >= 0 ? commands.slice(0, playIndex + 1) : commands;

  return (
    <Card
      title={
        <Space>
          <PlayCircleOutlined />
          Terminal 會話回放 #{sessionId}
        </Space>
      }
      extra={
        <Button
          type="primary"
          icon={<PlayCircleOutlined />}
          onClick={handlePlay}
          loading={playing}
          disabled={commands.length === 0}
        >
          {playing ? '播放中...' : '逐步播放'}
        </Button>
      }
    >
      {/* 會話資訊 */}
      <Descriptions size="small" bordered column={3} style={{ marginBottom: 16 }}>
        <Descriptions.Item label="目標型別">
          <Tag>{session.targetType}</Tag>
        </Descriptions.Item>
        {session.namespace && (
          <Descriptions.Item label="命名空間">{session.namespace}</Descriptions.Item>
        )}
        {session.pod && (
          <Descriptions.Item label="Pod">{session.pod}</Descriptions.Item>
        )}
        <Descriptions.Item label="狀態">
          <Badge
            status={session.status === 'closed' ? 'default' : 'processing'}
            text={session.status}
          />
        </Descriptions.Item>
        {session.startAt && (
          <Descriptions.Item label="開始時間">
            {new Date(session.startAt).toLocaleString('zh-TW')}
          </Descriptions.Item>
        )}
        {session.endAt && (
          <Descriptions.Item label="結束時間">
            {new Date(session.endAt).toLocaleString('zh-TW')}
          </Descriptions.Item>
        )}
        <Descriptions.Item label="指令數">{commands.length}</Descriptions.Item>
      </Descriptions>

      {/* 指令時間軸 */}
      {displayCommands.length === 0 ? (
        <EmptyState description="此會話無記錄指令" />
      ) : (
        <div
          style={{
            background: '#1a1a1a',
            borderRadius: 8,
            padding: 16,
            maxHeight: 500,
            overflow: 'auto',
            fontFamily: 'monospace',
          }}
        >
          {displayCommands.map((cmd, i) => (
            <div
              key={cmd.id}
              style={{
                marginBottom: 12,
                opacity: playIndex >= 0 && i === playIndex ? 1 : 0.85,
                transition: 'opacity 0.3s',
              }}
            >
              <Text style={{ color: '#888', fontSize: 11 }}>
                [{new Date(cmd.timestamp).toLocaleTimeString('zh-TW')}]
              </Text>
              <br />
              <Text
                style={{
                  color: '#4ade80',
                  fontFamily: 'monospace',
                }}
              >
                $ {cmd.parsedCmd || cmd.rawInput}
              </Text>
              {cmd.exitCode !== undefined && cmd.exitCode !== 0 && (
                <Tag color="error" style={{ marginLeft: 8, fontSize: 11 }}>
                  exit {cmd.exitCode}
                </Tag>
              )}
            </div>
          ))}
          {playing && (
            <Text style={{ color: '#facc15', fontFamily: 'monospace' }}>
              █ <span style={{ animation: 'blink 1s infinite' }}>_</span>
            </Text>
          )}
        </div>
      )}
    </Card>
  );
};

export default SessionReplay;

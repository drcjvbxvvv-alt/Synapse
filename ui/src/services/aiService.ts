import { request } from '../utils/api';
import type { AIConfig, ChatMessage } from '../types/ai';

export const aiService = {
  getConfig: () => {
    return request.get<AIConfig>('/ai/config');
  },

  updateConfig: (data: AIConfig) => {
    return request.put('/ai/config', data);
  },

  testConnection: (data: Partial<AIConfig>) => {
    return request.post<{ success: boolean }>('/ai/test-connection', data);
  },

  chatStream: (
    clusterId: string,
    messages: ChatMessage[],
    onEvent: (eventType: string, data: unknown) => void,
    onDone: () => void,
    onError: (error: string) => void,
    signal?: AbortSignal,
  ) => {
    const token = localStorage.getItem('token');
    const baseURL = import.meta.env.VITE_API_BASE_URL || '/api/v1';

    fetch(`${baseURL}/clusters/${clusterId}/ai/chat`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
        'Accept': 'text/event-stream',
      },
      body: JSON.stringify({ messages }),
      signal,
    })
      .then(async (response) => {
        if (!response.ok) {
          const errorData = await response.json().catch(() => ({ message: 'Unknown error' }));
          onError(errorData.message || `HTTP ${response.status}`);
          return;
        }

        const reader = response.body?.getReader();
        if (!reader) {
          onError('无法读取响应流');
          return;
        }

        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';

          let currentEvent = '';
          for (const line of lines) {
            if (line.startsWith('event: ')) {
              currentEvent = line.slice(7).trim();
            } else if (line.startsWith('data: ')) {
              const data = line.slice(6);
              if (currentEvent === 'done') {
                onDone();
                return;
              }
              try {
                const parsed = JSON.parse(data);
                onEvent(currentEvent, parsed);
              } catch {
                onEvent(currentEvent, data);
              }
              currentEvent = '';
            }
          }
        }
        onDone();
      })
      .catch((err) => {
        if (err.name === 'AbortError') return;
        onError(err.message || '网络请求失败');
      });
  },
};

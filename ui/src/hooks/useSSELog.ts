/**
 * useSSELog — Server-Sent Events log streaming hook
 *
 * 後端 SSE 事件格式（pipeline_log_handler.go）：
 *   event: log\n data: <line>\n\n
 *   event: done\n data: {"status":"completed"}\n\n
 *   event: error\n data: {"error":"..."}\n\n
 *
 * Usage:
 *   const { lines, status, clear } = useSSELog({ url: stepLogUrl });
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import { tokenManager } from '../services/authService';

export type SSEStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error';

interface UseSSELogOptions {
  /** Full API URL for the SSE stream. Pass null/undefined to disable. */
  url: string | null | undefined;
  /** Max log lines to keep in memory (default 2000) */
  maxLines?: number;
}

interface UseSSELogReturn {
  lines: string[];
  status: SSEStatus;
  clear: () => void;
}

export function useSSELog({ url, maxLines = 2000 }: UseSSELogOptions): UseSSELogReturn {
  const [lines, setLines] = useState<string[]>([]);
  const [status, setStatus] = useState<SSEStatus>('idle');
  const esRef = useRef<EventSource | null>(null);

  const clear = useCallback(() => setLines([]), []);

  useEffect(() => {
    // Close any previous connection
    esRef.current?.close();
    esRef.current = null;

    if (!url) {
      setStatus('idle');
      setLines([]);
      return;
    }

    setStatus('connecting');
    setLines([]);

    // Append JWT token as query param — same pattern as WS terminal
    const token = tokenManager.getToken();
    const sep = url.includes('?') ? '&' : '?';
    const fullUrl = token ? `${url}${sep}token=${encodeURIComponent(token)}` : url;

    const es = new EventSource(fullUrl);
    esRef.current = es;

    es.onopen = () => setStatus('open');

    // Named events from backend
    es.addEventListener('log', (e) => {
      const line = (e as MessageEvent).data as string;
      setLines((prev) => {
        const next = [...prev, line];
        return next.length > maxLines ? next.slice(-maxLines) : next;
      });
    });

    es.addEventListener('done', () => {
      setStatus('closed');
      es.close();
    });

    es.addEventListener('error', () => {
      // backend sent an error event (distinct from network error)
      setStatus('error');
      es.close();
    });

    // Network / connection error
    es.onerror = () => {
      setStatus('error');
      es.close();
    };

    return () => {
      es.close();
    };
  }, [url, maxLines]);

  return { lines, status, clear };
}

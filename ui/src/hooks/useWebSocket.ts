import { useCallback, useEffect, useRef, useState } from 'react'

export type WSStatus = 'connecting' | 'open' | 'closed' | 'error'

export interface UseWebSocketOptions {
  /** 是否自動重連，預設 true */
  autoReconnect?: boolean
  /** 最大重連次數，預設 10 */
  maxRetries?: number
  /** 初始重連延遲 ms，預設 1000；每次翻倍（指數退避） */
  baseDelay?: number
  /** 最大退避延遲 ms，預設 30000 */
  maxDelay?: number
  onOpen?: (event: Event) => void
  onMessage?: (event: MessageEvent) => void
  onClose?: (event: CloseEvent) => void
  onError?: (event: Event) => void
}

export interface UseWebSocketReturn {
  status: WSStatus
  send: (data: string | ArrayBufferLike | Blob | ArrayBufferView) => void
  close: () => void
  reconnect: () => void
}

export function useWebSocket(
  url: string | null,
  options: UseWebSocketOptions = {}
): UseWebSocketReturn {
  const {
    autoReconnect = true,
    maxRetries = 10,
    baseDelay = 1000,
    maxDelay = 30000,
    onOpen,
    onMessage,
    onClose,
    onError,
  } = options

  const [status, setStatus] = useState<WSStatus>('closed')
  const wsRef = useRef<WebSocket | null>(null)
  const retriesRef = useRef(0)
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const manualCloseRef = useRef(false)

  // Keep callbacks in ref so stale closures in event handlers see latest values
  const cbRef = useRef({ onOpen, onMessage, onClose, onError })
  cbRef.current = { onOpen, onMessage, onClose, onError }

  const clearRetryTimer = () => {
    if (retryTimerRef.current !== null) {
      clearTimeout(retryTimerRef.current)
      retryTimerRef.current = null
    }
  }

  const connect = useCallback(() => {
    if (!url) return
    clearRetryTimer()

    setStatus('connecting')
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = (e) => {
      retriesRef.current = 0
      setStatus('open')
      cbRef.current.onOpen?.(e)
    }

    ws.onmessage = (e) => {
      cbRef.current.onMessage?.(e)
    }

    ws.onerror = (e) => {
      setStatus('error')
      cbRef.current.onError?.(e)
    }

    ws.onclose = (e) => {
      setStatus('closed')
      cbRef.current.onClose?.(e)

      if (!manualCloseRef.current && autoReconnect && retriesRef.current < maxRetries) {
        const delay = Math.min(baseDelay * 2 ** retriesRef.current, maxDelay)
        retriesRef.current += 1
        retryTimerRef.current = setTimeout(() => connect(), delay)
      }
    }
  }, [url, autoReconnect, maxRetries, baseDelay, maxDelay])

  useEffect(() => {
    manualCloseRef.current = false
    retriesRef.current = 0
    if (url) connect()
    return () => {
      manualCloseRef.current = true
      clearRetryTimer()
      wsRef.current?.close()
    }
  }, [url, connect])

  const send = useCallback(
    (data: string | ArrayBufferLike | Blob | ArrayBufferView) => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(data)
      }
    },
    []
  )

  const close = useCallback(() => {
    manualCloseRef.current = true
    clearRetryTimer()
    wsRef.current?.close()
  }, [])

  const reconnect = useCallback(() => {
    manualCloseRef.current = false
    retriesRef.current = 0
    wsRef.current?.close()
    connect()
  }, [connect])

  return { status, send, close, reconnect }
}

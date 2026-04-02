export interface AIConfig {
  provider: string;
  endpoint: string;
  api_key: string;
  model: string;
  api_version?: string;
  enabled: boolean;
}

export interface ChatMessage {
  role: 'system' | 'user' | 'assistant' | 'tool';
  content: string;
  tool_calls?: ToolCall[];
  tool_call_id?: string;
}

export interface ToolCall {
  id: string;
  type: string;
  function: {
    name: string;
    arguments: string;
  };
}

export interface SSEEvent {
  event: string;
  data: string;
}

export interface ChatStreamContentEvent {
  content: string;
}

export interface ChatStreamToolCallEvent {
  id: string;
  name: string;
  arguments: string;
}

export interface ChatStreamToolResultEvent {
  id: string;
  name: string;
  result: unknown;
}

export interface ChatStreamErrorEvent {
  error: string;
}

export interface DisplayMessage {
  id: string;
  role: 'user' | 'assistant' | 'tool';
  content: string;
  toolCalls?: ToolCall[];
  toolResults?: { name: string; result: string }[];
  loading?: boolean;
  timestamp: number;
  runbooks?: Runbook[];
}

export interface Runbook {
  id: string;
  title: string;
  reasons: string[];
  keywords: string[];
  summary: string;
  steps: string[];
}

export interface NLQueryResult {
  question: string;
  tool_used: string;
  result: unknown;
  summary: string;
}

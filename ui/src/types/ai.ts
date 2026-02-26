export interface AIConfig {
  provider: string;
  endpoint: string;
  api_key: string;
  model: string;
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
}

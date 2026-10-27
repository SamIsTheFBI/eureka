export interface User {
  id: number;
  email: string;
  role: 'admin' | 'member';
  active: boolean;
  created_at: string;
  key_count?: number;
}

export interface Tool {
  name: string;
  description: string;
  inputSchema: {
    type: string;
    properties?: Record<string, any>;
    required?: string[];
  };
}

export interface MCPServer {
  id: number;
  slug: string;
  name: string;
  url?: string;
  enabled: boolean;
  status: 'Connected' | 'Unreachable' | 'Disabled';
  reason?: string;
  last_error?: string;
  last_success?: string;
  has_credential: boolean;
  tools: Tool[];
}

export interface APIKey {
  id: number;
  name: string;
  prefix: string;
  user_email: string;
  created_at: string;
  last_used_at?: string;
  revoked_at?: string;
}

export interface AuditRecord {
  id: number;
  ts: string;
  user_email: string;
  channel: string;
  server_slug: string;
  tool: string;
  exposed_name: string;
  outcome: 'success' | 'tool_error' | 'timeout' | 'unreachable' | 'denied' | 'auth_rejected';
  error_code?: string;
  duration_ms: number;
  request_id: string;
}

export interface ToolCallChip {
  id: string;
  name: string;
  args?: string;
  outcome?: string;
  duration?: number;
  preview?: string;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'tool';
  content: string;
  toolCalls?: ToolCallChip[];
}

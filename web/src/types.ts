export type TaskStatus = 'queued' | 'running' | 'waiting_for_approval' | 'completed' | 'failed' | 'cancelled';

export interface ToolResult {
  success: boolean;
  output: string;
  error?: string;
  data?: any;
  duration_ms: number;
}

export interface AgentEvent {
  type: string;
  sender?: 'user' | 'bob';
  task_id: string;
  session_id?: string;
  timestamp: string;
  message?: string;
  tool?: string;
  input?: any;
  output?: string;
  tool_result?: ToolResult;
  duration_ms?: number;
  status?: TaskStatus;
  error?: string;
}

export interface Task {
  id: string;
  session_id: string;
  prompt: string;
  status: TaskStatus;
  created_at: string;
  updated_at: string;
  result?: string;
  error?: string;
  events: AgentEvent[];
  pending_approval?: {
    tool_name: string;
    input: any;
    reason: string;
  };
}

export interface SessionMessage {
  role: string;
  content: string;
}

export interface Session {
  id: string;
  title: string;
  created_at: string;
  last_activity: string;
  messages?: SessionMessage[];
  task_ids: string[];
}

export interface SessionDetailResponse {
  session: Session;
  tasks: Task[];
}

export interface SystemInfo {
  os: string;
  macos_version: string;
  architecture: string;
  cpu_model: string;
  ram_bytes: number;
  ram_formatted: string;
  available_disk: string;
  ollama_running: boolean;
  tailscale_ip: string;
  recommended_models: string[];
}

export interface SystemStatus {
  agent_name: string;
  status: string;
  system: SystemInfo;
  llm: {
    provider: string;
    model: string;
    base_url: string;
  };
  security: {
    require_approval: boolean;
    allowed_paths: string[];
  };
  tailscale: {
    enabled: boolean;
    ip: string;
  };
  coding_agent: {
    name: string;
    available: boolean;
    reason: string;
  };
}

export interface AuditEntry {
  timestamp: string;
  task_id: string;
  session_id: string;
  action?: string;
  tool?: string;
  input?: any;
  result?: any;
  exit_code: number;
  duration_ms: number;
  approval_status?: string;
  error?: string;
}

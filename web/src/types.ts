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
  delta?: string;
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

export type SessionType = 'project' | 'conversation';

export interface SessionMessage {
  role: string;
  content: string;
}

export interface Project {
  id: string;
  name: string;
  path: string;
  summary?: string;
  tech_stack?: string[];
  created_at: string;
  updated_at: string;
}

export interface ProjectWithSessions extends Project {
  sessions: Session[];
}

export interface Session {
  id: string;
  project_id?: string;
  type: SessionType;
  title: string;
  project_path?: string;
  project_name?: string;
  created_at: string;
  last_activity: string;
  messages?: SessionMessage[];
  task_ids: string[];
}

export interface BrowseItem {
  name: string;
  path: string;
  is_project: boolean;
}

export interface BrowseResponse {
  current_path: string;
  parent_path: string;
  directories: BrowseItem[];
}

export interface ProjectSummary {
  name: string;
  path: string;
}

export interface SessionDetailResponse {
  session: Session;
  tasks: Task[];
  project?: Project;
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

export interface UserConfig {
  name: string;
  alias?: string;
  title?: string;
  description?: string;
}

export interface SystemStatus {
  agent_name: string;
  status: string;
  system: SystemInfo;
  user?: UserConfig;
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

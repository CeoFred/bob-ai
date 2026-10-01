import { SystemStatus, Session, Task, AuditEntry, BrowseResponse, ProjectSummary, SessionType, Project, ProjectWithSessions } from '../types';

const API_BASE = '';

export async function fetchStatus(): Promise<SystemStatus> {
  const res = await fetch(`${API_BASE}/api/status`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function fetchSessions(): Promise<Session[]> {
  const res = await fetch(`${API_BASE}/api/sessions`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function fetchSession(id: string): Promise<{ session: Session; tasks: Task[] }> {
  const res = await fetch(`${API_BASE}/api/sessions/${id}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const data = await res.json();
  // Handle both {session, tasks} or direct Session object for backwards compatibility
  if (data.session) {
    return data;
  }
  return { session: data, tasks: [] };
}

export async function createSession(params?: {
  type?: SessionType;
  project_path?: string;
  title?: string;
}): Promise<Session> {
  const res = await fetch(`${API_BASE}/api/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params || {}),
  });
  if (!res.ok) {
    const errData = await res.json().catch(() => ({}));
    throw new Error(errData.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function deleteSession(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/sessions/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) {
    const errData = await res.json().catch(() => ({}));
    throw new Error(errData.error || `HTTP ${res.status}`);
  }
}

export async function deleteProject(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/projects/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) {
    const errData = await res.json().catch(() => ({}));
    throw new Error(errData.error || `HTTP ${res.status}`);
  }
}

export async function fetchProjects(): Promise<ProjectWithSessions[]> {
  const res = await fetch(`${API_BASE}/api/projects`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function createOrOpenProject(path: string, name?: string): Promise<{ project: Project; session: Session }> {
  const res = await fetch(`${API_BASE}/api/projects`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, name }),
  });
  if (!res.ok) {
    const errData = await res.json().catch(() => ({}));
    throw new Error(errData.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function createProjectSession(projectId: string, title?: string): Promise<Session> {
  const res = await fetch(`${API_BASE}/api/projects/${projectId}/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title }),
  });
  if (!res.ok) {
    const errData = await res.json().catch(() => ({}));
    throw new Error(errData.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function fetchBrowseFilesystem(path?: string): Promise<BrowseResponse> {
  const query = path ? `?path=${encodeURIComponent(path)}` : '';
  const res = await fetch(`${API_BASE}/api/filesystem/browse${query}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function fetchRecentProjects(): Promise<ProjectSummary[]> {
  const res = await fetch(`${API_BASE}/api/filesystem/recent-projects`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function fetchTasks(): Promise<Task[]> {
  const res = await fetch(`${API_BASE}/api/tasks`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function fetchTask(id: string): Promise<Task> {
  const res = await fetch(`${API_BASE}/api/tasks/${id}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function createTask(prompt: string, sessionId?: string): Promise<Task> {
  const res = await fetch(`${API_BASE}/api/tasks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ prompt, session_id: sessionId }),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function cancelTask(taskId: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/tasks/${taskId}/cancel`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
}

export async function approveTask(taskId: string, approved: boolean): Promise<void> {
  const res = await fetch(`${API_BASE}/api/tasks/${taskId}/approve`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ approved }),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
}

export async function fetchAuditLogs(limit: number = 50): Promise<AuditEntry[]> {
  const res = await fetch(`${API_BASE}/api/audit?limit=${limit}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

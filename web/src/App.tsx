import React, { useState, useEffect, useRef } from 'react';
import { Header } from './components/Header';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './components/ChatView';
import { ScreenshotModal } from './components/ScreenshotModal';
import { AuditModal } from './components/AuditModal';
import { ProjectPickerModal } from './components/ProjectPickerModal';
import { SystemStatus, Session, Task, AgentEvent, ProjectWithSessions } from './types';
import {
  fetchStatus,
  fetchSessions,
  fetchSession,
  fetchProjects,
  createOrOpenProject,
  createProjectSession,
  createSession,
  deleteSession,
  deleteProject,
  createTask,
  cancelTask,
  approveTask,
} from './services/api';
import { wsClient } from './services/websocket';
import { getSessionIdFromUrl, navigateToChat } from './lib/router';

export const App: React.FC = () => {
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [online, setOnline] = useState(false);
  const [projects, setProjects] = useState<ProjectWithSessions[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string>('');
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [currentTask, setCurrentTask] = useState<Task | null>(null);
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [screenshotModalUrl, setScreenshotModalUrl] = useState<string | null>(null);
  const [auditModalOpen, setAuditModalOpen] = useState(false);
  const [projectModalOpen, setProjectModalOpen] = useState(false);

  // Keep a ref to activeSessionId to avoid stale closures in WS listener
  const activeSessionIdRef = useRef(activeSessionId);
  activeSessionIdRef.current = activeSessionId;

  // Initialize data, URL routing listener, and WebSocket connection
  useEffect(() => {
    loadInitialData();

    wsClient.connect();

    const unsubStatus = wsClient.onStatus((connected) => {
      setOnline(connected);
      if (connected) {
        loadStatus();
      }
    });

    const unsubEvent = wsClient.onEvent((ev) => {
      handleIncomingEvent(ev);
    });

    // Handle browser back and forward navigation
    const handlePopState = () => {
      const urlSessionId = getSessionIdFromUrl();
      if (urlSessionId) {
        selectSession(urlSessionId, false);
      } else {
        setActiveSessionId('');
        setEvents([]);
        setCurrentTask(null);
      }
    };

    window.addEventListener('popstate', handlePopState);

    return () => {
      unsubStatus();
      unsubEvent();
      window.removeEventListener('popstate', handlePopState);
      wsClient.disconnect();
    };
  }, []);

  const loadInitialData = async () => {
    await loadStatus();
    try {
      const [projList, sessList] = await Promise.all([
        fetchProjects().catch(() => []),
        fetchSessions().catch(() => []),
      ]);
      setProjects(projList);
      setSessions(sessList);

      const urlSessionId = getSessionIdFromUrl();
      if (urlSessionId) {
        await selectSession(urlSessionId, false);
      }
    } catch (e) {
      console.error('Failed to load initial data:', e);
    }
  };

  const loadStatus = async () => {
    try {
      const data = await fetchStatus();
      setStatus(data);
    } catch (e) {
      console.error('Failed to fetch status:', e);
    }
  };

  const refreshProjectsAndSessions = () => {
    fetchProjects().then(setProjects).catch(() => {});
    fetchSessions().then(setSessions).catch(() => {});
  };

  const selectSession = async (sessionId: string, updateUrl: boolean = true) => {
    setActiveSessionId(sessionId);
    if (updateUrl) {
      navigateToChat(sessionId);
    }

    try {
      const detail = await fetchSession(sessionId);
      const reconstructedEvents: AgentEvent[] = [];
      let latestRunningTask: Task | null = null;

      if (detail.tasks && detail.tasks.length > 0) {
        for (const t of detail.tasks) {
          // Add User Message event for each task prompt
          reconstructedEvents.push({
            type: 'user.message',
            sender: 'user',
            task_id: t.id,
            session_id: sessionId,
            timestamp: t.created_at,
            message: t.prompt,
          });

          // Add all events emitted by this task
          if (t.events && t.events.length > 0) {
            for (const ev of t.events) {
              // Avoid duplicate user messages if already added
              if (ev.type !== 'user.message' && ev.type !== 'task.status') {
                reconstructedEvents.push(ev);
              }
            }
          } else if (t.result) {
            reconstructedEvents.push({
              type: 'agent.message',
              sender: 'bob',
              task_id: t.id,
              session_id: sessionId,
              timestamp: t.updated_at,
              message: t.result,
            });
          }

          if (t.status === 'running' || t.status === 'waiting_for_approval' || t.status === 'queued') {
            latestRunningTask = t;
          }
        }
      } else if (detail.session && detail.session.messages) {
        for (const m of detail.session.messages) {
          if (m.role === 'user') {
            reconstructedEvents.push({
              type: 'user.message',
              sender: 'user',
              task_id: '',
              session_id: sessionId,
              timestamp: detail.session.last_activity,
              message: m.content,
            });
          } else if (m.role === 'assistant' && m.content) {
            reconstructedEvents.push({
              type: 'agent.message',
              sender: 'bob',
              task_id: '',
              session_id: sessionId,
              timestamp: detail.session.last_activity,
              message: m.content,
            });
          }
        }
      }

      setEvents(reconstructedEvents);
      setCurrentTask(latestRunningTask);
    } catch (e) {
      console.error('Failed to load session details:', e);
      setEvents([]);
      setCurrentTask(null);
    }
  };

  const handleIncomingEvent = (ev: AgentEvent) => {
    const currentActive = activeSessionIdRef.current;
    if (!ev.session_id || ev.session_id === currentActive) {
      setEvents((prev) => [...prev, ev]);
    }

    if (ev.type === 'task.status' || ev.type === 'task.completed') {
      setCurrentTask((prev) => {
        if (!prev || prev.id === ev.task_id) {
          const newStatus = ev.status || (ev.type === 'task.completed' ? 'completed' : 'running');
          return prev ? { ...prev, status: newStatus } : null;
        }
        return prev;
      });

      // Refresh data
      refreshProjectsAndSessions();
    }

    if (ev.type === 'tool.approval_required') {
      setCurrentTask((prev) =>
        prev
          ? {
              ...prev,
              status: 'waiting_for_approval',
              pending_approval: {
                tool_name: ev.tool || 'tool',
                input: ev.input,
                reason: ev.message || 'Approval needed',
              },
            }
          : null
      );
    }
  };

  const handleNewConversation = async () => {
    try {
      const newSess = await createSession({ type: 'conversation', title: 'New Conversation' });
      setSessions((prev) => [newSess, ...prev.filter((s) => s.id !== newSess.id)]);
      setActiveSessionId(newSess.id);
      navigateToChat(newSess.id);
      setEvents([]);
      setCurrentTask(null);
      refreshProjectsAndSessions();
    } catch (e) {
      console.error('Failed to create new conversation:', e);
    }
  };

  const handleNewProjectThread = async (projectId: string) => {
    try {
      const newSess = await createProjectSession(projectId);
      setSessions((prev) => [newSess, ...prev.filter((s) => s.id !== newSess.id)]);
      setActiveSessionId(newSess.id);
      navigateToChat(newSess.id);
      setEvents([]);
      setCurrentTask(null);
      refreshProjectsAndSessions();
    } catch (e) {
      console.error('Failed to create project thread:', e);
    }
  };

  const handleCreateProjectSession = async (projectPath: string, projectName: string) => {
    try {
      const resp = await createOrOpenProject(projectPath, projectName);
      await refreshProjectsAndSessions();
      if (resp.session) {
        setActiveSessionId(resp.session.id);
        navigateToChat(resp.session.id);
      }
      setEvents([]);
      setCurrentTask(null);
    } catch (e) {
      console.error('Failed to create project session:', e);
      alert(`Failed to open project: ${e instanceof Error ? e.message : String(e)}`);
    }
  };

  const handleSendPrompt = async (prompt: string) => {
    try {
      let sessId = activeSessionId;
      if (!sessId) {
        const newSess = await createSession({ type: 'conversation' });
        setSessions((prev) => [newSess, ...prev.filter((s) => s.id !== newSess.id)]);
        sessId = newSess.id;
        setActiveSessionId(sessId);
        navigateToChat(sessId);
      }

      const userEvent: AgentEvent = {
        type: 'user.message',
        sender: 'user',
        task_id: 'pending',
        session_id: sessId,
        timestamp: new Date().toISOString(),
        message: prompt,
      };
      setEvents((prev) => [...prev, userEvent]);

      const task = await createTask(prompt, sessId);
      setCurrentTask(task);
      refreshProjectsAndSessions();
    } catch (e) {
      console.error('Failed to send task:', e);
      setCurrentTask(null);
      setEvents((prev) => [
        ...prev,
        {
          type: 'agent.error',
          task_id: 'error',
          timestamp: new Date().toISOString(),
          error: String(e),
        },
      ]);
    }
  };

  const handleCancelTask = async (taskId: string) => {
    try {
      await cancelTask(taskId);
      setCurrentTask((prev) => (prev ? { ...prev, status: 'cancelled' } : null));
    } catch (e) {
      console.error('Failed to cancel task:', e);
    }
  };

  const handleApprove = async (taskId: string, approved: boolean) => {
    try {
      await approveTask(taskId, approved);
    } catch (e) {
      console.error('Failed to send approval:', e);
    }
  };

  const handleDeleteSession = async (sessionId: string) => {
    try {
      await deleteSession(sessionId);
      setSessions((prev) => prev.filter((s) => s.id !== sessionId));
      if (activeSessionId === sessionId) {
        const remaining = sessions.filter((s) => s.id !== sessionId);
        if (remaining.length > 0) {
          selectSession(remaining[0].id);
        } else {
          setActiveSessionId('');
          navigateToChat('');
          setEvents([]);
          setCurrentTask(null);
        }
      }
      refreshProjectsAndSessions();
    } catch (e) {
      console.error('Failed to delete session:', e);
    }
  };

  const handleDeleteProject = async (projectId: string) => {
    try {
      await deleteProject(projectId);
      setProjects((prev) => prev.filter((p) => p.id !== projectId));
      const currentActiveSession = sessions.find((s) => s.id === activeSessionId);
      if (currentActiveSession?.project_id === projectId) {
        const remaining = sessions.filter((s) => s.project_id !== projectId);
        if (remaining.length > 0) {
          selectSession(remaining[0].id);
        } else {
          setActiveSessionId('');
          navigateToChat('');
          setEvents([]);
          setCurrentTask(null);
        }
      }
      refreshProjectsAndSessions();
    } catch (e) {
      console.error('Failed to delete project:', e);
    }
  };

  const activeSession = sessions.find((s) => s.id === activeSessionId) || null;
  const activeProject =
    projects.find(
      (p) =>
        (activeSession?.project_id && p.id === activeSession.project_id) ||
        (activeSession?.project_path && p.path === activeSession.project_path)
    ) || null;

  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden bg-[#09090b] font-sans antialiased text-zinc-100">
      <Header
        status={status}
        online={online}
        onOpenAudit={() => setAuditModalOpen(true)}
        sidebarOpen={sidebarOpen}
        onToggleSidebar={() => setSidebarOpen(!sidebarOpen)}
        activeSession={activeSession}
        onNewChat={handleNewConversation}
      />

      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          projects={projects}
          sessions={sessions}
          activeSessionId={activeSessionId}
          onSelectSession={(id) => selectSession(id)}
          onNewConversation={handleNewConversation}
          onOpenProjectModal={() => setProjectModalOpen(true)}
          onNewProjectThread={handleNewProjectThread}
          onDeleteSession={handleDeleteSession}
          onDeleteProject={handleDeleteProject}
          open={sidebarOpen}
        />

        <main className="flex-1 flex flex-col overflow-hidden bg-[#09090b]">
          <ChatView
            currentTask={currentTask}
            events={events}
            session={activeSession}
            project={activeProject}
            onSend={handleSendPrompt}
            onCancel={handleCancelTask}
            onApprove={handleApprove}
            onViewScreenshot={(url) => setScreenshotModalUrl(url)}
            onOpenProjectModal={() => setProjectModalOpen(true)}
          />
        </main>
      </div>

      <ProjectPickerModal
        isOpen={projectModalOpen}
        onClose={() => setProjectModalOpen(false)}
        onSelectProject={handleCreateProjectSession}
      />

      {screenshotModalUrl && (
        <ScreenshotModal url={screenshotModalUrl} onClose={() => setScreenshotModalUrl(null)} />
      )}

      {auditModalOpen && <AuditModal onClose={() => setAuditModalOpen(false)} />}
    </div>
  );
};


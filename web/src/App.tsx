import React, { useState, useEffect, useRef } from 'react';
import { Header } from './components/Header';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './components/ChatView';
import { ScreenshotModal } from './components/ScreenshotModal';
import { AuditModal } from './components/AuditModal';
import { SystemStatus, Session, Task, AgentEvent } from './types';
import { fetchStatus, fetchSessions, fetchSession, createSession, createTask, cancelTask, approveTask } from './services/api';
import { wsClient } from './services/websocket';

export const App: React.FC = () => {
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [online, setOnline] = useState(false);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string>('');
  const [currentTask, setCurrentTask] = useState<Task | null>(null);
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [screenshotModalUrl, setScreenshotModalUrl] = useState<string | null>(null);
  const [auditModalOpen, setAuditModalOpen] = useState(false);

  // Keep a ref to activeSessionId to avoid stale closures in WS listener
  const activeSessionIdRef = useRef(activeSessionId);
  activeSessionIdRef.current = activeSessionId;

  // Initialize data and WebSocket connection
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

    return () => {
      unsubStatus();
      unsubEvent();
      wsClient.disconnect();
    };
  }, []);

  const loadInitialData = async () => {
    await loadStatus();
    try {
      const sessList = await fetchSessions();
      setSessions(sessList);
      if (sessList.length > 0) {
        selectSession(sessList[0].id);
      }
    } catch (e) {
      console.error('Failed to load sessions:', e);
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

  const selectSession = async (sessionId: string) => {
    setActiveSessionId(sessionId);
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
        // Fallback for direct session message history
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
    }
  };

  const handleIncomingEvent = (ev: AgentEvent) => {
    // Only append event if it matches currently active session or has no session specified
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

      // Refresh sessions to update sidebar conversation titles
      fetchSessions().then(setSessions).catch(() => {});
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

  const handleNewSession = async () => {
    try {
      const newSess = await createSession();
      setSessions((prev) => [newSess, ...prev]);
      setActiveSessionId(newSess.id);
      setEvents([]);
      setCurrentTask(null);
    } catch (e) {
      console.error('Failed to create session:', e);
    }
  };

  const handleSendPrompt = async (prompt: string) => {
    try {
      let sessId = activeSessionId;
      if (!sessId) {
        const newSess = await createSession();
        setSessions([newSess]);
        sessId = newSess.id;
        setActiveSessionId(sessId);
      }

      // Append user prompt event (RIGHT-aligned with sender: 'user')
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
      fetchSessions().then(setSessions).catch(() => {});
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

  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden bg-[#0d1117] font-sans antialiased text-gray-200">
      <Header status={status} online={online} onOpenAudit={() => setAuditModalOpen(true)} />

      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          sessions={sessions}
          activeSessionId={activeSessionId}
          onSelectSession={(id) => selectSession(id)}
          onNewSession={handleNewSession}
          status={status}
        />

        <main className="flex-1 flex flex-col overflow-hidden">
          <ChatView
            currentTask={currentTask}
            events={events}
            onSend={handleSendPrompt}
            onCancel={handleCancelTask}
            onApprove={handleApprove}
            onViewScreenshot={(url) => setScreenshotModalUrl(url)}
          />
        </main>
      </div>

      {screenshotModalUrl && (
        <ScreenshotModal url={screenshotModalUrl} onClose={() => setScreenshotModalUrl(null)} />
      )}

      {auditModalOpen && <AuditModal onClose={() => setAuditModalOpen(false)} />}
    </div>
  );
};

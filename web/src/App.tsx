import React, { useState, useEffect } from 'react';
import { Header } from './components/Header';
import { Sidebar } from './components/Sidebar';
import { ChatView } from './components/ChatView';
import { ScreenshotModal } from './components/ScreenshotModal';
import { AuditModal } from './components/AuditModal';
import { SystemStatus, Session, Task, AgentEvent } from './types';
import { fetchStatus, fetchSessions, createSession, createTask, cancelTask, approveTask } from './services/api';
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
        setActiveSessionId(sessList[0].id);
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

  const handleIncomingEvent = (ev: AgentEvent) => {
    setEvents((prev) => [...prev, ev]);

    if (ev.type === 'task.status' || ev.type === 'task.completed') {
      if (currentTask && ev.task_id === currentTask.id) {
        setCurrentTask((prev) => (prev ? { ...prev, status: ev.status || prev.status } : null));
      }
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

      // Append user prompt event
      const userEvent: AgentEvent = {
        type: 'agent.message',
        task_id: 'pending',
        session_id: sessId,
        timestamp: new Date().toISOString(),
        message: prompt,
      };
      setEvents((prev) => [...prev, userEvent]);

      const task = await createTask(prompt, sessId);
      setCurrentTask(task);
    } catch (e) {
      console.error('Failed to send task:', e);
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
          onSelectSession={(id) => {
            setActiveSessionId(id);
            setEvents([]);
            setCurrentTask(null);
          }}
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

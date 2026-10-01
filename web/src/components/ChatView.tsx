import React, { useState, useRef, useEffect } from 'react';
import { Task, AgentEvent, Session, Project } from '../types';
import { ToolActivity } from './ToolActivity';
import { ApprovalBanner } from './ApprovalBanner';
import { Button } from './ui/button';
import { ArrowUp, Square, Sparkles, Terminal, Camera, FolderSearch, Loader2, Bot, FolderGit2, ShieldCheck, FolderPlus } from 'lucide-react';

interface ChatViewProps {
  currentTask: Task | null;
  events: AgentEvent[];
  session?: Session | null;
  project?: Project | null;
  onSend: (prompt: string) => void;
  onCancel: (taskId: string) => void;
  onApprove: (taskId: string, approved: boolean) => void;
  onViewScreenshot: (url: string) => void;
  onOpenProjectModal?: () => void;
}

export const ChatView: React.FC<ChatViewProps> = ({
  currentTask,
  events,
  session,
  project,
  onSend,
  onCancel,
  onApprove,
  onViewScreenshot,
  onOpenProjectModal,
}) => {
  const [input, setInput] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const isProjectMode = session?.type === 'project';

  const isRunning =
    currentTask !== null &&
    (currentTask.status === 'running' ||
      currentTask.status === 'waiting_for_approval' ||
      currentTask.status === 'queued');

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [events, currentTask]);

  // Auto-resize textarea height
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 200)}px`;
    }
  }, [input]);

  const handleSend = () => {
    const trimmed = input.trim();
    if (!trimmed || isRunning) return;
    onSend(trimmed);
    setInput('');
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const getGreeting = () => {
    const hour = new Date().getHours();
    if (hour < 12) return 'Good morning';
    if (hour < 18) return 'Good afternoon';
    return 'Good evening';
  };

  return (
    <div className="flex-1 flex flex-col h-full bg-[#09090b] text-zinc-200 overflow-hidden relative">
      {/* Main chat or landing area */}
      <div className="flex-1 overflow-y-auto px-4 md:px-8 py-6">
        {events.length === 0 && !currentTask ? (
          /* Claude-style Landing View */
          <div className="h-full flex flex-col items-center justify-center max-w-2xl mx-auto space-y-6 animate-fadeIn">
            <div className="text-center space-y-2">
              {isProjectMode ? (
                <>
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-950/60 border border-blue-800/40 text-blue-300 text-xs font-medium mb-1">
                    <FolderGit2 className="w-3.5 h-3.5" />
                    <span>Project Workspace</span>
                  </div>
                  <h1 className="text-3xl font-medium tracking-tight text-zinc-100">
                    {project?.name || session?.project_name || 'Project Workspace'}
                  </h1>
                  <p className="text-xs text-zinc-400 font-mono">
                    {project?.path || session?.project_path}
                  </p>

                  {/* Tech stack badges */}
                  {project?.tech_stack && project.tech_stack.length > 0 && (
                    <div className="flex flex-wrap items-center justify-center gap-1.5 pt-1">
                      {project.tech_stack.map((tech) => (
                        <span
                          key={tech}
                          className="px-2 py-0.5 rounded-md bg-zinc-800/90 border border-zinc-700/60 text-zinc-300 text-[11px] font-medium"
                        >
                          {tech}
                        </span>
                      ))}
                    </div>
                  )}

                  {project?.summary && (
                    <p className="text-xs text-zinc-400 max-w-lg mx-auto leading-relaxed pt-1">
                      {project.summary}
                    </p>
                  )}
                </>
              ) : (
                <>
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-zinc-900 border border-zinc-800 text-emerald-400 text-xs font-medium mb-1">
                    <ShieldCheck className="w-3.5 h-3.5" />
                    <span>General Mac Assistant (Read-Only)</span>
                  </div>
                  <h1 className="text-3xl font-medium tracking-tight text-zinc-100">
                    {getGreeting()}
                  </h1>
                  <p className="text-sm text-zinc-500">
                    How can Bob help you on your Mac today? Cannot modify PC files.
                  </p>
                </>
              )}
            </div>

            {/* Central prompt input */}
            <div className="w-full">
              <div className="rounded-2xl border border-zinc-800/80 bg-zinc-900/60 focus-within:border-zinc-700/90 focus-within:bg-zinc-900 shadow-xl transition-all p-3">
                <textarea
                  ref={textareaRef}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder={
                    isProjectMode
                      ? `Ask Bob about this project, search code, run tests, or edit files...`
                      : 'Ask Bob to find info on your PC, take screenshots, or run diagnostics...'
                  }
                  rows={2}
                  className="w-full bg-transparent text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none resize-none px-2 py-1 leading-relaxed max-h-48"
                />

                <div className="flex items-center justify-between pt-2 px-1 border-t border-zinc-850/60 mt-1">
                  <span className="text-[11px] text-zinc-600 font-sans">
                    Press Enter to send, Shift+Enter for new line
                  </span>

                  <Button
                    size="icon"
                    onClick={handleSend}
                    disabled={!input.trim()}
                    className="h-8 w-8 rounded-xl bg-zinc-100 hover:bg-white text-zinc-900 disabled:opacity-30 transition-all"
                  >
                    <ArrowUp className="w-4 h-4 stroke-[2.5]" />
                  </Button>
                </div>
              </div>

              {/* Quick suggestion pills */}
              <div className="flex flex-wrap items-center justify-center gap-2 pt-4">
                {isProjectMode ? (
                  <>
                    <button
                      onClick={() => onSend('Tell me about this project, its purpose, tech stack, and architecture')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-blue-900/50 bg-blue-950/30 hover:bg-blue-900/40 text-xs text-blue-300 hover:text-blue-100 transition-colors"
                    >
                      <Sparkles className="w-3.5 h-3.5 text-blue-400" />
                      <span>Tell me about this project</span>
                    </button>

                    <button
                      onClick={() => onSend('List all project files and explain the directory structure')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
                    >
                      <FolderSearch className="w-3.5 h-3.5 text-zinc-500" />
                      <span>List directory tree</span>
                    </button>

                    <button
                      onClick={() => onSend('Inspect git status and recent commits')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
                    >
                      <Terminal className="w-3.5 h-3.5 text-zinc-500" />
                      <span>Inspect git status</span>
                    </button>
                  </>
                ) : (
                  <>
                    <button
                      onClick={() => onSend('Take a screenshot of the Mac desktop')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
                    >
                      <Camera className="w-3.5 h-3.5 text-zinc-500" />
                      <span>Capture desktop screenshot</span>
                    </button>

                    <button
                      onClick={() => onSend('Inspect Mac system information, memory, and disk usage')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
                    >
                      <Terminal className="w-3.5 h-3.5 text-zinc-500" />
                      <span>Mac System Info</span>
                    </button>

                    {onOpenProjectModal && (
                      <button
                        onClick={onOpenProjectModal}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-blue-900/50 bg-blue-950/30 hover:bg-blue-900/40 text-xs text-blue-300 hover:text-blue-100 transition-colors"
                      >
                        <FolderPlus className="w-3.5 h-3.5 text-blue-400" />
                        <span>Open a Project Folder</span>
                      </button>
                    )}
                  </>
                )}
              </div>
            </div>
          </div>
        ) : (
          /* Active Message Stream */
          <div className="max-w-3xl mx-auto space-y-6 pb-24">
            {events.map((ev, idx) => {
              // 1. User Message (Clean Right-aligned Bubble)
              if (ev.sender === 'user' || ev.type === 'user.message') {
                return (
                  <div key={idx} className="flex justify-end pl-12 animate-fadeIn">
                    <div className="max-w-[85%] rounded-2xl rounded-tr-sm bg-zinc-800/90 border border-zinc-750/50 px-4 py-3 text-sm text-zinc-100 leading-relaxed shadow-sm">
                      <p className="whitespace-pre-wrap">{ev.message}</p>
                    </div>
                  </div>
                );
              }

              // 2. Bob Assistant Message (Clean natural text stream with subtle bot icon)
              if (ev.type === 'agent.message') {
                return (
                  <div key={idx} className="flex items-start gap-3.5 pr-12 animate-fadeIn">
                    <div className="w-7 h-7 rounded-lg bg-zinc-800 border border-zinc-700/60 flex items-center justify-center text-zinc-300 mt-0.5 flex-shrink-0 shadow-sm">
                      <Bot className="w-4 h-4" />
                    </div>
                    <div className="flex-1 space-y-2 text-sm leading-relaxed text-zinc-200 pt-0.5">
                      <p className="whitespace-pre-wrap">{ev.message}</p>
                    </div>
                  </div>
                );
              }

              // 3. Tool Activity
              if (ev.type === 'tool.started' || ev.type === 'tool.completed') {
                return (
                  <div key={idx} className="pl-10.5 pr-2">
                    <ToolActivity event={ev} onViewScreenshot={onViewScreenshot} />
                  </div>
                );
              }

              // 4. Approval Required Banner
              if (ev.type === 'tool.approval_required') {
                return (
                  <div key={idx} className="pl-10.5 pr-2">
                    <ApprovalBanner
                      taskId={ev.task_id}
                      toolName={ev.tool || 'terminal'}
                      command={typeof ev.input === 'string' ? ev.input : JSON.stringify(ev.input)}
                      reason={ev.message || 'Requires security approval'}
                      onApprove={onApprove}
                    />
                  </div>
                );
              }

              // 5. Error message
              if (ev.type === 'agent.error') {
                return (
                  <div key={idx} className="pl-10.5 pr-2 p-3 rounded-xl bg-red-950/20 border border-red-800/40 text-red-300 text-xs">
                    {ev.error}
                  </div>
                );
              }

              return null;
            })}

            {/* Active Reasoning / Thinking Indicator */}
            {isRunning && currentTask?.status === 'running' && (
              <div className="flex items-center gap-2.5 text-xs text-zinc-400 pl-10.5 py-2 animate-pulse">
                <Loader2 className="w-3.5 h-3.5 animate-spin text-zinc-400" />
                <span className="font-medium text-zinc-400">
                  {([...events].reverse().find((e) => e.type === 'agent.thinking' && (!currentTask || e.task_id === currentTask.id))?.message) || 'Bob is working on your request...'}
                </span>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>
        )}
      </div>

      {/* Floating Bottom Input Bar (when conversation is active) */}
      {(events.length > 0 || currentTask) && (
        <div className="p-4 bg-gradient-to-t from-[#09090b] via-[#09090b]/90 to-transparent">
          <div className="max-w-3xl mx-auto rounded-2xl border border-zinc-800/90 bg-zinc-900/90 focus-within:border-zinc-700/90 shadow-xl backdrop-blur-md p-3 transition-all">
            <textarea
              ref={textareaRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              disabled={isRunning}
              placeholder={
                isRunning
                  ? 'Bob is executing your task...'
                  : 'Reply to Bob...'
              }
              rows={1}
              className="w-full bg-transparent text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none resize-none px-2 py-1 leading-relaxed max-h-40 disabled:opacity-60"
            />

            <div className="flex items-center justify-between pt-1.5 px-1">
              <span className="text-[11px] text-zinc-600 font-sans">
                Shift + Enter for new line
              </span>

              {isRunning && currentTask ? (
                <Button
                  size="sm"
                  variant="destructive"
                  onClick={() => onCancel(currentTask.id)}
                  className="h-8 px-3 rounded-xl gap-1.5 text-xs font-medium"
                >
                  <Square className="w-3.5 h-3.5 fill-current" />
                  <span>Stop</span>
                </Button>
              ) : (
                <Button
                  size="icon"
                  onClick={handleSend}
                  disabled={!input.trim()}
                  className="h-8 w-8 rounded-xl bg-zinc-100 hover:bg-white text-zinc-900 disabled:opacity-30 transition-all"
                >
                  <ArrowUp className="w-4 h-4 stroke-[2.5]" />
                </Button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

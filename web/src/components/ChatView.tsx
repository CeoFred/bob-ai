import React, { useState, useRef, useEffect } from 'react';
import { Task, AgentEvent, Session, Project, UserConfig } from '../types';
import { ToolActivity } from './ToolActivity';
import { ApprovalBanner } from './ApprovalBanner';
import { MarkdownRenderer } from './MarkdownRenderer';
import { Button } from './ui/button';
import { ArrowUp, Square, Sparkles, Terminal, Camera, FolderSearch, Loader2, Bot, FolderGit2, ShieldCheck, FolderPlus } from 'lucide-react';

interface ChatViewProps {
  currentTask: Task | null;
  events: AgentEvent[];
  session?: Session | null;
  project?: Project | null;
  user?: UserConfig | null;
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
  user,
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
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 180)}px`;
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
    // Only send on Enter on desktop without shift; on mobile touch keyboards let user use newline unless they tap Send
    if (e.key === 'Enter' && !e.shiftKey && window.innerWidth >= 768) {
      e.preventDefault();
      handleSend();
    }
  };

  const getGreeting = () => {
    const hour = new Date().getHours();
    const timeGreeting = hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening';
    const name = user?.name || 'codemon';
    return `${timeGreeting}, ${name}`;
  };

  return (
    <div className="flex-1 flex flex-col h-full bg-[#09090b] text-zinc-200 overflow-hidden relative">
      {/* Main chat or landing area */}
      <div className="flex-1 overflow-y-auto px-3 sm:px-6 md:px-8 py-4 sm:py-6 overscroll-contain">
        {events.length === 0 && !currentTask ? (
          /* Claude-style Landing View */
          <div className="min-h-full flex flex-col items-center justify-center max-w-2xl mx-auto space-y-5 sm:space-y-6 py-4 animate-fadeIn">
            <div className="text-center space-y-2 px-2">
              {isProjectMode ? (
                <>
                  <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-blue-950/60 border border-blue-800/40 text-blue-300 text-xs font-medium mb-1">
                    <FolderGit2 className="w-3.5 h-3.5" />
                    <span>Project Workspace</span>
                  </div>
                  <h1 className="text-2xl sm:text-3xl font-medium tracking-tight text-zinc-100">
                    {project?.name || session?.project_name || 'Project Workspace'}
                  </h1>
                  <p className="text-[11px] sm:text-xs text-zinc-400 font-mono break-all max-w-md mx-auto">
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
                  <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-zinc-900 border border-zinc-800 text-emerald-400 text-xs font-medium mb-1">
                    <ShieldCheck className="w-3.5 h-3.5" />
                    <span>General Mac Assistant</span>
                  </div>
                  <h1 className="text-2xl sm:text-3xl font-medium tracking-tight text-zinc-100">
                    {getGreeting()}
                  </h1>
                  <p className="text-xs sm:text-sm text-zinc-400 max-w-md mx-auto">
                    How can Bob help you on your Mac today? Full desktop, terminal, file & app control.
                  </p>
                </>
              )}
            </div>

            {/* Central prompt input */}
            <div className="w-full px-1">
              <div className="rounded-2xl border border-zinc-800/80 bg-zinc-900/60 focus-within:border-zinc-700/90 focus-within:bg-zinc-900 shadow-xl transition-all p-2.5 sm:p-3">
                <textarea
                  ref={textareaRef}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder={
                    isProjectMode
                      ? `Ask Bob about this project, search code, or run tests...`
                      : 'Ask Bob to search code, run commands, manage apps, or take screenshots...'
                  }
                  rows={2}
                  className="w-full bg-transparent text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none resize-none px-2 py-1 leading-relaxed max-h-40"
                />

                <div className="flex items-center justify-between pt-2 px-1 border-t border-zinc-850/60 mt-1">
                  <span className="text-[10px] sm:text-[11px] text-zinc-600 font-sans">
                    <span className="hidden sm:inline">Press Enter to send, Shift+Enter for new line</span>
                    <span className="sm:hidden">Ready to assist</span>
                  </span>

                  <Button
                    size="icon"
                    onClick={handleSend}
                    disabled={!input.trim()}
                    className="h-8 w-8 sm:h-8 sm:w-8 rounded-xl bg-zinc-100 hover:bg-white text-zinc-900 disabled:opacity-30 transition-all cursor-pointer"
                    aria-label="Send prompt"
                  >
                    <ArrowUp className="w-4 h-4 stroke-[2.5]" />
                  </Button>
                </div>
              </div>

              {/* Quick suggestion pills */}
              <div className="flex flex-wrap items-center justify-center gap-1.5 sm:gap-2 pt-3 sm:pt-4">
                {isProjectMode ? (
                  <>
                    <button
                      onClick={() => onSend('Tell me about this project, its purpose, tech stack, and architecture')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-blue-900/50 bg-blue-950/30 hover:bg-blue-900/40 text-[11px] sm:text-xs text-blue-300 hover:text-blue-100 transition-colors cursor-pointer"
                    >
                      <Sparkles className="w-3.5 h-3.5 text-blue-400" />
                      <span>About project</span>
                    </button>

                    <button
                      onClick={() => onSend('List all project files and explain the directory structure')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-[11px] sm:text-xs text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer"
                    >
                      <FolderSearch className="w-3.5 h-3.5 text-zinc-500" />
                      <span>Directory tree</span>
                    </button>

                    <button
                      onClick={() => onSend('Inspect git status and recent commits')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-[11px] sm:text-xs text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer"
                    >
                      <Terminal className="w-3.5 h-3.5 text-zinc-500" />
                      <span>Git status</span>
                    </button>
                  </>
                ) : (
                  <>
                    <button
                      onClick={() => onSend('Take a screenshot of the Mac desktop')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-[11px] sm:text-xs text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer"
                    >
                      <Camera className="w-3.5 h-3.5 text-zinc-500" />
                      <span>Desktop screenshot</span>
                    </button>

                    <button
                      onClick={() => onSend('Inspect Mac system information, memory, and disk usage')}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-[11px] sm:text-xs text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer"
                    >
                      <Terminal className="w-3.5 h-3.5 text-zinc-500" />
                      <span>Mac info</span>
                    </button>

                    {onOpenProjectModal && (
                      <button
                        onClick={onOpenProjectModal}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-blue-900/50 bg-blue-950/30 hover:bg-blue-900/40 text-[11px] sm:text-xs text-blue-300 hover:text-blue-100 transition-colors cursor-pointer"
                      >
                        <FolderPlus className="w-3.5 h-3.5 text-blue-400" />
                        <span>Open Folder</span>
                      </button>
                    )}
                  </>
                )}
              </div>
            </div>
          </div>
        ) : (
          /* Active Message Stream */
          <div className="max-w-3xl mx-auto space-y-4 sm:space-y-6 pb-20 sm:pb-24">
            {events.map((ev, idx) => {
              // 1. User Message (Clean Right-aligned Bubble)
              if (ev.sender === 'user' || ev.type === 'user.message') {
                return (
                  <div key={idx} className="flex justify-end pl-6 sm:pl-12 animate-fadeIn">
                    <div className="max-w-[90%] sm:max-w-[85%] rounded-2xl rounded-tr-sm bg-zinc-800/90 border border-zinc-750/50 px-3.5 py-2.5 sm:px-4 sm:py-3 text-xs sm:text-sm text-zinc-100 leading-relaxed shadow-sm break-words">
                      <p className="whitespace-pre-wrap">{ev.message}</p>
                    </div>
                  </div>
                );
              }

              // 2. Bob Assistant Message
              if (ev.type === 'agent.message') {
                return (
                  <div key={idx} className="flex items-start gap-2.5 sm:gap-3.5 pr-4 sm:pr-12 animate-fadeIn">
                    <div className="w-6 h-6 sm:w-7 sm:h-7 rounded-lg bg-zinc-800 border border-zinc-700/60 flex items-center justify-center text-zinc-300 mt-0.5 flex-shrink-0 shadow-sm">
                      <Bot className="w-3.5 h-3.5 sm:w-4 sm:h-4" />
                    </div>
                    <div className="flex-1 space-y-2 text-xs sm:text-sm leading-relaxed text-zinc-200 pt-0.5 min-w-0 break-words">
                      <MarkdownRenderer content={ev.message || ''} onImageClick={onViewScreenshot} />
                    </div>
                  </div>
                );
              }

              // 3. Tool Activity
              if (ev.type === 'tool.started' || ev.type === 'tool.completed') {
                return (
                  <div key={idx} className="pl-8 sm:pl-10.5 pr-1 sm:pr-2">
                    <ToolActivity event={ev} onViewScreenshot={onViewScreenshot} />
                  </div>
                );
              }

              // 4. Approval Required Banner
              if (ev.type === 'tool.approval_required') {
                const isCurrentlyWaiting =
                  currentTask !== null &&
                  currentTask.id === ev.task_id &&
                  currentTask.status === 'waiting_for_approval';

                let approvalStatus: 'pending' | 'approved' | 'rejected' = 'pending';
                if (!isCurrentlyWaiting) {
                  // Check subsequent events or task status
                  const subsequentEvents = events.slice(idx + 1);
                  const completedToolEvent = subsequentEvents.find(
                    (e) => e.task_id === ev.task_id && (e.type === 'tool.completed' || e.type === 'agent.error')
                  );

                  if (
                    completedToolEvent?.error?.toLowerCase().includes('reject') ||
                    completedToolEvent?.output?.toLowerCase().includes('reject') ||
                    (currentTask?.id === ev.task_id && currentTask?.status === 'cancelled')
                  ) {
                    approvalStatus = 'rejected';
                  } else {
                    approvalStatus = 'approved';
                  }
                }

                return (
                  <div key={idx} className="pl-8 sm:pl-10.5 pr-1 sm:pr-2">
                    <ApprovalBanner
                      taskId={ev.task_id}
                      toolName={ev.tool || 'terminal'}
                      command={typeof ev.input === 'string' ? ev.input : JSON.stringify(ev.input)}
                      reason={ev.message || 'Requires security approval'}
                      status={approvalStatus}
                      onApprove={onApprove}
                    />
                  </div>
                );
              }

              // 5. Error message
              if (ev.type === 'agent.error') {
                return (
                  <div key={idx} className="pl-8 sm:pl-10.5 pr-1 sm:pr-2 p-3 rounded-xl bg-red-950/20 border border-red-800/40 text-red-300 text-xs break-words">
                    {ev.error}
                  </div>
                );
              }

              return null;
            })}

            {/* Active Reasoning / Thinking Indicator */}
            {isRunning && currentTask?.status === 'running' && (
              <div className="flex items-center gap-2 text-xs text-zinc-400 pl-8 sm:pl-10.5 py-2 animate-pulse">
                <Loader2 className="w-3.5 h-3.5 animate-spin text-zinc-400 flex-shrink-0" />
                <span className="font-medium text-zinc-400 truncate">
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
        <div className="p-2 sm:p-4 bg-gradient-to-t from-[#09090b] via-[#09090b]/95 to-transparent pb-[max(0.75rem,env(safe-area-inset-bottom))]">
          <div className="max-w-3xl mx-auto rounded-2xl border border-zinc-800/90 bg-zinc-900/90 focus-within:border-zinc-700/90 shadow-xl backdrop-blur-md p-2.5 sm:p-3 transition-all">
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
              className="w-full bg-transparent text-xs sm:text-sm text-zinc-100 placeholder-zinc-500 focus:outline-none resize-none px-1.5 py-1 leading-relaxed max-h-36 disabled:opacity-60"
            />

            <div className="flex items-center justify-between pt-1.5 px-1">
              <span className="text-[10px] sm:text-[11px] text-zinc-600 font-sans hidden xs:inline">
                {isRunning ? 'Processing...' : 'Shift + Enter for new line'}
              </span>

              {isRunning && currentTask ? (
                <Button
                  size="sm"
                  variant="destructive"
                  onClick={() => onCancel(currentTask.id)}
                  className="h-7 sm:h-8 px-2.5 sm:px-3 rounded-xl gap-1.5 text-xs font-medium cursor-pointer ml-auto"
                >
                  <Square className="w-3 h-3 fill-current" />
                  <span>Stop</span>
                </Button>
              ) : (
                <Button
                  size="icon"
                  onClick={handleSend}
                  disabled={!input.trim()}
                  className="h-7 w-7 sm:h-8 sm:w-8 rounded-xl bg-zinc-100 hover:bg-white text-zinc-900 disabled:opacity-30 transition-all cursor-pointer ml-auto"
                  aria-label="Send message"
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

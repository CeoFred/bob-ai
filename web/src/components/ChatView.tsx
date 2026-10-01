import React, { useState, useRef, useEffect } from 'react';
import { Task, AgentEvent } from '../types';
import { ToolActivity } from './ToolActivity';
import { ApprovalBanner } from './ApprovalBanner';
import { Button } from './ui/button';
import { ArrowUp, Square, Sparkles, Terminal, Camera, FolderSearch, Loader2, Bot } from 'lucide-react';

interface ChatViewProps {
  currentTask: Task | null;
  events: AgentEvent[];
  onSend: (prompt: string) => void;
  onCancel: (taskId: string) => void;
  onApprove: (taskId: string, approved: boolean) => void;
  onViewScreenshot: (url: string) => void;
}

export const ChatView: React.FC<ChatViewProps> = ({
  currentTask,
  events,
  onSend,
  onCancel,
  onApprove,
  onViewScreenshot,
}) => {
  const [input, setInput] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

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
          <div className="h-full flex flex-col items-center justify-center max-w-2xl mx-auto space-y-8 animate-fadeIn">
            <div className="text-center space-y-2">
              <h1 className="text-3xl font-medium tracking-tight text-zinc-100">
                {getGreeting()}
              </h1>
              <p className="text-sm text-zinc-500">
                How can Bob help you on your Mac today?
              </p>
            </div>

            {/* Central Claude-style prompt input */}
            <div className="w-full">
              <div className="rounded-2xl border border-zinc-800/80 bg-zinc-900/60 focus-within:border-zinc-700/90 focus-within:bg-zinc-900 shadow-xl transition-all p-3">
                <textarea
                  ref={textareaRef}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Ask Bob to run tasks, check files, or execute commands..."
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
                <button
                  onClick={() => onSend('Run git status and check the repository status')}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
                >
                  <Terminal className="w-3.5 h-3.5 text-zinc-500" />
                  <span>Inspect git status</span>
                </button>

                <button
                  onClick={() => onSend('Take a screenshot of the Mac desktop')}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
                >
                  <Camera className="w-3.5 h-3.5 text-zinc-500" />
                  <span>Capture desktop screenshot</span>
                </button>

                <button
                  onClick={() => onSend('List all files in the current workspace directory')}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
                >
                  <FolderSearch className="w-3.5 h-3.5 text-zinc-500" />
                  <span>List workspace files</span>
                </button>

                <button
                  onClick={() => onSend('Tell me what directory Bob is running from and list the files')}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-850 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
                >
                  <Sparkles className="w-3.5 h-3.5 text-zinc-500" />
                  <span>Test acceptance task</span>
                </button>
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

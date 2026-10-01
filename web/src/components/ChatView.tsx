import React, { useState, useRef, useEffect } from 'react';
import { Task, AgentEvent } from '../types';
import { ToolActivity } from './ToolActivity';
import { ApprovalBanner } from './ApprovalBanner';
import { Send, Square, Bot, User, Sparkles, Terminal, Camera, FolderSearch, Loader2 } from 'lucide-react';

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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = input.trim();
    if (!trimmed || isRunning) return;
    onSend(trimmed);
    setInput('');
  };

  const handleQuickPrompt = (prompt: string) => {
    if (isRunning) return;
    onSend(prompt);
  };

  return (
    <div className="flex-1 flex flex-col h-full bg-[#0d1117] text-gray-200">
      <div className="flex-1 overflow-y-auto p-4 md:p-6 space-y-4">
        {events.length === 0 && !currentTask ? (
          <div className="h-full flex flex-col items-center justify-center text-center max-w-lg mx-auto py-12 space-y-6">
            <div className="p-4 rounded-2xl bg-[#161b22] border border-[#30363d] shadow-xl">
              <Bot className="w-10 h-10 text-blue-400" />
            </div>
            <div className="space-y-2">
              <h2 className="text-xl font-bold text-white">Hey, I'm Bob</h2>
              <p className="text-sm text-gray-400 leading-relaxed">
                Your local AI personal computer agent running on this Mac. I reason about tasks and execute terminal commands,
                filesystem operations, and screen captures securely with real-time audit logging.
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-2.5 w-full pt-4">
              <button
                onClick={() => handleQuickPrompt('Tell me what directory Bob is running from, list the files there, and take a screenshot')}
                className="p-3 rounded-xl bg-[#161b22] hover:bg-[#21262d] border border-[#30363d] text-left transition-all group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-blue-400 group-hover:text-blue-300">
                  <Sparkles className="w-3.5 h-3.5" /> Acceptance Test
                </div>
                <div className="text-[11px] text-gray-400 mt-1 line-clamp-2">
                  "Tell me what directory Bob is running from, list files, and take screenshot."
                </div>
              </button>

              <button
                onClick={() => handleQuickPrompt('Run git status and check the repository status')}
                className="p-3 rounded-xl bg-[#161b22] hover:bg-[#21262d] border border-[#30363d] text-left transition-all group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-emerald-400 group-hover:text-emerald-300">
                  <Terminal className="w-3.5 h-3.5" /> Git Status
                </div>
                <div className="text-[11px] text-gray-400 mt-1">
                  Inspect working tree and active branches.
                </div>
              </button>

              <button
                onClick={() => handleQuickPrompt('Take a screenshot of the Mac desktop')}
                className="p-3 rounded-xl bg-[#161b22] hover:bg-[#21262d] border border-[#30363d] text-left transition-all group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-purple-400 group-hover:text-purple-300">
                  <Camera className="w-3.5 h-3.5" /> Screen Capture
                </div>
                <div className="text-[11px] text-gray-400 mt-1">
                  Capture macOS desktop display.
                </div>
              </button>

              <button
                onClick={() => handleQuickPrompt('List all files in the current workspace directory')}
                className="p-3 rounded-xl bg-[#161b22] hover:bg-[#21262d] border border-[#30363d] text-left transition-all group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-amber-400 group-hover:text-amber-300">
                  <FolderSearch className="w-3.5 h-3.5" /> List Workspace
                </div>
                <div className="text-[11px] text-gray-400 mt-1">
                  Inspect project folders and structure.
                </div>
              </button>
            </div>
          </div>
        ) : (
          <div className="max-w-3xl mx-auto space-y-4">
            {events.map((ev, idx) => {
              // 1. User Message -> Aligned to the RIGHT with User icon & Blue bubble
              if (ev.sender === 'user' || ev.type === 'user.message') {
                return (
                  <div key={idx} className="flex items-start justify-end gap-3 pl-12 animate-fadeIn">
                    <div className="p-3.5 rounded-2xl rounded-tr-sm bg-blue-600 text-white text-sm leading-relaxed max-w-[80%] shadow-md shadow-blue-600/10">
                      <p className="whitespace-pre-wrap">{ev.message}</p>
                    </div>
                    <div className="p-2 rounded-xl bg-blue-500/20 text-blue-400 border border-blue-500/30 mt-0.5 flex-shrink-0" title="You">
                      <User className="w-4 h-4" />
                    </div>
                  </div>
                );
              }

              // 2. Bob Assistant Message -> Aligned to the LEFT with Bot icon & Slate bubble
              if (ev.type === 'agent.message') {
                return (
                  <div key={idx} className="flex items-start justify-start gap-3 pr-12 animate-fadeIn">
                    <div className="p-2 rounded-xl bg-gradient-to-tr from-blue-600 to-indigo-500 text-white mt-0.5 flex-shrink-0 shadow-md shadow-blue-500/20" title="Bob">
                      <Bot className="w-4 h-4" />
                    </div>
                    <div className="p-3.5 rounded-2xl rounded-tl-sm bg-[#161b22] border border-[#30363d] text-sm leading-relaxed text-gray-200 max-w-[85%] shadow-sm">
                      <p className="whitespace-pre-wrap">{ev.message}</p>
                    </div>
                  </div>
                );
              }

              // 3. Tool Activity (Started / Completed)
              if (ev.type === 'tool.started' || ev.type === 'tool.completed') {
                return (
                  <div key={idx} className="pl-11 pr-4">
                    <ToolActivity event={ev} onViewScreenshot={onViewScreenshot} />
                  </div>
                );
              }

              // 4. Approval Required
              if (ev.type === 'tool.approval_required') {
                return (
                  <div key={idx} className="pl-11 pr-4">
                    <ApprovalBanner
                      taskId={ev.task_id}
                      toolName={ev.tool || 'terminal'}
                      command={typeof ev.input === 'string' ? ev.input : JSON.stringify(ev.input)}
                      reason={ev.message || 'Requires manual approval'}
                      onApprove={onApprove}
                    />
                  </div>
                );
              }

              // 5. Agent Thinking Indicator
              if (ev.type === 'agent.thinking') {
                return (
                  <div key={idx} className="flex items-center gap-2 text-xs text-gray-400 pl-11 py-1">
                    <Loader2 className="w-3.5 h-3.5 text-blue-400 animate-spin" />
                    <span>{ev.message || 'Reasoning about next action...'}</span>
                  </div>
                );
              }

              // 6. Error
              if (ev.type === 'agent.error') {
                return (
                  <div key={idx} className="pl-11 pr-4 p-3 rounded-xl bg-rose-950/20 border border-rose-500/30 text-rose-300 text-xs">
                    ⚠️ {ev.error}
                  </div>
                );
              }

              return null;
            })}

            <div ref={messagesEndRef} />
          </div>
        )}
      </div>

      {/* Input bar */}
      <div className="p-4 border-t border-[#30363d] bg-[#161b22]">
        <form onSubmit={handleSubmit} className="max-w-3xl mx-auto flex items-center gap-3">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            disabled={isRunning}
            placeholder={
              isRunning
                ? 'Bob is working on a task...'
                : 'Ask Bob to do something on your Mac (e.g. "Run tests", "List files", "Take screenshot")...'
            }
            className="flex-1 bg-[#0d1117] text-white text-sm px-4 py-3 rounded-xl border border-[#30363d] focus:border-blue-500 focus:outline-none placeholder-gray-500 disabled:opacity-60 transition-all"
          />

          {isRunning && currentTask ? (
            <button
              type="button"
              onClick={() => onCancel(currentTask.id)}
              className="flex items-center gap-1.5 px-5 py-3 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-sm font-semibold transition-all shadow-md shadow-rose-600/20 active:scale-95 cursor-pointer"
            >
              <Square className="w-4 h-4 fill-current" />
              <span>Stop</span>
            </button>
          ) : (
            <button
              type="submit"
              disabled={!input.trim()}
              className="flex items-center gap-1.5 px-5 py-3 rounded-xl bg-blue-600 hover:bg-blue-500 disabled:bg-gray-800 disabled:text-gray-600 text-white text-sm font-semibold transition-all shadow-md shadow-blue-600/20 active:scale-95 cursor-pointer"
            >
              <Send className="w-4 h-4" />
              <span>Send</span>
            </button>
          )}
        </form>
      </div>
    </div>
  );
};

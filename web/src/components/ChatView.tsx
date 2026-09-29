import React, { useState, useRef, useEffect } from 'react';
import { Task, AgentEvent } from '../types';
import { ToolActivity } from './ToolActivity';
import { ApprovalBanner } from './ApprovalBanner';
import { Send, Square, Bot, Sparkles, Terminal, Camera, FolderSearch } from 'lucide-react';

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

  const isRunning = currentTask?.status === 'running' || currentTask?.status === 'waiting_for_approval';

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
                Your local AI computer agent. I run locally on this Apple Silicon Mac and can execute shell commands,
                manipulate project files, capture screenshots, and report findings securely over Tailscale.
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-2.5 w-full pt-4">
              <button
                onClick={() => handleQuickPrompt('Tell me what directory Bob is running from, list the files there, and take a screenshot')}
                className="p-3 rounded-xl bg-[#161b22] hover:bg-[#21262d] border border-[#30363d] text-left transition-all group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-blue-400 group-hover:text-blue-300">
                  <Sparkles className="w-3.5 h-3.5" /> Acceptance Test Prompt
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
                  Check working tree and git branch status.
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
                  Capture macOS desktop state and display image.
                </div>
              </button>

              <button
                onClick={() => handleQuickPrompt('List all Go files in this project and check code structure')}
                className="p-3 rounded-xl bg-[#161b22] hover:bg-[#21262d] border border-[#30363d] text-left transition-all group"
              >
                <div className="flex items-center gap-2 text-xs font-semibold text-amber-400 group-hover:text-amber-300">
                  <FolderSearch className="w-3.5 h-3.5" /> Inspect Project
                </div>
                <div className="text-[11px] text-gray-400 mt-1">
                  Search codebase structure and files.
                </div>
              </button>
            </div>
          </div>
        ) : (
          <div className="max-w-3xl mx-auto space-y-4">
            {events.map((ev, idx) => {
              if (ev.type === 'agent.message') {
                return (
                  <div key={idx} className="flex items-start gap-3">
                    <div className="p-2 rounded-xl bg-[#161b22] border border-[#30363d] text-blue-400 mt-0.5">
                      <Bot className="w-4 h-4" />
                    </div>
                    <div className="flex-1 p-3.5 rounded-2xl bg-[#161b22] border border-[#30363d] text-sm leading-relaxed text-gray-200">
                      <p className="whitespace-pre-wrap">{ev.message}</p>
                    </div>
                  </div>
                );
              }

              if (ev.type === 'tool.started' || ev.type === 'tool.completed') {
                return (
                  <div key={idx} className="pl-10">
                    <ToolActivity event={ev} onViewScreenshot={onViewScreenshot} />
                  </div>
                );
              }

              if (ev.type === 'tool.approval_required') {
                return (
                  <div key={idx} className="pl-10">
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

              if (ev.type === 'agent.thinking') {
                return (
                  <div key={idx} className="flex items-center gap-2 text-xs text-gray-500 pl-11 py-1">
                    <span className="w-2 h-2 rounded-full bg-blue-500 animate-ping" />
                    <span>{ev.message || 'Reasoning...'}</span>
                  </div>
                );
              }

              if (ev.type === 'agent.error') {
                return (
                  <div key={idx} className="pl-10 p-3 rounded-xl bg-rose-950/20 border border-rose-500/30 text-rose-300 text-xs">
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
                : 'Ask Bob to do something on your Mac (e.g. "Run tests", "List files")...'
            }
            className="flex-1 bg-[#0d1117] text-white text-sm px-4 py-3 rounded-xl border border-[#30363d] focus:border-blue-500 focus:outline-none placeholder-gray-500 disabled:opacity-60"
          />

          {isRunning && currentTask ? (
            <button
              type="button"
              onClick={() => onCancel(currentTask.id)}
              className="flex items-center gap-1.5 px-4 py-3 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-sm font-semibold transition-all shadow-md shadow-rose-600/20"
            >
              <Square className="w-4 h-4 fill-current" />
              <span>Stop</span>
            </button>
          ) : (
            <button
              type="submit"
              disabled={!input.trim()}
              className="flex items-center gap-1.5 px-5 py-3 rounded-xl bg-blue-600 hover:bg-blue-500 disabled:bg-gray-800 disabled:text-gray-600 text-white text-sm font-semibold transition-all shadow-md shadow-blue-600/20"
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

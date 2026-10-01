import React from 'react';
import { Session } from '../types';
import { Button } from './ui/button';
import { Plus, MessageSquare, Laptop, Bot } from 'lucide-react';
import { cn } from '../lib/utils';

interface SidebarProps {
  sessions: Session[];
  activeSessionId: string;
  onSelectSession: (id: string) => void;
  onNewSession: () => void;
  open: boolean;
}

export const Sidebar: React.FC<SidebarProps> = ({
  sessions,
  activeSessionId,
  onSelectSession,
  onNewSession,
  open,
}) => {
  if (!open) return null;

  return (
    <aside className="w-64 bg-zinc-950 border-r border-zinc-850 flex flex-col h-full text-zinc-300 select-none flex-shrink-0 animate-in slide-in-from-left duration-200">
      {/* New chat action */}
      <div className="p-3">
        <Button
          variant="outline"
          onClick={onNewSession}
          className="w-full justify-start gap-2.5 h-10 px-3 bg-zinc-900/50 hover:bg-zinc-900 border-zinc-800 text-zinc-200 hover:text-white rounded-xl shadow-none text-xs font-medium"
        >
          <Plus className="w-4 h-4 text-zinc-400" />
          <span>New chat</span>
        </Button>
      </div>

      {/* Session list */}
      <div className="flex-1 overflow-y-auto px-2 py-1 space-y-0.5">
        <div className="text-[11px] font-medium text-zinc-500 px-3 py-2 uppercase tracking-wider">
          Recent chats
        </div>

        {sessions.length === 0 ? (
          <div className="text-xs text-zinc-600 px-3 py-6 text-center">
            No conversations yet
          </div>
        ) : (
          sessions.map((sess) => {
            const isActive = sess.id === activeSessionId;
            return (
              <button
                key={sess.id}
                onClick={() => onSelectSession(sess.id)}
                className={cn(
                  'w-full text-left px-3 py-2 rounded-lg text-xs flex items-center gap-2.5 transition-colors group cursor-pointer',
                  isActive
                    ? 'bg-zinc-800/80 text-zinc-100 font-medium'
                    : 'text-zinc-400 hover:bg-zinc-900/80 hover:text-zinc-200'
                )}
              >
                <MessageSquare
                  className={cn(
                    'w-3.5 h-3.5 flex-shrink-0 transition-colors',
                    isActive ? 'text-zinc-300' : 'text-zinc-600 group-hover:text-zinc-400'
                  )}
                />
                <span className="truncate flex-1">{sess.title || 'New conversation'}</span>
              </button>
            );
          })
        )}
      </div>

      {/* Minimal Footer */}
      <div className="p-3 border-t border-zinc-850/80 bg-zinc-950 flex items-center justify-between text-xs text-zinc-500">
        <div className="flex items-center gap-2">
          <div className="p-1 rounded-md bg-zinc-900 text-zinc-400">
            <Laptop className="w-3.5 h-3.5" />
          </div>
          <span className="text-[11px] text-zinc-400 truncate">Mac Local Agent</span>
        </div>
        <div className="flex items-center gap-1 text-[11px] text-zinc-500">
          <Bot className="w-3 h-3 text-zinc-500" />
          <span>Bob</span>
        </div>
      </div>
    </aside>
  );
};

import React from 'react';
import { Session, SystemStatus } from '../types';
import { Plus, MessageSquare, Shield, Terminal, FolderOpen, Camera } from 'lucide-react';

interface SidebarProps {
  sessions: Session[];
  activeSessionId: string;
  onSelectSession: (id: string) => void;
  onNewSession: () => void;
  status: SystemStatus | null;
}

export const Sidebar: React.FC<SidebarProps> = ({
  sessions,
  activeSessionId,
  onSelectSession,
  onNewSession,
  status,
}) => {
  return (
    <aside className="w-72 bg-[#161b22] border-r border-[#30363d] flex flex-col h-full text-gray-300 select-none">
      <div className="p-3 border-b border-[#30363d]">
        <button
          onClick={onNewSession}
          className="w-full flex items-center justify-center gap-2 py-2 px-4 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-medium text-sm transition-all shadow-sm"
        >
          <Plus className="w-4 h-4" />
          <span>New Session</span>
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-2 space-y-1">
        <div className="text-[11px] font-semibold uppercase text-gray-500 tracking-wider px-3 py-1.5">
          Conversations
        </div>
        {sessions.length === 0 ? (
          <div className="text-xs text-gray-500 px-3 py-4 text-center">No active sessions</div>
        ) : (
          sessions.map((sess) => {
            const isActive = sess.id === activeSessionId;
            return (
              <button
                key={sess.id}
                onClick={() => onSelectSession(sess.id)}
                className={`w-full text-left px-3 py-2 rounded-lg text-xs flex items-center gap-2.5 transition-colors ${
                  isActive
                    ? 'bg-[#1f6feb]/20 text-blue-300 font-medium border border-blue-500/30'
                    : 'hover:bg-[#21262d] text-gray-400 hover:text-gray-200'
                }`}
              >
                <MessageSquare className={`w-3.5 h-3.5 flex-shrink-0 ${isActive ? 'text-blue-400' : 'text-gray-500'}`} />
                <span className="truncate">{sess.title || 'Untitled Session'}</span>
              </button>
            );
          })
        )}
      </div>

      {status && (
        <div className="p-3 border-t border-[#30363d] bg-[#0d1117]/50 text-xs space-y-2">
          <div className="flex items-center justify-between text-[11px] font-semibold text-gray-400 uppercase tracking-wider">
            <span>Capabilities</span>
            <span className="text-[10px] text-emerald-400">Active</span>
          </div>
          <div className="space-y-1.5 text-[11px] text-gray-400">
            <div className="flex items-center gap-2">
              <Terminal className="w-3.5 h-3.5 text-blue-400" />
              <span>Zsh Terminal Execution</span>
            </div>
            <div className="flex items-center gap-2">
              <FolderOpen className="w-3.5 h-3.5 text-amber-400" />
              <span>Workspace Filesystem</span>
            </div>
            <div className="flex items-center gap-2">
              <Camera className="w-3.5 h-3.5 text-purple-400" />
              <span>Screen Capture</span>
            </div>
            <div className="flex items-center gap-2">
              <Shield className="w-3.5 h-3.5 text-emerald-400" />
              <span>Command Security Policy</span>
            </div>
          </div>
        </div>
      )}
    </aside>
  );
};

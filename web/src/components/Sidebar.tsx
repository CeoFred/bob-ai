import React from 'react';
import { Session, ProjectWithSessions } from '../types';
import { Plus, MessageSquare, Laptop, Bot, FolderGit2, FolderPlus, Trash2 } from 'lucide-react';
import { cn } from '../lib/utils';

interface SidebarProps {
  projects: ProjectWithSessions[];
  sessions: Session[];
  activeSessionId: string;
  onSelectSession: (id: string) => void;
  onNewConversation: () => void;
  onOpenProjectModal: () => void;
  onNewProjectThread: (projectId: string) => void;
  onDeleteSession: (id: string) => void;
  onDeleteProject?: (id: string) => void;
  open: boolean;
}

export const Sidebar: React.FC<SidebarProps> = ({
  projects,
  sessions,
  activeSessionId,
  onSelectSession,
  onNewConversation,
  onOpenProjectModal,
  onNewProjectThread,
  onDeleteSession,
  onDeleteProject,
  open,
}) => {
  if (!open) return null;

  // Standalone general conversation sessions
  const standaloneSessions = sessions.filter((s) => s.type !== 'project' && !s.project_id);

  return (
    <aside className="w-64 bg-zinc-950 border-r border-zinc-850 flex flex-col h-full text-zinc-300 select-none flex-shrink-0 animate-in slide-in-from-left duration-200">
      {/* Dual New Actions */}
      <div className="p-3 space-y-2">
        <button
          onClick={onNewConversation}
          className="flex items-center gap-2.5 h-9 px-3 w-full bg-zinc-900/60 hover:bg-zinc-900 border border-zinc-800 text-zinc-200 hover:text-white rounded-xl text-xs font-medium transition-colors cursor-pointer"
        >
          <Plus className="w-3.5 h-3.5 text-zinc-400" />
          <span className="flex-1 text-left">New Conversation</span>
          <span className="text-[10px] text-zinc-500 font-normal">Read-Only</span>
        </button>

        <button
          onClick={onOpenProjectModal}
          className="flex items-center gap-2.5 h-9 px-3 w-full bg-blue-950/30 hover:bg-blue-900/40 border border-blue-900/40 text-blue-200 hover:text-white rounded-xl text-xs font-medium transition-colors cursor-pointer"
        >
          <FolderPlus className="w-3.5 h-3.5 text-blue-400" />
          <span className="flex-1 text-left">Open Project</span>
          <span className="text-[10px] bg-blue-900/60 text-blue-300 px-1.5 py-0.5 rounded font-mono">
            Scoped
          </span>
        </button>
      </div>

      {/* Sessions list */}
      <div className="flex-1 overflow-y-auto px-2 py-1 space-y-4">
        {/* Projects Section */}
        {projects.length > 0 && (
          <div className="space-y-3">
            <div className="text-[10px] font-semibold text-zinc-500 px-3 uppercase tracking-wider flex items-center justify-between">
              <span>Projects ({projects.length})</span>
            </div>

            {projects.map((proj) => {
              const isAnySessionActive = proj.sessions.some((s) => s.id === activeSessionId);

              return (
                <div key={proj.id} className="space-y-1 rounded-xl bg-zinc-900/30 border border-zinc-850/60 p-1.5 group/proj">
                  {/* Project Header Row */}
                  <div className="flex items-center justify-between px-2 py-1 text-xs text-zinc-200">
                    <div className="flex items-center gap-2 min-w-0 flex-1" title={proj.path}>
                      <FolderGit2 className={cn('w-3.5 h-3.5 flex-shrink-0', isAnySessionActive ? 'text-blue-400' : 'text-zinc-400')} />
                      <span className="font-semibold text-xs text-zinc-200 truncate">{proj.name}</span>
                    </div>

                    <div className="flex items-center gap-1">
                      {onDeleteProject && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            if (confirm(`Remove project "${proj.name}" from Bob?`)) {
                              onDeleteProject(proj.id);
                            }
                          }}
                          className="h-5 w-5 rounded flex items-center justify-center text-zinc-500 hover:text-red-400 opacity-0 group-hover/proj:opacity-100 hover:bg-zinc-800 transition-all"
                          title="Remove project"
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      )}
                      <button
                        onClick={() => onNewProjectThread(proj.id)}
                        className="h-5 w-5 rounded flex items-center justify-center text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
                        title="New chat in this project"
                      >
                        <Plus className="w-3 h-3" />
                      </button>
                    </div>
                  </div>

                  {/* Project Chats List */}
                  <div className="space-y-0.5 pl-2 border-l border-zinc-800/80 ml-2 mt-0.5">
                    {proj.sessions.length === 0 ? (
                      <button
                        onClick={() => onNewProjectThread(proj.id)}
                        className="w-full text-left px-2 py-1.5 rounded-lg text-[11px] text-zinc-500 hover:text-zinc-300 hover:bg-zinc-855/50 transition-colors"
                      >
                        + Start conversation
                      </button>
                    ) : (
                      proj.sessions.map((sess) => {
                        const isActive = sess.id === activeSessionId;
                        return (
                          <div
                            key={sess.id}
                            className={cn(
                              'group/item relative flex items-center rounded-lg transition-colors',
                              isActive
                                ? 'bg-blue-950/60 text-blue-200 font-medium border border-blue-800/50'
                                : 'text-zinc-400 hover:bg-zinc-850/60 hover:text-zinc-200'
                            )}
                          >
                            <a
                              href={`/c/${sess.id}`}
                              onClick={(e) => {
                                if (!e.metaKey && !e.ctrlKey && !e.shiftKey) {
                                  e.preventDefault();
                                  onSelectSession(sess.id);
                                }
                              }}
                              className="w-full text-left px-2 py-1.5 text-xs flex items-center gap-2 cursor-pointer no-underline min-w-0 pr-7"
                            >
                              <MessageSquare
                                className={cn(
                                  'w-3 h-3 flex-shrink-0 transition-colors',
                                  isActive ? 'text-blue-400' : 'text-zinc-500 group-hover/item:text-zinc-300'
                                )}
                              />
                              <span className="truncate flex-1 text-[11px]">{sess.title || 'New thread'}</span>
                            </a>
                            <button
                              onClick={(e) => {
                                e.preventDefault();
                                e.stopPropagation();
                                onDeleteSession(sess.id);
                              }}
                              className="absolute right-1.5 top-1/2 -translate-y-1/2 p-1 rounded text-zinc-500 hover:text-red-400 opacity-0 group-hover/item:opacity-100 hover:bg-zinc-800/80 transition-all cursor-pointer"
                              title="Delete conversation"
                            >
                              <Trash2 className="w-3 h-3" />
                            </button>
                          </div>
                        );
                      })
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Standalone Conversations Section */}
        <div className="space-y-0.5">
          <div className="text-[10px] font-semibold text-zinc-500 px-3 py-1 uppercase tracking-wider flex items-center justify-between">
            <span>Global Chats</span>
            <span className="text-zinc-600">{standaloneSessions.length}</span>
          </div>

          {standaloneSessions.length === 0 && projects.length === 0 ? (
            <div className="text-xs text-zinc-600 px-3 py-6 text-center">
              No conversations yet
            </div>
          ) : (
            standaloneSessions.map((sess) => {
              const isActive = sess.id === activeSessionId;
              return (
                <div
                  key={sess.id}
                  className={cn(
                    'group/item relative flex items-center rounded-lg transition-colors',
                    isActive
                      ? 'bg-zinc-800/80 text-zinc-100 font-medium'
                      : 'text-zinc-400 hover:bg-zinc-900/80 hover:text-zinc-200'
                  )}
                >
                  <a
                    href={`/c/${sess.id}`}
                    onClick={(e) => {
                      if (!e.metaKey && !e.ctrlKey && !e.shiftKey) {
                        e.preventDefault();
                        onSelectSession(sess.id);
                      }
                    }}
                    className="w-full text-left px-3 py-2 text-xs flex items-center gap-2.5 cursor-pointer no-underline min-w-0 pr-7"
                  >
                    <MessageSquare
                      className={cn(
                        'w-3.5 h-3.5 flex-shrink-0 transition-colors',
                        isActive ? 'text-zinc-300' : 'text-zinc-600 group-hover/item:text-zinc-400'
                      )}
                    />
                    <span className="truncate flex-1">{sess.title || 'New conversation'}</span>
                  </a>
                  <button
                    onClick={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      onDeleteSession(sess.id);
                    }}
                    className="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded text-zinc-500 hover:text-red-400 opacity-0 group-hover/item:opacity-100 hover:bg-zinc-800/80 transition-all cursor-pointer"
                    title="Delete conversation"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </div>
              );
            })
          )}
        </div>
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

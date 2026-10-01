import React from 'react';
import { Session, ProjectWithSessions } from '../types';
import { Plus, MessageSquare, Laptop, Bot, FolderGit2, FolderPlus, Trash2, X } from 'lucide-react';
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
  onClose?: () => void;
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
  onClose,
}) => {
  if (!open) return null;

  // Standalone general conversation sessions
  const standaloneSessions = sessions.filter((s) => s.type !== 'project' && !s.project_id);

  const handleSelectSession = (id: string) => {
    onSelectSession(id);
    if (window.innerWidth < 768 && onClose) {
      onClose();
    }
  };

  const handleNewConversation = () => {
    onNewConversation();
    if (window.innerWidth < 768 && onClose) {
      onClose();
    }
  };

  const handleOpenProjectModal = () => {
    onOpenProjectModal();
    if (window.innerWidth < 768 && onClose) {
      onClose();
    }
  };

  const handleNewProjectThread = (projectId: string) => {
    onNewProjectThread(projectId);
    if (window.innerWidth < 768 && onClose) {
      onClose();
    }
  };

  return (
    <>
      {/* Mobile Backdrop Overlay */}
      <div
        className="fixed inset-0 bg-black/70 backdrop-blur-xs z-40 md:hidden animate-fadeIn"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Sidebar Panel */}
      <aside className={cn(
        "bg-zinc-950 border-r border-zinc-850 flex flex-col h-full text-zinc-300 select-none flex-shrink-0 z-50",
        // Mobile layout: Drawer overlay
        "fixed inset-y-0 left-0 w-72 sm:w-80 shadow-2xl animate-slideInLeft",
        // Desktop layout: Relative flex column
        "md:static md:w-64 md:shadow-none md:animate-none"
      )}>
        {/* Mobile Header with Close Button */}
        <div className="md:hidden flex items-center justify-between px-3.5 pb-3 pt-[calc(0.75rem+env(safe-area-inset-top,0px))] border-b border-zinc-850/80 bg-zinc-950">
          <div className="flex items-center gap-2">
            <Bot className="w-4 h-4 text-emerald-400" />
            <span className="font-semibold text-sm text-zinc-100">Bob Assistant</span>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
            aria-label="Close sidebar"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Dual New Actions */}
        <div className="p-3 space-y-2 flex-shrink-0">
          <button
            onClick={handleNewConversation}
            className="flex items-center gap-2.5 h-10 md:h-9 px-3 w-full bg-zinc-900/70 hover:bg-zinc-900 border border-zinc-800 text-zinc-200 hover:text-white rounded-xl text-xs font-medium transition-colors cursor-pointer"
          >
            <Plus className="w-4 h-4 md:w-3.5 md:h-3.5 text-zinc-400" />
            <span className="flex-1 text-left">New Conversation</span>
            <span className="text-[10px] text-zinc-500 font-normal">General</span>
          </button>

          <button
            onClick={handleOpenProjectModal}
            className="flex items-center gap-2.5 h-10 md:h-9 px-3 w-full bg-blue-950/30 hover:bg-blue-900/40 border border-blue-900/40 text-blue-200 hover:text-white rounded-xl text-xs font-medium transition-colors cursor-pointer"
          >
            <FolderPlus className="w-4 h-4 md:w-3.5 md:h-3.5 text-blue-400" />
            <span className="flex-1 text-left">Open Project</span>
            <span className="text-[10px] bg-blue-900/60 text-blue-300 px-1.5 py-0.5 rounded font-mono">
              Scoped
            </span>
          </button>
        </div>

        {/* Sessions list */}
        <div className="flex-1 overflow-y-auto px-2 py-1 space-y-4 overscroll-contain">
          {/* Projects Section */}
          {projects.length > 0 && (
            <div className="space-y-3">
              <div className="text-[10px] font-semibold text-zinc-500 px-3 uppercase tracking-wider flex items-center justify-between">
                <span>Projects ({projects.length})</span>
              </div>

              {projects.map((proj) => {
                const isAnySessionActive = proj.sessions.some((s) => s.id === activeSessionId);

                return (
                  <div key={proj.id} className="space-y-1 rounded-xl bg-zinc-900/40 border border-zinc-850/60 p-1.5 group/proj">
                    {/* Project Header Row */}
                    <div className="flex items-center justify-between px-2 py-1.5 text-xs text-zinc-200">
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
                            className="h-6 w-6 rounded flex items-center justify-center text-zinc-500 hover:text-red-400 opacity-80 md:opacity-0 md:group-hover/proj:opacity-100 hover:bg-zinc-800 transition-all cursor-pointer"
                            title="Remove project"
                            aria-label={`Remove project ${proj.name}`}
                          >
                            <Trash2 className="w-3 h-3" />
                          </button>
                        )}
                        <button
                          onClick={() => handleNewProjectThread(proj.id)}
                          className="h-6 w-6 rounded flex items-center justify-center text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors cursor-pointer"
                          title="New chat in this project"
                          aria-label="New chat in this project"
                        >
                          <Plus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>

                    {/* Project Chats List */}
                    <div className="space-y-0.5 pl-2 border-l border-zinc-800/80 ml-2 mt-0.5">
                      {proj.sessions.length === 0 ? (
                        <button
                          onClick={() => handleNewProjectThread(proj.id)}
                          className="w-full text-left px-2 py-1.5 rounded-lg text-[11px] text-zinc-500 hover:text-zinc-300 hover:bg-zinc-850/50 transition-colors"
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
                                    handleSelectSession(sess.id);
                                  }
                                }}
                                className="w-full text-left px-2.5 py-2 md:py-1.5 text-xs flex items-center gap-2 cursor-pointer no-underline min-w-0 pr-8"
                              >
                                <MessageSquare
                                  className={cn(
                                    'w-3.5 h-3.5 md:w-3 md:h-3 flex-shrink-0 transition-colors',
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
                                className="absolute right-1.5 top-1/2 -translate-y-1/2 p-1.5 rounded text-zinc-500 hover:text-red-400 opacity-80 md:opacity-0 md:group-hover/item:opacity-100 hover:bg-zinc-800/80 transition-all cursor-pointer"
                                title="Delete conversation"
                                aria-label="Delete conversation"
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
                          handleSelectSession(sess.id);
                        }
                      }}
                      className="w-full text-left px-3 py-2.5 md:py-2 text-xs flex items-center gap-2.5 cursor-pointer no-underline min-w-0 pr-8"
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
                      className="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 rounded text-zinc-500 hover:text-red-400 opacity-80 md:opacity-0 md:group-hover/item:opacity-100 hover:bg-zinc-800/80 transition-all cursor-pointer"
                      title="Delete conversation"
                      aria-label="Delete conversation"
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
        <div className="p-3 border-t border-zinc-850/80 bg-zinc-950 flex items-center justify-between text-xs text-zinc-500 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
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
    </>
  );
};

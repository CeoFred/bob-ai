import React, { useState, useEffect } from 'react';
import { SystemStatus, Session } from '../types';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { Tooltip } from './ui/tooltip';
import {
  PanelLeft,
  Shield,
  Activity,
  Cpu,
  HardDrive,
  Globe,
  Link as LinkIcon,
  Check,
  FolderGit2,
  ShieldCheck,
  Download,
  Plus,
} from 'lucide-react';
import { promptPWAInstall, onPWAInstallChange } from '../lib/pwa';

interface HeaderProps {
  status: SystemStatus | null;
  online: boolean;
  onOpenAudit: () => void;
  sidebarOpen: boolean;
  onToggleSidebar: () => void;
  activeSession?: Session | null;
  onNewChat?: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  status,
  online,
  onOpenAudit,
  sidebarOpen,
  onToggleSidebar,
  activeSession,
  onNewChat,
}) => {
  const [showSystemDetails, setShowSystemDetails] = useState(false);
  const [copied, setCopied] = useState(false);
  const [canInstallPWA, setCanInstallPWA] = useState(false);

  useEffect(() => {
    return onPWAInstallChange((installable) => {
      setCanInstallPWA(installable);
    });
  }, []);

  const activeSessionId = activeSession?.id;

  const handleCopyLink = async () => {
    if (!activeSessionId) return;
    const url = `${window.location.origin}/c/${activeSessionId}`;
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (e) {
      console.error('Failed to copy chat URL:', e);
    }
  };

  const handleInstallClick = async () => {
    await promptPWAInstall();
  };

  return (
    <header className="w-full border-b border-zinc-800/80 bg-zinc-950/90 backdrop-blur-md select-none z-30 flex-shrink-0 pt-[env(safe-area-inset-top,0px)] pl-[env(safe-area-inset-left,0px)] pr-[env(safe-area-inset-right,0px)]">
      <div className="flex items-center justify-between px-3 sm:px-4 h-13 sm:h-14 w-full">
        {/* Left section: Sidebar toggle & Title */}
        <div className="flex items-center gap-2 sm:gap-3 min-w-0">
          <Button
            variant="ghost"
            size="icon"
            onClick={onToggleSidebar}
            className="text-zinc-400 hover:text-zinc-100 hover:bg-zinc-850 h-9 w-9 sm:h-8 sm:w-8 cursor-pointer flex-shrink-0"
            title={sidebarOpen ? 'Close sidebar' : 'Open sidebar'}
            aria-label="Toggle navigation menu"
          >
            <PanelLeft className="w-4 h-4 sm:w-4 sm:h-4" />
          </Button>

          <div className="flex items-center gap-1.5 sm:gap-2.5 min-w-0">
            <a
              href="/"
              onClick={(e) => {
                if (onNewChat && !e.metaKey && !e.ctrlKey && !e.shiftKey) {
                  e.preventDefault();
                  onNewChat();
                }
              }}
              className="font-semibold text-sm sm:text-base tracking-tight text-zinc-100 hover:text-white transition-colors cursor-pointer no-underline flex items-center gap-1.5 flex-shrink-0"
            >
              <span className="w-2 h-2 rounded-full bg-emerald-500 sm:hidden inline-block" />
              <span>Bob</span>
            </a>

            {activeSession && (
              <>
                <span className="text-zinc-600 hidden sm:inline">/</span>
                {activeSession.type === 'project' ? (
                  <div className="flex items-center gap-1 sm:gap-1.5 px-1.5 sm:px-2 py-0.5 rounded-lg bg-blue-950/50 border border-blue-800/40 text-blue-200 text-xs truncate max-w-[120px] xs:max-w-[170px] sm:max-w-[320px]">
                    <FolderGit2 className="w-3.5 h-3.5 text-blue-400 flex-shrink-0" />
                    <span className="font-medium truncate">{activeSession.project_name || activeSession.title}</span>
                    <span className="text-[10px] text-blue-400/80 font-mono hidden md:inline truncate" title={activeSession.project_path}>
                      ({activeSession.project_path})
                    </span>
                  </div>
                ) : (
                  <div className="flex items-center gap-1 sm:gap-1.5 px-1.5 sm:px-2 py-0.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-300 text-xs truncate max-w-[120px] xs:max-w-[170px] sm:max-w-[320px]">
                    <ShieldCheck className="w-3.5 h-3.5 text-emerald-400 flex-shrink-0" />
                    <span className="truncate">{activeSession.title || 'Conversation'}</span>
                  </div>
                )}

                {activeSessionId && (
                  <Tooltip content={copied ? 'Link copied!' : 'Copy chat link'}>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={handleCopyLink}
                      className="h-7 w-7 sm:h-6 sm:w-6 text-zinc-500 hover:text-zinc-200 hover:bg-zinc-800 rounded-md transition-colors flex-shrink-0"
                      aria-label="Copy session link"
                    >
                      {copied ? (
                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                      ) : (
                        <LinkIcon className="w-3.5 h-3.5" />
                      )}
                    </Button>
                  </Tooltip>
                )}
              </>
            )}
          </div>
        </div>

        {/* Right section: System Telemetry, PWA Install, Audit, Status */}
        <div className="flex items-center gap-1 sm:gap-2">
          {/* Mobile New Chat quick button when in active chat */}
          {activeSession && onNewChat && (
            <Button
              variant="ghost"
              size="icon"
              onClick={onNewChat}
              className="sm:hidden h-8 w-8 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-850 rounded-lg cursor-pointer"
              title="New Chat"
              aria-label="Start new conversation"
            >
              <Plus className="w-4 h-4" />
            </Button>
          )}

          {/* PWA Install Button if available */}
          {canInstallPWA && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleInstallClick}
              className="h-7 sm:h-8 px-2 sm:px-2.5 text-xs text-blue-300 border-blue-800/60 bg-blue-950/40 hover:bg-blue-900/60 gap-1 sm:gap-1.5"
              title="Install Bob PWA to Home Screen"
            >
              <Download className="w-3.5 h-3.5 text-blue-400" />
              <span className="hidden xs:inline font-medium">Install App</span>
            </Button>
          )}

          {/* System Info Popover Toggle */}
          {status?.system && (
            <div className="relative">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setShowSystemDetails(!showSystemDetails)}
                className="text-xs text-zinc-400 hover:text-zinc-200 h-8 px-2 sm:px-2.5 gap-1.5"
                aria-label="System status"
              >
                <Activity className="w-3.5 h-3.5 text-zinc-400" />
                <span className="hidden md:inline font-mono text-[11px] text-zinc-300">
                  {status.llm.model.replace('qwen2.5-coder:', 'qwen-coder:').split(':')[0]}
                </span>
              </Button>

              {showSystemDetails && (
                <>
                  <div
                    className="fixed inset-0 z-30"
                    onClick={() => setShowSystemDetails(false)}
                  />
                  <div className="absolute right-0 top-full mt-2 w-[calc(100vw-2rem)] sm:w-72 max-w-sm p-3 rounded-xl bg-zinc-900 border border-zinc-800 shadow-2xl z-40 space-y-2.5 text-xs text-zinc-300 animate-fadeIn">
                    <div className="flex items-center justify-between pb-2 border-b border-zinc-800 text-zinc-400 font-medium">
                      <span>Host Telemetry</span>
                      <Badge variant="outline" className="text-[10px] font-mono">
                        macOS
                      </Badge>
                    </div>

                    <div className="space-y-1.5 text-[11px]">
                      <div className="flex items-center justify-between">
                        <span className="text-zinc-500 flex items-center gap-1.5">
                          <Cpu className="w-3 h-3 text-zinc-400" /> CPU:
                        </span>
                        <span className="font-mono text-zinc-200">
                          {status.system.cpu_model || status.system.architecture}
                        </span>
                      </div>

                      <div className="flex items-center justify-between">
                        <span className="text-zinc-500 flex items-center gap-1.5">
                          <HardDrive className="w-3 h-3 text-zinc-400" /> RAM:
                        </span>
                        <span className="font-mono text-zinc-200">{status.system.ram_formatted}</span>
                      </div>

                      <div className="flex items-center justify-between">
                        <span className="text-zinc-500 flex items-center gap-1.5">
                          <Globe className="w-3 h-3 text-zinc-400" /> Tailscale:
                        </span>
                        <span className="font-mono text-zinc-200">
                          {status.tailscale.ip || 'Not connected'}
                        </span>
                      </div>

                      <div className="flex items-center justify-between pt-1 border-t border-zinc-800/60">
                        <span className="text-zinc-500">Local Model:</span>
                        <span className="font-mono text-emerald-400">{status.llm.model}</span>
                      </div>
                    </div>
                  </div>
                </>
              )}
            </div>
          )}

          {/* Audit Log button */}
          <Tooltip content="Audit Trail">
            <Button
              variant="ghost"
              size="sm"
              onClick={onOpenAudit}
              className="text-xs text-zinc-400 hover:text-zinc-200 h-8 px-2 sm:px-2.5 gap-1.5"
              aria-label="Open audit logs"
            >
              <Shield className="w-3.5 h-3.5 text-zinc-400" />
              <span className="hidden sm:inline">Audit</span>
            </Button>
          </Tooltip>

          {/* Connection status indicator */}
          <div className="flex items-center gap-1.5 px-2 sm:px-2.5 py-1 rounded-full bg-zinc-900/80 border border-zinc-800 text-[11px] flex-shrink-0">
            <span
              className={`w-2 h-2 rounded-full ${
                online ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]' : 'bg-red-500'
              }`}
            />
            <span className="text-zinc-400 font-medium hidden xs:inline">
              {online ? 'Online' : 'Offline'}
            </span>
          </div>
        </div>
      </div>
    </header>
  );
};

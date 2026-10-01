import React, { useState } from 'react';
import { SystemStatus } from '../types';
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
} from 'lucide-react';

interface HeaderProps {
  status: SystemStatus | null;
  online: boolean;
  onOpenAudit: () => void;
  sidebarOpen: boolean;
  onToggleSidebar: () => void;
  activeSessionTitle?: string;
}

export const Header: React.FC<HeaderProps> = ({
  status,
  online,
  onOpenAudit,
  sidebarOpen,
  onToggleSidebar,
  activeSessionTitle,
}) => {
  const [showSystemDetails, setShowSystemDetails] = useState(false);

  return (
    <header className="flex items-center justify-between px-4 h-13 border-b border-zinc-800/80 bg-zinc-950/70 backdrop-blur-md select-none z-20">
      {/* Left section: Sidebar toggle & Title */}
      <div className="flex items-center gap-3 min-w-0">
        <Button
          variant="ghost"
          size="icon"
          onClick={onToggleSidebar}
          className="text-zinc-400 hover:text-zinc-100 hover:bg-zinc-850 h-8 w-8"
          title={sidebarOpen ? 'Close sidebar' : 'Open sidebar'}
        >
          <PanelLeft className="w-4 h-4" />
        </Button>

        <div className="flex items-center gap-2.5 min-w-0">
          <span className="font-semibold text-sm tracking-tight text-zinc-100">Bob</span>
          {activeSessionTitle && (
            <>
              <span className="text-zinc-600 hidden sm:inline">/</span>
              <span className="text-xs text-zinc-400 truncate max-w-[200px] sm:max-w-[320px] hidden sm:inline">
                {activeSessionTitle}
              </span>
            </>
          )}
        </div>
      </div>

      {/* Right section: System Telemetry, Audit, Status */}
      <div className="flex items-center gap-2">
        {/* System Info Popover Toggle */}
        {status?.system && (
          <div className="relative">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowSystemDetails(!showSystemDetails)}
              className="text-xs text-zinc-400 hover:text-zinc-200 h-8 px-2.5 gap-1.5"
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
                <div className="absolute right-0 top-full mt-2 w-72 p-3 rounded-xl bg-zinc-900 border border-zinc-800 shadow-xl z-40 space-y-2.5 text-xs text-zinc-300 animate-fadeIn">
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
            className="text-xs text-zinc-400 hover:text-zinc-200 h-8 px-2.5 gap-1.5"
          >
            <Shield className="w-3.5 h-3.5 text-zinc-400" />
            <span className="hidden sm:inline">Audit</span>
          </Button>
        </Tooltip>

        {/* Connection status indicator */}
        <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-zinc-900/80 border border-zinc-800 text-[11px]">
          <span
            className={`w-2 h-2 rounded-full ${
              online ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]' : 'bg-red-500'
            }`}
          />
          <span className="text-zinc-400 font-medium">
            {online ? 'Connected' : 'Offline'}
          </span>
        </div>
      </div>
    </header>
  );
};

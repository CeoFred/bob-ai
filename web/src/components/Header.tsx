import React from 'react';
import { SystemStatus } from '../types';
import { Bot, Wifi, WifiOff, ShieldCheck, Cpu, HardDrive, FileText } from 'lucide-react';

interface HeaderProps {
  status: SystemStatus | null;
  online: boolean;
  onOpenAudit: () => void;
}

export const Header: React.FC<HeaderProps> = ({ status, online, onOpenAudit }) => {
  return (
    <header className="flex items-center justify-between px-5 py-3.5 bg-[#161b22] border-b border-[#30363d] select-none">
      <div className="flex items-center gap-3">
        <div className="p-2 rounded-xl bg-gradient-to-tr from-blue-600 to-indigo-500 text-white shadow-md shadow-blue-500/20">
          <Bot className="w-5 h-5" />
        </div>
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-base font-bold text-white tracking-tight">BOB</h1>
            <span className="text-xs px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 font-medium">
              v0.1.0 MVP
            </span>
          </div>
          <p className="text-xs text-gray-400">Local AI Personal Computer Agent</p>
        </div>
      </div>

      <div className="flex items-center gap-4">
        {status?.system && (
          <div className="hidden md:flex items-center gap-3 text-xs text-gray-300 bg-[#0d1117] px-3 py-1.5 rounded-lg border border-[#30363d]">
            <div className="flex items-center gap-1.5" title="CPU / Architecture">
              <Cpu className="w-3.5 h-3.5 text-blue-400" />
              <span>{status.system.cpu_model || status.system.architecture}</span>
            </div>
            <span className="text-gray-600">•</span>
            <div className="flex items-center gap-1.5" title="RAM & Disk">
              <HardDrive className="w-3.5 h-3.5 text-indigo-400" />
              <span>{status.system.ram_formatted} RAM</span>
            </div>
            <span className="text-gray-600">•</span>
            <div className="flex items-center gap-1.5" title="Local Model">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              <span className="font-mono text-[11px] text-emerald-300">{status.llm.model}</span>
            </div>
          </div>
        )}

        <button
          onClick={onOpenAudit}
          className="flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-lg bg-[#21262d] hover:bg-[#30363d] text-gray-300 border border-[#30363d] transition-colors"
          title="View Audit Trail"
        >
          <FileText className="w-3.5 h-3.5 text-gray-400" />
          <span>Audit Log</span>
        </button>

        <div
          className={`flex items-center gap-2 text-xs px-3 py-1.5 rounded-full font-medium border ${
            online
              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
              : 'bg-rose-500/10 text-rose-400 border-rose-500/30'
          }`}
        >
          {online ? <Wifi className="w-3.5 h-3.5 animate-pulse" /> : <WifiOff className="w-3.5 h-3.5" />}
          <span>{online ? (status?.tailscale.ip ? `Tailscale: ${status.tailscale.ip}` : 'Online') : 'Connecting...'}</span>
        </div>
      </div>
    </header>
  );
};

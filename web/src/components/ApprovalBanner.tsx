import React from 'react';
import { AlertTriangle, Check, X } from 'lucide-react';

interface ApprovalBannerProps {
  taskId: string;
  toolName: string;
  command: string;
  reason: string;
  onApprove: (taskId: string, approved: boolean) => void;
}

export const ApprovalBanner: React.FC<ApprovalBannerProps> = ({
  taskId,
  toolName,
  command,
  reason,
  onApprove,
}) => {
  return (
    <div className="my-3 p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-200 animate-fadeIn shadow-lg shadow-amber-950/20">
      <div className="flex items-start gap-3">
        <div className="p-2 rounded-lg bg-amber-500/20 text-amber-400 mt-0.5">
          <AlertTriangle className="w-5 h-5" />
        </div>
        <div className="flex-1 space-y-2">
          <div>
            <div className="flex items-center gap-2">
              <span className="font-semibold text-sm text-amber-300">Security Approval Required</span>
              <span className="text-[11px] px-2 py-0.5 rounded bg-amber-500/20 text-amber-400 font-mono">
                {toolName}
              </span>
            </div>
            <p className="text-xs text-amber-300/80 mt-1">{reason}</p>
          </div>

          <div className="p-2.5 rounded-lg bg-[#0d1117] border border-[#30363d] font-mono text-xs text-amber-100 overflow-x-auto">
            {command}
          </div>

          <div className="flex items-center gap-2 pt-1">
            <button
              onClick={() => onApprove(taskId, true)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-colors shadow-sm"
            >
              <Check className="w-3.5 h-3.5" />
              <span>Allow & Execute</span>
            </button>
            <button
              onClick={() => onApprove(taskId, false)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[#21262d] hover:bg-rose-950/40 text-rose-300 border border-rose-500/30 text-xs font-medium transition-colors"
            >
              <X className="w-3.5 h-3.5" />
              <span>Reject Execution</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

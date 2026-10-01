import { Check, X, ShieldAlert } from 'lucide-react';
import { Button } from './ui/button';
import { Badge } from './ui/badge';

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
    <div className="my-3 rounded-xl border border-amber-500/30 bg-amber-500/5 p-4 text-zinc-200 animate-fadeIn space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-2.5">
          <div className="p-1.5 rounded-lg bg-amber-500/10 text-amber-400 mt-0.5">
            <ShieldAlert className="w-4 h-4" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-medium text-xs text-amber-300">Approval Required</span>
              <Badge variant="warning" className="font-mono text-[10px] px-1.5 py-0">
                {toolName}
              </Badge>
            </div>
            <p className="text-xs text-zinc-400 mt-1">{reason}</p>
          </div>
        </div>
      </div>

      <div className="p-2.5 rounded-lg bg-zinc-950 border border-zinc-800 font-mono text-xs text-zinc-300 overflow-x-auto whitespace-pre-wrap">
        {command}
      </div>

      <div className="flex items-center gap-2 pt-1">
        <Button
          size="sm"
          onClick={() => onApprove(taskId, true)}
          className="bg-emerald-600 hover:bg-emerald-500 text-white gap-1.5 h-8 text-xs font-medium"
        >
          <Check className="w-3.5 h-3.5" />
          <span>Approve & Run</span>
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => onApprove(taskId, false)}
          className="border-zinc-700 text-zinc-400 hover:text-red-300 hover:bg-red-950/20 hover:border-red-800/40 gap-1.5 h-8 text-xs font-medium"
        >
          <X className="w-3.5 h-3.5" />
          <span>Reject</span>
        </Button>
      </div>
    </div>
  );
};

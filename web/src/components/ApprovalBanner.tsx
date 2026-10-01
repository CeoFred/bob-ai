import React, { useState } from 'react';
import { Check, X, ShieldAlert, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import { Button } from './ui/button';
import { Badge } from './ui/badge';
import { cn } from '../lib/utils';

export interface ApprovalBannerProps {
  taskId: string;
  toolName: string;
  command: string;
  reason: string;
  status?: 'pending' | 'approved' | 'rejected';
  onApprove: (taskId: string, approved: boolean) => Promise<void> | void;
}

export const ApprovalBanner: React.FC<ApprovalBannerProps> = ({
  taskId,
  toolName,
  command,
  reason,
  status = 'pending',
  onApprove,
}) => {
  const [localDecision, setLocalDecision] = useState<'approved' | 'rejected' | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Effective status: prioritize local action if taken, otherwise external status prop
  const currentStatus: 'pending' | 'approved' | 'rejected' =
    localDecision || (status !== 'pending' ? status : 'pending');

  const handleDecision = async (approved: boolean) => {
    if (isSubmitting || currentStatus !== 'pending') return;
    setIsSubmitting(true);
    setLocalDecision(approved ? 'approved' : 'rejected');
    try {
      await onApprove(taskId, approved);
    } catch (e) {
      console.error('Failed to submit approval decision:', e);
    } finally {
      setIsSubmitting(false);
    }
  };

  const isApproved = currentStatus === 'approved';
  const isRejected = currentStatus === 'rejected';
  const isPending = currentStatus === 'pending';

  return (
    <div
      className={cn(
        'my-2.5 sm:my-3 rounded-xl border p-3 sm:p-4 text-zinc-200 animate-fadeIn space-y-2.5 sm:space-y-3 transition-colors',
        isPending && 'border-amber-500/30 bg-amber-500/5',
        isApproved && 'border-emerald-500/30 bg-emerald-950/10',
        isRejected && 'border-rose-500/30 bg-rose-950/10'
      )}
    >
      <div className="flex items-start justify-between gap-2.5">
        <div className="flex items-start gap-2 sm:gap-2.5">
          <div
            className={cn(
              'p-1.5 rounded-lg mt-0.5 flex-shrink-0',
              isPending && 'bg-amber-500/10 text-amber-400',
              isApproved && 'bg-emerald-500/10 text-emerald-400',
              isRejected && 'bg-rose-500/10 text-rose-400'
            )}
          >
            {isPending && <ShieldAlert className="w-4 h-4" />}
            {isApproved && <CheckCircle2 className="w-4 h-4" />}
            {isRejected && <XCircle className="w-4 h-4" />}
          </div>
          <div>
            <div className="flex items-center gap-1.5 sm:gap-2 flex-wrap">
              <span
                className={cn(
                  'font-medium text-xs',
                  isPending && 'text-amber-300',
                  isApproved && 'text-emerald-300',
                  isRejected && 'text-rose-300'
                )}
              >
                {isPending && 'Approval Required'}
                {isApproved && 'Command Approved'}
                {isRejected && 'Command Rejected'}
              </span>
              <Badge
                variant={isApproved ? 'success' : isRejected ? 'destructive' : 'warning'}
                className="font-mono text-[10px] px-1.5 py-0"
              >
                {toolName}
              </Badge>
            </div>
            <p className="text-[11px] sm:text-xs text-zinc-400 mt-1">{reason}</p>
          </div>
        </div>
      </div>

      <div className="p-2 sm:p-2.5 rounded-lg bg-zinc-950 border border-zinc-800 font-mono text-[11px] sm:text-xs text-zinc-300 overflow-x-auto whitespace-pre-wrap break-all max-h-48">
        {command}
      </div>

      {isPending ? (
        <div className="flex items-center gap-2 pt-1">
          <Button
            size="sm"
            onClick={() => handleDecision(true)}
            disabled={isSubmitting}
            className="bg-emerald-600 hover:bg-emerald-500 text-white gap-1.5 h-8 text-xs font-medium cursor-pointer disabled:opacity-50"
          >
            {isSubmitting ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Check className="w-3.5 h-3.5" />
            )}
            <span>Approve & Run</span>
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => handleDecision(false)}
            disabled={isSubmitting}
            className="border-zinc-700 text-zinc-400 hover:text-red-300 hover:bg-red-950/20 hover:border-red-800/40 gap-1.5 h-8 text-xs font-medium cursor-pointer disabled:opacity-50"
          >
            <X className="w-3.5 h-3.5" />
            <span>Reject</span>
          </Button>
        </div>
      ) : (
        <div className="flex items-center gap-1.5 pt-0.5">
          {isApproved && (
            <div className="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-400">
              <Check className="w-3.5 h-3.5 stroke-[2.5]" />
              <span>Approved by user</span>
            </div>
          )}
          {isRejected && (
            <div className="inline-flex items-center gap-1 text-[11px] font-medium text-rose-400">
              <X className="w-3.5 h-3.5 stroke-[2.5]" />
              <span>Execution rejected</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

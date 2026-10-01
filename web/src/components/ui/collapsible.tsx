import React, { useState } from 'react';
import { cn } from '../../lib/utils';
import { ChevronRight } from 'lucide-react';

interface CollapsibleProps {
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  trigger: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  triggerClassName?: string;
  contentClassName?: string;
}

export const Collapsible: React.FC<CollapsibleProps> = ({
  open: controlledOpen,
  defaultOpen = false,
  onOpenChange,
  trigger,
  children,
  className,
  triggerClassName,
  contentClassName,
}) => {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(defaultOpen);
  const isControlled = controlledOpen !== undefined;
  const isOpen = isControlled ? controlledOpen : uncontrolledOpen;

  const toggle = () => {
    const next = !isOpen;
    if (!isControlled) {
      setUncontrolledOpen(next);
    }
    onOpenChange?.(next);
  };

  return (
    <div className={cn('rounded-lg border border-zinc-800/80 bg-zinc-900/40 text-xs overflow-hidden', className)}>
      <div
        onClick={toggle}
        className={cn(
          'flex items-center justify-between px-3 py-2 cursor-pointer hover:bg-zinc-800/40 transition-colors select-none text-zinc-300',
          triggerClassName
        )}
      >
        <div className="flex items-center gap-2 flex-1 min-w-0">{trigger}</div>
        <ChevronRight
          className={cn('w-3.5 h-3.5 text-zinc-500 transition-transform duration-200 flex-shrink-0', isOpen && 'rotate-90')}
        />
      </div>
      {isOpen && (
        <div className={cn('border-t border-zinc-800/60 p-3 bg-zinc-950/40', contentClassName)}>
          {children}
        </div>
      )}
    </div>
  );
};

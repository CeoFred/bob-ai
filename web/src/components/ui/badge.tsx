import React from 'react';
import { cn } from '../../lib/utils';

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'secondary' | 'outline' | 'success' | 'warning' | 'destructive';
}

export const Badge: React.FC<BadgeProps> = ({
  className,
  variant = 'default',
  children,
  ...props
}) => {
  const baseStyles =
    'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium transition-colors select-none';

  const variants = {
    default: 'bg-zinc-800 text-zinc-300 border border-zinc-700/60',
    secondary: 'bg-zinc-800/60 text-zinc-400 border border-zinc-800',
    outline: 'border border-zinc-700 text-zinc-300',
    success: 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20',
    warning: 'bg-amber-500/10 text-amber-400 border border-amber-500/20',
    destructive: 'bg-red-500/10 text-red-400 border border-red-500/20',
  };

  return (
    <div className={cn(baseStyles, variants[variant], className)} {...props}>
      {children}
    </div>
  );
};

import React from 'react';
import { cn } from '../../lib/utils';

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive' | 'subtle';
  size?: 'default' | 'sm' | 'lg' | 'icon';
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'default', size = 'default', disabled, ...props }, ref) => {
    const baseStyles =
      'inline-flex items-center justify-center whitespace-nowrap rounded-lg text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400 disabled:pointer-events-none disabled:opacity-50 select-none cursor-pointer';

    const variants = {
      default:
        'bg-zinc-100 text-zinc-900 hover:bg-zinc-200 shadow-sm dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-200',
      secondary:
        'bg-zinc-800/80 text-zinc-200 hover:bg-zinc-700/80 border border-zinc-700/50',
      outline:
        'border border-zinc-700/60 bg-transparent text-zinc-300 hover:bg-zinc-800/60 hover:text-zinc-100',
      ghost:
        'hover:bg-zinc-800/60 text-zinc-400 hover:text-zinc-100',
      destructive:
        'bg-red-600/90 text-white hover:bg-red-600 shadow-sm',
      subtle:
        'bg-zinc-800/40 text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100',
    };

    const sizes = {
      default: 'h-9 px-4 py-2',
      sm: 'h-7 rounded-md px-2.5 text-xs',
      lg: 'h-10 rounded-lg px-6 text-base',
      icon: 'h-8 w-8 p-0 rounded-lg',
    };

    return (
      <button
        ref={ref}
        disabled={disabled}
        className={cn(baseStyles, variants[variant], sizes[size], className)}
        {...props}
      />
    );
  }
);

Button.displayName = 'Button';

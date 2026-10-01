import React, { useState } from 'react';
import { AgentEvent } from '../types';
import {
  Terminal,
  FileCode,
  Folder,
  Camera,
  Check,
  AlertCircle,
  Clock,
  Copy,
  ChevronDown,
  ChevronRight,
  Maximize2,
} from 'lucide-react';
import { cn } from '../lib/utils';

interface ToolActivityProps {
  event: AgentEvent;
  onViewScreenshot?: (url: string) => void;
}

export const ToolActivity: React.FC<ToolActivityProps> = ({ event, onViewScreenshot }) => {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);

  const getToolIcon = (toolName?: string) => {
    switch (toolName) {
      case 'terminal_exec':
        return <Terminal className="w-3.5 h-3.5 text-zinc-400" />;
      case 'read_file':
      case 'write_file':
        return <FileCode className="w-3.5 h-3.5 text-zinc-400" />;
      case 'list_directory':
      case 'search_files':
        return <Folder className="w-3.5 h-3.5 text-zinc-400" />;
      case 'take_screenshot':
        return <Camera className="w-3.5 h-3.5 text-zinc-400" />;
      default:
        return <Terminal className="w-3.5 h-3.5 text-zinc-400" />;
    }
  };

  const isCompleted = event.type === 'tool.completed';
  const isFailed = isCompleted && event.tool_result && !event.tool_result.success;

  const handleCopy = (text: string, e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const outputText = event.output || event.error || (event.tool_result ? JSON.stringify(event.tool_result, null, 2) : '');

  return (
    <div className="my-2 rounded-xl border border-zinc-800/80 bg-zinc-900/40 overflow-hidden text-xs transition-colors hover:border-zinc-700/80">
      {/* Header bar */}
      <div
        onClick={() => setExpanded(!expanded)}
        className="flex items-center justify-between px-3.5 py-2.5 cursor-pointer hover:bg-zinc-850/40 select-none transition-colors"
      >
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="text-zinc-500">
            {expanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
          </div>
          <div className="flex items-center gap-2 font-mono text-zinc-300">
            {getToolIcon(event.tool)}
            <span className="font-medium text-xs text-zinc-200">{event.tool || 'tool_exec'}</span>
          </div>
          {typeof event.input === 'string' && (
            <span className="text-zinc-500 font-mono text-[11px] truncate max-w-[200px] sm:max-w-[320px]">
              {event.input}
            </span>
          )}
        </div>

        <div className="flex items-center gap-3 flex-shrink-0">
          {event.duration_ms !== undefined && (
            <span className="text-[11px] text-zinc-500 font-mono flex items-center gap-1">
              <Clock className="w-3 h-3 text-zinc-600" />
              {event.duration_ms}ms
            </span>
          )}

          {isCompleted ? (
            isFailed ? (
              <span className="inline-flex items-center gap-1 text-[11px] text-red-400">
                <AlertCircle className="w-3.5 h-3.5" /> Failed
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 text-[11px] text-emerald-400">
                <Check className="w-3.5 h-3.5" /> Success
              </span>
            )
          ) : (
            <span className="inline-flex items-center gap-1 text-[11px] text-zinc-400 animate-pulse">
              <span className="w-1.5 h-1.5 rounded-full bg-zinc-400" /> Running...
            </span>
          )}
        </div>
      </div>

      {/* Expanded body */}
      {expanded && (
        <div className="p-3.5 space-y-3 border-t border-zinc-800/60 bg-zinc-950/60 font-mono text-xs">
          {event.input && (
            <div className="space-y-1">
              <div className="flex items-center justify-between text-[10px] uppercase font-sans font-semibold text-zinc-500 tracking-wider">
                <span>Input Parameters</span>
                <button
                  onClick={(e) =>
                    handleCopy(
                      typeof event.input === 'string' ? event.input : JSON.stringify(event.input, null, 2),
                      e
                    )
                  }
                  className="hover:text-zinc-300 flex items-center gap-1"
                >
                  {copied ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                  <span>{copied ? 'Copied' : 'Copy'}</span>
                </button>
              </div>
              <pre className="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-300 overflow-x-auto whitespace-pre-wrap word-break-all text-[11px] leading-relaxed">
                {typeof event.input === 'string' ? event.input : JSON.stringify(event.input, null, 2)}
              </pre>
            </div>
          )}

          {outputText && (
            <div className="space-y-1">
              <div className="flex items-center justify-between text-[10px] uppercase font-sans font-semibold text-zinc-500 tracking-wider">
                <span>{event.error ? 'Error Output' : 'Execution Output'}</span>
                <button
                  onClick={(e) => handleCopy(outputText, e)}
                  className="hover:text-zinc-300 flex items-center gap-1"
                >
                  {copied ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                  <span>{copied ? 'Copied' : 'Copy'}</span>
                </button>
              </div>
              <pre
                className={cn(
                  'p-2.5 rounded-lg border overflow-x-auto whitespace-pre font-mono max-h-96 text-[11px] leading-relaxed',
                  event.error
                    ? 'bg-red-950/20 text-red-300 border-red-900/40'
                    : 'bg-zinc-900 text-zinc-300 border-zinc-800'
                )}
              >
                {outputText}
              </pre>
            </div>
          )}

          {event.tool_result?.data?.url && (
            <div className="space-y-1.5 pt-1">
              <div className="text-[10px] uppercase font-sans font-semibold text-zinc-500 tracking-wider">
                Screenshot
              </div>
              <div
                onClick={() => onViewScreenshot && event.tool_result?.data?.url && onViewScreenshot(event.tool_result.data.url)}
                className="relative group cursor-pointer rounded-xl overflow-hidden border border-zinc-800 max-w-sm hover:border-zinc-600 transition-colors"
              >
                <img
                  src={event.tool_result.data.url}
                  alt="Screenshot"
                  className="w-full h-auto object-cover max-h-48"
                />
                <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 flex items-center justify-center text-white text-xs font-sans font-medium transition-opacity gap-1.5">
                  <Maximize2 className="w-3.5 h-3.5" />
                  <span>Expand preview</span>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

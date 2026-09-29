import React, { useState } from 'react';
import { AgentEvent } from '../types';
import { Terminal, Folder, FileCode, Camera, CheckCircle2, XCircle, ChevronDown, ChevronRight, Clock } from 'lucide-react';

interface ToolActivityProps {
  event: AgentEvent;
  onViewScreenshot?: (url: string) => void;
}

export const ToolActivity: React.FC<ToolActivityProps> = ({ event, onViewScreenshot }) => {
  const [expanded, setExpanded] = useState(true);

  const getToolIcon = (toolName?: string) => {
    switch (toolName) {
      case 'terminal_exec':
        return <Terminal className="w-4 h-4 text-blue-400" />;
      case 'read_file':
      case 'write_file':
        return <FileCode className="w-4 h-4 text-emerald-400" />;
      case 'list_directory':
      case 'search_files':
        return <Folder className="w-4 h-4 text-amber-400" />;
      case 'take_screenshot':
        return <Camera className="w-4 h-4 text-purple-400" />;
      default:
        return <Terminal className="w-4 h-4 text-gray-400" />;
    }
  };

  const isCompleted = event.type === 'tool.completed';
  const isFailed = isCompleted && event.tool_result && !event.tool_result.success;

  return (
    <div className="my-2 rounded-xl bg-[#0d1117] border border-[#30363d] overflow-hidden text-xs shadow-sm">
      <div
        onClick={() => setExpanded(!expanded)}
        className="flex items-center justify-between px-3 py-2 bg-[#161b22]/70 hover:bg-[#161b22] cursor-pointer border-b border-[#30363d]/50 select-none transition-colors"
      >
        <div className="flex items-center gap-2">
          {expanded ? <ChevronDown className="w-3.5 h-3.5 text-gray-400" /> : <ChevronRight className="w-3.5 h-3.5 text-gray-400" />}
          <div className="flex items-center gap-1.5 font-mono text-gray-300">
            {getToolIcon(event.tool)}
            <span className="font-semibold">{event.tool}</span>
          </div>
        </div>

        <div className="flex items-center gap-2.5">
          {event.duration_ms !== undefined && (
            <div className="flex items-center gap-1 text-[11px] text-gray-400 font-mono">
              <Clock className="w-3 h-3 text-gray-500" />
              <span>{event.duration_ms}ms</span>
            </div>
          )}

          {isCompleted ? (
            isFailed ? (
              <span className="flex items-center gap-1 text-rose-400 font-medium">
                <XCircle className="w-3.5 h-3.5" /> Failed
              </span>
            ) : (
              <span className="flex items-center gap-1 text-emerald-400 font-medium">
                <CheckCircle2 className="w-3.5 h-3.5" /> Success
              </span>
            )
          ) : (
            <span className="text-blue-400 animate-pulse font-medium">Running...</span>
          )}
        </div>
      </div>

      {expanded && (
        <div className="p-3 space-y-2 bg-[#0d1117] font-mono text-[11px]">
          {event.input && (
            <div>
              <div className="text-gray-500 text-[10px] uppercase font-sans font-semibold mb-1">Input</div>
              <pre className="p-2 rounded-md bg-[#161b22] text-blue-200 border border-[#30363d] overflow-x-auto whitespace-pre-wrap word-break-all">
                {typeof event.input === 'string' ? event.input : JSON.stringify(event.input, null, 2)}
              </pre>
            </div>
          )}

          {(event.output || event.error) && (
            <div>
              <div className="text-gray-500 text-[10px] uppercase font-sans font-semibold mb-1">
                {event.error ? 'Error / Stderr' : 'Output'}
              </div>
              <pre
                className={`p-2 rounded-md border overflow-x-auto whitespace-pre-wrap word-break-all max-h-64 ${
                  event.error
                    ? 'bg-rose-950/20 text-rose-300 border-rose-500/30'
                    : 'bg-[#161b22] text-gray-300 border-[#30363d]'
                }`}
              >
                {event.output || event.error}
              </pre>
            </div>
          )}

          {event.tool_result?.data?.url && (
            <div className="mt-2">
              <div className="text-gray-500 text-[10px] uppercase font-sans font-semibold mb-1">Captured Screenshot</div>
              <div
                onClick={() => onViewScreenshot && event.tool_result?.data?.url && onViewScreenshot(event.tool_result.data.url)}
                className="relative group cursor-pointer rounded-lg overflow-hidden border border-[#30363d] inline-block max-w-sm hover:border-blue-500 transition-all"
              >
                <img src={event.tool_result.data.url} alt="Screenshot capture" className="w-full h-auto object-cover max-h-48" />
                <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 flex items-center justify-center text-white text-xs font-sans font-medium transition-opacity">
                  Click to Expand
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

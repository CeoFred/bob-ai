import React, { useEffect, useState } from 'react';
import { AuditEntry } from '../types';
import { fetchAuditLogs } from '../services/api';
import { X, RefreshCw, Clock, Shield } from 'lucide-react';

interface AuditModalProps {
  onClose: () => void;
}

export const AuditModal: React.FC<AuditModalProps> = ({ onClose }) => {
  const [logs, setLogs] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);

  const loadLogs = async () => {
    setLoading(true);
    try {
      const data = await fetchAuditLogs(100);
      setLogs(data.reverse()); // Most recent first
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadLogs();
  }, []);

  const getActionBadge = (log: AuditEntry) => {
    const action = log.action || (log.tool ? 'TOOL_EXEC' : 'STEP_REASONING');
    switch (action) {
      case 'TASK_START':
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-blue-500/10 text-blue-400 border border-blue-500/20">TASK START</span>;
      case 'STEP_REASONING':
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-purple-500/10 text-purple-300 border border-purple-500/20">REASONING</span>;
      case 'TOOL_EXEC':
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/20">TOOL EXEC</span>;
      case 'APPROVAL_REQUEST':
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-amber-500/10 text-amber-300 border border-amber-500/20">APPROVAL REQ</span>;
      case 'APPROVAL_DECISION':
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-cyan-500/10 text-cyan-300 border border-cyan-500/20">APPROVAL DECISION</span>;
      case 'TASK_COMPLETE':
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">COMPLETE</span>;
      case 'TASK_FAIL':
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">FAILED</span>;
      case 'TASK_CANCEL':
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-gray-500/10 text-gray-400 border border-gray-500/20">CANCELLED</span>;
      default:
        return <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-gray-500/10 text-gray-400 border border-gray-500/20">{action}</span>;
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-fadeIn">
      <div className="relative max-w-4xl w-full bg-[#161b22] border border-[#30363d] rounded-2xl overflow-hidden shadow-2xl flex flex-col max-h-[85vh]">
        <div className="flex items-center justify-between px-5 py-4 border-b border-[#30363d] bg-[#0d1117]">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold text-white flex items-center gap-2">
              <Shield className="w-4 h-4 text-emerald-400" />
              Security & Execution Audit Trail
            </h3>
            <span className="text-xs px-2 py-0.5 rounded-full bg-[#21262d] text-gray-400 border border-[#30363d]">
              {logs.length} records
            </span>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={loadLogs}
              disabled={loading}
              className="p-1.5 rounded-lg hover:bg-[#30363d] text-gray-400 hover:text-white transition-colors"
              title="Refresh logs"
            >
              <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            </button>
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg hover:bg-rose-900/30 text-gray-400 hover:text-rose-300 transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-4 space-y-2 bg-[#05070a]">
          {logs.length === 0 ? (
            <div className="text-center py-12 text-gray-500 text-xs">No audit logs recorded yet.</div>
          ) : (
            logs.map((log, idx) => (
              <div
                key={idx}
                className="p-3 rounded-lg bg-[#0d1117] border border-[#30363d] text-xs font-mono space-y-1.5 hover:border-[#484f58] transition-colors"
              >
                <div className="flex items-center justify-between text-[11px] text-gray-400 font-sans">
                  <div className="flex items-center gap-2">
                    {getActionBadge(log)}
                    {log.tool && (
                      <span className="font-semibold text-blue-400 font-mono">[{log.tool}]</span>
                    )}
                    <span className="text-gray-600">•</span>
                    <span className="text-gray-400">{new Date(log.timestamp).toLocaleTimeString()}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    {log.approval_status && (
                      <span
                        className={`px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase ${
                          log.approval_status === 'APPROVED' || log.approval_status === 'AUTOMATIC'
                            ? 'bg-emerald-950/40 text-emerald-400 border border-emerald-500/30'
                            : log.approval_status === 'REQUIRED' || log.approval_status === 'PENDING'
                            ? 'bg-amber-950/40 text-amber-400 border border-amber-500/30'
                            : 'bg-rose-950/40 text-rose-400 border border-rose-500/30'
                        }`}
                      >
                        {log.approval_status}
                      </span>
                    )}
                    {log.duration_ms > 0 && (
                      <span className="flex items-center gap-1 text-gray-500 text-[10px]">
                        <Clock className="w-3 h-3" />
                        {log.duration_ms}ms
                      </span>
                    )}
                  </div>
                </div>

                {log.input !== undefined && log.input !== "" && (
                  <div className="text-gray-300 text-[11px] break-all">
                    <span className="text-gray-500">Input: </span>
                    {typeof log.input === 'string' ? log.input : JSON.stringify(log.input)}
                  </div>
                )}

                {log.result !== undefined && log.result !== "" && (
                  <div className="text-gray-400 text-[11px] break-all">
                    <span className="text-gray-500">Result: </span>
                    {typeof log.result === 'string' ? log.result : JSON.stringify(log.result)}
                  </div>
                )}

                {log.error && (
                  <div className="text-rose-400 text-[11px] break-all">
                    <span className="text-rose-500">Error: </span>
                    {log.error}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
};

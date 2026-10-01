import React, { useEffect, useState } from 'react';
import { AuditEntry } from '../types';
import { fetchAuditLogs } from '../services/api';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from './ui/dialog';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Shield, RefreshCw, Clock, Search, CheckCircle2, AlertCircle, HelpCircle } from 'lucide-react';

interface AuditModalProps {
  onClose: () => void;
}

export const AuditModal: React.FC<AuditModalProps> = ({ onClose }) => {
  const [logs, setLogs] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');

  const loadLogs = async () => {
    setLoading(true);
    try {
      const data = await fetchAuditLogs(100);
      setLogs(data.reverse());
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadLogs();
  }, []);

  const filteredLogs = logs.filter((log) => {
    if (!searchQuery.trim()) return true;
    const query = searchQuery.toLowerCase();
    const actionMatch = (log.action || '').toLowerCase().includes(query);
    const toolMatch = (log.tool || '').toLowerCase().includes(query);
    const inputMatch = typeof log.input === 'string' && log.input.toLowerCase().includes(query);
    const resultMatch = typeof log.result === 'string' && log.result.toLowerCase().includes(query);
    return actionMatch || toolMatch || inputMatch || resultMatch;
  });

  const getStatusBadge = (log: AuditEntry) => {
    const status = log.approval_status;
    if (status === 'APPROVED' || status === 'AUTOMATIC') {
      return (
        <Badge variant="success" className="text-[10px] gap-1">
          <CheckCircle2 className="w-2.5 h-2.5" />
          <span>{status}</span>
        </Badge>
      );
    }
    if (status === 'REQUIRED' || status === 'PENDING') {
      return (
        <Badge variant="warning" className="text-[10px] gap-1">
          <HelpCircle className="w-2.5 h-2.5" />
          <span>{status}</span>
        </Badge>
      );
    }
    if (status === 'REJECTED') {
      return (
        <Badge variant="destructive" className="text-[10px] gap-1">
          <AlertCircle className="w-2.5 h-2.5" />
          <span>REJECTED</span>
        </Badge>
      );
    }
    return null;
  };

  return (
    <Dialog open={true} onOpenChange={(open) => !open && onClose()}>
      <DialogContent onClose={onClose} className="max-w-3xl">
        <DialogHeader className="flex flex-row items-center justify-between pb-3 pr-8 space-y-0">
          <div className="flex items-center gap-2">
            <Shield className="w-4 h-4 text-zinc-400" />
            <DialogTitle>Audit Trail</DialogTitle>
            <Badge variant="secondary" className="text-[10px]">
              {logs.length} events
            </Badge>
          </div>

          <Button
            variant="ghost"
            size="sm"
            onClick={loadLogs}
            disabled={loading}
            className="h-7 px-2 text-xs text-zinc-400 hover:text-zinc-100 gap-1.5"
          >
            <RefreshCw className={`w-3 h-3 ${loading ? 'animate-spin' : ''}`} />
            <span>Refresh</span>
          </Button>
        </DialogHeader>

        {/* Filter / Search input */}
        <div className="my-2 relative">
          <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search audit logs..."
            className="w-full bg-zinc-900 border border-zinc-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-600 transition-colors"
          />
        </div>

        {/* Log Entries */}
        <div className="flex-1 overflow-y-auto space-y-1.5 max-h-[60vh] pr-1 py-1">
          {filteredLogs.length === 0 ? (
            <div className="text-center py-12 text-zinc-500 text-xs">
              {searchQuery ? 'No matching audit records' : 'No audit records logged yet.'}
            </div>
          ) : (
            filteredLogs.map((log, idx) => (
              <div
                key={idx}
                className="p-3 rounded-lg bg-zinc-900/60 border border-zinc-850 hover:border-zinc-750 text-xs font-mono space-y-1.5 transition-colors"
              >
                <div className="flex items-center justify-between text-[11px] text-zinc-400 font-sans">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-zinc-200">
                      {log.action || (log.tool ? 'TOOL_EXEC' : 'EVENT')}
                    </span>
                    {log.tool && (
                      <Badge variant="outline" className="font-mono text-[10px] px-1.5 py-0">
                        {log.tool}
                      </Badge>
                    )}
                    <span className="text-zinc-600">•</span>
                    <span className="text-zinc-500">{new Date(log.timestamp).toLocaleTimeString()}</span>
                  </div>

                  <div className="flex items-center gap-2">
                    {getStatusBadge(log)}
                    {log.duration_ms > 0 && (
                      <span className="flex items-center gap-1 text-zinc-500 text-[10px]">
                        <Clock className="w-2.5 h-2.5" />
                        {log.duration_ms}ms
                      </span>
                    )}
                  </div>
                </div>

                {log.input !== undefined && log.input !== '' && (
                  <div className="text-zinc-300 text-[11px] break-all">
                    <span className="text-zinc-500">Input: </span>
                    {typeof log.input === 'string' ? log.input : JSON.stringify(log.input)}
                  </div>
                )}

                {log.result !== undefined && log.result !== '' && (
                  <div className="text-zinc-400 text-[11px] break-all">
                    <span className="text-zinc-500">Result: </span>
                    {typeof log.result === 'string' ? log.result : JSON.stringify(log.result)}
                  </div>
                )}

                {log.error && (
                  <div className="text-red-400 text-[11px] break-all">
                    <span className="text-red-500">Error: </span>
                    {log.error}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
};

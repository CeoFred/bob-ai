import React, { useState, useEffect } from 'react';
import { Folder, FolderGit2, ArrowUp, X, Loader2, Search, HardDrive } from 'lucide-react';
import { Button } from './ui/button';
import { ProjectSummary, BrowseResponse } from '../types';
import { fetchRecentProjects, fetchBrowseFilesystem } from '../services/api';

interface ProjectPickerModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSelectProject: (projectPath: string, projectName: string) => void;
}

export const ProjectPickerModal: React.FC<ProjectPickerModalProps> = ({
  isOpen,
  onClose,
  onSelectProject,
}) => {
  const [activeTab, setActiveTab] = useState<'recent' | 'browse'>('recent');
  const [recentProjects, setRecentProjects] = useState<ProjectSummary[]>([]);
  const [loadingRecent, setLoadingRecent] = useState(false);

  // Browser state
  const [currentPath, setCurrentPath] = useState<string>('');
  const [browseData, setBrowseData] = useState<BrowseResponse | null>(null);
  const [loadingBrowse, setLoadingBrowse] = useState(false);
  const [customPath, setCustomPath] = useState('');
  const [searchFilter, setSearchFilter] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      loadRecent();
      loadBrowse();
    }
  }, [isOpen]);

  const loadRecent = async () => {
    setLoadingRecent(true);
    try {
      const list = await fetchRecentProjects();
      setRecentProjects(list);
    } catch (e) {
      console.error('Failed to load recent projects:', e);
    } finally {
      setLoadingRecent(false);
    }
  };

  const loadBrowse = async (path?: string) => {
    setLoadingBrowse(true);
    setError(null);
    try {
      const data = await fetchBrowseFilesystem(path);
      setBrowseData(data);
      setCurrentPath(data.current_path);
      setCustomPath(data.current_path);
    } catch (e: any) {
      setError(e.message || 'Failed to browse directory');
    } finally {
      setLoadingBrowse(false);
    }
  };

  if (!isOpen) return null;

  const handleSelect = (path: string, name?: string) => {
    const projName = name || path.split('/').filter(Boolean).pop() || 'project';
    onSelectProject(path, projName);
    onClose();
  };

  const filteredRecent = recentProjects.filter(
    (p) =>
      p.name.toLowerCase().includes(searchFilter.toLowerCase()) ||
      p.path.toLowerCase().includes(searchFilter.toLowerCase())
  );

  const filteredDirs = browseData?.directories.filter((d) =>
    d.name.toLowerCase().includes(searchFilter.toLowerCase())
  );

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-2.5 sm:p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-150">
      <div className="w-full max-w-2xl bg-zinc-900 border border-zinc-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[92dvh] sm:max-h-[85vh] animate-in zoom-in-95 duration-150">
        {/* Header */}
        <div className="p-3.5 sm:p-4 px-4 sm:px-6 border-b border-zinc-800/80 flex items-center justify-between bg-zinc-950/60">
          <div className="flex items-center gap-2.5 sm:gap-3">
            <div className="p-2 rounded-xl bg-blue-500/10 text-blue-400 border border-blue-500/20 flex-shrink-0">
              <FolderGit2 className="w-4 h-4 sm:w-5 sm:h-5" />
            </div>
            <div>
              <h2 className="text-xs sm:text-sm font-semibold text-zinc-100">Open Project Workspace</h2>
              <p className="text-[10px] sm:text-xs text-zinc-400">
                Bob will be strictly scoped to operate within this folder
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded-lg transition-colors cursor-pointer"
            aria-label="Close dialog"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Tab switcher & Search */}
        <div className="p-3 px-4 sm:px-6 bg-zinc-900/90 border-b border-zinc-800/60 flex flex-col sm:flex-row gap-2.5 sm:gap-3 items-stretch sm:items-center justify-between">
          <div className="flex p-0.5 bg-zinc-950/80 rounded-lg border border-zinc-800 text-xs">
            <button
              onClick={() => {
                setActiveTab('recent');
                setSearchFilter('');
              }}
              className={`flex-1 sm:flex-initial px-3 py-1.5 rounded-md font-medium transition-all cursor-pointer ${
                activeTab === 'recent'
                  ? 'bg-zinc-800 text-zinc-100 shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              Detected Projects
            </button>
            <button
              onClick={() => {
                setActiveTab('browse');
                setSearchFilter('');
              }}
              className={`flex-1 sm:flex-initial px-3 py-1.5 rounded-md font-medium transition-all cursor-pointer ${
                activeTab === 'browse'
                  ? 'bg-zinc-800 text-zinc-100 shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              Browse Folders
            </button>
          </div>

          <div className="relative w-full sm:w-60">
            <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
            <input
              type="text"
              value={searchFilter}
              onChange={(e) => setSearchFilter(e.target.value)}
              placeholder="Filter by name..."
              className="w-full pl-8 pr-3 py-1.5 bg-zinc-950 border border-zinc-800 rounded-lg text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
            />
          </div>
        </div>

        {/* Modal Body */}
        <div className="flex-1 overflow-y-auto p-4 sm:p-6 min-h-[240px] sm:min-h-[300px]">
          {error && (
            <div className="mb-4 p-3 rounded-xl bg-red-950/30 border border-red-800/50 text-red-300 text-xs flex items-center justify-between">
              <span>{error}</span>
              <Button size="sm" variant="ghost" onClick={() => setError(null)} className="h-6 text-xs">
                Dismiss
              </Button>
            </div>
          )}

          {activeTab === 'recent' ? (
            <div>
              {loadingRecent ? (
                <div className="flex flex-col items-center justify-center py-12 sm:py-16 text-zinc-500 text-xs gap-2">
                  <Loader2 className="w-5 h-5 animate-spin text-zinc-400" />
                  <span>Scanning for projects on Mac...</span>
                </div>
              ) : filteredRecent.length === 0 ? (
                <div className="text-center py-10 sm:py-12 text-zinc-500 space-y-3">
                  <Folder className="w-8 h-8 mx-auto text-zinc-600 opacity-60" />
                  <p className="text-xs">No matching projects discovered automatically.</p>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setActiveTab('browse')}
                    className="text-xs"
                  >
                    Browse Filesystem
                  </Button>
                </div>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 sm:gap-2.5">
                  {filteredRecent.map((proj) => (
                    <button
                      key={proj.path}
                      onClick={() => handleSelect(proj.path, proj.name)}
                      className="flex items-start gap-2.5 sm:gap-3 p-2.5 sm:p-3 text-left rounded-xl border border-zinc-800 bg-zinc-950/50 hover:bg-zinc-800/80 hover:border-zinc-700 transition-all group cursor-pointer"
                    >
                      <div className="p-2 rounded-lg bg-zinc-900 group-hover:bg-zinc-850 text-blue-400 border border-zinc-800 transition-colors flex-shrink-0">
                        <FolderGit2 className="w-4 h-4" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="text-xs font-semibold text-zinc-200 group-hover:text-white truncate">
                          {proj.name}
                        </div>
                        <div className="text-[10px] sm:text-[11px] text-zinc-500 truncate mt-0.5" title={proj.path}>
                          {proj.path}
                        </div>
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          ) : (
            /* Browse Tab */
            <div className="space-y-3 sm:space-y-4">
              {/* Path bar */}
              <div className="flex items-center gap-2">
                <div className="flex-1 flex items-center gap-1.5 px-3 py-1.5 bg-zinc-950 border border-zinc-800 rounded-xl text-xs text-zinc-300 font-mono min-w-0">
                  <HardDrive className="w-3.5 h-3.5 text-zinc-500 flex-shrink-0" />
                  <input
                    type="text"
                    value={customPath}
                    onChange={(e) => setCustomPath(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        loadBrowse(customPath);
                      }
                    }}
                    placeholder="/Users/..."
                    className="w-full bg-transparent text-xs text-zinc-200 focus:outline-none truncate"
                  />
                </div>
                <Button
                  size="sm"
                  onClick={() => loadBrowse(customPath)}
                  disabled={loadingBrowse}
                  className="h-8 px-3 text-xs flex-shrink-0 cursor-pointer"
                >
                  Go
                </Button>
              </div>

              {/* Navigation list */}
              {loadingBrowse ? (
                <div className="flex items-center justify-center py-10 sm:py-12 text-zinc-500 text-xs gap-2">
                  <Loader2 className="w-4 h-4 animate-spin text-zinc-400" />
                  <span>Loading folder contents...</span>
                </div>
              ) : (
                <div className="border border-zinc-800 rounded-xl bg-zinc-950/40 overflow-hidden divide-y divide-zinc-800/50 max-h-56 sm:max-h-64 overflow-y-auto">
                  {browseData?.parent_path && (
                    <button
                      onClick={() => loadBrowse(browseData.parent_path)}
                      className="w-full px-3 py-2 text-left flex items-center gap-2 text-xs text-zinc-400 hover:text-zinc-200 hover:bg-zinc-900/60 transition-colors cursor-pointer"
                    >
                      <ArrowUp className="w-3.5 h-3.5 text-zinc-500 flex-shrink-0" />
                      <span>.. (Parent Directory)</span>
                    </button>
                  )}

                  {filteredDirs && filteredDirs.length > 0 ? (
                    filteredDirs.map((dir) => (
                      <div
                        key={dir.path}
                        className="px-3 py-2 flex items-center justify-between hover:bg-zinc-900/60 group transition-colors"
                      >
                        <button
                          onClick={() => loadBrowse(dir.path)}
                          className="flex items-center gap-2 text-xs text-zinc-300 hover:text-white flex-1 truncate text-left cursor-pointer pr-2"
                        >
                          {dir.is_project ? (
                            <FolderGit2 className="w-3.5 h-3.5 text-blue-400 flex-shrink-0" />
                          ) : (
                            <Folder className="w-3.5 h-3.5 text-zinc-500 group-hover:text-zinc-400 flex-shrink-0" />
                          )}
                          <span className="truncate">{dir.name}</span>
                          {dir.is_project && (
                            <span className="text-[10px] bg-blue-950/60 text-blue-400 border border-blue-800/40 px-1.5 py-0.2 rounded flex-shrink-0">
                              Project
                            </span>
                          )}
                        </button>

                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleSelect(dir.path, dir.name)}
                          className="h-6 px-2 text-[11px] text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800 opacity-90 sm:opacity-0 sm:group-hover:opacity-100 transition-opacity flex-shrink-0 cursor-pointer"
                        >
                          Select
                        </Button>
                      </div>
                    ))
                  ) : (
                    <div className="py-8 text-center text-xs text-zinc-500">
                      No subdirectories found in this folder.
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-3 px-4 sm:px-6 bg-zinc-950 border-t border-zinc-800/80 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-2.5 text-xs text-zinc-500">
          <div className="flex items-center gap-1.5 truncate">
            <span className="text-zinc-500 flex-shrink-0">Selected:</span>
            <span className="text-zinc-300 font-mono text-[10px] sm:text-[11px] truncate">
              {currentPath || 'None'}
            </span>
          </div>

          <div className="flex items-center justify-end gap-2">
            <Button size="sm" variant="ghost" onClick={onClose} className="h-8 text-xs cursor-pointer">
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={!currentPath}
              onClick={() => handleSelect(currentPath)}
              className="h-8 text-xs bg-zinc-100 hover:bg-white text-zinc-900 font-medium cursor-pointer"
            >
              Open This Folder
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};

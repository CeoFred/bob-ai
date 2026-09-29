import React from 'react';
import { X, Download, ExternalLink } from 'lucide-react';

interface ScreenshotModalProps {
  url: string;
  onClose: () => void;
}

export const ScreenshotModal: React.FC<ScreenshotModalProps> = ({ url, onClose }) => {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-fadeIn">
      <div className="relative max-w-5xl w-full bg-[#161b22] border border-[#30363d] rounded-2xl overflow-hidden shadow-2xl flex flex-col max-h-[90vh]">
        <div className="flex items-center justify-between px-4 py-3 border-b border-[#30363d] bg-[#0d1117]">
          <h3 className="text-sm font-semibold text-white">Mac Screen Capture</h3>
          <div className="flex items-center gap-2">
            <a
              href={url}
              target="_blank"
              rel="noreferrer"
              className="p-1.5 rounded-lg hover:bg-[#30363d] text-gray-400 hover:text-white transition-colors"
              title="Open in new tab"
            >
              <ExternalLink className="w-4 h-4" />
            </a>
            <a
              href={url}
              download
              className="p-1.5 rounded-lg hover:bg-[#30363d] text-gray-400 hover:text-white transition-colors"
              title="Download image"
            >
              <Download className="w-4 h-4" />
            </a>
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg hover:bg-rose-900/30 text-gray-400 hover:text-rose-300 transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
        <div className="flex-1 overflow-auto p-4 flex items-center justify-center bg-[#05070a]">
          <img src={url} alt="Screenshot" className="max-h-[75vh] w-auto object-contain rounded-lg border border-[#30363d]" />
        </div>
      </div>
    </div>
  );
};

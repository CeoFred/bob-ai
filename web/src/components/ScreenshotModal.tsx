import React from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from './ui/dialog';
import { Button } from './ui/button';
import { Download, ExternalLink, Camera } from 'lucide-react';

interface ScreenshotModalProps {
  url: string;
  onClose: () => void;
}

export const ScreenshotModal: React.FC<ScreenshotModalProps> = ({ url, onClose }) => {
  return (
    <Dialog open={true} onOpenChange={(open) => !open && onClose()}>
      <DialogContent onClose={onClose} className="max-w-4xl p-3 sm:p-5">
        <DialogHeader className="flex flex-row items-center justify-between pb-3 pr-8 space-y-0">
          <div className="flex items-center gap-2">
            <Camera className="w-4 h-4 text-zinc-400 flex-shrink-0" />
            <DialogTitle className="truncate">Screen Capture</DialogTitle>
          </div>

          <div className="flex items-center gap-1.5 sm:gap-2">
            <a href={url} target="_blank" rel="noreferrer">
              <Button variant="ghost" size="sm" className="h-7 px-2 text-xs text-zinc-400 gap-1.5 cursor-pointer">
                <ExternalLink className="w-3.5 h-3.5" />
                <span className="hidden xs:inline">Open</span>
              </Button>
            </a>
            <a href={url} download>
              <Button variant="secondary" size="sm" className="h-7 px-2 text-xs gap-1.5 cursor-pointer">
                <Download className="w-3.5 h-3.5" />
                <span>Save</span>
              </Button>
            </a>
          </div>
        </DialogHeader>

        <div className="flex-1 overflow-auto p-1.5 sm:p-2 flex items-center justify-center bg-zinc-950 rounded-xl border border-zinc-900 mt-2 max-h-[70dvh]">
          <img
            src={url}
            alt="Screenshot"
            className="max-h-[65dvh] max-w-full w-auto object-contain rounded-lg shadow-md"
          />
        </div>
      </DialogContent>
    </Dialog>
  );
};

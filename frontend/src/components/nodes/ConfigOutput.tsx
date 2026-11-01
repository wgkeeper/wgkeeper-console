import { DownloadIcon, FileTextIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { CopyButton } from '@/components/CopyButton';
import { renderConfig } from '@/lib/config-utils';
import { cn } from '@/lib/utils';

type Props = {
  config: string;
  fileName: string;
  className?: string;
};

const download = (config: string, fileName: string) => {
  const url = URL.createObjectURL(new Blob([config], { type: 'text/plain' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = fileName;
  link.click();
  // Revoking in the same tick can cancel the download in some browsers.
  setTimeout(() => URL.revokeObjectURL(url), 1000);
};

/** A generated WireGuard config with its file name, copy and download. */
export const ConfigOutput = ({ config, fileName, className }: Props) => (
  <div className={cn('overflow-hidden rounded-lg border bg-muted/40', className)}>
    <div className="flex items-center gap-2 border-b bg-background/60 py-1.5 pr-1.5 pl-3">
      <FileTextIcon className="size-3.5 text-muted-foreground" />
      <span className="font-mono text-xs text-muted-foreground">{fileName}</span>
      <div className="ml-auto flex items-center gap-1">
        <CopyButton value={config} label="Copy config" />
        <Button
          type="button"
          variant="ghost"
          size="xs"
          className="text-muted-foreground hover:text-foreground pointer-coarse:h-9"
          onClick={() => download(config, fileName)}
        >
          <DownloadIcon />
          Download
        </Button>
      </div>
    </div>
    <pre
      // A scrollable block must be reachable by keyboard to be scrolled
      // without a mouse (WCAG 2.1.1).
      // oxlint-disable-next-line jsx-a11y/no-noninteractive-tabindex
      tabIndex={0}
      aria-label={fileName}
      className="max-h-72 overflow-auto px-4 py-3 font-mono text-xs leading-relaxed outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
    >
      {renderConfig(config)}
    </pre>
  </div>
);

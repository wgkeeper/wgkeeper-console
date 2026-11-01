import { useEffect, useRef, useState } from 'react';
import { CheckIcon, CopyIcon } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

type Props = {
  value: string;
  label: string;
  className?: string;
};

/** Icon button that copies `value` and confirms with a check mark. */
export const CopyButton = ({ value, label, className }: Props) => {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current !== null) clearTimeout(timer.current);
    },
    [],
  );

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      if (timer.current !== null) clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 1500);
    } catch {
      toast.error('Could not copy to the clipboard.');
    }
  };

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(
            'size-7 text-muted-foreground hover:text-foreground',
            className,
            'pointer-coarse:size-9',
          )}
          aria-label={label}
          onClick={copy}
        >
          {copied ? <CheckIcon /> : <CopyIcon />}
          <span className="sr-only" aria-live="polite">
            {copied ? 'Copied' : ''}
          </span>
        </Button>
      </TooltipTrigger>
      <TooltipContent>{copied ? 'Copied' : label}</TooltipContent>
    </Tooltip>
  );
};

export const copyToClipboard = async (value: string, message: string) => {
  try {
    await navigator.clipboard.writeText(value);
    toast.success(message);
  } catch {
    toast.error('Could not copy to the clipboard.');
  }
};

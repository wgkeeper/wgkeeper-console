import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '@/lib/utils';

const badgeVariants = cva(
  'inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring [&_svg]:size-3 [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground',
        secondary: 'border-border bg-secondary text-secondary-foreground',
        outline: 'border-border text-foreground',
        // Subtle, tinted status pills. `destructive` uses `text-destructive`
        // (red reads on its own muted tint) since `destructive-foreground` is
        // reserved as the white label on solid danger buttons.
        destructive: 'border-transparent bg-destructive-muted text-destructive',
        success: 'border-transparent bg-success-muted text-success-foreground',
        warning: 'border-transparent bg-warning-muted text-warning-foreground',
        info: 'border-transparent bg-info-muted text-info-foreground',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>, VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { Badge, badgeVariants };

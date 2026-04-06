import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';

import { ChevronLeft, ChevronRight } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from './tooltip';

type SidebarContextValue = {
  isCollapsed: boolean;
  setCollapsed: (value: boolean) => void;
};

const SidebarContext = React.createContext<SidebarContextValue | null>(null);

const useSidebar = () => {
  const context = React.useContext(SidebarContext);
  if (!context) {
    throw new Error('Sidebar components must be used within SidebarProvider.');
  }
  return context;
};

interface SidebarProviderProps {
  children: React.ReactNode;
  defaultOpen?: boolean;
}

const SidebarProvider = ({ children, defaultOpen = true }: SidebarProviderProps) => {
  const [isCollapsed, setCollapsed] = React.useState(!defaultOpen);

  return (
    <SidebarContext.Provider value={{ isCollapsed, setCollapsed }}>
      <TooltipProvider delayDuration={0}>{children}</TooltipProvider>
    </SidebarContext.Provider>
  );
};

const Sidebar = ({ className, children }: React.HTMLAttributes<HTMLDivElement>) => {
  const { isCollapsed } = useSidebar();
  return (
    <aside
      data-collapsed={isCollapsed}
      className={cn(
        'group/sidebar sticky top-0 z-20 flex h-screen flex-col overflow-hidden border-r border-border bg-card transition-all duration-200',
        isCollapsed ? 'w-20' : 'w-72',
        className,
      )}
    >
      {children}
    </aside>
  );
};

const SidebarHeader = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn('flex items-center justify-between gap-3 px-4 py-3', className)} {...props} />
);

const SidebarContent = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn('flex-1 space-y-3 px-4 pt-1 pb-5', className)} {...props} />
);

const SidebarFooter = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn('border-t border-border px-4 py-3', className)} {...props} />
);

const SidebarTrigger = ({ className, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement>) => {
  const { isCollapsed, setCollapsed } = useSidebar();
  return (
    <button
      type="button"
      className={cn(
        'inline-flex h-9 w-9 items-center justify-center rounded-md border border-border bg-background text-muted-foreground transition hover:text-foreground',
        className,
      )}
      onClick={() => setCollapsed(!isCollapsed)}
      {...props}
    >
      {isCollapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
    </button>
  );
};

const sidebarMenuButtonVariants = cva(
  'flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition hover:bg-muted/70',
  {
    variants: {
      isActive: {
        true: 'bg-muted text-foreground shadow-xs',
        false: 'text-muted-foreground',
      },
    },
    defaultVariants: {
      isActive: false,
    },
  },
);

type SidebarMenuButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof sidebarMenuButtonVariants> & {
    asChild?: boolean;
    tooltip?: string;
  };

const SidebarMenuButton = React.forwardRef<HTMLButtonElement, SidebarMenuButtonProps>(
  ({ className, asChild, tooltip, isActive, ...props }, ref) => {
    const { isCollapsed } = useSidebar();
    const Comp = asChild ? Slot : 'button';
    const button = (
      <Comp
        ref={ref}
        className={cn(
          sidebarMenuButtonVariants({ isActive }),
          isCollapsed && 'justify-center px-2',
          isCollapsed && 'shadow-none',
          className,
        )}
        {...props}
      />
    );

    if (!tooltip || !isCollapsed) {
      return button;
    }

    return (
      <Tooltip>
        <TooltipTrigger asChild>{button}</TooltipTrigger>
        <TooltipContent side="right">{tooltip}</TooltipContent>
      </Tooltip>
    );
  },
);
SidebarMenuButton.displayName = 'SidebarMenuButton';

const SidebarMenu = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn('space-y-1', className)} {...props} />
);

const SidebarGroup = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn('space-y-2', className)} {...props} />
);

const SidebarInset = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn('flex flex-1 flex-col', className)} {...props} />
);

const SidebarSeparator = ({ className }: { className?: string }) => (
  <div className={cn('h-px w-full bg-border', className)} />
);

export {
  SidebarProvider,
  Sidebar,
  SidebarHeader,
  SidebarContent,
  SidebarFooter,
  SidebarTrigger,
  SidebarMenu,
  SidebarMenuButton,
  SidebarGroup,
  SidebarInset,
  SidebarSeparator,
  useSidebar,
};

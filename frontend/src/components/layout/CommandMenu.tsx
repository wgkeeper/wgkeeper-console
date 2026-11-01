import { type Dispatch, type SetStateAction, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  LogOutIcon,
  MonitorIcon,
  MoonIcon,
  PlusIcon,
  RefreshCwIcon,
  ServerIcon,
  SunIcon,
  UserIcon,
} from 'lucide-react';
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command';
import { StatusDot } from '@/components/nodes/NodeStatusBadge';
import { useTheme } from '@/components/theme-provider';
import { nodeHost } from '@/lib/node-utils';
import type { NodeItem } from '@/types';

type Props = {
  open: boolean;
  setOpen: Dispatch<SetStateAction<boolean>>;
  nodes: NodeItem[];
  onRefresh: () => void;
  onLogout: () => void;
};

export const CommandMenu = ({ open, setOpen, nodes, onRefresh, onLogout }: Props) => {
  const navigate = useNavigate();
  const { setTheme } = useTheme();

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setOpen((value) => !value);
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [setOpen]);

  const run = (action: () => void) => {
    setOpen(false);
    action();
  };

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder="Search nodes and actions…" />
      <CommandList>
        <CommandEmpty>Nothing matches.</CommandEmpty>
        {nodes.length > 0 ? (
          <CommandGroup heading="Nodes">
            {nodes.map((node) => (
              <CommandItem
                key={node.id}
                value={`${node.address} ${node.id}`}
                onSelect={() => run(() => void navigate(`/nodes/${node.id}`))}
              >
                <StatusDot tone={node.status === 'online' ? 'success' : 'offline'} />
                <span className="font-mono text-xs">{nodeHost(node.address)}</span>
                <span className="ml-auto text-xs text-muted-foreground capitalize">
                  {node.status}
                </span>
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}
        <CommandSeparator />
        <CommandGroup heading="Actions">
          <CommandItem onSelect={() => run(() => void navigate('/nodes?add=1'))}>
            <PlusIcon />
            Add node
          </CommandItem>
          <CommandItem onSelect={() => run(onRefresh)}>
            <RefreshCwIcon />
            Refresh node status
          </CommandItem>
          <CommandItem onSelect={() => run(() => void navigate('/nodes'))}>
            <ServerIcon />
            Go to nodes
          </CommandItem>
          <CommandItem onSelect={() => run(() => void navigate('/profile'))}>
            <UserIcon />
            Go to profile
          </CommandItem>
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="Theme">
          <CommandItem onSelect={() => run(() => setTheme('light'))}>
            <SunIcon />
            Light
          </CommandItem>
          <CommandItem onSelect={() => run(() => setTheme('dark'))}>
            <MoonIcon />
            Dark
          </CommandItem>
          <CommandItem onSelect={() => run(() => setTheme('system'))}>
            <MonitorIcon />
            System
          </CommandItem>
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="Account">
          <CommandItem onSelect={() => run(onLogout)}>
            <LogOutIcon />
            Sign out
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
};

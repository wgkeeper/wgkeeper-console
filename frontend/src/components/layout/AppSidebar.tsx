import { Link, useLocation } from 'react-router-dom';
import {
  ArrowUpCircleIcon,
  ChevronsUpDownIcon,
  LogOutIcon,
  MonitorIcon,
  MoonIcon,
  PlusIcon,
  ServerIcon,
  SunIcon,
  UserIcon,
} from 'lucide-react';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupAction,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuBadge,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar';
import { StatusDot } from '@/components/nodes/NodeStatusBadge';
import { type Theme, useTheme } from '@/components/theme-provider';
import { WGKeeperLogo } from '@/components/WGKeeperLogo';
import { WGKeeperMark } from '@/components/WGKeeperMark';
import { nodeHost } from '@/lib/node-utils';
import type { NodeItem } from '@/types';

type Props = {
  nodes: NodeItem[];
  nodesTotal: number;
  username?: string;
  onLogout: () => void;
};

export const AppSidebar = ({ nodes, nodesTotal, username, onLogout }: Props) => {
  const { pathname } = useLocation();
  const { state } = useSidebar();

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarHeader>
        <Link
          to="/nodes"
          className="flex h-8 items-center rounded-md px-2 outline-none group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0 focus-visible:ring-2 focus-visible:ring-sidebar-ring"
        >
          {state === 'collapsed' ? (
            <WGKeeperMark className="h-4 w-auto" />
          ) : (
            <WGKeeperLogo className="h-[1.125rem] w-auto" />
          )}
        </Link>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton asChild isActive={pathname === '/nodes'} tooltip="Nodes">
                <Link to="/nodes">
                  <ServerIcon />
                  <span>Nodes</span>
                </Link>
              </SidebarMenuButton>
              <SidebarMenuBadge>{nodesTotal}</SidebarMenuBadge>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <SidebarMenuButton
                asChild
                isActive={pathname.startsWith('/profile')}
                tooltip="Profile"
              >
                <Link to="/profile">
                  <UserIcon />
                  <span>Profile</span>
                </Link>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>

        {nodes.length > 0 ? (
          <SidebarGroup className="group-data-[collapsible=icon]:hidden">
            <SidebarGroupLabel>Fleet</SidebarGroupLabel>
            <SidebarGroupAction asChild title="Add node">
              <Link to="/nodes?add=1">
                <PlusIcon />
                <span className="sr-only">Add node</span>
              </Link>
            </SidebarGroupAction>
            <SidebarMenu>
              {nodes.map((node) => (
                <SidebarMenuItem key={node.id}>
                  <SidebarMenuButton
                    asChild
                    size="sm"
                    isActive={pathname.startsWith(`/nodes/${node.id}`)}
                  >
                    <Link
                      to={`/nodes/${node.id}`}
                      title={`${node.address}: ${node.status}${node.isOutdated ? ', update available' : ''}`}
                    >
                      <StatusDot tone={node.status === 'online' ? 'success' : 'offline'} />
                      <span className="truncate font-mono text-xs">{nodeHost(node.address)}</span>
                      <span className="sr-only">
                        , {node.status}
                        {node.isOutdated ? ', update available' : ''}
                      </span>
                      {node.isOutdated ? (
                        <ArrowUpCircleIcon aria-hidden className="ml-auto size-3.5 text-warning" />
                      ) : null}
                    </Link>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroup>
        ) : null}
      </SidebarContent>

      <SidebarFooter>
        <NavUser username={username ?? 'admin'} onLogout={onLogout} />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
};

const NavUser = ({ username, onLogout }: { username: string; onLogout: () => void }) => {
  const { isMobile } = useSidebar();
  const { theme, setTheme } = useTheme();
  const initials = username.slice(0, 2).toUpperCase();

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <Avatar className="size-8 rounded-lg">
                <AvatarFallback className="rounded-lg text-xs">{initials}</AvatarFallback>
              </Avatar>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-medium">{username}</span>
                <span className="truncate text-xs text-muted-foreground">Administrator</span>
              </div>
              <ChevronsUpDownIcon className="ml-auto size-4" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
            side={isMobile ? 'bottom' : 'right'}
            align="end"
            sideOffset={4}
          >
            <DropdownMenuLabel className="p-0 font-normal">
              <div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
                <Avatar className="size-8 rounded-lg">
                  <AvatarFallback className="rounded-lg text-xs">{initials}</AvatarFallback>
                </Avatar>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-medium">{username}</span>
                  <span className="truncate text-xs text-muted-foreground">Administrator</span>
                </div>
              </div>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuItem asChild>
                <Link to="/profile">
                  <UserIcon />
                  Profile
                </Link>
              </DropdownMenuItem>
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <SunIcon className="dark:hidden" />
                  <MoonIcon className="hidden dark:block" />
                  Theme
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent>
                  <DropdownMenuRadioGroup
                    value={theme}
                    onValueChange={(value) => setTheme(value as Theme)}
                  >
                    <DropdownMenuRadioItem value="light">
                      <SunIcon />
                      Light
                    </DropdownMenuRadioItem>
                    <DropdownMenuRadioItem value="dark">
                      <MoonIcon />
                      Dark
                    </DropdownMenuRadioItem>
                    <DropdownMenuRadioItem value="system">
                      <MonitorIcon />
                      System
                    </DropdownMenuRadioItem>
                  </DropdownMenuRadioGroup>
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={onLogout}>
              <LogOutIcon />
              Sign out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
};

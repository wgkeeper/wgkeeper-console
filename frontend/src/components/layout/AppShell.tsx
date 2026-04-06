import { LogOut, Moon, Server, Sun, User } from 'lucide-react';
import { NavLink, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { Suspense, lazy, useEffect } from 'react';
import { Switch } from '@/components/ui/switch';
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarProvider,
  SidebarSeparator,
  SidebarTrigger,
  useSidebar,
} from '@/components/ui/sidebar';
import { Skeleton } from '@/components/ui/skeleton';
import type { NodeItem } from '@/types';
import { WGKeeperLogo } from '@/components/WGKeeperLogo';

const NodesPage = lazy(() => import('@/pages/NodesPage').then((m) => ({ default: m.NodesPage })));
const NodeDetailPage = lazy(() =>
  import('@/pages/NodeDetailPage').then((m) => ({ default: m.NodeDetailPage })),
);
const ProfilePage = lazy(() =>
  import('@/pages/ProfilePage').then((m) => ({ default: m.ProfilePage })),
);

type AppShellProps = {
  nodes: NodeItem[];
  nodesPage: number;
  nodesTotalPages: number;
  nodesTotal: number;
  nodesLoading?: boolean;
  nodesError?: string | null;
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  onReloadNodes: (page?: number) => Promise<void>;
  theme: 'light' | 'dark';
  onThemeToggle: (checked: boolean) => void;
  onLogout: () => void;
  username?: string;
};

export const AppShell = ({
  nodes,
  nodesPage,
  nodesTotalPages,
  nodesTotal,
  nodesLoading,
  nodesError,
  apiFetch,
  onReloadNodes,
  theme,
  onThemeToggle,
  onLogout,
  username,
}: AppShellProps) => {
  const location = useLocation();
  const pathname = location.pathname;

  const pageMeta = (() => {
    if (pathname.startsWith('/profile'))
      return { title: 'Profile', description: 'Manage your console account.' };
    return { title: 'Nodes', description: 'Manage your WireGuard nodes.' };
  })();

  useEffect(() => {
    let page: string;
    if (pathname.startsWith('/profile')) page = 'Profile';
    else if (/^\/nodes\/[^/]+/.test(pathname)) {
      const node = nodes.find((n) => pathname.includes(n.id));
      page = node ? node.address : 'Node';
    } else {
      page = 'Nodes';
    }
    document.title = `WGKeeper Console · ${page}`;
  }, [pathname, nodes]);

  return (
    <SidebarProvider defaultOpen>
      <div className="flex min-h-screen bg-background text-foreground">
        <Sidebar>
          <SidebarHeader>
            <SidebarHeaderContent />
          </SidebarHeader>
          <SidebarSeparator />
          <SidebarContent>
            <SidebarSection />
          </SidebarContent>
          <SidebarFooter>
            <SidebarLogoutButton onLogout={onLogout} username={username ?? undefined} />
          </SidebarFooter>
        </Sidebar>
        <SidebarInset>
          <main className="flex-1 bg-muted/20">
            <div className="sticky top-0 z-10 border-b border-border/60 bg-background/95 backdrop-blur">
              <div className="mx-auto flex w-full max-w-6xl flex-wrap items-center justify-between gap-4 px-6 py-8">
                <div className="flex flex-col gap-0.5">
                  <h1 className="text-2xl font-semibold">{pageMeta.title}</h1>
                  <p className="text-sm text-muted-foreground">{pageMeta.description}</p>
                </div>
                <div className="flex items-center gap-2">
                  <Sun
                    className={`size-4 ${theme === 'light' ? 'text-foreground' : 'text-muted-foreground'}`}
                  />
                  <Switch
                    id="theme-toggle"
                    checked={theme === 'dark'}
                    onCheckedChange={onThemeToggle}
                    aria-label="Toggle theme"
                  />
                  <Moon
                    className={`size-4 ${theme === 'dark' ? 'text-foreground' : 'text-muted-foreground'}`}
                  />
                </div>
              </div>
            </div>
            <div className="mx-auto w-full max-w-6xl space-y-8 px-6 py-8">
              <Suspense
                fallback={
                  <div className="flex flex-col gap-6">
                    <div className="flex flex-col gap-2">
                      <Skeleton className="h-7 w-36" />
                      <Skeleton className="h-4 w-72" />
                    </div>
                    <Skeleton className="h-[110px] w-full rounded-xl" />
                    <Skeleton className="h-[320px] w-full rounded-xl" />
                  </div>
                }
              >
                <Routes>
                  <Route path="/" element={<Navigate to="/nodes" replace />} />
                  <Route path="/login" element={<Navigate to="/nodes" replace />} />
                  <Route
                    path="/nodes"
                    element={
                      <NodesPage
                        nodes={nodes}
                        nodesPage={nodesPage}
                        nodesTotalPages={nodesTotalPages}
                        nodesTotal={nodesTotal}
                        nodesLoading={nodesLoading}
                        nodesError={nodesError}
                        apiFetch={apiFetch}
                        onReloadNodes={onReloadNodes}
                      />
                    }
                  />
                  <Route
                    path="/nodes/:id"
                    element={
                      <NodeDetailPage
                        nodes={nodes}
                        apiFetch={apiFetch}
                        onReloadNodes={onReloadNodes}
                      />
                    }
                  />
                  <Route
                    path="/profile"
                    element={
                      <ProfilePage apiFetch={apiFetch} initialUsername={username ?? undefined} />
                    }
                  />
                  <Route
                    path="*"
                    element={
                      <div className="flex flex-col gap-3 py-10 text-center text-muted-foreground">
                        <p className="text-4xl font-semibold">404</p>
                        <p className="text-sm">Page not found.</p>
                      </div>
                    }
                  />
                </Routes>
              </Suspense>
            </div>
          </main>
        </SidebarInset>
      </div>
    </SidebarProvider>
  );
};

const SidebarHeaderContent = () => {
  const { isCollapsed } = useSidebar();
  if (isCollapsed) {
    return (
      <div className="flex w-full justify-center">
        <SidebarTrigger />
      </div>
    );
  }
  return (
    <div className="flex w-full items-center justify-between">
      <SidebarBrand />
      <SidebarTrigger />
    </div>
  );
};

const SidebarBrand = () => {
  return (
    <div className="flex flex-col gap-0.5">
      <WGKeeperLogo className="h-7 w-auto" />
      <p className="text-[10px] tracking-widest text-muted-foreground/70 uppercase">Console</p>
    </div>
  );
};

const SidebarSection = () => {
  const { isCollapsed } = useSidebar();
  const location = useLocation();
  const pathname = location.pathname;
  const isNodes = pathname === '/' || pathname.startsWith('/nodes');
  const isProfile = pathname.startsWith('/profile');

  const items = [
    { to: '/nodes', label: 'Nodes', Icon: Server, isActive: isNodes },
    { to: '/profile', label: 'Profile', Icon: User, isActive: isProfile },
  ] as const;

  return (
    <SidebarGroup>
      <SidebarMenu>
        {items.map(({ to, label, Icon, isActive }) => (
          <SidebarMenuButton key={to} asChild isActive={isActive} tooltip={label}>
            <NavLink to={to}>
              <Icon className="size-4" />
              {!isCollapsed && <span>{label}</span>}
            </NavLink>
          </SidebarMenuButton>
        ))}
      </SidebarMenu>
    </SidebarGroup>
  );
};

const SidebarLogoutButton = ({
  onLogout,
  username,
}: {
  onLogout: () => void;
  username?: string;
}) => {
  const { isCollapsed } = useSidebar();
  return (
    <SidebarMenu>
      <SidebarMenuButton
        tooltip="Sign out"
        onClick={onLogout}
        className="text-muted-foreground hover:text-foreground"
      >
        <LogOut className="size-4" />
        {!isCollapsed && (
          <span className="flex flex-col items-start truncate">
            <span className="text-sm font-medium text-foreground">Sign out</span>
            {username && <span className="text-xs text-muted-foreground">@{username}</span>}
          </span>
        )}
      </SidebarMenuButton>
    </SidebarMenu>
  );
};

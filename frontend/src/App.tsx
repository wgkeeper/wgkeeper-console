import { useCallback, useEffect, useState } from 'react';
import { Navigate, Route, Routes, useNavigate } from 'react-router-dom';
import type { NodeItem, NodeListResponse } from './types';
import { LoginScreen } from '@/components/auth/LoginScreen';
import { ChangePasswordScreen } from '@/components/auth/ChangePasswordScreen';
import { AppShell } from '@/components/layout/AppShell';
import { Skeleton } from '@/components/ui/skeleton';
import { createApiFetch } from '@/lib/api-client';
import { NODES_PAGE_SIZE } from '@/lib/peer-utils';

const getInitialTheme = (): 'light' | 'dark' => {
  if (typeof document !== 'undefined' && document.documentElement.classList.contains('dark')) {
    return 'dark';
  }

  try {
    return localStorage.getItem('wg-theme') === 'dark' ? 'dark' : 'light';
  } catch {
    return 'light';
  }
};

function App() {
  const [isLoading, setIsLoading] = useState(true);
  const [isAuthed, setIsAuthed] = useState(false);
  const [mustChangePassword, setMustChangePassword] = useState(false);
  const [loginError, setLoginError] = useState('');
  const [loginForm, setLoginForm] = useState({ username: '', password: '' });
  const [isLoginSubmitting, setIsLoginSubmitting] = useState(false);
  const [passwordForm, setPasswordForm] = useState({ currentPassword: '', newPassword: '' });
  const [passwordError, setPasswordError] = useState('');
  const [isPasswordSubmitting, setIsPasswordSubmitting] = useState(false);
  const [nodes, setNodes] = useState<NodeItem[]>([]);
  const [nodesPage, setNodesPage] = useState(1);
  const [nodesTotalPages, setNodesTotalPages] = useState(1);
  const [nodesTotal, setNodesTotal] = useState(0);
  const [nodesLoading, setNodesLoading] = useState(false);
  const [nodesError, setNodesError] = useState<string | null>(null);
  const [username, setUsername] = useState<string | null>(null);
  const [theme, setTheme] = useState<'light' | 'dark'>(getInitialTheme);
  const navigate = useNavigate();

  const [apiFetch] = useState(() =>
    createApiFetch(() => {
      setIsAuthed(false);
      setMustChangePassword(false);
      setNodes([]);
      setUsername(null);
    }),
  );

  useEffect(() => {
    const controller = new AbortController();
    const signal = controller.signal;

    const bootstrap = async () => {
      try {
        const meRes = await fetch('/api/me', {
          credentials: 'include',
          signal,
        });
        if (meRes.ok) {
          setIsAuthed(true);
          const meBody = (await meRes.json()) as {
            username?: string;
            mustChangePassword?: boolean;
          };
          if (meBody.username) setUsername(meBody.username);
          setMustChangePassword(Boolean(meBody.mustChangePassword));
        } else {
          setIsAuthed(false);
        }
      } catch (err) {
        if ((err as Error).name === 'AbortError') return;
        throw err;
      } finally {
        if (!signal.aborted) setIsLoading(false);
      }
    };

    void bootstrap();
    return () => controller.abort();
  }, [setUsername]);

  const loadNodes = useCallback(
    async (page = 1) => {
      setNodesLoading(true);
      setNodesError(null);
      try {
        const res = await apiFetch(`/api/nodes?page=${page}&limit=${NODES_PAGE_SIZE}`);
        if (!res.ok) {
          setNodesError('Failed to load nodes. Please try again.');
          return;
        }
        const data = (await res.json()) as NodeListResponse;
        setNodes(data.nodes);
        setNodesPage(data.page);
        setNodesTotalPages(data.totalPages);
        setNodesTotal(data.total);
      } catch {
        setNodesError('Failed to load nodes. Please try again.');
      } finally {
        setNodesLoading(false);
      }
    },
    [apiFetch],
  );

  useEffect(() => {
    if (isAuthed && !mustChangePassword) {
      void loadNodes();
    }
  }, [isAuthed, mustChangePassword, loadNodes]);

  useEffect(() => {
    const isDark = theme === 'dark';
    document.documentElement.classList.toggle('dark', isDark);
    document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';
    localStorage.setItem('wg-theme', theme);
  }, [theme]);

  const handleLogin = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setLoginError('');
    setIsLoginSubmitting(true);

    const res = await fetch('/api/login', {
      method: 'POST',
      body: JSON.stringify(loginForm),
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
    });

    if (!res.ok) {
      setLoginError('Invalid username or password.');
      setIsLoginSubmitting(false);
      return;
    }

    const body = (await res.json()) as {
      ok: boolean;
      mustChangePassword?: boolean;
      username?: string;
    };
    if (!body.ok) {
      setLoginError('Invalid username or password.');
      setIsLoginSubmitting(false);
      return;
    }

    setIsAuthed(true);
    if (body.username) setUsername(body.username);
    setMustChangePassword(Boolean(body.mustChangePassword));
    setIsLoginSubmitting(false);
  };

  const handleLogout = async () => {
    await apiFetch('/api/logout', { method: 'POST' });
    setIsAuthed(false);
    setNodes([]);
    setUsername(null);
    setLoginForm({ username: '', password: '' });
    setPasswordForm({ currentPassword: '', newPassword: '' });
    setPasswordError('');
    setMustChangePassword(false);
    void navigate('/login', { replace: true });
  };

  const handleChangePassword = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setPasswordError('');
    setIsPasswordSubmitting(true);

    const payload: { currentPassword?: string; newPassword: string } = {
      newPassword: passwordForm.newPassword,
    };
    if (!mustChangePassword) {
      payload.currentPassword = passwordForm.currentPassword;
    }

    const res = await apiFetch('/api/change-password', {
      method: 'POST',
      body: JSON.stringify(payload),
    });

    if (!res.ok) {
      try {
        const body = (await res.json()) as { error?: string };
        if (body?.error === 'password_too_short') {
          setPasswordError('Password is too short (minimum 8 characters).');
        } else if (body?.error === 'password_not_allowed') {
          setPasswordError("The password 'admin' is not allowed. Choose a different password.");
        } else if (body?.error === 'current_password_required') {
          setPasswordError('Current password is required.');
        } else if (body?.error === 'invalid_current_password') {
          setPasswordError('Current password is incorrect.');
        } else {
          setPasswordError('Unable to change password. Please try again.');
        }
      } catch {
        setPasswordError('Unable to change password. Please try again.');
      }
      setIsPasswordSubmitting(false);
      return;
    }

    setMustChangePassword(false);
    setPasswordForm({ currentPassword: '', newPassword: '' });
    setIsPasswordSubmitting(false);
    await loadNodes();
  };

  const handleThemeToggle = (checked: boolean) => {
    setTheme(checked ? 'dark' : 'light');
  };

  if (isLoading) {
    return (
      <div className="flex min-h-screen bg-background">
        <div className="hidden w-64 flex-shrink-0 flex-col gap-4 border-r border-border p-4 md:flex">
          <div className="flex items-center gap-3 p-2">
            <Skeleton className="size-10 rounded-lg" />
            <div className="flex flex-col gap-1.5">
              <Skeleton className="h-4 w-28" />
              <Skeleton className="h-3 w-20" />
            </div>
          </div>
          <div className="flex flex-col gap-2 pt-4">
            <Skeleton className="h-8 w-full rounded-md" />
            <Skeleton className="h-8 w-full rounded-md" />
          </div>
        </div>
        <div className="flex flex-1 flex-col">
          <div className="border-b border-border px-6 py-8">
            <div className="mx-auto flex max-w-6xl flex-col gap-2">
              <Skeleton className="h-3 w-32" />
              <Skeleton className="h-7 w-24" />
              <Skeleton className="h-4 w-80" />
            </div>
          </div>
          <div className="mx-auto flex w-full max-w-6xl flex-col gap-6 px-6 py-8">
            <Skeleton className="h-[100px] w-full rounded-xl" />
            <Skeleton className="h-[280px] w-full rounded-xl" />
          </div>
        </div>
      </div>
    );
  }

  // Not authenticated: expose only /login and redirect all other routes there
  if (!isAuthed) {
    return (
      <Routes>
        <Route
          path="/login"
          element={
            <LoginScreen
              form={loginForm}
              error={loginError}
              isSubmitting={isLoginSubmitting}
              onChange={(field, value) =>
                setLoginForm((prev) => ({
                  ...prev,
                  [field]: value,
                }))
              }
              onSubmit={handleLogin}
            />
          }
        />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  // Authenticated, but password change is required: show the password change screen for any URL
  if (mustChangePassword) {
    return (
      <Routes>
        <Route
          path="*"
          element={
            <ChangePasswordScreen
              mustChangePassword={mustChangePassword}
              form={passwordForm}
              error={passwordError}
              isSubmitting={isPasswordSubmitting}
              onChange={(field, value) =>
                setPasswordForm((prev) => ({
                  ...prev,
                  [field]: value,
                }))
              }
              onSubmit={handleChangePassword}
            />
          }
        />
      </Routes>
    );
  }

  // Fully authenticated user: all remaining routes are handled inside AppShell
  return (
    <AppShell
      nodes={nodes}
      nodesPage={nodesPage}
      nodesTotalPages={nodesTotalPages}
      nodesTotal={nodesTotal}
      nodesLoading={nodesLoading}
      nodesError={nodesError}
      apiFetch={apiFetch}
      onReloadNodes={loadNodes}
      theme={theme}
      onThemeToggle={handleThemeToggle}
      onLogout={handleLogout}
      username={username ?? undefined}
    />
  );
}
export default App;

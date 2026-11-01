import { useCallback, useEffect, useRef, useState } from 'react';
import { Navigate, Route, Routes, useNavigate } from 'react-router-dom';
import type { FleetSummary, NodeItem, NodeListResponse } from './types';
import { LoginScreen } from '@/components/auth/LoginScreen';
import { ChangePasswordScreen } from '@/components/auth/ChangePasswordScreen';
import { AppShell } from '@/components/layout/AppShell';
import { Skeleton } from '@/components/ui/skeleton';
import {
  createApiFetch,
  fetchErrorMessage,
  rateLimitMessage,
  REQUEST_TIMEOUT_MS,
} from '@/lib/api-client';
import { NODES_PAGE_SIZE } from '@/lib/peer-utils';

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
  const [nodesSummary, setNodesSummary] = useState<FleetSummary | null>(null);
  // The page on screen and the latest list request: a slow response for a
  // page the user already left must not replace the one they are looking at.
  const currentPage = useRef(1);
  const latestNodesRequest = useRef(0);
  const [username, setUsername] = useState<string | null>(null);
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
      // Fetch the landing page's code while the session check is in flight.
      void import('@/pages/NodesPage');
      try {
        const mergedSignal = AbortSignal.any([signal, AbortSignal.timeout(REQUEST_TIMEOUT_MS)]);
        const meRes = await fetch('/api/me', {
          credentials: 'include',
          signal: mergedSignal,
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
        const name = (err as Error).name;
        // Treat unmount/timeout as "not authenticated" so the login screen
        // appears instead of an indefinite skeleton.
        if (name === 'AbortError' && signal.aborted) return;
        setIsAuthed(false);
      } finally {
        if (!signal.aborted) setIsLoading(false);
      }
    };

    void bootstrap();
    return () => controller.abort();
  }, [setUsername]);

  /** Loads a page of nodes (the current one by default); resolves to whether it worked. */
  const loadNodes = useCallback(
    async (page?: number) => {
      const target = page ?? currentPage.current;
      const request = ++latestNodesRequest.current;
      const isLatest = () => request === latestNodesRequest.current;
      setNodesLoading(true);
      setNodesError(null);
      try {
        const res = await apiFetch(`/api/nodes?page=${target}&limit=${NODES_PAGE_SIZE}`);
        if (!isLatest()) return false;
        if (!res.ok) {
          setNodesError('The console could not load the node list. Try again.');
          return false;
        }
        const data = (await res.json()) as NodeListResponse;
        if (!isLatest()) return false;
        currentPage.current = data.page;
        setNodes(data.nodes);
        setNodesPage(data.page);
        setNodesTotalPages(data.totalPages);
        setNodesTotal(data.total);
        setNodesSummary(data.summary ?? null);
        return true;
      } catch {
        if (isLatest()) setNodesError('The console could not load the node list. Try again.');
        return false;
      } finally {
        if (isLatest()) setNodesLoading(false);
      }
    },
    [apiFetch],
  );

  useEffect(() => {
    if (isAuthed && !mustChangePassword) {
      void loadNodes();
    }
  }, [isAuthed, mustChangePassword, loadNodes]);

  const handleLogin = async (event: React.SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setLoginError('');
    setIsLoginSubmitting(true);

    let res: Response;
    try {
      res = await fetch('/api/login', {
        method: 'POST',
        body: JSON.stringify(loginForm),
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
      });
    } catch {
      setLoginError('Network error. Please try again.');
      setIsLoginSubmitting(false);
      return;
    }

    if (!res.ok) {
      const rateLimited = rateLimitMessage(res);
      if (rateLimited) {
        setLoginError(rateLimited);
      } else if (res.status === 401) {
        setLoginError('Invalid username or password.');
      } else {
        setLoginError('Login failed. Please try again.');
      }
      setIsLoginSubmitting(false);
      return;
    }

    let body: { ok: boolean; mustChangePassword?: boolean; username?: string };
    try {
      body = (await res.json()) as typeof body;
    } catch {
      setLoginError('Login failed. Please try again.');
      setIsLoginSubmitting(false);
      return;
    }
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
    // Always clear local state, even if the server request fails — otherwise
    // a network blip leaves the UI thinking we're still logged in with a
    // cookie the backend may have already invalidated.
    try {
      await apiFetch('/api/logout', { method: 'POST' });
    } catch {
      // ignore — local cleanup below is what matters for the user
    } finally {
      setIsAuthed(false);
      setNodes([]);
      setUsername(null);
      setLoginForm({ username: '', password: '' });
      setPasswordForm({ currentPassword: '', newPassword: '' });
      setPasswordError('');
      setMustChangePassword(false);
      void navigate('/login', { replace: true });
    }
  };

  const handleChangePassword = async (event: React.SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setPasswordError('');
    setIsPasswordSubmitting(true);

    const payload: { currentPassword?: string; newPassword: string } = {
      newPassword: passwordForm.newPassword,
    };
    if (!mustChangePassword) {
      payload.currentPassword = passwordForm.currentPassword;
    }

    let res: Response;
    try {
      res = await apiFetch('/api/change-password', {
        method: 'POST',
        body: JSON.stringify(payload),
      });
    } catch (err) {
      setPasswordError(fetchErrorMessage(err) ?? 'Network error. Please try again.');
      setIsPasswordSubmitting(false);
      return;
    }

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

  if (isLoading) {
    return (
      <div className="flex min-h-svh bg-sidebar">
        <div className="hidden w-64 shrink-0 flex-col gap-2 p-4 md:flex">
          <Skeleton className="h-5 w-32" />
          <div className="mt-6 flex flex-col gap-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        </div>
        <div className="m-2 flex flex-1 flex-col rounded-xl bg-background shadow-sm md:ml-0">
          <div className="flex h-14 items-center border-b px-4">
            <Skeleton className="h-4 w-40" />
          </div>
          <div className="mx-auto flex w-full max-w-6xl flex-col gap-6 p-6">
            <Skeleton className="h-8 w-48" />
            <Skeleton className="h-24 w-full rounded-xl" />
            <Skeleton className="h-80 w-full rounded-xl" />
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
      nodesSummary={nodesSummary}
      apiFetch={apiFetch}
      onReloadNodes={loadNodes}
      onLogout={handleLogout}
      username={username ?? undefined}
    />
  );
}
export default App;

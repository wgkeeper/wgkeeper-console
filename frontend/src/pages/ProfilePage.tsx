import { useEffect, useState } from 'react';
import { fetchErrorMessage } from '@/lib/api-client';
import { AlertCircle, CheckCircle2, UserCircle2 } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';

type Props = {
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  initialUsername?: string;
};

export const ProfilePage = ({ apiFetch, initialUsername }: Props) => {
  const [username, setUsername] = useState(initialUsername ?? '');
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (username) return;
    const loadMe = async () => {
      try {
        const res = await apiFetch('/api/me');
        if (!res.ok) return;
        const body = (await res.json()) as { authenticated?: boolean; username?: string };
        if (body.authenticated && body.username) {
          setUsername(body.username);
        }
      } catch {
        // ignore
      }
    };

    void loadMe();
  }, [apiFetch, username]);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError('');
    setNotice('');

    if (!currentPassword.trim() || !newPassword.trim()) {
      setError('Fill in current and new password.');
      return;
    }
    if (newPassword.length < 8) {
      setError('New password is too short (minimum 8 characters).');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('New password and confirmation do not match.');
      return;
    }

    setIsSubmitting(true);
    try {
      const res = await apiFetch('/api/change-password', {
        method: 'POST',
        body: JSON.stringify({
          currentPassword,
          newPassword,
        }),
      });

      if (!res.ok) {
        try {
          const body = (await res.json()) as { error?: string };
          if (body?.error === 'password_too_short') {
            setError('New password is too short (minimum 8 characters).');
          } else if (body?.error === 'password_not_allowed') {
            setError("The password 'admin' is not allowed. Choose a different password.");
          } else if (body?.error === 'current_password_required') {
            setError('Current password is required.');
          } else if (body?.error === 'invalid_current_password') {
            setError('Current password is incorrect.');
          } else {
            setError('Unable to change password. Please try again.');
          }
        } catch {
          setError('Unable to change password. Please try again.');
        }
        return;
      }

      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setNotice('Password has been updated.');
    } catch (err) {
      setError(fetchErrorMessage(err) ?? 'Unable to change password. Please try again.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="flex max-w-lg flex-col gap-4">
      <Card>
        {username ? (
          <>
            <CardContent className="flex items-center gap-3 px-5 py-4">
              <div className="flex size-9 flex-shrink-0 items-center justify-center rounded-full bg-muted">
                <UserCircle2 className="size-4 text-muted-foreground" />
              </div>
              <div className="flex flex-col gap-0.5">
                <span className="text-sm font-medium">@{username}</span>
                <span className="text-xs text-muted-foreground">Console account</span>
              </div>
            </CardContent>
            <Separator />
          </>
        ) : null}
        <CardHeader className="pb-4">
          <CardTitle className="text-base">Change password</CardTitle>
          <CardDescription>Use a strong password that you do not reuse elsewhere.</CardDescription>
        </CardHeader>
        <CardContent>
          <form id="change-password-form" className="flex flex-col gap-4" onSubmit={handleSubmit}>
            <div className="flex flex-col gap-1.5">
              <Label
                htmlFor="currentPassword"
                className="text-xs font-medium tracking-wide text-muted-foreground uppercase"
              >
                Current password
              </Label>
              <Input
                id="currentPassword"
                name="currentPassword"
                type="password"
                autoComplete="current-password"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                required
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label
                htmlFor="newPassword"
                className="text-xs font-medium tracking-wide text-muted-foreground uppercase"
              >
                New password
              </Label>
              <Input
                id="newPassword"
                name="newPassword"
                type="password"
                autoComplete="new-password"
                placeholder="Min. 8 characters"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                required
                minLength={8}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label
                htmlFor="confirmPassword"
                className="text-xs font-medium tracking-wide text-muted-foreground uppercase"
              >
                Confirm new password
              </Label>
              <Input
                id="confirmPassword"
                name="confirmPassword"
                type="password"
                autoComplete="new-password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                required
              />
            </div>
            {notice ? (
              <Alert className="border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
                <CheckCircle2 className="size-4 !text-emerald-600 dark:!text-emerald-400" />
                <AlertDescription>{notice}</AlertDescription>
              </Alert>
            ) : null}
            {error ? (
              <Alert variant="destructive">
                <AlertCircle className="size-4" />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
          </form>
        </CardContent>
        <Separator />
        <CardFooter className="flex justify-end pt-4">
          <Button type="submit" form="change-password-form" disabled={isSubmitting}>
            {isSubmitting ? 'Saving…' : 'Save password'}
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
};

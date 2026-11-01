import { useEffect, useState } from 'react';
import { CircleAlertIcon, MonitorIcon, MoonIcon, SunIcon } from 'lucide-react';
import { toast } from 'sonner';
import { Alert, AlertTitle } from '@/components/ui/alert';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { PageHeader } from '@/components/layout/PageHeader';
import { type Theme, useTheme } from '@/components/theme-provider';
import { fetchErrorMessage } from '@/lib/api-client';

type Props = {
  apiFetch: (path: string, options?: RequestInit) => Promise<Response>;
  initialUsername?: string;
};

export const ProfilePage = ({ apiFetch, initialUsername }: Props) => {
  const { theme, setTheme } = useTheme();
  const [username, setUsername] = useState(initialUsername ?? '');
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
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

  const handleSubmit = async (event: React.SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError('');

    if (!currentPassword.trim() || !newPassword.trim()) {
      setError('Enter your current and new password.');
      return;
    }
    if (newPassword.length < 8) {
      setError('The new password needs at least 8 characters.');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('The new password and its confirmation do not match.');
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
            setError('The new password needs at least 8 characters.');
          } else if (body?.error === 'password_not_allowed') {
            setError("The password 'admin' is not allowed. Choose a different password.");
          } else if (body?.error === 'current_password_required') {
            setError('Enter your current password.');
          } else if (body?.error === 'invalid_current_password') {
            setError('The current password is incorrect.');
          } else {
            setError('The password was not changed. Try again.');
          }
        } catch {
          setError('The password was not changed. Try again.');
        }
        return;
      }

      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      toast.success('Password changed', {
        description: 'Other sessions for this account were signed out.',
      });
    } catch (err) {
      setError(fetchErrorMessage(err) ?? 'The password was not changed. Try again.');
    } finally {
      setIsSubmitting(false);
    }
  };

  const name = username || 'admin';

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <PageHeader title="Profile" description="Your console account and preferences." />

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Account</h2>
          </CardTitle>
          <CardDescription>The account you are signed in with.</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center gap-3">
          <Avatar className="size-10 rounded-lg">
            <AvatarFallback className="rounded-lg">{name.slice(0, 2).toUpperCase()}</AvatarFallback>
          </Avatar>
          <div className="flex flex-col">
            <span className="font-medium">{name}</span>
            <span className="text-sm text-muted-foreground">Administrator</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Appearance</h2>
          </CardTitle>
          <CardDescription>System follows the theme of your operating system.</CardDescription>
        </CardHeader>
        <CardContent>
          <ToggleGroup
            type="single"
            variant="outline"
            value={theme}
            onValueChange={(value) => value && setTheme(value as Theme)}
            aria-label="Theme"
          >
            <ToggleGroupItem value="light" className="px-3">
              <SunIcon />
              Light
            </ToggleGroupItem>
            <ToggleGroupItem value="dark" className="px-3">
              <MoonIcon />
              Dark
            </ToggleGroupItem>
            <ToggleGroupItem value="system" className="px-3">
              <MonitorIcon />
              System
            </ToggleGroupItem>
          </ToggleGroup>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>
            <h2>Password</h2>
          </CardTitle>
          <CardDescription>
            Changing it signs out every other session of this account.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form id="change-password-form" onSubmit={handleSubmit}>
            <FieldGroup className="max-w-sm">
              <Field>
                <FieldLabel htmlFor="currentPassword">Current password</FieldLabel>
                <Input
                  id="currentPassword"
                  name="currentPassword"
                  type="password"
                  autoComplete="current-password"
                  value={currentPassword}
                  onChange={(event) => setCurrentPassword(event.target.value)}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="newPassword">New password</FieldLabel>
                <Input
                  id="newPassword"
                  name="newPassword"
                  type="password"
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                  required
                  minLength={8}
                />
                <FieldDescription>At least 8 characters.</FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="confirmPassword">Confirm new password</FieldLabel>
                <Input
                  id="confirmPassword"
                  name="confirmPassword"
                  type="password"
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  required
                />
              </Field>
              {error ? (
                <Alert variant="destructive">
                  <CircleAlertIcon />
                  <AlertTitle>{error}</AlertTitle>
                </Alert>
              ) : null}
            </FieldGroup>
          </form>
        </CardContent>
        <CardFooter className="justify-end border-t">
          <Button type="submit" form="change-password-form" disabled={isSubmitting}>
            {isSubmitting ? <Spinner /> : null}
            Change password
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
};

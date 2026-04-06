import { AlertCircle } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { WGKeeperLogo } from '@/components/WGKeeperLogo';

type ChangePasswordFormState = {
  currentPassword: string;
  newPassword: string;
};

type Props = {
  mustChangePassword: boolean;
  form: ChangePasswordFormState;
  error: string;
  isSubmitting: boolean;
  onChange: (field: 'currentPassword' | 'newPassword', value: string) => void;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
};

export const ChangePasswordScreen = ({
  mustChangePassword,
  form,
  error,
  isSubmitting,
  onChange,
  onSubmit,
}: Props) => {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex min-h-screen w-full max-w-md flex-col justify-center px-6 py-10">
        <div className="mb-8 text-center">
          <WGKeeperLogo className="mx-auto mb-2 h-10 w-auto" />
          <p className="text-xs tracking-[0.18em] text-muted-foreground uppercase">Console</p>
          <h1 className="mt-3 text-3xl font-semibold">Set new password</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Please choose a new password for your account.
          </p>
        </div>
        <Card>
          <CardHeader>
            <CardTitle>Change password</CardTitle>
            <CardDescription>
              For security reasons, you must change the default password.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form className="flex flex-col gap-4" onSubmit={onSubmit}>
              {!mustChangePassword ? (
                <div className="flex flex-col gap-2">
                  <Label htmlFor="currentPassword">Current password</Label>
                  <Input
                    id="currentPassword"
                    name="currentPassword"
                    type="password"
                    autoComplete="current-password"
                    value={form.currentPassword}
                    onChange={(event) => onChange('currentPassword', event.target.value)}
                  />
                </div>
              ) : null}
              <div className="flex flex-col gap-2">
                <Label htmlFor="newPassword">New password</Label>
                <Input
                  id="newPassword"
                  name="newPassword"
                  type="password"
                  autoComplete="new-password"
                  value={form.newPassword}
                  onChange={(event) => onChange('newPassword', event.target.value)}
                  required
                  minLength={8}
                />
              </div>
              {error ? (
                <Alert variant="destructive">
                  <AlertCircle className="size-4" />
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              ) : null}
              <Button className="w-full" type="submit" disabled={isSubmitting}>
                {isSubmitting ? 'Saving…' : 'Save password'}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

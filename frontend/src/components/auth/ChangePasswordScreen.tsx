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
    <div className="relative flex min-h-screen flex-col items-center justify-center gap-7 bg-background px-4 py-12 text-foreground">
      <div className="flex flex-col items-center gap-2">
        <WGKeeperLogo className="h-7 w-auto" />
        <p className="text-[0.7rem] font-medium tracking-[0.2em] text-muted-foreground uppercase">
          Console
        </p>
      </div>

      <Card className="w-full max-w-sm">
        <CardHeader className="gap-1.5">
          <CardTitle>Set a new password</CardTitle>
          <CardDescription>
            {mustChangePassword
              ? 'For security, replace the default password before continuing.'
              : 'Choose a strong password you do not reuse elsewhere.'}
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
                placeholder="Min. 8 characters"
                value={form.newPassword}
                onChange={(event) => onChange('newPassword', event.target.value)}
                required
                minLength={8}
              />
            </div>
            {error ? (
              <Alert variant="destructive">
                <AlertCircle />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            <Button className="mt-1 w-full" type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Saving…' : 'Save password'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
};

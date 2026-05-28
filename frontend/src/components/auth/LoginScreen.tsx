import { Alert, AlertDescription } from '@/components/ui/alert';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { AlertCircle } from 'lucide-react';
import { WGKeeperLogo } from '@/components/WGKeeperLogo';

type LoginFormState = {
  username: string;
  password: string;
};

type Props = {
  form: LoginFormState;
  error: string;
  isSubmitting: boolean;
  onChange: (field: 'username' | 'password', value: string) => void;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
};

export const LoginScreen = ({ form, error, isSubmitting, onChange, onSubmit }: Props) => {
  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center gap-7 overflow-hidden bg-background px-4 py-12 text-foreground">
      {/* Faint technical dot-grid, fading out toward the edges. Decorative. */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0"
        style={{
          backgroundImage: 'radial-gradient(circle at center, var(--border) 1px, transparent 1px)',
          backgroundSize: '22px 22px',
          maskImage: 'radial-gradient(ellipse 70% 55% at 50% 45%, #000 0%, transparent 100%)',
          WebkitMaskImage: 'radial-gradient(ellipse 70% 55% at 50% 45%, #000 0%, transparent 100%)',
        }}
      />

      <div className="relative flex flex-col items-center gap-2">
        <WGKeeperLogo className="h-7 w-auto" />
        <p className="text-[0.7rem] font-medium tracking-[0.2em] text-muted-foreground uppercase">
          Console
        </p>
      </div>

      <Card className="relative w-full max-w-sm">
        <CardHeader className="gap-1.5">
          <CardTitle>Sign in</CardTitle>
          <CardDescription>Enter your credentials to access the console.</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-4" onSubmit={onSubmit}>
            <div className="flex flex-col gap-2">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                name="username"
                autoComplete="username"
                placeholder="admin"
                value={form.username}
                onChange={(event) => onChange('username', event.target.value)}
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                name="password"
                type="password"
                autoComplete="current-password"
                placeholder="••••••••"
                value={form.password}
                onChange={(event) => onChange('password', event.target.value)}
                required
              />
            </div>
            {error ? (
              <Alert variant="destructive">
                <AlertCircle />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            <Button className="mt-1 w-full" type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>
        </CardContent>
      </Card>

      <p className="relative text-xs text-muted-foreground">
        Self-hosted WireGuard management console
      </p>
    </div>
  );
};

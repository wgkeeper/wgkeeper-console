import { Alert, AlertDescription } from '@/components/ui/alert';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
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
    <div
      className="flex min-h-screen items-center justify-center px-6 py-10 text-foreground"
      style={{
        backgroundColor: 'hsl(var(--muted))',
        backgroundImage: 'radial-gradient(hsl(var(--border)) 1px, transparent 1px)',
        backgroundSize: '20px 20px',
      }}
    >
      <div className="w-full max-w-sm">
        <Card className="border-border/60 shadow-xl">
          <CardHeader className="items-center gap-1 pt-8 pb-6 text-center">
            <WGKeeperLogo className="mb-3 h-10 w-auto" />
            <CardTitle className="text-xs font-medium tracking-[0.2em] text-muted-foreground uppercase">
              Console
            </CardTitle>
          </CardHeader>
          <CardContent className="px-6 pb-8">
            <form className="flex flex-col gap-5" onSubmit={onSubmit}>
              <div className="flex flex-col gap-1.5">
                <Label
                  htmlFor="username"
                  className="text-xs font-medium tracking-wide text-muted-foreground uppercase"
                >
                  Username
                </Label>
                <Input
                  id="username"
                  name="username"
                  autoComplete="username"
                  placeholder="Enter your username"
                  value={form.username}
                  onChange={(event) => onChange('username', event.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label
                  htmlFor="password"
                  className="text-xs font-medium tracking-wide text-muted-foreground uppercase"
                >
                  Password
                </Label>
                <Input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  placeholder="Enter your password"
                  value={form.password}
                  onChange={(event) => onChange('password', event.target.value)}
                  required
                />
              </div>
              {error ? (
                <Alert variant="destructive">
                  <AlertCircle className="size-4" />
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              ) : null}
              <Button className="mt-1 w-full" type="submit" disabled={isSubmitting}>
                {isSubmitting ? 'Signing in…' : 'Sign in'}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

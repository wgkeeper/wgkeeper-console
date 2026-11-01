import { CircleAlertIcon } from 'lucide-react';
import { Alert, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { AuthLayout } from '@/components/auth/AuthLayout';

type LoginFormState = {
  username: string;
  password: string;
};

type Props = {
  form: LoginFormState;
  error: string;
  isSubmitting: boolean;
  onChange: (field: 'username' | 'password', value: string) => void;
  onSubmit: (event: React.SubmitEvent<HTMLFormElement>) => void;
};

export const LoginScreen = ({ form, error, isSubmitting, onChange, onSubmit }: Props) => (
  <AuthLayout title="Sign in">
    <Card>
      <CardHeader className="text-center">
        <CardTitle>
          <h1 className="text-xl">Sign in</h1>
        </CardTitle>
        <CardDescription>Use the console admin account.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit}>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="username">Username</FieldLabel>
              <Input
                id="username"
                name="username"
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                placeholder="admin"
                value={form.username}
                onChange={(event) => onChange('username', event.target.value)}
                required
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="password">Password</FieldLabel>
              <Input
                id="password"
                name="password"
                type="password"
                autoComplete="current-password"
                value={form.password}
                onChange={(event) => onChange('password', event.target.value)}
                required
              />
            </Field>
            {error ? (
              <Alert variant="destructive">
                <CircleAlertIcon />
                <AlertTitle>{error}</AlertTitle>
              </Alert>
            ) : null}
            <Field>
              <Button type="submit" disabled={isSubmitting}>
                {isSubmitting ? <Spinner /> : null}
                Sign in
              </Button>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  </AuthLayout>
);

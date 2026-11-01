import { CircleAlertIcon } from 'lucide-react';
import { Alert, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { AuthLayout } from '@/components/auth/AuthLayout';

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
  onSubmit: (event: React.SubmitEvent<HTMLFormElement>) => void;
};

export const ChangePasswordScreen = ({
  mustChangePassword,
  form,
  error,
  isSubmitting,
  onChange,
  onSubmit,
}: Props) => (
  <AuthLayout title="Set a new password">
    <Card>
      <CardHeader className="text-center">
        <CardTitle>
          <h1 className="text-xl">Set a new password</h1>
        </CardTitle>
        <CardDescription>
          {mustChangePassword
            ? 'Replace the bootstrap password before you continue.'
            : 'Choose a strong password you do not reuse elsewhere.'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={onSubmit}>
          <FieldGroup>
            {mustChangePassword ? null : (
              <Field>
                <FieldLabel htmlFor="currentPassword">Current password</FieldLabel>
                <Input
                  id="currentPassword"
                  name="currentPassword"
                  type="password"
                  autoComplete="current-password"
                  value={form.currentPassword}
                  onChange={(event) => onChange('currentPassword', event.target.value)}
                />
              </Field>
            )}
            <Field>
              <FieldLabel htmlFor="newPassword">New password</FieldLabel>
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
              <FieldDescription>At least 8 characters.</FieldDescription>
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
                Save password
              </Button>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  </AuthLayout>
);

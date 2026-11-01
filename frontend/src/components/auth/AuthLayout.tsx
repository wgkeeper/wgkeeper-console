import { useEffect } from 'react';
import { WGKeeperLogo } from '@/components/WGKeeperLogo';

/** Centered single-card layout shared by the sign-in and first-password screens. */
export const AuthLayout = ({ title, children }: { title: string; children: React.ReactNode }) => {
  useEffect(() => {
    document.title = `${title} · wgkeeper console`;
  }, [title]);

  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-8 bg-sidebar p-6 md:p-10">
      <WGKeeperLogo className="h-7 w-auto" />
      <div className="flex w-full max-w-sm flex-col gap-6">{children}</div>
      <p className="text-center text-xs text-muted-foreground">
        Self-hosted admin for wgkeeper nodes ·{' '}
        <a
          href="https://github.com/wgkeeper/wgkeeper-console"
          target="_blank"
          rel="noreferrer"
          className="underline-offset-4 hover:text-foreground hover:underline"
        >
          GitHub
        </a>
      </p>
    </main>
  );
};

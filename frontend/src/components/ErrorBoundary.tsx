import { Component, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { WGKeeperLogo } from '@/components/WGKeeperLogo';

type Props = { children: ReactNode };
type State = { error: Error | null };

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <main className="flex min-h-svh flex-col items-center justify-center gap-4 bg-sidebar px-6 text-foreground">
          <WGKeeperLogo className="mb-4 h-6 w-auto" />
          <h1 className="text-2xl font-semibold">Something went wrong</h1>
          <p className="max-w-sm text-center text-sm text-muted-foreground">
            {this.state.error.message}
          </p>
          <Button variant="outline" onClick={() => this.setState({ error: null })}>
            Try again
          </Button>
        </main>
      );
    }
    return this.props.children;
  }
}

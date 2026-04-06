import { Component, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';

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
        <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-background px-6 text-foreground">
          <p className="text-xs tracking-[0.18em] text-muted-foreground uppercase">
            WGKeeper Console
          </p>
          <h1 className="text-2xl font-semibold">Something went wrong</h1>
          <p className="max-w-sm text-center text-sm text-muted-foreground">
            {this.state.error.message}
          </p>
          <Button variant="outline" onClick={() => this.setState({ error: null })}>
            Try again
          </Button>
        </div>
      );
    }
    return this.props.children;
  }
}
